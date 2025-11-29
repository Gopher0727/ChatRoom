package models

import (
	"time"

	"gorm.io/gorm"
)

// Group 群组模型
type Group struct {
	ID uint `gorm:"primaryKey" json:"id"`

	Name        string        `gorm:"not null" json:"name"`
	Description string        `json:"description"`
	AvatarURL   string        `json:"avatar_url"`
	OwnerID     uint          `gorm:"not null" json:"owner_id"`
	InviteCode  string        `gorm:"uniqueIndex;not null" json:"invite_code"`
	MaxMembers  int           `gorm:"default:10000" json:"max_members"`
	MemberCount int           `gorm:"default:1" json:"member_count"`
	Status      string        `gorm:"default:active" json:"status"` // active, archived
	Owner       *User         `gorm:"foreignKey:OwnerID" json:"-"`
	Members     []GroupMember `gorm:"foreignKey:GroupID" json:"-"`
	Messages    []Message     `gorm:"foreignKey:GroupID" json:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Group) TableName() string {
	return "groups"
}
