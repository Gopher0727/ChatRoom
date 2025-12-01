package db

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Gopher0727/ChatRoom/internal/models"
)

// InitPostgres 初始化 PostgreSQL 连接
func InitPostgres(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return nil, err
	}

	// 自动迁移
	err = db.AutoMigrate(
		&models.User{},
		&models.Guild{},
		&models.Invite{},
		&models.Message{},
	)
	if err != nil {
		log.Printf("Failed to migrate models: %v", err)
		return nil, err
	}
	return db, nil
}

// BuildDSN 构建PostgreSQL DSN
func BuildDSN(host, port, user, password, dbname string) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
}
