package redis

import (
	"context"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Gopher0727/ChatRoom/config"
)

func TestNewClient(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		cfg := &config.RedisConfig{
			Host:         "127.0.0.1",
			Port:         6379,
			Password:     "",
			DB:           15,
			PoolSize:     10,
			MinIdleConns: 2,
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Skipf("Skipping test: Redis not available: %v", err)
		}
		defer client.Close()

		assert.NotNil(t, client)
		assert.NotNil(t, client.client)
	})

	t.Run("connection failure with invalid address", func(t *testing.T) {
		cfg := &config.RedisConfig{
			Host:         "invalid",
			Port:         9999,
			Password:     "",
			DB:           0,
			PoolSize:     10,
			MinIdleConns: 2,
		}

		client, err := NewClient(cfg)
		assert.Error(t, err)
		assert.Nil(t, client)
	})
}

func TestClient_Ping(t *testing.T) {
	client := setupTestRedis(t)
	ctx := context.Background()

	err := client.Ping(ctx)
	assert.NoError(t, err)
}

func TestClient_GenerateSeqID(t *testing.T) {
	client := setupTestRedis(t)
	ctx := context.Background()

	t.Run("generates incrementing IDs", func(t *testing.T) {
		guildID := "test-guild-1"

		// Generate multiple IDs
		id1, err := client.GenerateSeqID(ctx, guildID)
		require.NoError(t, err)
		assert.Greater(t, id1, int64(0))

		id2, err := client.GenerateSeqID(ctx, guildID)
		require.NoError(t, err)
		assert.Equal(t, id1+1, id2)

		id3, err := client.GenerateSeqID(ctx, guildID)
		require.NoError(t, err)
		assert.Equal(t, id2+1, id3)
	})

	t.Run("different guilds have independent sequences", func(t *testing.T) {
		guild1 := "test-guild-2"
		guild2 := "test-guild-3"

		id1_1, err := client.GenerateSeqID(ctx, guild1)
		require.NoError(t, err)

		id2_1, err := client.GenerateSeqID(ctx, guild2)
		require.NoError(t, err)

		id1_2, err := client.GenerateSeqID(ctx, guild1)
		require.NoError(t, err)

		id2_2, err := client.GenerateSeqID(ctx, guild2)
		require.NoError(t, err)

		// Each guild should have its own sequence
		assert.Equal(t, id1_1+1, id1_2)
		assert.Equal(t, id2_1+1, id2_2)
	})

	t.Run("concurrent ID generation maintains order", func(t *testing.T) {
		guildID := "test-guild-concurrent"
		numGoroutines := 10
		idsPerGoroutine := 10

		results := make(chan int64, numGoroutines*idsPerGoroutine)
		errors := make(chan error, numGoroutines*idsPerGoroutine)

		// Launch concurrent goroutines
		for i := 0; i < numGoroutines; i++ {
			go func() {
				for j := 0; j < idsPerGoroutine; j++ {
					id, err := client.GenerateSeqID(ctx, guildID)
					if err != nil {
						errors <- err
						return
					}
					results <- id
				}
			}()
		}

		// Collect results
		ids := make([]int64, 0, numGoroutines*idsPerGoroutine)
		for i := 0; i < numGoroutines*idsPerGoroutine; i++ {
			select {
			case id := <-results:
				ids = append(ids, id)
			case err := <-errors:
				t.Fatalf("Error generating ID: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for ID generation")
			}
		}

		// Verify all IDs are unique
		idSet := make(map[int64]bool)
		for _, id := range ids {
			assert.False(t, idSet[id], "Duplicate ID found: %d", id)
			idSet[id] = true
		}

		assert.Equal(t, numGoroutines*idsPerGoroutine, len(idSet))
	})
}

