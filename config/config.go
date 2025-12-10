package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Postgres   PostgresConfig   `mapstructure:"postgres"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	Snowflake  SnowflakeConfig  `mapstructure:"snowflake"`
	RateLimit  RateLimitConfig  `mapstructure:"ratelimit"`
	Websocket  WebsocketConfig  `mapstructure:"websocket"`
	GRPC       GRPCConfig       `mapstructure:"grpc"`
	WorkerPool WorkerPoolConfig `mapstructure:"worker_pool"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type PostgresConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
}

type KafkaConfig struct {
	Brokers       []string       `mapstructure:"brokers"`
	Topics        TopicsConfig   `mapstructure:"topics"`
	ConsumerGroup string         `mapstructure:"consumer_group"`
	Producer      ProducerConfig `mapstructure:"producer"`
	Consumer      ConsumerConfig `mapstructure:"consumer"`
}

type TopicsConfig struct {
	Message string `mapstructure:"message"`
	DLQ     string `mapstructure:"dlq"`
}

type ProducerConfig struct {
	MaxRetries     int `mapstructure:"max_retries"`
	RetryBackoffMs int `mapstructure:"retry_backoff_ms"`
}

type ConsumerConfig struct {
	MaxRetries     int `mapstructure:"max_retries"`
	RetryBackoffMs int `mapstructure:"retry_backoff_ms"`
}

type JWTConfig struct {
	Secret       string `mapstructure:"secret"`
	ExpireHours  int    `mapstructure:"expire_hours"`
	RefreshHours int    `mapstructure:"refresh_hours"`
}

type SnowflakeConfig struct {
	WorkerIDBits uint8 `mapstructure:"worker_id_bits"`
	SequenceBits uint8 `mapstructure:"sequence_bits"`
	DatacenterID int64 `mapstructure:"datacenter_id"`
	WorkerID     int64 `mapstructure:"worker_id"`
}

type RateLimitConfig struct {
	RegisterPerMinute int `mapstructure:"register_per_minute"`
	LoginPerMinute    int `mapstructure:"login_per_minute"`
	MessagePerMinute  int `mapstructure:"message_per_minute"`
	APIPerMinute      int `mapstructure:"api_per_minute"`
}

type WebsocketConfig struct {
	ReadBufferSize    int `mapstructure:"read_buffer_size"`
	WriteBufferSize   int `mapstructure:"write_buffer_size"`
	HeartbeatInterval int `mapstructure:"heartbeat_interval"`
	ConnectionTimeout int `mapstructure:"connection_timeout"`
}

type GRPCConfig struct {
	Port              int `mapstructure:"port"`
	MaxConnectionIdle int `mapstructure:"max_connection_idle"`
	MaxConnectionAge  int `mapstructure:"max_connection_age"`
}

type WorkerPoolConfig struct {
	Size      int `mapstructure:"size"`
	QueueSize int `mapstructure:"queue_size"`
}

type LoggingConfig struct {
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"`
	Output   string `mapstructure:"output"`
	FilePath string `mapstructure:"file_path"`
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 将配置反序列化到结构体
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return &config, nil
}
