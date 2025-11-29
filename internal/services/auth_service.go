package services

import (
	"errors"

	"github.com/Gopher0727/ChatRoom/internal/models"
	"github.com/Gopher0727/ChatRoom/internal/repositories"
	"github.com/Gopher0727/ChatRoom/pkg/utils"
)

// AuthService 认证服务
type AuthService struct {
	userRepo *repositories.UserRepository
}

// NewAuthService 创建认证服务实例
func NewAuthService(userRepo *repositories.UserRepository) *AuthService {
	return &AuthService{
		userRepo: userRepo,
	}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	Token   string   `json:"token"`
	User    *UserDTO `json:"user"`
	Message string   `json:"message"`
}

// UserDTO 用户数据传输对象
type UserDTO struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	Status   string `json:"status"`
}

// Register 注册用户
func (s *AuthService) Register(req *RegisterRequest) (*AuthResponse, error) {
	// 验证输入
	if !utils.ValidateUsername(req.Username) {
		return nil, errors.New("invalid username format")
	}
	if !utils.ValidateEmail(req.Email) {
		return nil, errors.New("invalid email format")
	}
	if !utils.ValidatePassword(req.Password) {
		return nil, errors.New("password too short")
	}

	// 检查用户名和邮箱是否已存在
	if _, err := s.userRepo.GetByUsername(req.Username); err == nil {
		return nil, errors.New("username already exists")
	}
	if _, err := s.userRepo.GetByEmail(req.Email); err == nil {
		return nil, errors.New("email already exists")
	}

	// 密码哈希
	hashPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// 创建用户
	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashPassword,
		Nickname:     req.Username,
		Status:       "offline",
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	// 生成token
	token, err := utils.GenerateToken(user.ID, user.Username, user.Email)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token: token,
		User: &UserDTO{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Nickname: user.Nickname,
			Status:   user.Status,
		},
		Message: "register success",
	}, nil
}

// Login 登录用户
func (s *AuthService) Login(req *LoginRequest) (*AuthResponse, error) {
	// 根据用户名获取用户
	user, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		return nil, errors.New("username or password incorrect")
	}

	// 验证密码
	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, errors.New("username or password incorrect")
	}

	// 更新用户状态为在线
	user.Status = "online"
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	// 生成token
	token, err := utils.GenerateToken(user.ID, user.Username, user.Email)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token: token,
		User: &UserDTO{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Nickname: user.Nickname,
			Status:   user.Status,
		},
		Message: "login success",
	}, nil
}

// Logout 登出用户
func (s *AuthService) Logout(userID uint) error {
	return s.userRepo.UpdateStatus(userID, "offline")
}
