package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// setupTestRedis creates a miniredis instance for testing
func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, mr
}

// TestTokenBucketLimiter_Allow tests basic rate limiting functionality
func TestTokenBucketLimiter_Allow(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false)

	ctx := context.Background()
	key := "test:user:123"
	limit := 5
	window := time.Minute

	// First 5 requests should be allowed
	for i := range limit {
		allowed, err := limiter.Allow(ctx, key, limit, window)
		assert.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}

	// 6th request should be denied
	allowed, err := limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.False(t, allowed, "request should be denied after limit exceeded")
}

// TestTokenBucketLimiter_AllowN tests consuming multiple tokens at once
func TestTokenBucketLimiter_AllowN(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false)

	ctx := context.Background()
	key := "test:user:456"
	limit := 10
	window := time.Minute

	// Consume 3 tokens
	allowed, err := limiter.AllowN(ctx, key, 3, limit, window)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Consume 5 more tokens (total 8)
	allowed, err = limiter.AllowN(ctx, key, 5, limit, window)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Consume 2 more tokens (total 10) - should succeed
	allowed, err = limiter.AllowN(ctx, key, 2, limit, window)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Try to consume 1 more token (would be 11 total) - should fail
	allowed, err = limiter.AllowN(ctx, key, 1, limit, window)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

