package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/internal/model"
)

// IUserRepository defines the interface for user data operations
type IUserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByID(ctx context.Context, id string) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
}

// UserRepository implements IUserRepository interface
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new IUserRepository instance
func NewUserRepository(db *gorm.DB) IUserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user in the database
func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindByID finds a user by ID
func (r *UserRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByUsername finds a user by username
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update updates an existing user
func (r *UserRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}
