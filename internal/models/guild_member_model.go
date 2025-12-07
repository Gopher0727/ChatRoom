package models

import "time"

// GuildMember 定义中间表结构，确保创建联合主键/索引
type GuildMember struct {
	GuildID       uint      `gorm:"primaryKey" json:"guild_id"`
	UserID        uint      `gorm:"primaryKey" json:"user_id"`
	Role          string    `gorm:"type:varchar(20);default:'member'" json:"role"` // owner, admin, member
	LastReadMsgID int64     `gorm:"default:0" json:"last_read_msg_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (GuildMember) TableName() string {
	return "members"
}
