package api

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/configs"
	"github.com/Gopher0727/ChatRoom/internal/handlers"
	"github.com/Gopher0727/ChatRoom/internal/middlewares"
)

// SetupRoutes 设置所有路由
func SetupRoutes(r *gin.Engine, cfg *configs.Config,
	userHandler *handlers.UserHandler,
	guildHandler *handlers.GuildHandler,
) {
	r.Use(cors.Default())

	// 全局限流中间件 (防止 QPS 过高)
	// 使用配置中的参数，并设置等待超时时间
	// r.Use(middlewares.RateLimitMiddleware(2 * time.Second))

	// 异步处理中间件
	// 将请求放入 Worker Pool 中排队执行
	r.Use(middlewares.AsyncMiddleware())

	// 注册路由
	RegisterUserRoutes(r, userHandler)
	RegisterGuildRoutes(r, guildHandler)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"Status": "OK",
		})
	})
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
		guildGroup.POST("", guildHandler.CreateGuild) // 创建服务器

		// 成员管理
		guildGroup.POST("/join", guildHandler.JoinGuild) // 加入服务器 (通过邀请码)

		// 邀请码
		guildGroup.POST("/:guild_id/invites", guildHandler.CreateInvite) // 生成邀请链接

		// 消息相关
		guildGroup.POST("/:guild_id/messages", guildHandler.SendMessage) // 发送消息
		guildGroup.GET("/:guild_id/messages", guildHandler.GetMessages)  // 获取消息列表
	}
}

/*

// ChannelHandler 接口定义
func RegisterChannelRoutes(r *gin.Engine) {
    // 频道通常隶属于某个 Guild，但在 API 路径上可以直接操作 ID
    channelGroup := r.Group("/api/v1/channels")
    {
        // 创建频道 (需在 Body 中指定 guild_id, parent_id(分组), type(文字/语音))
        channelGroup.POST("", CreateChannel)

        channelGroup.GET("/:channel_id", GetChannel)        // 获取频道信息
        channelGroup.PATCH("/:channel_id", UpdateChannel)   // 修改频道 (名称、Topic、NSFW设置)
        channelGroup.DELETE("/:channel_id", DeleteChannel)  // 删除频道

        // 消息相关 (HTTP 部分用于获取历史记录)
        channelGroup.GET("/:channel_id/messages", GetChannelMessages) // 分页拉取历史消息

        // 消息置顶
        channelGroup.GET("/:channel_id/pins", GetPinnedMessages)
        channelGroup.PUT("/:channel_id/pins/:message_id", PinMessage)
        channelGroup.DELETE("/:channel_id/pins/:message_id", UnpinMessage)
    }
}


// RoleHandler 接口定义
func RegisterRoleRoutes(r *gin.Engine) {
    // 角色是依附于 Guild 的
    roleGroup := r.Group("/api/v1/guilds/:guild_id/roles")
    {
        roleGroup.GET("", GetGuildRoles)            // 获取该服务器所有角色
        roleGroup.POST("", CreateRole)              // 创建新角色 (设置颜色、名称、权限位图)
        roleGroup.PATCH("/:role_id", UpdateRole)    // 修改角色权限/排序
        roleGroup.DELETE("/:role_id", DeleteRole)   // 删除角色

        // 给成员分配角色
        roleGroup.PUT("/:role_id/members/:user_id", AddRoleToMember)
        roleGroup.DELETE("/:role_id/members/:user_id", RemoveRoleFromMember)
    }
}

*/
