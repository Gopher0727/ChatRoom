package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/internal/model"
)

// IGuildRepository defines the interface for guild data operations
type IGuildRepository interface {
	Create(ctx context.Context, guild *model.Guild) error
	FindByID(ctx context.Context, id string) (*model.Guild, error)
	FindByInviteCode(ctx context.Context, code string) (*model.Guild, error)
	AddMember(ctx context.Context, guildID, userID string) error
	GetMemberGuilds(ctx context.Context, userID string) ([]*model.Guild, error)
	GetGuildMembers(ctx context.Context, guildID string) ([]*model.User, error)
	GetMembers(ctx context.Context, guildID string) ([]*model.GuildMember, error)
	IsMember(ctx context.Context, guildID, userID string) (bool, error)
}

// GuildRepository implements IGuildRepository interface
type GuildRepository struct {
	db *gorm.DB
}

// NewGuildRepository creates a new IGuildRepository instance
func NewGuildRepository(db *gorm.DB) IGuildRepository {
	return &GuildRepository{db: db}
}

// Create creates a new guild in the database
func (r *GuildRepository) Create(ctx context.Context, guild *model.Guild) error {
	return r.db.WithContext(ctx).Create(guild).Error
}

// FindByID finds a guild by ID
func (r *GuildRepository) FindByID(ctx context.Context, id string) (*model.Guild, error) {
	var guild model.Guild
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&guild).Error
	if err != nil {
		return nil, err
	}
	return &guild, nil
}

// FindByInviteCode finds a guild by invite code
func (r *GuildRepository) FindByInviteCode(ctx context.Context, code string) (*model.Guild, error) {
	var guild model.Guild
	err := r.db.WithContext(ctx).Where("invite_code = ?", code).First(&guild).Error
	if err != nil {
		return nil, err
	}
	return &guild, nil
}

// AddMember adds a user to a guild
func (r *GuildRepository) AddMember(ctx context.Context, guildID, userID string) error {
	member := &model.GuildMember{
		ID:      generateID(),
		GuildID: guildID,
		UserID:  userID,
	}
	return r.db.WithContext(ctx).Create(member).Error
}

// GetMemberGuilds retrieves all guilds that a user is a member of
func (r *GuildRepository) GetMemberGuilds(ctx context.Context, userID string) ([]*model.Guild, error) {
	var guilds []*model.Guild
	err := r.db.WithContext(ctx).
		Table("guilds").
		Joins("JOIN guild_members ON guilds.id = guild_members.guild_id").
		Where("guild_members.user_id = ?", userID).
		Find(&guilds).Error
	if err != nil {
		return nil, err
	}
	return guilds, nil
}

// GetGuildMembers retrieves all users that are members of a guild
func (r *GuildRepository) GetGuildMembers(ctx context.Context, guildID string) ([]*model.User, error) {
	var users []*model.User
	err := r.db.WithContext(ctx).
		Table("users").
		Joins("JOIN guild_members ON users.id = guild_members.user_id").
		Where("guild_members.guild_id = ?", guildID).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// GetMembers retrieves all guild members
func (r *GuildRepository) GetMembers(ctx context.Context, guildID string) ([]*model.GuildMember, error) {
	var members []*model.GuildMember
	err := r.db.WithContext(ctx).
		Where("guild_id = ?", guildID).
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// IsMember checks if a user is a member of a guild
func (r *GuildRepository) IsMember(ctx context.Context, guildID, userID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.GuildMember{}).
		Where("guild_id = ? AND user_id = ?", guildID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// generateID is a placeholder for ID generation (will be replaced with Snowflake later)
func generateID() string {
	// TODO: Using UUID for now to ensure uniqueness
	return "temp_" + uuid.New().String()
}
