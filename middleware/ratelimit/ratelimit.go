package ratelimit

import (
	"context"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Limiter defines the interface for rate limiting operations
type Limiter interface {
	// Allow checks if a request should be allowed based on rate limits
	// Returns true if allowed, false if rate limit exceeded
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)

	// AllowN checks if N requests should be allowed
	AllowN(ctx context.Context, key string, n int, limit int, window time.Duration) (bool, error)

	// Reset resets the rate limit counter for a key
	Reset(ctx context.Context, key string) error

	// GetRemaining returns the number of remaining requests in the current window
	GetRemaining(ctx context.Context, key string, limit int, window time.Duration) (int, error)
}

// TokenBucketLimiter implements rate limiting using the token bucket algorithm with Redis
// It uses Redis atomic operations to ensure thread-safety across distributed systems
type TokenBucketLimiter struct {
	redisClient *redis.Client
	logger      *zap.Logger
	fallback    bool // If true, allow requests when Redis is unavailable (fail-open)
}

// NewTokenBucketLimiter creates a new token bucket rate limiter
//
// Parameters:
//   - redisClient: Redis client for storing rate limit state
//   - logger: Logger for recording rate limit events
//   - fallback: If true, allows requests when Redis fails (fail-open strategy)
//
// Returns:
//   - *TokenBucketLimiter: The initialized rate limiter
func NewTokenBucketLimiter(redisClient *redis.Client, logger *zap.Logger, fallback bool) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		redisClient: redisClient,
		logger:      logger,
		fallback:    fallback,
	}
}

// Allow checks if a single request should be allowed based on rate limits
// It implements the token bucket algorithm using Redis INCR and EXPIRE commands
//
// Parameters:
//   - ctx: Context for the operation
//   - key: Unique identifier for the rate limit bucket (e.g., "user:123" or "ip:192.168.1.1")
//   - limit: Maximum number of requests allowed in the time window
//   - window: Time window for the rate limit (e.g., 1 minute)
//
// Returns:
//   - bool: true if the request is allowed, false if rate limit exceeded
//   - error: Any error encountered during the check
//
// Validates: Requirements 8.1 (Rate limiting)
func (l *TokenBucketLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return l.AllowN(ctx, key, 1, limit, window)
}

// AllowN checks if N requests should be allowed based on rate limits
// This is useful for operations that consume multiple tokens
//
// Parameters:
//   - ctx: Context for the operation
//   - key: Unique identifier for the rate limit bucket
//   - n: Number of tokens to consume
//   - limit: Maximum number of requests allowed in the time window
//   - window: Time window for the rate limit
//
// Returns:
//   - bool: true if the requests are allowed, false if rate limit exceeded
//   - error: Any error encountered during the check
func (l *TokenBucketLimiter) AllowN(ctx context.Context, key string, n int, limit int, window time.Duration) (bool, error) {
	// Create a time-based bucket key
	now := time.Now()
	bucketKey := l.getBucketKey(key, now, window)

	// Use Redis pipeline for atomic operations
	pipe := l.redisClient.Pipeline()

	// Increment the counter by n
	incrCmd := pipe.IncrBy(ctx, bucketKey, int64(n))

	// Set expiration if this is the first request in the window
	pipe.Expire(ctx, bucketKey, window+time.Second) // Add 1 second buffer

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		l.logger.Error("rate limit check failed",
			zap.String("key", bucketKey),
			zap.Error(err),
		)

		// Fail-open: allow request if Redis is unavailable and fallback is enabled
		if l.fallback {
			l.logger.Warn("rate limit check failed, allowing request (fail-open)",
				zap.String("key", key),
			)
			return true, nil
		}

		return false, fmt.Errorf("rate limit check failed: %w", err)
	}

	// Get the current count
	count := incrCmd.Val()

	// Check if limit exceeded
	allowed := count <= int64(limit)

	if !allowed {
		l.logger.Warn("rate limit exceeded",
			zap.String("key", key),
			zap.Int64("count", count),
			zap.Int("limit", limit),
			zap.Duration("window", window),
		)
	}

	return allowed, nil
}

