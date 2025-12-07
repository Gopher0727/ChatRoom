package storage

import (
	"context"
	"fmt"
	"log"

	redis "github.com/redis/go-redis/v9"
)

// InitRedis 初始化 Redis 连接
func InitRedis(host, port, password string, db, poolSize, minIdleConns int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", host, port),
		Password:     password,
		DB:           db,
		PoolSize:     poolSize,     // 最大连接数
		MinIdleConns: minIdleConns, // 最小空闲连接数
	})

	// 测试连接
	ctx, cancel := context.Background(), context.CancelFunc(func() {})
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		log.Printf("连接 Redis 失败: %v", err)
		return nil, err
	}

	return client, nil
}
