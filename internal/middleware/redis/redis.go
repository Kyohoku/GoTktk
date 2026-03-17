package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"gotik/internal/config"
	"strconv"
	"time"

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

func randToken(n int) (string, error) { //用  token作为锁的value
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (token string, ok bool, err error) {
	if c == nil || c.rdb == nil {
		return "", false, nil
	}
	token, err = randToken(16)
	if err != nil {
		return "", false, err
	}
	ok, err = c.rdb.SetNX(ctx, key, token, ttl).Result() //ttl 防止死锁
	return token, ok, err
}

// lua 脚本，用于解锁
var unlockScript = redis.NewScript(`  
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
else
  return 0
end
`)

func (c *Client) Unlock(ctx context.Context, key string, token string) error {
	if c == nil || c.rdb == nil { //降级
		return nil
	}
	_, err := unlockScript.Run(ctx, c.rdb, []string{key}, token).Result()
	return err
}
