package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/service"
)

type MessageHandler struct {
	messageService service.IMessageService
}

func NewMessageHandler(messageService service.IMessageService) *MessageHandler {
	return &MessageHandler{
		messageService: messageService,
	}
}

// SendMessage handles sending a message to a guild
func (h *MessageHandler) SendMessage(c *gin.Context) {
	var req service.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Ensure the user ID in request matches the authenticated user
	// Or just override it
	req.UserID = userID

	msg, err := h.messageService.SendMessage(c.Request.Context(), req.UserID, req.GuildID, req.Content)
	if err != nil {
		switch err {
		case service.ErrUserNotInGuild:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case service.ErrInvalidMessageContent:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		}
		return
	}

	c.JSON(http.StatusCreated, msg)
}

// GetMessages retrieves messages for a guild
func (h *MessageHandler) GetMessages(c *gin.Context) {
	guildID := c.Query("guild_id")
	if guildID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "guild_id is required"})
		return
	}

	lastSeqIDStr := c.Query("last_seq_id")
	var lastSeqID int64
	var err error
	if lastSeqIDStr != "" {
		lastSeqID, err = strconv.ParseInt(lastSeqIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid last_seq_id"})
			return
		}
	}

	limitStr := c.Query("limit")
	limit := 50
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
			return
		}
	}

	messages, hasMore, err := h.messageService.GetMessages(c.Request.Context(), guildID, lastSeqID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"has_more": hasMore,
	})
}
