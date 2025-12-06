package services

import (
	"errors"

	"github.com/Gopher0727/ChatRoom/internal/models"
	"github.com/Gopher0727/ChatRoom/internal/repositories"
	"github.com/Gopher0727/ChatRoom/pkg/utils"
)

// 定义业务错误，供 Handler 层判断状态码
var (
	ErrUserNotFound      = errors.New("用户不存在")
	ErrUserAlreadyExists = errors.New("用户已存在")
	ErrPasswordIncorrect = errors.New("密码错误")
	ErrInvalidParams     = errors.New("参数无效")
	ErrUserNotLogin      = errors.New("用户未登录")
	ErrUserAlreadyLogin  = errors.New("用户已登录")
)

type UserService struct {
	UserRepo *repositories.UserRepository
}

func NewUserService(userRepo *repositories.UserRepository) *UserService {
	return &UserService{
		UserRepo: userRepo,
	}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	UserName string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	UserName string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LogoutRequest 登出请求
type LogoutRequest struct {
	UserName string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// CancelRequest 注销请求
type CancelRequest struct {
	UserName string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterResponse 注册响应
type RegisterResponse struct {
	UserID   uint   `json:"user_id"`
	UserName string `json:"username"`
	Email    string `json:"email"`
	Status   string `json:"status"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	UserID   uint   `json:"user_id"`
	UserName string `json:"username"`
	Email    string `json:"email"`
	Status   string `json:"status"`
	Token    string `json:"token"`
	Message  string `json:"message,omitempty"`
}

// LogoutResponse 登出响应
type LogoutResponse struct {
	UserName string `json:"username"`
	Status   string `json:"status"`
}

// CancelResponse 注销响应
type CancelResponse struct {
	UserName string `json:"username"`
	Email    string `json:"email"`
}

func (s *UserService) Register(req *RegisterRequest) (*RegisterResponse, error) {
	if !utils.ValidateUserName(req.UserName) {
		return nil, errors.New("用户名格式无效")
	}
	if !utils.ValidateEmail(req.Email) {
		return nil, errors.New("邮箱格式无效")
	}
	if !utils.ValidatePassword(req.Password) {
		return nil, errors.New("密码长度至少为8位")
	}

	existsUserName, err := s.UserRepo.ExistsByUserName(req.UserName)
	if err != nil {
		return nil, err
	}
	if existsUserName {
		return nil, errors.New("用户名已存在")
	}

	existsEmail, err := s.UserRepo.ExistsByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if existsEmail {
		return nil, errors.New("邮箱已存在")
	}

	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		UserName:     req.UserName,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Status:       "offline",
	}

	if err := s.UserRepo.Create(user); err != nil {
		return nil, err
	}

	return &RegisterResponse{
		UserID:   user.ID,
		UserName: user.UserName,
		Email:    user.Email,
		Status:   "offline",
	}, nil
}

func (s *UserService) Login(req *LoginRequest) (*LoginResponse, error) {
	user, err := s.UserRepo.GetByUserName(req.UserName)
	if err != nil {
		// 为了安全，不透露是用户名不存在还是密码错误，但在 Service 层可以区分
		// Handler 层统一处理
		return nil, ErrUserNotFound
	}

	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, ErrPasswordIncorrect
	}

	token, err := utils.GenerateToken(user.ID, user.UserName, user.Email)
	if err != nil {
		return nil, err
	}

	// 如果用户已经是在线状态，依然返回成功，但可以提示
	if user.Status == "online" {
		// 这里视业务需求而定，通常重复登录是允许的，或者踢掉旧连接
		return &LoginResponse{
			Token:    token,
			UserID:   user.ID,
			UserName: user.UserName,
			Email:    user.Email,
			Status:   "online",
			Message:  "用户已登录",
		}, ErrUserAlreadyLogin
	}

	s.UserRepo.UpdateStatus(user.ID, "online")

	return &LoginResponse{
		Token:    token,
		UserID:   user.ID,
		UserName: user.UserName,
		Email:    user.Email,
		Status:   "online",
	}, nil
}

func (s *UserService) Logout(req *LogoutRequest) (*LogoutResponse, error) {
	user, err := s.UserRepo.GetByUserName(req.UserName)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if user.Status == "offline" {
		return nil, ErrUserNotLogin
	}

	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, ErrPasswordIncorrect
	}

	s.UserRepo.UpdateStatus(user.ID, "offline")

	return &LogoutResponse{
		UserName: user.UserName,
		Status:   "offline",
	}, nil
}

func (s *UserService) Cancel(req *CancelRequest) (*CancelResponse, error) {
	user, err := s.UserRepo.GetByUserName(req.UserName)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, ErrPasswordIncorrect
	}

	s.UserRepo.Delete(user.ID)

	return &CancelResponse{
		UserName: user.UserName,
		Email:    user.Email,
	}, nil
}

func (s *UserService) GetProfile(userID uint) (*models.User, error) {
	user, err := s.UserRepo.GetByID(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if user.Status != "online" {
		return nil, ErrUserNotLogin
	}
	return user, nil
}

func (s *UserService) UpdateProfile(userID uint, nickname, avatarURL string) error {
	user, err := s.UserRepo.GetByID(userID)
	if err != nil {
		return ErrUserNotFound
	}
	if user.Status != "online" {
		return ErrUserNotLogin
	}

	if nickname != "" {
		user.Nickname = nickname
	}
	if avatarURL != "" {
		user.AvatarURL = avatarURL
	}

	return s.UserRepo.Update(user)
}

func (s *UserService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	if !utils.ValidatePassword(newPassword) {
		return errors.New("新密码格式无效")
	}

	user, err := s.UserRepo.GetByID(userID)
	if err != nil {
		return ErrUserNotFound
	}

	if !utils.CheckPassword(user.PasswordHash, oldPassword) {
		return errors.New("旧密码错误")
	}

	newHash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = newHash

	return s.UserRepo.Update(user)
}

func (s *UserService) UpdateStatus(userID uint, status string) error {
	return s.UserRepo.UpdateStatus(userID, status)
}