func TestClient_UserOnlineStatus(t *testing.T) {
	client := setupTestRedis(t)
	ctx := context.Background()

	t.Run("set and check user online", func(t *testing.T) {
		userID := "user-1"
		ttl := 10 * time.Second

		// User should not be online initially
		online, err := client.IsUserOnline(ctx, userID)
		require.NoError(t, err)
		assert.False(t, online)

		// Set user online
		err = client.SetUserOnline(ctx, userID, ttl)
		require.NoError(t, err)

		// User should now be online
		online, err = client.IsUserOnline(ctx, userID)
		require.NoError(t, err)
		assert.True(t, online)
	})

	t.Run("user status expires after TTL", func(t *testing.T) {
		userID := "user-2"
		ttl := 1 * time.Second

		// Set user online with short TTL
		err := client.SetUserOnline(ctx, userID, ttl)
		require.NoError(t, err)

		// User should be online
		online, err := client.IsUserOnline(ctx, userID)
		require.NoError(t, err)
		assert.True(t, online)

		// Wait for expiration
		time.Sleep(2 * time.Second)

		// User should no longer be online
		online, err = client.IsUserOnline(ctx, userID)
		require.NoError(t, err)
		assert.False(t, online)
	})

	t.Run("remove user online status", func(t *testing.T) {
		userID := "user-3"
		ttl := 10 * time.Second

		// Set user online
		err := client.SetUserOnline(ctx, userID, ttl)
		require.NoError(t, err)

		// Verify user is online
		online, err := client.IsUserOnline(ctx, userID)
		require.NoError(t, err)
		assert.True(t, online)

		// Remove user online status
		err = client.RemoveUserOnline(ctx, userID)
		require.NoError(t, err)

		// User should no longer be online
		online, err = client.IsUserOnline(ctx, userID)
		require.NoError(t, err)
		assert.False(t, online)
	})

	t.Run("multiple users independent status", func(t *testing.T) {
		user1 := "user-4"
		user2 := "user-5"
		ttl := 10 * time.Second

		// Set user1 online
		err := client.SetUserOnline(ctx, user1, ttl)
		require.NoError(t, err)

		// Check both users
		online1, err := client.IsUserOnline(ctx, user1)
		require.NoError(t, err)
		assert.True(t, online1)

		online2, err := client.IsUserOnline(ctx, user2)
		require.NoError(t, err)
		assert.False(t, online2)

		// Set user2 online
		err = client.SetUserOnline(ctx, user2, ttl)
		require.NoError(t, err)

		// Both should be online
		online1, err = client.IsUserOnline(ctx, user1)
		require.NoError(t, err)
		assert.True(t, online1)

		online2, err = client.IsUserOnline(ctx, user2)
		require.NoError(t, err)
		assert.True(t, online2)
	})
}

func TestClient_PubSub(t *testing.T) {
	client := setupTestRedis(t)
	ctx := context.Background()

	t.Run("publish and subscribe to channel", func(t *testing.T) {
		channel := "test-channel-1"
		message := "Hello, World!"

		// Subscribe to channel
		pubsub, err := client.Subscribe(ctx, channel)
		require.NoError(t, err)
		defer pubsub.Close()

		// Publish message
		err = client.Publish(ctx, channel, message)
		require.NoError(t, err)

		// Receive message
		msg, err := pubsub.ReceiveMessage(ctx)
		require.NoError(t, err)
		assert.Equal(t, channel, msg.Channel)
		assert.Equal(t, message, msg.Payload)
	})

	t.Run("subscribe to multiple channels", func(t *testing.T) {
		channel1 := "test-channel-2"
		channel2 := "test-channel-3"
		message1 := "Message 1"
		message2 := "Message 2"

		// Subscribe to multiple channels
		pubsub, err := client.Subscribe(ctx, channel1, channel2)
		require.NoError(t, err)
		defer pubsub.Close()

		// Publish to both channels
		err = client.Publish(ctx, channel1, message1)
		require.NoError(t, err)

		err = client.Publish(ctx, channel2, message2)
		require.NoError(t, err)

		// Receive messages (order may vary)
		receivedMessages := make(map[string]string)
		for i := 0; i < 2; i++ {
			msg, err := pubsub.ReceiveMessage(ctx)
			require.NoError(t, err)
			receivedMessages[msg.Channel] = msg.Payload
		}

		assert.Equal(t, message1, receivedMessages[channel1])
		assert.Equal(t, message2, receivedMessages[channel2])
	})

	t.Run("multiple subscribers receive same message", func(t *testing.T) {
		channel := "test-channel-4"
		message := "Broadcast message"

		// Create two subscribers
		pubsub1, err := client.Subscribe(ctx, channel)
		require.NoError(t, err)
		defer pubsub1.Close()

		pubsub2, err := client.Subscribe(ctx, channel)
		require.NoError(t, err)
		defer pubsub2.Close()

		// Publish message
		err = client.Publish(ctx, channel, message)
		require.NoError(t, err)

		// Both subscribers should receive the message
		msg1, err := pubsub1.ReceiveMessage(ctx)
		require.NoError(t, err)
		assert.Equal(t, message, msg1.Payload)

		msg2, err := pubsub2.ReceiveMessage(ctx)
		require.NoError(t, err)
		assert.Equal(t, message, msg2.Payload)
	})
}

