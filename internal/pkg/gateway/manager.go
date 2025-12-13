package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Gopher0727/ChatRoom/config"
	redis "github.com/Gopher0727/ChatRoom/internal/pkg/redis"
)

// ConnectionManager manages all WebSocket connections for the gateway.
// It provides thread-safe operations for adding, removing, and retrieving connections.
// It also handles heartbeat monitoring to detect and clean up dead connections.
type ConnectionManager struct {
	// connections maps userID to their Connection
	connections map[string]*Connection

	// mu protects the connections map
	mu sync.RWMutex

	// config holds the WebSocket configuration
	config *config.WebsocketConfig

	// redisClient is used to update user online status
	redisClient redis.RedisClient

	// nodeID is the unique identifier for this gateway node
	nodeID string

	// ctx is the manager context
	ctx context.Context

	// cancel cancels the manager context
	cancel context.CancelFunc

	// wg tracks active goroutines
	wg sync.WaitGroup
}

// NewConnectionManager creates a new ConnectionManager instance.
//
// Parameters:
//   - ctx: Parent context for the manager
//   - cfg: WebSocket configuration
//   - redisClient: Redis client for online status management
//   - nodeID: Unique identifier for this gateway node
//
// Returns:
//   - *ConnectionManager: The initialized connection manager
func NewConnectionManager(ctx context.Context, cfg *config.WebsocketConfig, redisClient redis.RedisClient, nodeID string) *ConnectionManager {
	managerCtx, cancel := context.WithCancel(ctx)

	cm := &ConnectionManager{
		connections: make(map[string]*Connection),
		config:      cfg,
		redisClient: redisClient,
		nodeID:      nodeID,
		ctx:         managerCtx,
		cancel:      cancel,
	}

	// Start heartbeat monitor
	cm.wg.Add(1)
	go cm.monitorHeartbeats()

	return cm
}

// AddConnection adds a new WebSocket connection to the manager.
// If a connection already exists for the user, it will be closed and replaced.
// This supports multiple browser windows (Requirement 6.4).
//
// Parameters:
//   - userID: The user identifier
//   - guildID: The guild identifier
//   - conn: The WebSocket connection
//
// Returns:
//   - *Connection: The created connection object
//   - error: Any error encountered during addition
func (cm *ConnectionManager) AddConnection(userID, guildID string, conn *websocket.Conn) (*Connection, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Close existing connection if present
	if existingConn, exists := cm.connections[userID]; exists {
		existingConn.Close()
	}

	// Create new connection
	connection := NewConnection(cm.ctx, userID, guildID, conn)
	cm.connections[userID] = connection

	// Update Redis online status
	// TTL is set to 2x heartbeat interval to allow for network delays
	ttl := time.Duration(cm.config.HeartbeatInterval*2) * time.Second
	if err := cm.redisClient.SetUserOnline(cm.ctx, userID, cm.nodeID, ttl); err != nil {
		// Log error but don't fail the connection
		// This is a degraded mode where online status tracking is unavailable
		fmt.Printf("Warning: failed to set user %s online in Redis: %v\n", userID, err)
	}

	return connection, nil
}

// RemoveConnection removes a WebSocket connection from the manager.
// It performs comprehensive cleanup including:
// 1. Closing the WebSocket connection
// 2. Removing from the connections map
// 3. Updating Redis to remove online status
// 4. Logging all cleanup steps
//
// Parameters:
//   - userID: The user identifier
//
// Returns:
//   - error: Any error encountered during removal
func (cm *ConnectionManager) RemoveConnection(userID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, exists := cm.connections[userID]
	if !exists {
		return fmt.Errorf("connection not found for user %s", userID)
	}

	// Log the removal with connection details
	fmt.Printf("[MANAGER] Removing connection for user %s (guild: %s)\n", userID, conn.GuildID)

	// Step 1: Close the connection to release resources
	if err := conn.Close(); err != nil {
		fmt.Printf("[MANAGER ERROR] Error closing connection for user %s: %v\n", userID, err)
		// Continue with cleanup even if close fails
	} else {
		fmt.Printf("[MANAGER] Connection closed for user %s\n", userID)
	}

	// Step 2: Remove from connections map
	delete(cm.connections, userID)
	fmt.Printf("[MANAGER] User %s removed from connections map (remaining: %d)\n", userID, len(cm.connections))

	// Step 3: Remove from Redis online status
	ctx, cancel := context.WithTimeout(cm.ctx, 2*time.Second)
	defer cancel()

	if err := cm.redisClient.RemoveUserOnline(ctx, userID); err != nil {
		fmt.Printf("[MANAGER ERROR] Failed to remove user %s online status from Redis: %v\n", userID, err)
		// Return error but cleanup is still partially successful
		return fmt.Errorf("failed to remove online status: %w", err)
	}

	fmt.Printf("[MANAGER] User %s online status removed from Redis\n", userID)
	fmt.Printf("[MANAGER] Successfully removed connection for user %s\n", userID)

	return nil
}

