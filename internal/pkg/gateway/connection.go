package gateway

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Connection represents a WebSocket connection with heartbeat management.
// It wraps the underlying WebSocket connection and provides methods for
// sending/receiving messages and managing connection lifecycle.
type Connection struct {
	// UserID is the unique identifier of the connected user
	UserID string

	// GuildID is the guild this connection is associated with
	GuildID string

	// Conn is the underlying WebSocket connection
	Conn *websocket.Conn

	// Send is a buffered channel for outbound messages
	Send chan []byte

	// mu protects concurrent writes to the WebSocket connection
	mu sync.Mutex

	// lastHeartbeat tracks the last time a heartbeat was received
	lastHeartbeat time.Time

	// heartbeatMu protects lastHeartbeat
	heartbeatMu sync.RWMutex

	// ctx is the connection context
	ctx context.Context

	// cancel cancels the connection context
	cancel context.CancelFunc

	// closed indicates if the connection has been closed
	closed bool

	// closedMu protects the closed flag
	closedMu sync.RWMutex
}

// NewConnection creates a new Connection instance.
//
// Parameters:
//   - ctx: Parent context for the connection
//   - userID: The user identifier
//   - guildID: The guild identifier
//   - conn: The WebSocket connection
//
// Returns:
//   - *Connection: The initialized connection
func NewConnection(ctx context.Context, userID, guildID string, conn *websocket.Conn) *Connection {
	connCtx, cancel := context.WithCancel(ctx)

	return &Connection{
		UserID:        userID,
		GuildID:       guildID,
		Conn:          conn,
		Send:          make(chan []byte, 256),
		lastHeartbeat: time.Now(),
		ctx:           connCtx,
		cancel:        cancel,
		closed:        false,
	}
}

// WriteMessage writes a message to the WebSocket connection.
// It is thread-safe and handles connection closure gracefully.
//
// Parameters:
//   - messageType: The WebSocket message type (e.g., websocket.TextMessage)
//   - data: The message data to send
//
// Returns:
//   - error: Any error encountered during writing
func (c *Connection) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.IsClosed() {
		return websocket.ErrCloseSent
	}

	return c.Conn.WriteMessage(messageType, data)
}

// ReadMessage reads a message from the WebSocket connection.
//
// Returns:
//   - messageType: The WebSocket message type
//   - data: The message data
//   - error: Any error encountered during reading
func (c *Connection) ReadMessage() (int, []byte, error) {
	return c.Conn.ReadMessage()
}

// Close closes the WebSocket connection and cleans up resources.
// It performs the following cleanup steps:
// 1. Marks the connection as closed (idempotent)
// 2. Cancels the connection context to stop goroutines
// 3. Closes the send channel to prevent further writes
// 4. Closes the underlying WebSocket connection
//
// It is safe to call multiple times.
//
// Returns:
//   - error: Any error from closing the WebSocket connection
func (c *Connection) Close() error {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()

	// Idempotent - safe to call multiple times
	if c.closed {
		return nil
	}

	// Mark as closed first to prevent new operations
	c.closed = true

	// Cancel context to signal goroutines to stop
	c.cancel()

	// Close send channel to unblock any writers
	// This must be done after marking closed to prevent panic
	close(c.Send)

	// Close the underlying WebSocket connection
	// This releases network resources
	return c.Conn.Close()
}

// IsClosed returns whether the connection has been closed.
func (c *Connection) IsClosed() bool {
	c.closedMu.RLock()
	defer c.closedMu.RUnlock()

	return c.closed
}

// UpdateHeartbeat updates the last heartbeat timestamp.
// This should be called when a heartbeat message is received.
func (c *Connection) UpdateHeartbeat() {
	c.heartbeatMu.Lock()
	defer c.heartbeatMu.Unlock()

	c.lastHeartbeat = time.Now()
}

// GetLastHeartbeat returns the last heartbeat timestamp.
func (c *Connection) GetLastHeartbeat() time.Time {
	c.heartbeatMu.RLock()
	defer c.heartbeatMu.RUnlock()

	return c.lastHeartbeat
}

// IsAlive checks if the connection is still alive based on heartbeat timeout.
//
// Parameters:
//   - timeout: The maximum duration since last heartbeat
//
// Returns:
//   - bool: true if the connection is alive, false otherwise
func (c *Connection) IsAlive(timeout time.Duration) bool {
	lastHB := c.GetLastHeartbeat()
	return time.Since(lastHB) < timeout
}

// Context returns the connection context.
func (c *Connection) Context() context.Context {
	return c.ctx
}
