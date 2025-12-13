package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
	redislib "github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	"github.com/Gopher0727/ChatRoom/config"
	"github.com/Gopher0727/ChatRoom/internal/pkg/kafka"
	chat "github.com/Gopher0727/ChatRoom/internal/pkg/proto"
	redis "github.com/Gopher0727/ChatRoom/internal/pkg/redis"
)

// MessageHandler handles Websocket message processing for the gateway.
// It manages both upstream (client -> server) and downstream (server -> client) message flows.
type MessageHandler struct {
	connManager   *ConnectionManager
	kafkaProducer *kafka.Producer
	redisClient   redis.RedisClient
	config        *config.Config
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewMessageHandler creates a new MessageHandler instance.
//
// Parameters:
//   - ctx: Parent context for the handler
//   - connManager: Connection manager for Websocket connections
//   - kafkaProducer: Kafka producer for upstream messages
//   - redisClient: Redis client for Pub/Sub
//   - cfg: Application configuration
//
// Returns:
//   - *MessageHandler: The initialized message handler
func NewMessageHandler(
	ctx context.Context,
	connManager *ConnectionManager,
	kafkaProducer *kafka.Producer,
	redisClient redis.RedisClient,
	cfg *config.Config,
) *MessageHandler {
	handlerCtx, cancel := context.WithCancel(ctx)

	return &MessageHandler{
		connManager:   connManager,
		kafkaProducer: kafkaProducer,
		redisClient:   redisClient,
		config:        cfg,
		ctx:           handlerCtx,
		cancel:        cancel,
	}
}

// HandleConnection manages a Websocket connection lifecycle.
// It starts goroutines for reading upstream messages and writing downstream messages.
//
// Parameters:
//   - conn: The Websocket connection to handle
func (h *MessageHandler) HandleConnection(conn *Connection) {
	// Start reader goroutine for upstream messages
	go h.readPump(conn)

	// Start writer goroutine for downstream messages
	go h.writePump(conn)
}

// readPump reads messages from the Websocket connection and processes them.
// It handles upstream message flow: Client -> Gateway -> Kafka
func (h *MessageHandler) readPump(conn *Connection) {
	defer func() {
		h.handleDisconnect(conn)
	}()

	// Set read deadline
	conn.Conn.SetReadDeadline(time.Now().Add(time.Duration(h.config.Websocket.ConnectionTimeout) * time.Second))

	// Set pong handler to update heartbeat
	conn.Conn.SetPongHandler(func(string) error {
		conn.UpdateHeartbeat()
		conn.Conn.SetReadDeadline(time.Now().Add(time.Duration(h.config.Websocket.ConnectionTimeout) * time.Second))
		return nil
	})

	for {
		select {
		case <-conn.Context().Done():
			return
		default:
		}

		messageType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Websocket read error for user %s: %v", conn.UserID, err)
			}
			return
		}

		// Handle different message types
		switch messageType {
		case websocket.TextMessage, websocket.BinaryMessage:
			if err := h.handleUpstreamMessage(conn, data); err != nil {
				log.Printf("Error handling upstream message from user %s: %v", conn.UserID, err)
				h.sendError(conn, fmt.Sprintf("Failed to process message: %v", err))
			}
		case websocket.PingMessage:
			// Ping is handled automatically by the library
			conn.UpdateHeartbeat()
		}
	}
}

// writePump writes messages from the send channel to the Websocket connection.
// It handles downstream message flow: Gateway -> Client
func (h *MessageHandler) writePump(conn *Connection) {
	ticker := time.NewTicker(time.Duration(h.config.Websocket.HeartbeatInterval) * time.Second)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-conn.Context().Done():
			return
		case message, ok := <-conn.Send:
			if !ok {
				// Channel closed
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Set write deadline
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			if err := conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
				log.Printf("Error writing message to user %s: %v", conn.UserID, err)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Error sending ping to user %s: %v", conn.UserID, err)
				return
			}
		}
	}
}

