package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/services"
	"github.com/Gopher0727/ChatRoom/pkg/mq"
	"github.com/Gopher0727/ChatRoom/pkg/ws"
)

type GuildHandler struct {
	GuildService  *services.GuildService
	Hub           *ws.Hub
	KafkaProducer *mq.KafkaProducer
}

func NewGuildHandler(guildService *services.GuildService, hub *ws.Hub, kafkaProducer *mq.KafkaProducer) *GuildHandler {
	return &GuildHandler{
		GuildService:  guildService,
		Hub:           hub,
		KafkaProducer: kafkaProducer,
	}
}

// CreateGuild 从 Context 获取当前登录用户 ID，解析请求体中的 Topic，创建 Guild
func (h *GuildHandler) CreateGuild(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	var req services.CreateGuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	resp, err := h.GuildService.CreateGuild(userID.(uint), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// CreateInvite 解析 URL 参数中的 guild_id，为该 Guild 生成邀请码
func (h *GuildHandler) CreateInvite(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	guildIDStr := c.Param("guild_id")
	guildID, err := strconv.ParseUint(guildIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的服务器ID"})
		return
	}

	resp, err := h.GuildService.CreateInvite(userID.(uint), uint(guildID))
	if err != nil {
		if errors.Is(err, services.ErrUserNotMember) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// JoinGuild 解析请求体中的邀请码，验证邀请码并添加成员
func (h *GuildHandler) JoinGuild(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	var req struct {
		InviteCode string `json:"invite_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	guild, err := h.GuildService.JoinGuild(userID.(uint), req.InviteCode)
	if err != nil {
		if errors.Is(err, services.ErrInviteNotFound) || errors.Is(err, services.ErrInviteExpired) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else if errors.Is(err, services.ErrAlreadyMember) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "成功加入服务器",
		"guild":   guild,
	})
}

// SendMessage 解析 URL 参数 guild_id 和请求体内容，发送消息
func (h *GuildHandler) SendMessage(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	guildIDStr := c.Param("guild_id")
	guildID, err := strconv.ParseUint(guildIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的服务器ID"})
		return
	}

	var req services.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	// 如果 Kafka Producer 可用，优先走 Kafka 异步处理
	if h.KafkaProducer != nil {
		kafkaMsg := struct {
			UserID  uint                         `json:"user_id"`
			GuildID uint                         `json:"guild_id"`
			Content *services.SendMessageRequest `json:"content"`
		}{
			UserID:  userID.(uint),
			GuildID: uint(guildID),
			Content: &req,
		}

		// 使用 GuildID 作为 Key，保证有序性
		if err := h.KafkaProducer.SendMessage(strconv.Itoa(int(guildID)), kafkaMsg); err != nil {
			// 发送失败，降级为同步处理
			goto SyncProcess
		}

		// 异步处理成功，返回 202 Accepted
		c.JSON(http.StatusAccepted, gin.H{"message": "消息已投递"})
		return
	}

SyncProcess:
	resp, err := h.GuildService.SendMessage(userID.(uint), uint(guildID), &req)
	if err != nil {
		if errors.Is(err, services.ErrUserNotMember) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// 广播消息给 WebSocket 客户端
	h.Hub.BroadcastToGuild(uint(guildID), resp)

	c.JSON(http.StatusCreated, resp)
}

// GetMessages 解析 URL 参数 guild_id 和分页参数 limit/offset，获取消息列表
func (h *GuildHandler) GetMessages(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	guildIDStr := c.Param("guild_id")
	guildID, err := strconv.ParseUint(guildIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的服务器ID"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	// 检查是否有 after_seq 参数 (增量同步)
	afterSeqStr := c.Query("after_seq")
	if afterSeqStr != "" {
		afterSeq, err := strconv.ParseInt(afterSeqStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 sequence_id"})
			return
		}

		resp, err := h.GuildService.GetMessagesAfterSequence(userID.(uint), uint(guildID), afterSeq, limit)
		if err != nil {
			if errors.Is(err, services.ErrUserNotMember) {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	// 原有的分页逻辑 (全量/历史同步)
	offsetStr := c.DefaultQuery("offset", "0")
	offset, _ := strconv.Atoi(offsetStr)

	resp, err := h.GuildService.GetMessages(userID.(uint), uint(guildID), limit, offset)
	if err != nil {
		if errors.Is(err, services.ErrUserNotMember) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetMyGuilds 获取当前用户加入的所有 Guild
func (h *GuildHandler) GetMyGuilds(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	guilds, err := h.GuildService.GetUserGuildsWithUnread(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, guilds)
}

// AckMessage 确认消息已读
func (h *GuildHandler) AckMessage(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	guildIDStr := c.Param("guild_id")
	guildID, err := strconv.ParseUint(guildIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的服务器ID"})
		return
	}

	var req struct {
		SequenceID int64 `json:"sequence_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	if err := h.GuildService.AckMessage(userID.(uint), uint(guildID), req.SequenceID); err != nil {
		if errors.Is(err, services.ErrUserNotMember) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "消息已确认"})
}
