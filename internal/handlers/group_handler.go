package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/services"
)

// GroupHandler 群组处理器
type GroupHandler struct {
	groupService *services.GroupService
}

// NewGroupHandler 创建群组处理器实例
func NewGroupHandler(groupService *services.GroupService) *GroupHandler {
	return &GroupHandler{
		groupService: groupService,
	}
}

// CreateGroup 创建群组
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		log.Printf("CreateGroup: userID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	var req services.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("CreateGroup: JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	group, err := h.groupService.CreateGroup(userID.(uint), &req)
	if err != nil {
		log.Printf("CreateGroup: service error for userID %v: %v", userID, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    group,
	})
}

// GetGroupByInviteCode 通过邀请码获取群组
func (h *GroupHandler) GetGroupByInviteCode(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		log.Printf("GetGroupByInviteCode: missing invite code")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "missing invite code",
		})
		return
	}

	group, err := h.groupService.GetGroupByInviteCode(code)
	if err != nil {
		log.Printf("GetGroupByInviteCode: error for code %s: %v", code, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    group,
	})
}

// JoinGroup 加入群组
func (h *GroupHandler) JoinGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		log.Printf("JoinGroup: userID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	type JoinGroupRequest struct {
		GroupID uint `json:"group_id" binding:"required"`
	}

	var req JoinGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("JoinGroup: JSON binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	err := h.groupService.JoinGroup(userID.(uint), req.GroupID)
	if err != nil {
		log.Printf("JoinGroup: service error for userID %v, groupID %v: %v", userID, req.GroupID, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "join group success",
		"data":    nil,
	})
}

// LeaveGroup 离开群组
func (h *GroupHandler) LeaveGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		log.Printf("LeaveGroup: userID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		log.Printf("LeaveGroup: invalid group id: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid group id",
		})
		return
	}

	err = h.groupService.LeaveGroup(userID.(uint), uint(groupID))
	if err != nil {
		log.Printf("LeaveGroup: service error for userID %v, groupID %v: %v", userID, groupID, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "leave group success",
		"data":    nil,
	})
}

// GetUserGroups 获取用户所在的群组列表
func (h *GroupHandler) GetUserGroups(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		log.Printf("GetUserGroups: userID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	page := 1
	pageSize := 20

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

	groups, total, err := h.groupService.GetUserGroups(userID.(uint), page, pageSize)
	if err != nil {
		log.Printf("GetUserGroups: service error for userID %v: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"groups":    groups,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetGroupMembers 获取群组成员列表
func (h *GroupHandler) GetGroupMembers(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		log.Printf("GetGroupMembers: invalid group id: %v", err)
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

	members, total, err := h.groupService.GetGroupMembers(uint(groupID), page, pageSize)
	if err != nil {
		log.Printf("GetGroupMembers: service error for groupID %v: %v", groupID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"members":   members,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetGroupDetail 获取群组详情
func (h *GroupHandler) GetGroupDetail(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		log.Printf("GetGroupDetail: invalid group id: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid group id",
		})
		return
	}

	group, err := h.groupService.GetGroupDetail(uint(groupID))
	if err != nil {
		log.Printf("GetGroupDetail: service error for groupID %v: %v", groupID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    group,
	})
}
