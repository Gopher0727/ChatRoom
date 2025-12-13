package model

import "time"

type Guild struct {
	ID         string `gorm:"primaryKey" json:"id"`
	Name       string `gorm:"not null;type:varchar(255)" json:"name"`
	OwnerID    string `gorm:"not null" json:"owner_id"`
	InviteCode string `gorm:"uniqueIndex;not null;type:varchar(32)" json:"invite_code"`

	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (gu *Guild) TableName() string {
	return "guilds"
}

type GuildMember struct {
	ID      string `gorm:"primaryKey;type:varchar(64)" json:"id"`
	GuildID string `gorm:"index;not null;type:varchar(64)" json:"guild_id"`
	UserID  string `gorm:"index;not null;type:varchar(64)" json:"user_id"`

	JoinedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"joined_at"`
}

func (GuildMember) TableName() string {
	return "guild_members"
}
