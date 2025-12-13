package redis

import (
	"context"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"

	"github.com/Gopher0727/ChatRoom/config"
)

type RedisClient interface {
	Close() error
	GetClient() *redis.Client
	Ping(ctx context.Context) error
	GenerateSeqID(ctx context.Context, guildID string) (int64, error)
	SetUserOnline(ctx context.Context, userID string, ttl time.Duration) error
	IsUserOnline(ctx context.Context, userID string) (bool, error)
	RemoveUserOnline(ctx context.Context, userID string) error
	Publish(ctx context.Context, channel string, message any) error
	Subscribe(ctx context.Context, channels ...string) (*redis.PubSub, error)
	PSubscribe(ctx context.Context, patterns ...string) (*redis.PubSub, error)
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
}

type Client struct {
	client *redis.Client
	config *config.RedisConfig
}

func NewClient(cfg *config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Client{
		client: rdb,
		config: cfg,
	}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) GetClient() *redis.Client {
	return c.client
}

func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Client) GenerateSeqID(ctx context.Context, guildID string) (int64, error) {
	key := fmt.Sprintf("guild:%s:seq_id", guildID)
	result, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to generate seq id for guild %s: %w", guildID, err)
	}
	return result, nil
}

func (c *Client) SetUserOnline(ctx context.Context, userID string, ttl time.Duration) error {
	key := fmt.Sprintf("user:%s:online", userID)
	err := c.client.Set(ctx, key, "1", ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set user %s online: %w", userID, err)
	}
	return nil
}

func (c *Client) IsUserOnline(ctx context.Context, userID string) (bool, error) {
	key := fmt.Sprintf("user:%s:online", userID)
	result, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if user %s is online: %w", userID, err)
	}
	return result > 0, nil
}

func (c *Client) RemoveUserOnline(ctx context.Context, userID string) error {
	key := fmt.Sprintf("user:%s:online", userID)
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to remove user %s online status: %w", userID, err)
	}
	return nil
}

func (c *Client) Publish(ctx context.Context, channel string, message any) error {
	err := c.client.Publish(ctx, channel, message).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to channel %s: %w", channel, err)
	}
	return nil
}

func (c *Client) Subscribe(ctx context.Context, channels ...string) (*redis.PubSub, error) {
	pubsub := c.client.Subscribe(ctx, channels...)
	// Wait for confirmation that subscription is created
	_, err := pubsub.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to channels: %w", err)
	}
	return pubsub, nil
}

func (c *Client) PSubscribe(ctx context.Context, patterns ...string) (*redis.PubSub, error) {
	pubsub := c.client.PSubscribe(ctx, patterns...)
	// Wait for confirmation that subscription is created
	_, err := pubsub.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to psubscribe to patterns: %w", err)
	}
	return pubsub, nil
}

func (c *Client) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Exists(ctx, keys...).Result()
}
