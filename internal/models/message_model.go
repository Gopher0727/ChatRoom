package models

import (
	"time"

	"gorm.io/gorm"
)

// Message 消息模型
type Message struct {
	ID         int64          `gorm:"primaryKey" json:"id"`
	GuildID    uint           `gorm:"not null;index" json:"guild_id"` // GroupID -> GuildID
	SenderID   uint           `gorm:"not null;index" json:"sender_id"`
	Content    string         `gorm:"type:text;not null" json:"content"`
	MsgType    string         `gorm:"type:varchar(20);default:'text'" json:"msg_type"`
	SequenceID int64          `gorm:"index:idx_guild_seq,priority:2;default:0" json:"sequence_id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	Sender     *User          `gorm:"foreignKey:SenderID" json:"-"`
}

func (Message) TableName() string {
	return "messages"
}
