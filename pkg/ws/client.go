package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/Gopher0727/ChatRoom/internal/services"
	"github.com/Gopher0727/ChatRoom/pkg/mq"
)

const (
	writeWait      = 10 * time.Second    // 允许写入消息到对端的最大时间
	pongWait       = 60 * time.Second    // 允许读取下一个 pong 消息的最大时间
	pingPeriod     = (pongWait * 9) / 10 // 发送 ping 到对端的周期。必须小于 pongWait
	maxMessageSize = 512                 // 允许来自对端的最大消息大小
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client 代表一个 WebSocket 连接客户端
type Client struct {
	hub           *Hub
	conn          *websocket.Conn        // WebSocket 连接
	send          chan *BroadcastMessage // 缓冲通道，用于发送消息
	userID        uint                   // 用户 ID
	guildIDs      []uint                 // 用户所属的 Guild ID 列表 (用于订阅)
	guildService  *services.GuildService // 服务引用，用于处理接收到的消息
	kafkaProducer *mq.KafkaProducer      // Kafka Producer
}

// readPump 泵送来自 WebSocket 连接的消息到 Hub
// 这里处理客户端发送过来的消息 (如果是纯 WS 聊天)
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		// 收到 Pong，说明客户端还活着，刷新在线状态
		// 异步执行，避免阻塞
		go c.guildService.RefreshUserOnlineStatus(c.userID, c.guildIDs)
		// 续期 Redis 路由键 TTL
		if c.hub != nil && c.hub.redis != nil {
			key := "User:Connect:" + strconv.Itoa(int(c.userID))
			// 重设过期时间
			_ = c.hub.redis.Expire(context.Background(), key, 5*time.Minute).Err()
		}
		return nil
	})

	// 拉取最近的历史消息，确保用户登录后能看到上下文
	go c.pushRecentMessages()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("错误: %v", err)
			}
			break
		}

		// 处理客户端发送的消息
		// 假设客户端发送的是 JSON 格式: {"guild_id": 1, "content": "hello", "msg_type": "text"}
		var req struct {
			GuildID uint   `json:"guild_id"`
			Content string `json:"content"`
			MsgType string `json:"msg_type"`
		}
		if err := json.Unmarshal(message, &req); err != nil {
			log.Printf("json 反序列化错误: %v", err)
			continue
		}

		// 调用 Service 保存消息（附带 nodeID，便于下游分桶）
		// 通过一致性哈希获取节点ID
		var nodeID string
		if c.hub != nil && c.hub.hashRing != nil {
			nodeID = c.hub.hashRing.Get(strconv.Itoa(int(c.userID)))
		}
		sendReq := &services.SendMessageRequest{
			Content: req.Content,
			MsgType: req.MsgType,
			NodeID:  nodeID,
		}

		// 修改：不再直接调用 Service，而是发送到 Kafka
		if c.kafkaProducer != nil {
			kafkaMsg := struct {
				UserID  uint                         `json:"user_id"`
				GuildID uint                         `json:"guild_id"`
				Content *services.SendMessageRequest `json:"content"`
			}{
				UserID:  c.userID,
				GuildID: req.GuildID,
				Content: sendReq,
			}
			// 使用 GuildID 作为 Key，保证同一个群的消息在同一个 Partition，从而有序
			if err := c.kafkaProducer.SendMessage(strconv.Itoa(int(req.GuildID)), kafkaMsg); err != nil {
				log.Printf("发送消息到 kafka 失败: %v", err)
				continue
			}
		} else {
			// 降级处理：如果没有 Kafka，直接调用 Service
			resp, err := c.guildService.SendMessage(c.userID, req.GuildID, sendReq)
			if err != nil {
				log.Printf("发送消息错误: %v", err)
				// 可以选择发送错误消息回客户端
				continue
			}

			// 构造完整的消息模型用于广播
			c.hub.BroadcastToGuild(req.GuildID, resp)

			log.Printf("用户 %d 发送消息到服务器 %d: %s", c.userID, req.GuildID, resp.Content)
		}
	}
}

