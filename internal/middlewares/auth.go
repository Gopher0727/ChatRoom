package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/utils"
)

// AuthMiddleware JWT 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// 1. 尝试从请求头获取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			// 解析 Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}

		// 2. 如果请求头没有，尝试从 Query 参数获取 (主要用于 WebSocket)
		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			c.JSON(
				http.StatusUnauthorized,
				gin.H{"error": "未提供认证 Token"},
			)
			c.Abort()
			return
		}

		// 解析 token
		claims, err := utils.ParseToken(token)
		if err != nil {
			c.JSON(
				http.StatusUnauthorized,
				gin.H{"error": "Token 无效或已过期"},
			)
			c.Abort()
			return
		}

		// 将 claims 存储在 context 中
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.UserName)
		c.Set("email", claims.Email)

		c.Next()
	}
}
