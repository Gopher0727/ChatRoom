package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/Gopher0727/ChatRoom/config"
	"github.com/Gopher0727/ChatRoom/internal/pkg/redis"
	"github.com/Gopher0727/ChatRoom/middleware/jwt"
	"github.com/Gopher0727/ChatRoom/middleware/ratelimit"
)

type MiddlewareManager struct {
	tokenManager *jwt.TokenManager
	redisClient  *redis.Client
	rateLimiter  ratelimit.Limiter
	logger       *zap.Logger
	rateLimitCfg *config.RateLimitConfig
}

func NewMiddlewareManager(
	tokenManager *jwt.TokenManager,
	redisClient *redis.Client,
	logger *zap.Logger,
	rateLimitCfg *config.RateLimitConfig,
) *MiddlewareManager {
	// Create rate limiter with fail-open strategy (allow requests if Redis fails)
	rateLimiter := ratelimit.NewTokenBucketLimiter(redisClient.GetClient(), logger, true)

	return &MiddlewareManager{
		tokenManager: tokenManager,
		redisClient:  redisClient,
		rateLimiter:  rateLimiter,
		logger:       logger,
		rateLimitCfg: rateLimitCfg,
	}
}

func (m *MiddlewareManager) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authorization header required",
			})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := m.tokenManager.ParseToken(tokenString)
		if err != nil {
			m.logger.Warn("token validation failed",
				zap.String("error", err.Error()),
				zap.String("ip", c.ClientIP()),
			)

			var statusCode int
			var message string

			switch err {
			case jwt.ErrExpiredToken:
				statusCode = http.StatusUnauthorized
				message = "token has expired"
			case jwt.ErrTokenNotYetValid:
				statusCode = http.StatusUnauthorized
				message = "token not yet valid"
			default:
				statusCode = http.StatusUnauthorized
				message = "invalid token"
			}

			c.JSON(statusCode, gin.H{
				"error": message,
			})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.UserName)

		c.Next()
	}
}

func (m *MiddlewareManager) RateLimit(limitPerMinute int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// Determine the rate limit key
		// Use user_id if authenticated, otherwise use IP
		var key string
		if userID, exists := c.Get("user_id"); exists {
			key = fmt.Sprintf("user:%s", userID)
		} else {
			key = fmt.Sprintf("ip:%s", c.ClientIP())
		}

		allowed, err := m.rateLimiter.Allow(ctx, key, limitPerMinute, time.Minute)
		if err != nil {
			m.logger.Error("rate limit check failed",
				zap.String("error", err.Error()),
				zap.String("key", key),
			)
			// Error already handled by rate limiter (fail-open if configured)
			if allowed {
				c.Next()
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "rate limit check failed",
				})
				c.Abort()
			}
			return
		}

		// Check if limit exceeded
		if !allowed {
			// Get remaining tokens for retry-after calculation
			remaining, _ := m.rateLimiter.GetRemaining(ctx, key, limitPerMinute, time.Minute)

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": 60, // seconds until next window
				"remaining":   remaining,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (m *MiddlewareManager) RateLimiterByEndpoint(endpoint string) gin.HandlerFunc {
	rule := ratelimit.GetRuleForEndpoint(endpoint, &ratelimit.RateLimitConfig{
		RegisterPerMinute: m.rateLimitCfg.RegisterPerMinute,
		LoginPerMinute:    m.rateLimitCfg.LoginPerMinute,
		MessagePerMinute:  m.rateLimitCfg.MessagePerMinute,
		APIPerMinute:      m.rateLimitCfg.APIPerMinute,
	})

	return func(c *gin.Context) {
		ctx := context.Background()

		// Determine the rate limit key
		// Use user_id if authenticated, otherwise use IP
		var key string
		if userID, exists := c.Get("user_id"); exists {
			key = fmt.Sprintf("user:%s:%s", userID, endpoint)
		} else {
			key = fmt.Sprintf("ip:%s:%s", c.ClientIP(), endpoint)
		}

		// Check rate limit
		allowed, err := m.rateLimiter.Allow(ctx, key, rule.Limit, rule.Window)
		if err != nil {
			m.logger.Error("rate limit check failed",
				zap.String("error", err.Error()),
				zap.String("key", key),
				zap.String("endpoint", endpoint),
			)
			// Error already handled by rate limiter (fail-open if configured)
			if allowed {
				c.Next()
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "rate limit check failed",
				})
				c.Abort()
			}
			return
		}

		if !allowed {
			remaining, _ := m.rateLimiter.GetRemaining(ctx, key, rule.Limit, rule.Window)

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": int(rule.Window.Seconds()),
				"remaining":   remaining,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (m *MiddlewareManager) Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		statusCode := c.Writer.Status()

		// Get user ID if available
		userID, _ := c.Get("user_id")

		// Log the request
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		}

		if userID != nil {
			fields = append(fields, zap.String("user_id", userID.(string)))
		}

		// Log at different levels based on status code
		if statusCode >= 500 {
			m.logger.Error("server error", fields...)
		} else if statusCode >= 400 {
			m.logger.Warn("client error", fields...)
		} else {
			m.logger.Info("request completed", fields...)
		}
	}
}

func (m *MiddlewareManager) CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (m *MiddlewareManager) Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				m.logger.Error("panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)

				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
				c.Abort()
			}
		}()

		c.Next()
	}
}
