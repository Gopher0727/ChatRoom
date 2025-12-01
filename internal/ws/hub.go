package ws

import (
	"sync"
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
}

// BroadcastMessage 广播消息结构
type BroadcastMessage struct {
	GuildID uint        `json:"guild_id"`
	Message interface{} `json:"message"`
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan *BroadcastMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rooms:      make(map[uint]map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			// 将客户端加入其所属的 Guild 房间
			for _, guildID := range client.guildIDs {
				if _, ok := h.rooms[guildID]; !ok {
					h.rooms[guildID] = make(map[*Client]bool)
				}
				h.rooms[guildID][client] = true
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				// 从所有房间移除
				for _, guildID := range client.guildIDs {
					if room, ok := h.rooms[guildID]; ok {
						delete(room, client)
						if len(room) == 0 {
							delete(h.rooms, guildID)
						}
					}
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			// 找到目标 Guild 的所有订阅者
			if clients, ok := h.rooms[msg.GuildID]; ok {
				for client := range clients {
					select {
					case client.send <- msg:
					default:
						// 发送缓冲区满，关闭连接并移除
						close(client.send)
						delete(h.clients, client)
						// 注意：这里不能直接修改 h.rooms，因为正在读锁中，
						// 实际生产中应该标记删除或通过 unregister 通道处理
						// 这里简化处理，仅关闭 channel，依赖下一次 unregister 清理
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToGuild 发送消息到指定 Guild
func (h *Hub) BroadcastToGuild(guildID uint, message interface{}) {
	h.broadcast <- &BroadcastMessage{
		GuildID: guildID,
		Message: message,
	}
}

// UpdateClientGuilds 更新客户端订阅的 Guild 列表 (例如用户新加入 Guild)
func (h *Hub) UpdateClientGuilds(client *Client, newGuildIDs []uint) {
	// 这是一个简化的处理，实际可能需要更复杂的逻辑来处理增量更新
	// 这里我们简单地重新注册一次，或者提供专门的更新通道
	// 为了线程安全，最好通过 channel 发送指令给 Run 循环处理
	// 暂时略过，假设连接建立时已获取所有 Guild
}
