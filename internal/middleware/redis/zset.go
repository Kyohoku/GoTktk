package redis

import (
	"context"
	"time"

	redis "github.com/redis/go-redis/v9"
)

func (c *Client) ZincrBy(ctx context.Context, key string, member string, score float64) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.ZIncrBy(ctx, key, score, member).Err()
}

func (c *Client) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.ZAdd(ctx, key, members...).Err()
}

func (c *Client) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) { //热度分数排名
	if c == nil || c.rdb == nil {
		return nil, nil
	}
	return c.rdb.ZRevRange(ctx, key, start, stop).Result()
}

func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Expire(ctx, key, ttl).Err()
}

func (c *Client) ZUnionStore(ctx context.Context, dst string, keys []string, aggregate string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.ZUnionStore(ctx, dst, &redis.ZStore{
		Keys:      keys,
		Aggregate: aggregate,
	}).Err()
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, nil
	}
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}