// GetConnection retrieves a WebSocket connection by user ID.
//
// Parameters:
//   - userID: The user identifier
//
// Returns:
//   - *Connection: The connection object, or nil if not found
//   - bool: true if the connection exists, false otherwise
func (cm *ConnectionManager) GetConnection(userID string) (*Connection, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conn, exists := cm.connections[userID]
	return conn, exists
}

// GetAllConnections returns a snapshot of all current connections.
// The returned map is a copy and safe to iterate over.
//
// Returns:
//   - map[string]*Connection: A map of userID to Connection
func (cm *ConnectionManager) GetAllConnections() map[string]*Connection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Create a copy to avoid holding the lock during iteration
	snapshot := make(map[string]*Connection, len(cm.connections))
	for userID, conn := range cm.connections {
		snapshot[userID] = conn
	}

	return snapshot
}

// GetConnectionsByGuild returns all connections for a specific guild.
// This is used for broadcasting messages to all users in a guild.
//
// Parameters:
//   - guildID: The guild identifier
//
// Returns:
//   - []*Connection: A slice of connections for the guild
func (cm *ConnectionManager) GetConnectionsByGuild(guildID string) []*Connection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var connections []*Connection
	for _, conn := range cm.connections {
		if conn.GuildID == guildID {
			connections = append(connections, conn)
		}
	}

	return connections
}

// ConnectionCount returns the total number of active connections.
func (cm *ConnectionManager) ConnectionCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections)
}

// monitorHeartbeats periodically checks all connections for heartbeat timeouts.
// Connections that haven't sent a heartbeat within the timeout period are closed.
func (cm *ConnectionManager) monitorHeartbeats() {
	defer cm.wg.Done()

	// Check heartbeats every heartbeat interval
	ticker := time.NewTicker(time.Duration(cm.config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	// Timeout is 2x the heartbeat interval
	timeout := time.Duration(cm.config.HeartbeatInterval*2) * time.Second

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.checkHeartbeats(timeout)
		}
	}
}

// checkHeartbeats checks all connections for heartbeat timeouts and closes dead ones.
func (cm *ConnectionManager) checkHeartbeats(timeout time.Duration) {
	cm.mu.RLock()
	// Collect dead connections and active connections
	var deadConnections []string
	var activeConnections []string
	for userID, conn := range cm.connections {
		if !conn.IsAlive(timeout) {
			deadConnections = append(deadConnections, userID)
		} else {
			activeConnections = append(activeConnections, userID)
		}
	}
	cm.mu.RUnlock()

	// Remove dead connections (requires write lock)
	for _, userID := range deadConnections {
		fmt.Printf("Removing dead connection for user %s (heartbeat timeout)\n", userID)
		cm.RemoveConnection(userID)
	}

	// Refresh active connections in Redis
	// We do this without holding the lock to avoid blocking other operations
	ttl := time.Duration(cm.config.HeartbeatInterval*2) * time.Second
	for _, userID := range activeConnections {
		if err := cm.redisClient.SetUserOnline(cm.ctx, userID, cm.nodeID, ttl); err != nil {
			fmt.Printf("Failed to refresh online status for user %s: %v\n", userID, err)
		}
	}
}

// Shutdown gracefully shuts down the connection manager.
// It closes all active connections and waits for background goroutines to finish.
func (cm *ConnectionManager) Shutdown() error {
	// Cancel context to stop background goroutines
	cm.cancel()

	// Close all connections
	cm.mu.Lock()
	for userID, conn := range cm.connections {
		if err := conn.Close(); err != nil {
			fmt.Printf("Warning: error closing connection for user %s: %v\n", userID, err)
		}
		delete(cm.connections, userID)
	}
	cm.mu.Unlock()

	// Wait for background goroutines to finish
	cm.wg.Wait()

	return nil
}
