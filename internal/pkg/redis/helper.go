package redis

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"

	"github.com/Gopher0727/ChatRoom/config"
)

// setupTestRedis creates a test Redis client.
// ! This requires a running Redis instance for integration testing.
func setupTestRedis(t *testing.T) *Client {
	cfg := &config.RedisConfig{
		Host:         "127.0.0.1",
		Port:         6379,
		Password:     "",
		DB:           15, // Use a separate DB for testing
		PoolSize:     10,
		MinIdleConns: 2,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping test: Redis not available: %v", err)
	}

	// Clean up test DB before tests
	ctx := context.Background()
	client.client.FlushDB(ctx)

	t.Cleanup(func() {
		client.client.FlushDB(ctx)
		client.Close()
	})

	return client
}

// genGuildID generates random guild IDs for property testing
func genGuildID() gopter.Gen {
	return gen.Identifier()
}

// genUserID generates random user IDs for property testing
func genUserID() gopter.Gen {
	return gen.Identifier()
}

// genTTL generates random TTL durations for property testing (1-60 seconds)
func genTTL() gopter.Gen {
	return gen.IntRange(1, 60).Map(func(seconds int) time.Duration {
		return time.Duration(seconds) * time.Second
	})
}
