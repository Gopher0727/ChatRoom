package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/services"
)

// MessageHandler 消息处理器
type MessageHandler struct {
	messageService *services.MessageService
}

// NewMessageHandler 创建消息处理器实例
func NewMessageHandler(messageService *services.MessageService) *MessageHandler {
	return &MessageHandler{
		messageService: messageService,
	}
}

// SendMessage 发送消息
func (h *MessageHandler) SendMessage(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		log.Printf("SendMessage: userID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	var req services.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("SendMessage: JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	message, err := h.messageService.SendMessage(userID.(uint), &req)
	if err != nil {
		log.Printf("SendMessage: service error for userID %v: %v", userID, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    message,
	})
}

// GetGroupMessages 获取群组消息列表
func (h *MessageHandler) GetGroupMessages(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("groupId"), 10, 32)
	if err != nil {
		log.Printf("GetGroupMessages: invalid group id: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid group id",
		})
		return
	}

	page := 1
	pageSize := 50

	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	if ps := c.Query("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}

	messages, total, err := h.messageService.GetGroupMessages(uint(groupID), page, pageSize)
	if err != nil {
		log.Printf("GetGroupMessages: service error for groupID %v: %v", groupID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"messages":  messages,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// MarkAsRead 标记消息已读
func (h *MessageHandler) MarkAsRead(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		log.Printf("MarkAsRead: userID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	type MarkAsReadRequest struct {
		GroupID uint  `json:"group_id" binding:"required"`
		MsgID   int64 `json:"msg_id" binding:"required"`
	}

	var req MarkAsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("MarkAsRead: JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	err := h.messageService.MarkAsRead(req.GroupID, userID.(uint), req.MsgID)
	if err != nil {
		log.Printf("MarkAsRead: service error for groupID %v, userID %v, msgID %v: %v", req.GroupID, userID, req.MsgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "marked as read",
		"data":    nil,
	})
}

// GetUnreadCount 获取未读消息数
func (h *MessageHandler) GetUnreadCount(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		log.Printf("GetUnreadCount: userID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	groupID, err := strconv.ParseUint(c.Param("groupId"), 10, 32)
	if err != nil {
		log.Printf("GetUnreadCount: invalid group id: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid group id",
		})
		return
	}

	count, err := h.messageService.GetUnreadCount(uint(groupID), userID.(uint))
	if err != nil {
		log.Printf("GetUnreadCount: service error for groupID %v, userID %v: %v", groupID, userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"unread_count": count},
	})
}

// GetLatestSequence 获取群组最新序列号
func (h *MessageHandler) GetLatestSequence(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("groupId"), 10, 32)
	if err != nil {
		log.Printf("GetLatestSequence: invalid group id: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid group id",
		})
		return
	}

	seq, err := h.messageService.GetGroupLatestSequence(uint(groupID))
	if err != nil {
		log.Printf("GetLatestSequence: service error for groupID %v: %v", groupID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"sequence_id": seq},
	})
}
