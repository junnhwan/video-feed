package redis

import "context"

func (c *Client) SAdd(ctx context.Context, key string, members ...any) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	return c.rdb.SAdd(ctx, key, members...).Result()
}

func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	if c == nil || c.rdb == nil {
		return nil, nil
	}
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *Client) SRem(ctx context.Context, key string, members ...any) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	return c.rdb.SRem(ctx, key, members...).Result()
}
