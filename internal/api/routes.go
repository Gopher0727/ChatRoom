package api

import (
	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/handlers"
	"github.com/Gopher0727/ChatRoom/internal/middlewares"
)

// SetupRoutes 设置所有路由
func SetupRoutes(
	r *gin.Engine,
	authHandler *handlers.AuthHandler,
	groupHandler *handlers.GroupHandler,
	messageHandler *handlers.MessageHandler,
) {
	// 应用全局中间件
	r.Use(middlewares.CORSMiddleware())

	// 认证路由（无需认证）
	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
	}

	// 用户路由（需要认证）
	userGroup := r.Group("/api/v1/user")
	userGroup.Use(middlewares.AuthMiddleware())
	{
		userGroup.POST("/logout", authHandler.Logout)
	}

	// 群组路由（需要认证）
	groupGroup := r.Group("/api/v1/groups")
	groupGroup.Use(middlewares.AuthMiddleware())
	{
		groupGroup.POST("", groupHandler.CreateGroup)                // 创建群组
		groupGroup.GET("/search", groupHandler.GetGroupByInviteCode) // 通过邀请码搜索群组
		groupGroup.POST("/join", groupHandler.JoinGroup)             // 加入群组
		groupGroup.GET("/my", groupHandler.GetUserGroups)            // 获取用户的群组列表
		groupGroup.GET("/:id", groupHandler.GetGroupDetail)          // 获取群组详情
		groupGroup.GET("/:id/members", groupHandler.GetGroupMembers) // 获取群组成员
		groupGroup.DELETE("/:id", groupHandler.LeaveGroup)           // 离开群组
	}

	// 消息路由（需要认证）
	messageGroup := r.Group("/api/v1/messages")
	messageGroup.Use(middlewares.AuthMiddleware())
	{
		messageGroup.POST("", messageHandler.SendMessage)                              // 发送消息
		messageGroup.GET("/group/:groupId", messageHandler.GetGroupMessages)           // 获取群组消息
		messageGroup.POST("/mark-read", messageHandler.MarkAsRead)                     // 标记已读
		messageGroup.GET("/group/:groupId/unread", messageHandler.GetUnreadCount)      // 获取未读数
		messageGroup.GET("/group/:groupId/sequence", messageHandler.GetLatestSequence) // 获取最新序列号
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
}
