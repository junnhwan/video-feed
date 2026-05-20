package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func (c *Client) ZIncrBy(ctx context.Context, key string, member string, score float64) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.ZIncrBy(ctx, key, score, member).Err()
}

func (c *Client) ZAdd(ctx context.Context, key string, members ...goredis.Z) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.ZAdd(ctx, key, members...).Err()
}

func (c *Client) ZRangeWithScores(ctx context.Context, key string, start int64, stop int64) ([]goredis.Z, error) {
	if c == nil || c.rdb == nil {
		return nil, errors.New("redis client not initialized")
	}
	return c.rdb.ZRangeWithScores(ctx, key, start, stop).Result()
}

func (c *Client) ZRevRange(ctx context.Context, key string, start int64, stop int64) ([]string, error) {
	if c == nil || c.rdb == nil {
		return nil, nil
	}
	return c.rdb.ZRevRange(ctx, key, start, stop).Result()
}

func (c *Client) ZRevRangeByScore(ctx context.Context, key string, max string, min string, offset int64, count int64) ([]string, error) {
	if c == nil || c.rdb == nil {
		return nil, nil
	}
	return c.rdb.ZRevRangeByScore(ctx, key, &goredis.ZRangeBy{
		Max:    max,
		Min:    min,
		Offset: offset,
		Count:  count,
	}).Result()
}

func (c *Client) ZUnionStore(ctx context.Context, dest string, keys []string, aggregate string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.ZUnionStore(ctx, dest, &goredis.ZStore{
		Keys:      keys,
		Aggregate: aggregate,
	}).Err()
}

func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Expire(ctx, key, ttl).Err()
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, nil
	}
	count, err := c.rdb.Exists(ctx, key).Result()
	return count > 0, err
}
