package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Postgres   PostgresConfig   `mapstructure:"postgres"`
	Redis      RedisConfig      `mapstructure:"redis"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	RateLimit  RateLimitConfig  `mapstructure:"ratelimit"`
	WorkerPool WorkerPoolConfig `mapstructure:"worker_pool"`
	Gateway    GatewayConfig    `mapstructure:"gateway"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type PostgresConfig struct {
	Host         string `mapstructure:"host"`
	Port         string `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         string `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

type RateLimitConfig struct {
	QPS            int64 `mapstructure:"qps"`
	Burst          int64 `mapstructure:"burst"`
	MaxConcurrency int   `mapstructure:"max_concurrency"`
}

type WorkerPoolConfig struct {
	Size      int `mapstructure:"size"`
	QueueSize int `mapstructure:"queue_size"`
}

type GatewayConfig struct {
	NodeID string         `mapstructure:"node_id"`
	Nodes  map[string]int `mapstructure:"nodes"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
	GroupID string   `mapstructure:"group_id"`
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		// 如果文件不存在，可以根据情况决定是报错还是使用默认值
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 将配置反序列化到结构体
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return &config, nil
}