// handleUpstreamMessage processes an upstream message from the client.
// It validates the message and sends it to Kafka for processing.
//
// Parameters:
//   - conn: The connection that sent the message
//   - data: The raw message data
//
// Returns:
//   - error: Any error encountered during processing
func (h *MessageHandler) handleUpstreamMessage(conn *Connection, data []byte) error {
	// Parse the message
	var wsMsg chat.WSMessage
	if err := proto.Unmarshal(data, &wsMsg); err != nil {
		// Try JSON as fallback
		if jsonErr := json.Unmarshal(data, &wsMsg); jsonErr != nil {
			return fmt.Errorf("failed to parse message: protobuf error: %w, json error: %v", err, jsonErr)
		}
	}

	// Validate message
	if err := h.validateMessage(&wsMsg, conn); err != nil {
		return fmt.Errorf("message validation failed: %w", err)
	}

	// Set user ID from connection (prevent spoofing)
	wsMsg.UserId = conn.UserID
	if conn.GuildID != "" {
		wsMsg.GuildId = conn.GuildID
	}
	wsMsg.Timestamp = time.Now().UnixMilli()

	// Serialize message
	msgData, err := proto.Marshal(&wsMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Send to Kafka
	topic := h.config.Kafka.Topics.Message
	key := []byte(wsMsg.GuildId) // Use guild ID as key for partitioning

	_, _, err = h.kafkaProducer.Produce(h.ctx, topic, key, msgData)
	if err != nil {
		return fmt.Errorf("failed to send message to kafka: %w", err)
	}

	log.Printf("Message from user %s sent to Kafka topic %s", conn.UserID, topic)
	return nil
}

// validateMessage validates an upstream message.
//
// Parameters:
//   - msg: The message to validate
//   - conn: The connection that sent the message
//
// Returns:
//   - error: Validation error if the message is invalid
func (h *MessageHandler) validateMessage(msg *chat.WSMessage, conn *Connection) error {
	if msg.Content == "" {
		return fmt.Errorf("message content cannot be empty")
	}

	if len(msg.Content) > 2000 {
		return fmt.Errorf("message content exceeds maximum length of 2000 characters")
	}

	// Ensure user is sending to their connected guild
	// If conn.GuildID is empty, we assume it's a global connection and allow sending to any guild
	if conn.GuildID != "" && msg.GuildId != "" && msg.GuildId != conn.GuildID {
		return fmt.Errorf("cannot send message to guild %s, connected to guild %s", msg.GuildId, conn.GuildID)
	}

	return nil
}

// sendError sends an error message to the client.
//
// Parameters:
//   - conn: The connection to send the error to
//   - errorMsg: The error message
func (h *MessageHandler) sendError(conn *Connection, errorMsg string) {
	errMsg := &chat.WSMessage{
		Type:      chat.MessageType_SYSTEM,
		Content:   errorMsg,
		Timestamp: time.Now().Unix(),
	}

	data, err := proto.Marshal(errMsg)
	if err != nil {
		log.Printf("Failed to marshal error message: %v", err)
		return
	}

	select {
	case conn.Send <- data:
	case <-time.After(5 * time.Second):
		log.Printf("Timeout sending error message to user %s", conn.UserID)
	}
}

// StartSubscriber starts the Redis Pub/Sub subscriber for downstream messages.
// It subscribes to guild channels and forwards messages to connected clients.
func (h *MessageHandler) StartSubscriber(pattern string) error {
	// Subscribe to pattern
	pubsub, err := h.redisClient.PSubscribe(h.ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to subscribe to pattern %s: %w", pattern, err)
	}

	// Start message receiver
	go h.receiveMessages(pubsub)

	log.Printf("Subscribed to redis pattern: %s", pattern)
	return nil
}

// receiveMessages receives messages from Redis Pub/Sub and forwards them to clients.
//
// Parameters:
//   - pubsub: The Redis Pub/Sub subscription
func (h *MessageHandler) receiveMessages(pubsub *redislib.PubSub) {
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-h.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				log.Println("Pub/Sub channel closed")
				return
			}

			if err := h.handleDownstreamMessage(msg); err != nil {
				log.Printf("Error handling downstream message: %v", err)
			}
		}
	}
}

