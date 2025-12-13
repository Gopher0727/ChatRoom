package api

import (
	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/handler"
	middlewares "github.com/Gopher0727/ChatRoom/middleware/auth"
)

// RegisterRoutes registers all API routes
func RegisterRoutes(
	r *gin.Engine,
	authHandler *handler.AuthHandler,
	guildHandler *handler.GuildHandler,
	messageHandler *handler.MessageHandler,
) {
	// Public routes
	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}
	}

	// Protected routes
	protected := api.Group("/")
	protected.Use(middlewares.AuthMiddleware())
	{
		// Guild routes
		guilds := protected.Group("/guilds")
		{
			guilds.POST("", guildHandler.CreateGuild)
			guilds.POST("/join", guildHandler.JoinGuild)
			guilds.GET("", guildHandler.GetUserGuilds)
			guilds.GET("/:id/members", guildHandler.GetGuildMembers)
		}

		// Message routes
		messages := protected.Group("/messages")
		{
			messages.POST("", messageHandler.SendMessage)
			messages.GET("", messageHandler.GetMessages)
		}
	}
}