// syncOfflineMessages 拉取并发送离线消息
func (c *Client) syncOfflineMessages() {
	// 防止向已关闭的 channel 发送导致 panic
	defer func() {
		if r := recover(); r != nil {
			log.Printf("syncOfflineMessages 发生 panic 并恢复: %v", r)
		}
	}()

	messages, err := c.guildService.GetOfflineMessages(c.userID)
	if err != nil {
		log.Printf("获取用户 %d 的离线消息失败: %v", c.userID, err)
		return
	}

	for _, msg := range messages {
		// 包装成 BroadcastMessage 发送到客户端
		broadcastMsg := &BroadcastMessage{
			GuildID: msg.GuildID,
			Message: msg, // Service 返回的是 MessageResponse
		}

		// 阻塞发送，确保消息不丢失（除非连接断开）
		// 因为是在独立的 goroutine 中，不会阻塞心跳检测
		c.send <- broadcastMsg
	}
}

// pushRecentMessages 拉取并发送最近的历史消息
func (c *Client) pushRecentMessages() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("pushRecentMessages 发生 panic 并恢复: %v", r)
		}
	}()

	// 限制每个 Guild 推送的消息数量
	const recentCount = 20

	for _, guildID := range c.guildIDs {
		msgs, err := c.guildService.GetMessages(c.userID, guildID, recentCount, 0)
		if err != nil {
			log.Printf("获取服务器 %d 的最近消息失败: %v", guildID, err)
			continue
		}

		// GetMessages 返回的是按时间倒序 (Newest -> Oldest)
		// 我们需要按时间正序发送 (Oldest -> Newest)
		for i := len(msgs) - 1; i >= 0; i-- {
			broadcastMsg := &BroadcastMessage{
				GuildID: msgs[i].GuildID,
				Message: msgs[i],
			}
			c.send <- broadcastMsg
		}
	}
}

// writePump 泵送来自 Hub 的消息到 WebSocket 连接
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub 关闭了通道
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			// 序列化消息
			// 这里我们将 BroadcastMessage 序列化发送给客户端
			// 客户端收到后根据 guild_id 判断是哪个群组的消息
			json.NewEncoder(w).Encode(msg)

			// 添加队列中的其他消息（如果有）
			n := len(c.send)
			for range n {
				json.NewEncoder(w).Encode(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWs 处理 WebSocket 请求
func ServeWs(hub *Hub, guildService *services.GuildService, kafkaProducer *mq.KafkaProducer, c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	// 升级连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("升级 websocket 失败: %v", err)
		return
	}

	// 获取用户加入的 Guild 列表
	uID := userID.(uint)
	guildIDs, err := guildService.GetUserGuildIDs(uID)
	if err != nil {
		log.Printf("获取用户服务器列表失败: %v", err)
		conn.Close()
		return
	}

	// 一致性哈希选择目标节点
	targetNode := ""
	if hub.hashRing != nil {
		targetNode = hub.hashRing.Get(strconv.Itoa(int(uID)))
	}

	// 命中当前节点：写入 Redis 路由并注册到本地 Hub
	if targetNode == hub.nodeID || targetNode == "" {
		if hub.redis != nil {
			key := "User:Connect:" + strconv.Itoa(int(uID))
			// TTL 选择心跳周期的2-3倍，这里暂定 5 分钟，心跳续期在 Pong 处刷新
			if err := hub.redis.Set(c, key, hub.nodeID, 5*time.Minute).Err(); err != nil {
				log.Printf("设置用户路由失败: %v", err)
			}
		}
		// 创建 Client 并注册
		client := &Client{
			hub:           hub,
			conn:          conn,
			send:          make(chan *BroadcastMessage, 256),
			userID:        uID,
			guildIDs:      guildIDs,
			guildService:  guildService,
			kafkaProducer: kafkaProducer,
		}
		client.hub.register <- client
		go client.writePump()
		go client.readPump()
		return
	}

	// 未命中当前节点：策略1 仍接入本节点（简单版本）
	// 可选策略2：返回目标节点信息，指导客户端重连
	log.Printf("用户 %d 映射到节点 %s, 当前节点 %s", uID, targetNode, hub.nodeID)
	client := &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan *BroadcastMessage, 256),
		userID:        uID,
		guildIDs:      guildIDs,
		guildService:  guildService,
		kafkaProducer: kafkaProducer,
	}
	client.hub.register <- client
	go client.writePump()
	go client.readPump()
}