// handleDownstreamMessage processes a downstream message from Redis Pub/Sub.
// It finds all connections for the target guild and pushes the message to them.
//
// Parameters:
//   - msg: The Redis Pub/Sub message
//
// Returns:
//   - error: Any error encountered during processing
func (h *MessageHandler) handleDownstreamMessage(msg *redislib.Message) error {
	// Parse the message
	var wsMsg chat.WSMessage
	if err := proto.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Extract guild ID from channel name (format: "guild:{guildID}")
	guildID := wsMsg.GuildId

	// Get all connections for this guild
	connections := h.connManager.GetConnectionsByGuild(guildID)

	if len(connections) == 0 {
		log.Printf("No connections found for guild %s", guildID)
		return nil
	}

	// Serialize message once
	msgData, err := proto.Marshal(&wsMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Push message to all connections
	successCount := 0
	for _, conn := range connections {
		// Don't send message back to the sender
		// if conn.UserID == wsMsg.UserId {
		// 	continue
		// }

		select {
		case conn.Send <- msgData:
			successCount++
		case <-time.After(1 * time.Second):
			log.Printf("Timeout sending message to user %s", conn.UserID)
		}
	}

	log.Printf("Pushed message to %d/%d connections in guild %s", successCount, len(connections), guildID)
	return nil
}

// handleDisconnect handles connection disconnection and cleanup.
// It performs comprehensive cleanup including:
// 1. Resource cleanup - closes connection and removes from manager
// 2. Online status update - removes user from Redis online status
// 3. Detailed logging - logs all disconnect events and errors
//
// Parameters:
//   - conn: The connection that disconnected
func (h *MessageHandler) handleDisconnect(conn *Connection) {
	startTime := time.Now()

	// Log initial disconnect event with context
	log.Printf("[DISCONNECT] User %s disconnecting from guild %s (connection established: %v ago)",
		conn.UserID, conn.GuildID, time.Since(conn.GetLastHeartbeat()))

	// Step 1: Remove connection from manager
	// This will also handle Redis online status cleanup
	if err := h.connManager.RemoveConnection(conn.UserID); err != nil {
		log.Printf("[DISCONNECT ERROR] Failed to remove connection for user %s: %v", conn.UserID, err)
	} else {
		log.Printf("[DISCONNECT] Successfully removed user %s from connection manager", conn.UserID)
	}

	// Step 2: Close the Websocket connection
	// This releases network resources and closes the send channel
	if err := conn.Close(); err != nil {
		log.Printf("[DISCONNECT ERROR] Failed to close Websocket for user %s: %v", conn.UserID, err)
	} else {
		log.Printf("[DISCONNECT] Successfully closed Websocket for user %s", conn.UserID)
	}

	// Step 3: Verify online status was removed from Redis
	// This is a verification step to ensure cleanup was successful
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if isOnline, err := h.redisClient.IsUserOnline(ctx, conn.UserID); err != nil {
		log.Printf("[DISCONNECT WARNING] Could not verify online status removal for user %s: %v", conn.UserID, err)
	} else if isOnline {
		log.Printf("[DISCONNECT WARNING] User %s still appears online in Redis after disconnect", conn.UserID)
		// Attempt to force remove
		if err := h.redisClient.RemoveUserOnline(ctx, conn.UserID); err != nil {
			log.Printf("[DISCONNECT ERROR] Failed to force remove online status for user %s: %v", conn.UserID, err)
		}
	} else {
		log.Printf("[DISCONNECT] Verified user %s online status removed from Redis", conn.UserID)
	}

	// Log completion with timing
	duration := time.Since(startTime)
	log.Printf("[DISCONNECT COMPLETE] Cleanup completed for user %s in guild %s (took %v)",
		conn.UserID, conn.GuildID, duration)
}

// Shutdown gracefully shuts down the message handler.
func (h *MessageHandler) Shutdown() error {
	h.cancel()
	return nil
}
