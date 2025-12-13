package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/internal/model"
	"github.com/Gopher0727/ChatRoom/internal/repository"
	"github.com/Gopher0727/ChatRoom/middleware/jwt"
)

var (
	ErrUserAlreadyExists  = errors.New("username already exists")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidToken       = errors.New("invalid token")
)

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20"`
	Password string `json:"password" binding:"required,min=8,max=64"`
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the response after successful login
type LoginResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

// IAuthService defines the interface for authentication operations
type IAuthService interface {
	Register(ctx context.Context, req *RegisterRequest) (*model.User, error)
	Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
	ValidateToken(ctx context.Context, token string) (*model.User, error)
}

// AuthService implements the IAuthService interface
type AuthService struct {
	userRepo     repository.IUserRepository
	tokenManager *jwt.TokenManager
}

// NewAuthService creates a new IAuthService instance
func NewAuthService(userRepo repository.IUserRepository, tokenManager *jwt.TokenManager) IAuthService {
	return &AuthService{
		userRepo:     userRepo,
		tokenManager: tokenManager,
	}
}

// Register registers a new user with the provided credentials
// It validates the input, hashes the password, assigns a Hub, and stores the user
func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*model.User, error) {
	// Check if username already exists
	existingUser, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	// Hash the password using bcrypt
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create new user with auto-generated ID and Hub assignment
	user := &model.User{
		ID:           uuid.New().String(),
		UserName:     req.Username,
		PasswordHash: hashedPassword,
		HubID:        assignHub(), // Automatically bind user to a Hub
	}

	// Save user to database
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// Login authenticates a user and returns a JWT token
// It verifies credentials and generates an authentication token
func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// Find user by username
	user, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Verify password
	if err := verifyPassword(user.PasswordHash, req.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := s.tokenManager.GenerateToken(user.ID, user.UserName, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginResponse{
		Token: token,
		User:  user,
	}, nil
}

// ValidateToken validates a JWT token and returns the associated user
func (s *AuthService) ValidateToken(ctx context.Context, token string) (*model.User, error) {
	// Validate the token and extract claims
	claims, err := s.tokenManager.ParseToken(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Retrieve user from database
	user, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	return user, nil
}

// hashPassword hashes a plain text password using bcrypt with cost 12
func hashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// verifyPassword compares a hashed password with a plain text password
func verifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// assignHub assigns a user to a Hub
// In a real implementation, this would use load balancing logic
// For now, it returns a default Hub ID
func assignHub() string {
	// TODO: Implement proper Hub assignment logic with load balancing
	// For now, assign to a default Hub
	return "hub_001"
}
