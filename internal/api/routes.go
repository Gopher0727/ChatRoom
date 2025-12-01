package api

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/handlers"
	"github.com/Gopher0727/ChatRoom/internal/middlewares"
)

// SetupRoutes 设置所有路由
func SetupRoutes(r *gin.Engine,
	userHandler *handlers.UserHandler,
) {
	// 应用全局中间件
	r.Use(cors.Default())

	RegisterUserRoutes(r, userHandler)

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

/*

// GuildHandler 接口定义
func RegisterGuildRoutes(r *gin.Engine) {
    guildGroup := r.Group("/api/v1/guilds")
    {
        guildGroup.POST("", CreateGuild)                // 创建服务器
        guildGroup.GET("/:guild_id", GetGuildInfo)      // 获取服务器详情（含频道列表、角色列表）
        guildGroup.PUT("/:guild_id", UpdateGuild)       // 修改服务器信息（图标、名称）
        guildGroup.DELETE("/:guild_id", DeleteGuild)    // 删除/解散服务器

        // 成员管理
        guildGroup.POST("/:guild_id/join", JoinGuild)         // 加入服务器 (通过邀请码)
        guildGroup.DELETE("/:guild_id/leave", LeaveGuild)     // 退出服务器
        guildGroup.DELETE("/:guild_id/members/:user_id", KickMember) // 踢人
        guildGroup.PUT("/:guild_id/members/:user_id/ban", BanMember) // 封禁

        // 邀请码
        guildGroup.POST("/:guild_id/invites", CreateInvite)   // 生成邀请链接
        guildGroup.GET("/:guild_id/invites", GetInvites)      // 获取活跃邀请列表
    }
}


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


// MessageHandler 接口定义
func RegisterMessageRoutes(r *gin.Engine) {
    msgGroup := r.Group("/api/v1/messages")
    {
        msgGroup.PATCH("/:message_id", EditMessage)    // 编辑消息 (Discord 允许修改已发内容)
        msgGroup.DELETE("/:message_id", DeleteMessage) // 撤回/删除消息

        // 表情回应 (Reactions)
        // PUT /messages/123/reactions/🔥/me
        msgGroup.PUT("/:message_id/reactions/:emoji", AddReaction)
        msgGroup.DELETE("/:message_id/reactions/:emoji", RemoveReaction)
    }
}

*/
