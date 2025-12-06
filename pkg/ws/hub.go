package ws

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"

	redis "github.com/redis/go-redis/v9"

	"github.com/Gopher0727/ChatRoom/internal/repositories"
	"github.com/Gopher0727/ChatRoom/pkg/utils"
)

const (
	redisChannelName = "chat:broadcast"
)

// Hub 维护活跃的客户端连接并广播消息
type Hub struct {
	// 注册的客户端
	clients map[*Client]bool

	// 房间（Guild）对应的客户端集合 GuildID -> Client -> bool
	rooms map[uint]map[*Client]bool

	// 互斥锁，保护 map 的并发读写
	mu sync.RWMutex

	// 注册请求通道
	register chan *Client

	// 注销请求通道
	unregister chan *Client

	// 广播消息通道 (内部使用)
	broadcast chan *BroadcastMessage

	// 注入 GuildRepository 以访问在线状态
	guildRepo *repositories.GuildRepository

	// Redis 客户端，用于分布式广播
	redis *redis.Client

	// 用户 ID 到客户端的映射，方便查找
	userClients map[uint]*Client

	// 一致性哈希环与当前节点
	hashRing *utils.HashRing
	nodeID   string
}

// BroadcastMessage 广播消息结构
type BroadcastMessage struct {
	GuildID uint `json:"guild_id"`
	Message any  `json:"message"`
}

func NewHub(guildRepo *repositories.GuildRepository, redisClient *redis.Client, ring *utils.HashRing, nodeID string) *Hub {
	return &Hub{
		broadcast:   make(chan *BroadcastMessage),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
		rooms:       make(map[uint]map[*Client]bool),
		userClients: make(map[uint]*Client),
		guildRepo:   guildRepo,
		redis:       redisClient,
		hashRing:    ring,
		nodeID:      nodeID,
	}
}

// 可选：后续外部更新哈希环时使用
func (h *Hub) SetHashRing(ring *utils.HashRing) {
	hash := ring
	h.mu.Lock()
	h.hashRing = hash
	h.mu.Unlock()
}

func (h *Hub) Run() {
	// 启动 Redis 订阅协程
	if h.redis != nil {
		go h.subscribeToRedis()
	}

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.userClients[client.userID] = client
			// 将客户端加入其所属的 Guild 房间
			for _, guildID := range client.guildIDs {
				if _, ok := h.rooms[guildID]; !ok {
					h.rooms[guildID] = make(map[*Client]bool)
				}
				h.rooms[guildID][client] = true
				// 标记用户在此 Guild 在线
				h.guildRepo.SetUserOnline(guildID, client.userID)
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.userClients, client.userID)
				close(client.send)
				// 从所有房间移除
				for _, guildID := range client.guildIDs {
					if room, ok := h.rooms[guildID]; ok {
						delete(room, client)
						if len(room) == 0 {
							delete(h.rooms, guildID)
						}
					}
					// 标记用户在此 Guild 离线
					h.guildRepo.SetUserOffline(guildID, client.userID)
				}
			}
			h.mu.Unlock()
			// 删除 Redis 路由键，避免脏路由
			if h.redis != nil {
				key := "User:Connect:" + strconv.Itoa(int(client.userID))
				_ = h.redis.Del(context.Background(), key).Err()
			}

		case msg := <-h.broadcast:
			h.mu.RLock()
			// 收集需要关闭的客户端，避免在 RLock 中修改 map
			var closedClients []*Client

			// 找到目标 Guild 的所有订阅者
			if clients, ok := h.rooms[msg.GuildID]; ok {
				for client := range clients {
					select {
					case client.send <- msg:
					default:
						// 发送缓冲区满，标记为需要关闭
						closedClients = append(closedClients, client)
					}
				}
			}
			h.mu.RUnlock()

			// 处理需要关闭的客户端
			if len(closedClients) > 0 {
				h.mu.Lock()
				for _, client := range closedClients {
					// Double check，防止已经处理过
					if _, ok := h.clients[client]; ok {
						close(client.send)
						delete(h.clients, client)
						delete(h.userClients, client.userID)
						// 从所有房间移除
						for _, guildID := range client.guildIDs {
							if room, ok := h.rooms[guildID]; ok {
								delete(room, client)
								if len(room) == 0 {
									delete(h.rooms, guildID)
								}
							}
							// 标记用户在此 Guild 离线
							h.guildRepo.SetUserOffline(guildID, client.userID)
						}
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

func (h *Hub) subscribeToRedis() {
	ctx := context.Background()
	pubsub := h.redis.Subscribe(ctx, redisChannelName)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		var broadcastMsg BroadcastMessage
		if err := json.Unmarshal([]byte(msg.Payload), &broadcastMsg); err == nil {
			// 将从 Redis 收到的消息发送到本地广播通道
			// 注意：这里不需要再 Publish 到 Redis，否则会死循环
			// 直接送入 h.broadcast，由 Run() 中的循环分发给本地 WebSocket 连接
			h.broadcast <- &broadcastMsg
		}
	}
}

// BroadcastToGuild 发送消息到指定 Guild
func (h *Hub) BroadcastToGuild(guildID uint, message any) {
	msg := &BroadcastMessage{
		GuildID: guildID,
		Message: message,
	}

	if h.redis != nil {
		// 发布到 Redis，让所有实例（包括自己）通过订阅收到消息
		// 这样可以确保分布式环境下的消息同步
		payload, err := json.Marshal(msg)
		if err == nil {
			h.redis.Publish(context.Background(), redisChannelName, payload)
		}
	} else {
		// 如果没有 Redis，回退到仅本地广播
		h.broadcast <- msg
	}
}
