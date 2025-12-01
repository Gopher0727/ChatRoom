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
	Content    string         `gorm:"not null" json:"content"`
	MsgType    string         `gorm:"default:text" json:"msg_type"` // text, image, file, system
	SequenceID int64          `gorm:"not null" json:"sequence_id"`  // 移除唯一索引，简化逻辑，实际生产需配合 Redis 生成
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	Sender     *User          `gorm:"foreignKey:SenderID" json:"-"`
}

func (Message) TableName() string {
	return "messages"
}
