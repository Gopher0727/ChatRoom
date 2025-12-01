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
		// 从请求头获取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(
				http.StatusUnauthorized,
				"missing authorization header",
			)
			c.Abort()
			return
		}

		// 解析 Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(
				http.StatusUnauthorized,
				"invalid authorization header format",
			)
			c.Abort()
			return
		}

		token := parts[1]

		// 解析 token
		claims, err := utils.ParseToken(token)
		if err != nil {
			c.JSON(
				http.StatusUnauthorized,
				"invalid or expired token",
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
