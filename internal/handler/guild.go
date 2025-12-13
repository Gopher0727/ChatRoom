package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/service"
)

type GuildHandler struct {
	guildService service.IGuildService
}

func NewGuildHandler(guildService service.IGuildService) *GuildHandler {
	return &GuildHandler{
		guildService: guildService,
	}
}

// CreateGuild handles guild creation
func (h *GuildHandler) CreateGuild(c *gin.Context) {
	var req service.CreateGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	guild, err := h.guildService.CreateGuild(c.Request.Context(), userID, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create guild"})
		return
	}

	c.JSON(http.StatusCreated, guild)
}

// JoinGuild handles joining a guild via invite code
func (h *GuildHandler) JoinGuild(c *gin.Context) {
	var req struct {
		InviteCode string `json:"invite_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	err := h.guildService.JoinGuild(c.Request.Context(), userID, req.InviteCode)
	if err != nil {
		switch err {
		case service.ErrInvalidInviteCode:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case service.ErrAlreadyMember:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join guild"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Joined guild successfully"})
}

// GetUserGuilds retrieves guilds for the authenticated user
func (h *GuildHandler) GetUserGuilds(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	guilds, err := h.guildService.GetUserGuilds(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve guilds"})
		return
	}

	c.JSON(http.StatusOK, guilds)
}

// GetGuildMembers retrieves members of a specific guild
func (h *GuildHandler) GetGuildMembers(c *gin.Context) {
	guildID := c.Param("id")
	if guildID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Guild ID is required"})
		return
	}

	// Optional: Check if user is member of the guild before showing members?
	// For now, we'll just call the service.

	members, err := h.guildService.GetGuildMembers(c.Request.Context(), guildID)
	if err != nil {
		if err == service.ErrGuildNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve members"})
		return
	}

	c.JSON(http.StatusOK, members)
}
