package services

import (
	"errors"

	"github.com/Gopher0727/ChatRoom/internal/models"
	"github.com/Gopher0727/ChatRoom/internal/repositories"
	"github.com/Gopher0727/ChatRoom/internal/utils"
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

// RegisterResponse 注册响应
type RegisterResponse struct {
	UserID   uint   `json:"user_id"`
	UserName string `json:"username"`
	Email    string `json:"email"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token    string `json:"token"`
	UserID   uint   `json:"user_id"`
	UserName string `json:"username"`
	Email    string `json:"email"`
}

func (s *UserService) Register(req *RegisterRequest) (*RegisterResponse, error) {
	if !utils.ValidateUserName(req.UserName) {
		return nil, errors.New("invalid username format")
	}
	if !utils.ValidateEmail(req.Email) {
		return nil, errors.New("invalid email format")
	}
	if !utils.ValidatePassword(req.Password) {
		return nil, errors.New("password must be at least 8 characters")
	}

	existsUserName, err := s.UserRepo.ExistsByUserName(req.UserName)
	if err != nil {
		return nil, err
	}
	if existsUserName {
		return nil, errors.New("username already exists")
	}

	existsEmail, err := s.UserRepo.ExistsByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if existsEmail {
		return nil, errors.New("email already exists")
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
	}, nil
}

func (s *UserService) Login(req *LoginRequest) (*LoginResponse, error) {
	user, err := s.UserRepo.GetByUserName(req.UserName)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		return nil, errors.New("invalid username or password")
	}

	token, err := utils.GenerateToken(user.ID, user.UserName, user.Email)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		Token:    token,
		UserID:   user.ID,
		UserName: user.UserName,
		Email:    user.Email,
	}, nil
}

func (s *UserService) GetProfile(userID uint) (*models.User, error) {
	return s.UserRepo.GetByID(userID)
}

func (s *UserService) UpdateProfile(userID uint, nickname, avatarURL string) error {
	user, err := s.UserRepo.GetByID(userID)
	if err != nil {
		return err
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
		return errors.New("invalid password")
	}

	user, err := s.UserRepo.GetByID(userID)
	if err != nil {
		return err
	}

	if !utils.CheckPassword(user.PasswordHash, oldPassword) {
		return errors.New("old password is incorrect")
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
