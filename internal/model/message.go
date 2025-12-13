package model

import (
	"time"
)

// Message 消息模型
type Message struct {
	ID      string `gorm:"primaryKey;type:varchar(64)" json:"id"`
	UserID  string `gorm:"index;not null;type:varchar(64)" json:"user_id"`
	GuildID string `gorm:"index;not null;type:varchar(64)" json:"guild_id"`
	Content string `gorm:"type:text;not null" json:"content"`
	SeqID   int64  `gorm:"index;not null" json:"seq_id"`

	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (Message) TableName() string {
	return "messages"
}
