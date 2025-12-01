package models

import "time"

type Guild struct {
	ID uint `gorm:"primaryKey" json:"id"`

	OwnerID uint   `gorm:"not null" json:"owner_id"`
	Owner   User   `gorm:"foreignKey:OwnerID" json:"owner"`
	Topic   string `gorm:"type:varchar(255)" json:"topic"`

	Members  []User    `gorm:"many2many:members" json:"members"`
	Messages []Message `gorm:"foreignKey:GuildID" json:"messages"`

	CreatedAt time.Time `json:"created_at"`
}

func (gu *Guild) TableName() string {
	return "guilds"
}
