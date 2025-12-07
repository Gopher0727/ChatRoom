package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Gopher0727/ChatRoom/internal/services"
)

type UserHandler struct {
	UserService *services.UserService
}

func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{
		UserService: userService,
	}
}

func (h *UserHandler) Register(c *gin.Context) {
	req := services.RegisterRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	resp, err := h.UserService.Register(&req)
	if err != nil {
		if errors.Is(err, services.ErrUserAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "用户已存在"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *UserHandler) Login(c *gin.Context) {
	req := services.LoginRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	resp, err := h.UserService.Login(&req)
	if err != nil {
		// 如果用户已登录，Service 层会返回 token 和 ErrUserAlreadyLogin
		if errors.Is(err, services.ErrUserAlreadyLogin) {
			c.JSON(http.StatusOK, resp)
			return
		}

		if errors.Is(err, services.ErrUserNotFound) || errors.Is(err, services.ErrPasswordIncorrect) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) Logout(c *gin.Context) {
	req := services.LogoutRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	resp, err := h.UserService.Logout(&req)
	if err != nil {
		if errors.Is(err, services.ErrUserNotLogin) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "用户未登录"})
		} else if errors.Is(err, services.ErrPasswordIncorrect) || errors.Is(err, services.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) Cancel(c *gin.Context) {
	req := services.CancelRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	resp, err := h.UserService.Cancel(&req)
	if err != nil {
		if errors.Is(err, services.ErrPasswordIncorrect) || errors.Is(err, services.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	user, err := h.UserService.GetProfile(userID.(uint))
	if err != nil {
		if errors.Is(err, services.ErrUserNotLogin) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未登录"})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		}
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	var req struct {
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	err := h.UserService.UpdateProfile(userID.(uint), req.Nickname, req.AvatarURL)
	if err != nil {
		if errors.Is(err, services.ErrUserNotLogin) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户未登录"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "个人信息更新成功"})
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	err := h.UserService.ChangePassword(userID.(uint), req.OldPassword, req.NewPassword)
	if err != nil {
		if errors.Is(err, services.ErrPasswordIncorrect) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "旧密码错误"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}