// TestTokenBucketLimiter_Reset tests resetting rate limits
func TestTokenBucketLimiter_Reset(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false)

	ctx := context.Background()
	key := "test:user:789"
	limit := 3
	window := time.Minute

	// Exhaust the limit
	for i := 0; i < limit; i++ {
		allowed, err := limiter.Allow(ctx, key, limit, window)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// Next request should be denied
	allowed, err := limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Reset the limit
	err = limiter.Reset(ctx, key)
	assert.NoError(t, err)

	// Should be able to make requests again
	allowed, err = limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

// TestTokenBucketLimiter_GetRemaining tests getting remaining tokens
func TestTokenBucketLimiter_GetRemaining(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false)

	ctx := context.Background()
	key := "test:user:remaining"
	limit := 10
	window := time.Minute

	// Initially, all tokens should be available
	remaining, err := limiter.GetRemaining(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.Equal(t, limit, remaining)

	// Consume 3 tokens
	allowed, err := limiter.AllowN(ctx, key, 3, limit, window)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Should have 7 remaining
	remaining, err = limiter.GetRemaining(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.Equal(t, 7, remaining)

	// Consume 7 more tokens
	allowed, err = limiter.AllowN(ctx, key, 7, limit, window)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Should have 0 remaining
	remaining, err = limiter.GetRemaining(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.Equal(t, 0, remaining)
}

// TestTokenBucketLimiter_ConcurrentRequests tests rate limiting under concurrent load
func TestTokenBucketLimiter_ConcurrentRequests(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false)

	ctx := context.Background()
	key := "test:user:concurrent"
	limit := 100
	window := time.Minute
	numGoroutines := 50
	requestsPerGoroutine := 3

	allowedCount := 0
	deniedCount := 0

	var wg sync.WaitGroup
	var mu sync.Mutex
	// Launch concurrent requests
	for range numGoroutines {
		wg.Go(func() {
			for j := 0; j < requestsPerGoroutine; j++ {
				allowed, err := limiter.Allow(ctx, key, limit, window)
				assert.NoError(t, err)

				mu.Lock()
				if allowed {
					allowedCount++
				} else {
					deniedCount++
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	// Total requests = 50 * 3 = 150
	// Limit = 100
	// So we should have exactly 100 allowed and 50 denied
	assert.Equal(t, limit, allowedCount, "should allow exactly %d requests", limit)
	assert.Equal(t, numGoroutines*requestsPerGoroutine-limit, deniedCount, "should deny excess requests")
}

// TestTokenBucketLimiter_DifferentKeys tests that different keys have independent limits
func TestTokenBucketLimiter_DifferentKeys(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false)

	ctx := context.Background()
	key1 := "test:user:alice"
	key2 := "test:user:bob"
	limit := 3
	window := time.Minute

	// Exhaust limit for key1
	for range limit {
		allowed, err := limiter.Allow(ctx, key1, limit, window)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// key1 should be denied
	allowed, err := limiter.Allow(ctx, key1, limit, window)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// key2 should still be allowed (independent limit)
	for range limit {
		allowed, err := limiter.Allow(ctx, key2, limit, window)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}
}

// TestTokenBucketLimiter_IPLevelRateLimit tests IP-based rate limiting
func TestTokenBucketLimiter_IPLevelRateLimit(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false)

	ctx := context.Background()
	ip := "192.168.1.100"
	key := fmt.Sprintf("ip:%s", ip)
	limit := 10
	window := time.Minute

	// Make requests up to the limit
	for i := range limit {
		allowed, err := limiter.Allow(ctx, key, limit, window)
		assert.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}

	// Next request should be denied
	allowed, err := limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.False(t, allowed, "request should be denied for IP after limit")
}

// TestTokenBucketLimiter_UserLevelRateLimit tests user-based rate limiting
func TestTokenBucketLimiter_UserLevelRateLimit(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false)

	ctx := context.Background()
	userID := "user_12345"
	key := fmt.Sprintf("user:%s", userID)
	limit := 60
	window := time.Minute

	// Make requests up to the limit
	for range limit {
		allowed, err := limiter.Allow(ctx, key, limit, window)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// Next request should be denied
	allowed, err := limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

// TestTokenBucketLimiter_RateLimitRecovery tests that rate limits recover after the window expires
func TestTokenBucketLimiter_RateLimitRecovery(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false)

	ctx := context.Background()
	key := "test:user:recovery"
	limit := 3
	window := 2 * time.Second // Short window for testing

	// Exhaust the limit
	for range limit {
		allowed, err := limiter.Allow(ctx, key, limit, window)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// Should be denied
	allowed, err := limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Fast-forward time in miniredis
	mr.FastForward(window + time.Second)

	// Should be allowed again in new window
	allowed, err = limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

// TestTokenBucketLimiter_FailOpen tests fail-open behavior when Redis is unavailable
func TestTokenBucketLimiter_FailOpen(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, true) // Enable fail-open

	ctx := context.Background()
	key := "test:user:failopen"
	limit := 5
	window := time.Minute

	// Close Redis to simulate failure
	mr.Close()

	// With fail-open enabled, requests should be allowed even when Redis fails
	allowed, err := limiter.Allow(ctx, key, limit, window)
	// Error should be nil because fail-open is enabled
	assert.NoError(t, err)
	assert.True(t, allowed, "should allow request when Redis fails with fail-open enabled")
}

// TestTokenBucketLimiter_FailClosed tests fail-closed behavior when Redis is unavailable
func TestTokenBucketLimiter_FailClosed(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false) // Disable fail-open

	ctx := context.Background()
	key := "test:user:failclosed"
	limit := 5
	window := time.Minute

	// Close Redis to simulate failure
	mr.Close()

	// With fail-open disabled, requests should be denied when Redis fails
	allowed, err := limiter.Allow(ctx, key, limit, window)
	assert.Error(t, err, "should return error when Redis fails with fail-open disabled")
	assert.False(t, allowed, "should deny request when Redis fails with fail-open disabled")
}

// TestGetRuleForEndpoint tests rate limit rule configuration
func TestGetRuleForEndpoint(t *testing.T) {
	config := &RateLimitConfig{
		RegisterPerMinute: 10,
		LoginPerMinute:    20,
		MessagePerMinute:  60,
		APIPerMinute:      100,
	}

	tests := []struct {
		endpoint string
		expected RateLimitRule
	}{
		{
			endpoint: "register",
			expected: RateLimitRule{Limit: 10, Window: time.Minute},
		},
		{
			endpoint: "login",
			expected: RateLimitRule{Limit: 20, Window: time.Minute},
		},
		{
			endpoint: "message",
			expected: RateLimitRule{Limit: 60, Window: time.Minute},
		},
		{
			endpoint: "api",
			expected: RateLimitRule{Limit: 100, Window: time.Minute},
		},
		{
			endpoint: "unknown",
			expected: RateLimitRule{Limit: 100, Window: time.Minute}, // Default
		},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			rule := GetRuleForEndpoint(tt.endpoint, config)
			assert.Equal(t, tt.expected.Limit, rule.Limit)
			assert.Equal(t, tt.expected.Window, rule.Window)
		})
	}
}

// TestTokenBucketLimiter_DifferentWindows tests rate limiting with different time windows
func TestTokenBucketLimiter_DifferentWindows(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewTokenBucketLimiter(client, logger, false)

	ctx := context.Background()
	key := "test:user:windows"

	tests := []struct {
		name   string
		limit  int
		window time.Duration
	}{
		{"1 minute window", 10, time.Minute},
		{"5 minute window", 50, 5 * time.Minute},
		{"1 hour window", 100, time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testKey := fmt.Sprintf("%s:%s", key, tt.name)

			// Make requests up to limit
			for i := 0; i < tt.limit; i++ {
				allowed, err := limiter.Allow(ctx, testKey, tt.limit, tt.window)
				assert.NoError(t, err)
				assert.True(t, allowed)
			}

			// Next request should be denied
			allowed, err := limiter.Allow(ctx, testKey, tt.limit, tt.window)
			assert.NoError(t, err)
			assert.False(t, allowed)
		})
	}
}
