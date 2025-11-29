package models

import (
	"time"

	"gorm.io/gorm"
)

// GroupMember 群组成员模型
type GroupMember struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	GroupID       uint           `gorm:"not null;uniqueIndex:idx_group_user" json:"group_id"`
	UserID        uint           `gorm:"not null;uniqueIndex:idx_group_user" json:"user_id"`
	Role          string         `gorm:"default:member" json:"role"` // admin, member
	JoinedAt      time.Time      `json:"joined_at"`
	LastReadMsgID int64          `gorm:"default:0" json:"last_read_msg_id"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	Group *Group `gorm:"foreignKey:GroupID" json:"-"`
	User  *User  `gorm:"foreignKey:UserID" json:"-"`
}

func (GroupMember) TableName() string {
	return "group_members"
}
