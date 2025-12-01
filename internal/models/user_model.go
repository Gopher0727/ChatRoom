package models

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID uint `gorm:"primaryKey" json:"id"`

	UserName     string `gorm:"column:username;uniqueIndex;not null" json:"username"`
	Email        string `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string `gorm:"not null" json:"-"`
	Nickname     string `json:"nickname"`
	AvatarURL    string `json:"avatar_url"`
	Status       string `gorm:"default:offline" json:"status"` // online, offline

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	// OwnedGroups  []Group       `gorm:"foreignKey:OwnerID" json:"-"`
	// GroupMembers []GroupMember `gorm:"foreignKey:UserID" json:"-"`
	// Messages     []Message     `gorm:"foreignKey:SenderID" json:"-"`
}

func (User) TableName() string {
	return "users"
}
