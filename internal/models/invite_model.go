package models

import (
	"time"

	"gorm.io/gorm"
)

// Invite 邀请码模型
type Invite struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	GuildID   uint           `gorm:"not null;index" json:"guild_id"`
	Code      string         `gorm:"uniqueIndex;size:10" json:"code"`
	CreatorID uint           `gorm:"not null" json:"creator_id"`
	ExpiresAt time.Time      `json:"expires_at"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Invite) TableName() string {
	return "invites"
}
