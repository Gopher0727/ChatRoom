package models

import (
	"time"

	"gorm.io/gorm"
)

// Message 消息模型
type Message struct {
	ID         int64          `gorm:"primaryKey" json:"id"`
	GroupID    uint           `gorm:"not null;index" json:"group_id"`
	SenderID   uint           `gorm:"not null;index" json:"sender_id"`
	Content    string         `gorm:"not null" json:"content"`
	MsgType    string         `gorm:"default:text" json:"msg_type"` // text, image, file, system
	SequenceID int64          `gorm:"not null;uniqueIndex:idx_group_seq" json:"sequence_id"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Group  *Group `gorm:"foreignKey:GroupID" json:"-"`
	Sender *User  `gorm:"foreignKey:SenderID" json:"-"`
}

func (Message) TableName() string {
	return "messages"
}
