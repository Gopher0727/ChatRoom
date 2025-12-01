package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/Gopher0727/ChatRoom/internal/services"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
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
	hub *Hub

	// WebSocket 连接
	conn *websocket.Conn

	// 缓冲通道，用于发送消息
	send chan *BroadcastMessage

	// 用户 ID
	userID uint

	// 用户所属的 Guild ID 列表 (用于订阅)
	guildIDs []uint

	// 服务引用，用于处理接收到的消息
	guildService *services.GuildService
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
		return nil
	})

	// 拉取离线消息
	// 异步执行，防止阻塞 readPump 导致心跳超时
	go c.syncOfflineMessages()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
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
			log.Printf("json unmarshal error: %v", err)
			continue
		}

		// 调用 Service 保存消息
		sendReq := &services.SendMessageRequest{
			Content: req.Content,
			MsgType: req.MsgType,
		}
		resp, err := c.guildService.SendMessage(c.userID, req.GuildID, sendReq)
		if err != nil {
			log.Printf("send message error: %v", err)
			// 可以选择发送错误消息回客户端
			continue
		}

		// 构造完整的消息模型用于广播
		c.hub.BroadcastToGuild(req.GuildID, resp)

		log.Printf("User %d sent message to guild %d: %s", c.userID, req.GuildID, resp.Content)
	}
}

// syncOfflineMessages 拉取并发送离线消息
func (c *Client) syncOfflineMessages() {
	// 防止向已关闭的 channel 发送导致 panic
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in syncOfflineMessages: %v", r)
		}
	}()

	messages, err := c.guildService.GetOfflineMessages(c.userID)
	if err != nil {
		log.Printf("Error getting offline messages for user %d: %v", c.userID, err)
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
// TODO
func ServeWs(hub *Hub, guildService *services.GuildService, c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		// 如果是 WebSocket 连接请求，可能无法直接通过 Header 传递 Token
		// 通常通过 Query 参数传递 ?token=...
		// 这里假设 AuthMiddleware 已经处理了 Query Token 的情况
		// 如果没有，我们需要在这里手动处理
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 升级连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket: %v", err)
		return
	}

	// 获取用户加入的 Guild 列表
	uID := userID.(uint)
	guildIDs, err := guildService.GetUserGuildIDs(uID)
	if err != nil {
		log.Printf("Failed to get user guilds: %v", err)
		conn.Close()
		return
	}

	// 创建 Client 实例
	client := &Client{
		hub:          hub,
		conn:         conn,
		send:         make(chan *BroadcastMessage, 256),
		userID:       uID,
		guildIDs:     guildIDs,
		guildService: guildService,
	}

	// 注册到 Hub
	client.hub.register <- client

	// 启动读写协程
	go client.writePump()
	go client.readPump()
}
