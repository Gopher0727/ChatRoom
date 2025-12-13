package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/internal/model"
	"github.com/Gopher0727/ChatRoom/internal/repository"
)

var (
	ErrGuildNotFound     = errors.New("guild not found")
	ErrInvalidInviteCode = errors.New("invalid invite code")
	ErrAlreadyMember     = errors.New("user is already a member of this guild")
	ErrNotMember         = errors.New("user is not a member of this guild")
)

// CreateGuildRequest represents a request to create a new guild
type CreateGuildRequest struct {
	Name string `json:"name" binding:"required,min=1,max=50"`
}

// IGuildService defines the interface for guild management operations
type IGuildService interface {
	CreateGuild(ctx context.Context, userID string, name string) (*model.Guild, error)
	JoinGuild(ctx context.Context, userID string, inviteCode string) error
	GetUserGuilds(ctx context.Context, userID string) ([]*model.Guild, error)
	GetGuildMembers(ctx context.Context, guildID string) ([]*model.User, error)
	IsMember(ctx context.Context, userID string, guildID string) (bool, error)
}

// GuildService implements the IGuildService interface
type GuildService struct {
	guildRepo repository.IGuildRepository
	userRepo  repository.IUserRepository
}

// NewGuildService creates a new IGuildService instance
func NewGuildService(guildRepo repository.IGuildRepository, userRepo repository.IUserRepository) IGuildService {
	return &GuildService{
		guildRepo: guildRepo,
		userRepo:  userRepo,
	}
}

// CreateGuild creates a new guild with a unique invite code
// The creator becomes the owner and is automatically added as a member
func (s *GuildService) CreateGuild(ctx context.Context, userID string, name string) (*model.Guild, error) {
	// Verify user exists
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Generate unique invite code
	inviteCode, err := s.generateUniqueInviteCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate invite code: %w", err)
	}

	// Create guild
	guild := &model.Guild{
		ID:         uuid.New().String(),
		Name:       name,
		OwnerID:    user.ID,
		InviteCode: inviteCode,
	}

	// Save guild to database
	if err := s.guildRepo.Create(ctx, guild); err != nil {
		return nil, fmt.Errorf("failed to create guild: %w", err)
	}

	// Add creator as first member
	if err := s.guildRepo.AddMember(ctx, guild.ID, userID); err != nil {
		return nil, fmt.Errorf("failed to add creator as member: %w", err)
	}

	return guild, nil
}

// JoinGuild allows a user to join a guild using an invite code
func (s *GuildService) JoinGuild(ctx context.Context, userID string, inviteCode string) error {
	// Verify user exists
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return fmt.Errorf("failed to find user: %w", err)
	}

	// Find guild by invite code
	guild, err := s.guildRepo.FindByInviteCode(ctx, inviteCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidInviteCode
		}
		return fmt.Errorf("failed to find guild: %w", err)
	}

	// Check if user is already a member
	isMember, err := s.IsMember(ctx, user.ID, guild.ID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if isMember {
		return ErrAlreadyMember
	}

	// Add user as member
	if err := s.guildRepo.AddMember(ctx, guild.ID, user.ID); err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	return nil
}

// GetUserGuilds retrieves all guilds that a user is a member of
func (s *GuildService) GetUserGuilds(ctx context.Context, userID string) ([]*model.Guild, error) {
	// Verify user exists
	_, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Get all guilds for user
	guilds, err := s.guildRepo.GetMemberGuilds(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user guilds: %w", err)
	}

	return guilds, nil
}

// GetGuildMembers retrieves all members of a guild
func (s *GuildService) GetGuildMembers(ctx context.Context, guildID string) ([]*model.User, error) {
	// Verify guild exists
	_, err := s.guildRepo.FindByID(ctx, guildID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGuildNotFound
		}
		return nil, fmt.Errorf("failed to find guild: %w", err)
	}

	// Get all members
	members, err := s.guildRepo.GetGuildMembers(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild members: %w", err)
	}

	return members, nil
}

// IsMember checks if a user is a member of a guild
// This is used for permission verification
func (s *GuildService) IsMember(ctx context.Context, userID string, guildID string) (bool, error) {
	// Get all guilds for the user
	guilds, err := s.guildRepo.GetMemberGuilds(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user guilds: %w", err)
	}

	// Check if the guild is in the user's guild list
	for _, guild := range guilds {
		if guild.ID == guildID {
			return true, nil
		}
	}

	return false, nil
}

// generateUniqueInviteCode generates a unique invite code for a guild
// It ensures uniqueness by checking against existing codes
func (s *GuildService) generateUniqueInviteCode(ctx context.Context) (string, error) {
	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		// Generate random 8-character code
		code := generateInviteCode()

		// Check if code already exists
		_, err := s.guildRepo.FindByInviteCode(ctx, code)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Code is unique
				return code, nil
			}
			return "", fmt.Errorf("failed to check invite code: %w", err)
		}
		// Code exists, try again
	}

	return "", errors.New("failed to generate unique invite code after maximum attempts")
}

// generateInviteCode generates a random 8-character alphanumeric invite code
func generateInviteCode() string {
	bytes := make([]byte, 4) // 4 bytes = 8 hex characters
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to UUID-based generation if crypto/rand fails
		return uuid.New().String()[:8]
	}
	return hex.EncodeToString(bytes)
}
