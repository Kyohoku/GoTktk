package redis

import (
	"context"
	"gotik/internal/config"
	"strconv"

	redis "github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewFromEnv(cfg *config.RedisConfig) (*Client, error) { //初始化 redis客户端
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func (c *Client) Ping(ctx context.Context) error { //ping test
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Ping(ctx).Err()
}

func IsMiss(err error) bool { //redis 未命中
	return err == redis.Nil
}
