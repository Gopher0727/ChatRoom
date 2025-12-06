package middlewares

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/pkg/utils"
)

var (
	globalLimiter *utils.TokenBucket
	limitOnce     sync.Once
)

// InitGlobalLimiter 初始化全局限流器
// capacity: 突发流量容量
// rate: 每秒允许的请求数 (QPS)
func InitGlobalLimiter(capacity, rate int64) {
	limitOnce.Do(func() {
		globalLimiter = utils.NewTokenBucket(capacity, rate)
	})
}

// RateLimitMiddleware 全局限流中间件
// 使用令牌桶算法限制请求速率，平滑突发流量
// waitTimeout: 等待令牌的最大时长，超过该时长仍未获取到令牌则拒绝请求
func RateLimitMiddleware(waitTimeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if globalLimiter != nil {
			// 尝试等待令牌，而不是直接拒绝
			// 这样可以处理瞬时的流量尖峰，只要在超时时间内能拿到令牌即可
			if !globalLimiter.WaitN(1, waitTimeout) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "Too Many Requests - Server is busy, please try again later",
				})
				return
			}
		}
		c.Next()
	}
}

// MaxConcurrencyMiddleware 最大并发控制中间件
// 限制同时处理的请求数量，防止 Goroutine 数量无限增长导致 OOM (Out Of Memory)
// 这是控制内存占用最直接有效的方法
func MaxConcurrencyMiddleware(maxConcurrent int) gin.HandlerFunc {
	// 使用带缓冲的 channel 作为信号量
	sem := make(chan struct{}, maxConcurrent)

	return func(c *gin.Context) {
		select {
		case sem <- struct{}{}: // 尝试获取信号量
			defer func() { <-sem }() // 处理完释放信号量
			c.Next()
		default:
			// 获取失败，说明并发已满，直接拒绝
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "Service Unavailable - Too many concurrent requests",
			})
		}
	}
}
