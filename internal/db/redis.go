package db

import (
	"context"
	"fmt"
	"log"

	redis "github.com/redis/go-redis/v9"
)

// InitRedis 初始化Redis连接
func InitRedis(host, port, password string, db int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})

	// 测试连接
	ctx, cancel := context.Background(), context.CancelFunc(func() {})
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		log.Printf("Failed to connect to Redis: %v", err)
		return nil, err
	}

	return client, nil
}
