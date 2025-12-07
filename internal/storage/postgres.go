package storage

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Gopher0727/ChatRoom/internal/models"
)

// InitPostgres 初始化 PostgreSQL 连接
func InitPostgres(dsn string, maxIdleConns, maxOpenConns int) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Printf("连接数据库失败: %v", err)
		return nil, err
	}

	// 获取底层 sql.DB 对象以设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("获取 sql.DB 失败: %v", err)
		return nil, err
	}

	// 设置连接池参数
	// SetMaxIdleConns 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(maxIdleConns)
	// SetMaxOpenConns 设置打开数据库连接的最大数量
	sqlDB.SetMaxOpenConns(maxOpenConns)
	// SetConnMaxLifetime 设置了连接可复用的最大时间
	// sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移
	err = db.AutoMigrate(
		&models.User{},
		&models.Guild{},
		&models.GuildMember{}, // 添加中间表模型，确保联合主键索引被创建
		&models.Invite{},
		&models.Message{},
	)
	if err != nil {
		log.Printf("模型迁移失败: %v", err)
		return nil, err
	}
	return db, nil
}

// BuildDSN 构建PostgreSQL DSN
func BuildDSN(host, port, user, password, dbname string) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
}
