package configs

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		// 如果文件不存在，可以根据情况决定是报错还是使用默认值
		// TODO
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 监听配置文件变化 (热加载)
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		// 注意：这里虽然监听了变化，但如果你的程序已经用旧配置连接了数据库，
		// 你需要自己实现逻辑去重新连接，Viper 只是更新了内存中的值。
		// TODO
	})

	// 将配置反序列化到结构体
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &config, nil
}