// Reset resets the rate limit counter for a key
// This can be used to manually clear rate limits for a user or IP
//
// Parameters:
//   - ctx: Context for the operation
//   - key: The rate limit key to reset
//
// Returns:
//   - error: Any error encountered during reset
func (l *TokenBucketLimiter) Reset(ctx context.Context, key string) error {
	// Delete all bucket keys for this identifier
	// We need to delete both current and previous windows
	now := time.Now()
	windows := []time.Duration{time.Minute, time.Hour, 24 * time.Hour}

	var keys []string
	for _, window := range windows {
		keys = append(keys, l.getBucketKey(key, now, window))
		keys = append(keys, l.getBucketKey(key, now.Add(-window), window))
	}

	err := l.redisClient.Del(ctx, keys...).Err()
	if err != nil {
		return fmt.Errorf("failed to reset rate limit for key %s: %w", key, err)
	}

	l.logger.Info("rate limit reset",
		zap.String("key", key),
	)

	return nil
}

// GetRemaining returns the number of remaining requests in the current window
//
// Parameters:
//   - ctx: Context for the operation
//   - key: The rate limit key
//   - limit: Maximum number of requests allowed
//   - window: Time window for the rate limit
//
// Returns:
//   - int: Number of remaining requests (0 if limit exceeded)
//   - error: Any error encountered during the check
func (l *TokenBucketLimiter) GetRemaining(ctx context.Context, key string, limit int, window time.Duration) (int, error) {
	now := time.Now()
	bucketKey := l.getBucketKey(key, now, window)

	// Get current count
	count, err := l.redisClient.Get(ctx, bucketKey).Int64()
	if err != nil {
		if err == redis.Nil {
			// Key doesn't exist, all tokens available
			return limit, nil
		}
		return 0, fmt.Errorf("failed to get remaining tokens: %w", err)
	}

	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	return remaining, nil
}

// getBucketKey generates a time-based bucket key for rate limiting
// The key includes a timestamp bucket to implement sliding window
func (l *TokenBucketLimiter) getBucketKey(key string, now time.Time, window time.Duration) string {
	// Calculate the bucket timestamp based on the window
	var bucketTime int64

	switch {
	case window <= time.Minute:
		// For minute-based windows, use seconds as bucket
		bucketTime = now.Unix() / int64(window.Seconds())
	case window <= time.Hour:
		// For hour-based windows, use minutes as bucket
		bucketTime = now.Unix() / 60 / int64(window.Minutes())
	default:
		// For day-based windows, use hours as bucket
		bucketTime = now.Unix() / 3600 / int64(window.Hours())
	}

	return fmt.Sprintf("ratelimit:%s:%d", key, bucketTime)
}

// RateLimitConfig defines configuration for different rate limit rules
type RateLimitConfig struct {
	RegisterPerMinute int
	LoginPerMinute    int
	MessagePerMinute  int
	APIPerMinute      int
}

// RateLimitRule defines a rate limiting rule
type RateLimitRule struct {
	Limit  int
	Window time.Duration
}

// GetRuleForEndpoint returns the appropriate rate limit rule for an endpoint
func GetRuleForEndpoint(endpoint string, config *RateLimitConfig) RateLimitRule {
	switch endpoint {
	case "register":
		return RateLimitRule{
			Limit:  config.RegisterPerMinute,
			Window: time.Minute,
		}
	case "login":
		return RateLimitRule{
			Limit:  config.LoginPerMinute,
			Window: time.Minute,
		}
	case "message":
		return RateLimitRule{
			Limit:  config.MessagePerMinute,
			Window: time.Minute,
		}
	case "api":
		return RateLimitRule{
			Limit:  config.APIPerMinute,
			Window: time.Minute,
		}
	default:
		// Default rule: 100 requests per minute
		return RateLimitRule{
			Limit:  100,
			Window: time.Minute,
		}
	}
}
