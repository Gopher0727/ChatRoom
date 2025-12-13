package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/Gopher0727/ChatRoom/internal/model"
)

type IMessageRepository interface {
	Create(ctx context.Context, message *model.Message) error
	FindByGuild(ctx context.Context, guildID string, afterSeqID int64, limit int) ([]*model.Message, error)
	FindByID(ctx context.Context, id string) (*model.Message, error)
}

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) IMessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(ctx context.Context, message *model.Message) error {
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *MessageRepository) FindByGuild(ctx context.Context, guildID string, afterSeqID int64, limit int) ([]*model.Message, error) {
	var messages []*model.Message

	query := r.db.WithContext(ctx).Where("guild_id = ?", guildID)
	if afterSeqID > 0 {
		query = query.Where("seq_id > ?", afterSeqID)
	}
	err := query.Order("seq_id ASC").Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *MessageRepository) FindByID(ctx context.Context, id string) (*model.Message, error) {
	var message model.Message
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&message).Error; err != nil {
		return nil, err
	}
	return &message, nil
}
