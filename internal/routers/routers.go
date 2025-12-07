package routers

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/config"
	"github.com/Gopher0727/ChatRoom/internal/handlers"
	"github.com/Gopher0727/ChatRoom/internal/services"
	"github.com/Gopher0727/ChatRoom/pkg/middlewares"
	"github.com/Gopher0727/ChatRoom/pkg/mq"
	"github.com/Gopher0727/ChatRoom/pkg/ws"
)

// SetupRoutes 设置所有路由
func SetupRoutes(r *gin.Engine, cfg *config.Config,
	userHandler *handlers.UserHandler,
	guildHandler *handlers.GuildHandler,
	hub *ws.Hub, // 注入 Hub
	guildService *services.GuildService, // 注入 GuildService 用于 WS
	kafkaProducer *mq.KafkaProducer, // 注入 KafkaProducer 用于 WS
) {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	// 全局限流中间件 (防止 QPS 过高)
	// 使用配置中的参数，并设置等待超时时间
	// r.Use(middlewares.RateLimitMiddleware(2 * time.Second))

	// WebSocket 路由 (必须在 AsyncMiddleware 之前注册，避免握手请求被放入 Worker Pool)
	r.GET("/ws", middlewares.AuthMiddleware(), func(c *gin.Context) {
		ws.ServeWs(hub, guildService, kafkaProducer, c)
	})

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"Status": "OK",
		})
	})

	// 异步处理中间件
	// 将请求放入 Worker Pool 中排队执行
	r.Use(middlewares.AsyncMiddleware())

	// 注册路由
	RegisterUserRoutes(r, userHandler)
	RegisterGuildRoutes(r, guildHandler)

	// 静态文件服务 (用于前端演示)
	r.StaticFile("/", "./web/index.html")
	r.Static("/static", "./web")
}

// UserHandler 接口定义
func RegisterUserRoutes(r *gin.Engine, userHandler *handlers.UserHandler) {
	userGroup := r.Group("/api/v1/users")
	{
		userGroup.POST("/register", userHandler.Register) // 注册
		userGroup.POST("/login", userHandler.Login)       // 登录
	}
	userGroup.Use(middlewares.AuthMiddleware())
	{
		userGroup.POST("/logout", userHandler.Logout) // 登出
		userGroup.POST("/cancel", userHandler.Cancel) // 注销

		// 用户个人信息
		userGroup.GET("/me", userHandler.GetProfile)                // 获取当前用户信息
		userGroup.PUT("/me", userHandler.UpdateProfile)             // 更新头像、昵称、状态(Online/DND/Idle)
		userGroup.PATCH("/me/password", userHandler.ChangePassword) // 修改密码
	}
}

// GuildHandler 接口定义
func RegisterGuildRoutes(r *gin.Engine, guildHandler *handlers.GuildHandler) {
	guildGroup := r.Group("/api/v1/guilds")
	guildGroup.Use(middlewares.AuthMiddleware())
	{
		guildGroup.POST("", guildHandler.CreateGuild)     // 创建服务器
		guildGroup.GET("/mine", guildHandler.GetMyGuilds) // 获取我的服务器列表

		// 成员管理
		guildGroup.POST("/join", guildHandler.JoinGuild) // 加入服务器 (通过邀请码)

		// 邀请码
		guildGroup.POST("/:guild_id/invites", guildHandler.CreateInvite) // 生成邀请链接

		// 消息相关
		guildGroup.POST("/:guild_id/messages", guildHandler.SendMessage) // 发送消息
		guildGroup.GET("/:guild_id/messages", guildHandler.GetMessages)  // 获取消息列表
		guildGroup.POST("/:guild_id/ack", guildHandler.AckMessage)       // 确认消息已读
	}
}
