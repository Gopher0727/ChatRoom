package model

import (
	"time"
)

// User 用户模型
type User struct {
	ID           string `gorm:"primaryKey;type:varchar(64)" json:"id"`
	UserName     string `gorm:"column:username;uniqueIndex;not null;type:varchar(255)" json:"username"`
	Email        string `gorm:"uniqueIndex;not null;type:varchar(255)" json:"email"`
	PasswordHash string `gorm:"not null;type:varchar(255)" json:"-"`
	AvatarURL    string `json:"avatar_url"`
	Status       string `gorm:"default:offline" json:"status"` // online, offline
	HubID        string `gorm:"index;type:varchar(64)" json:"hub_id"`

	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}