func TestClient_BasicOperations(t *testing.T) {
	client := setupTestRedis(t)
	ctx := context.Background()

	t.Run("set and get", func(t *testing.T) {
		key := "test-key-1"
		value := "test-value-1"

		// Set value
		err := client.Set(ctx, key, value, 0)
		require.NoError(t, err)

		// Get value
		result, err := client.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		key := "non-existent-key"

		result, err := client.Get(ctx, key)
		assert.Error(t, err)
		assert.Equal(t, redis.Nil, err)
		assert.Empty(t, result)
	})

	t.Run("set with expiration", func(t *testing.T) {
		key := "test-key-2"
		value := "test-value-2"
		ttl := 1 * time.Second

		// Set value with TTL
		err := client.Set(ctx, key, value, ttl)
		require.NoError(t, err)

		// Value should exist
		result, err := client.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, result)

		// Wait for expiration
		time.Sleep(2 * time.Second)

		// Value should be gone
		_, err = client.Get(ctx, key)
		assert.Error(t, err)
		assert.Equal(t, redis.Nil, err)
	})

	t.Run("delete key", func(t *testing.T) {
		key := "test-key-3"
		value := "test-value-3"

		// Set value
		err := client.Set(ctx, key, value, 0)
		require.NoError(t, err)

		// Delete key
		err = client.Del(ctx, key)
		require.NoError(t, err)

		// Key should not exist
		_, err = client.Get(ctx, key)
		assert.Error(t, err)
		assert.Equal(t, redis.Nil, err)
	})

	t.Run("exists check", func(t *testing.T) {
		key1 := "test-key-4"
		key2 := "test-key-5"
		value := "test-value"

		// Set only key1
		err := client.Set(ctx, key1, value, 0)
		require.NoError(t, err)

		// Check existence
		count, err := client.Exists(ctx, key1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)

		count, err = client.Exists(ctx, key2)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)

		// Check multiple keys
		count, err = client.Exists(ctx, key1, key2)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}

func TestClient_ErrorHandling(t *testing.T) {
	client := setupTestRedis(t)

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.GenerateSeqID(ctx, "test-guild")
		assert.Error(t, err)
	})

	t.Run("context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout

		_, err := client.GenerateSeqID(ctx, "test-guild")
		assert.Error(t, err)
	})
}

func TestClient_Close(t *testing.T) {
	cfg := &config.RedisConfig{
		Host:         "127.0.0.1",
		Port:         6379,
		Password:     "",
		DB:           15,
		PoolSize:     10,
		MinIdleConns: 2,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
	}

	// Close should not error
	err = client.Close()
	assert.NoError(t, err)

	// Operations after close should fail
	ctx := context.Background()
	_, err = client.GenerateSeqID(ctx, "test-guild")
	assert.Error(t, err)
}
