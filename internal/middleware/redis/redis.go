package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"video-feed/internal/config"

	goredis "github.com/redis/go-redis/v9"
)

const defaultKeyPrefix = "v1:"

type Client struct {
	rdb       *goredis.Client
	keyPrefix string
}

func NewClient(rdb *goredis.Client, keyPrefix string) *Client {
	if keyPrefix == "" {
		keyPrefix = defaultKeyPrefix
	}
	return &Client{rdb: rdb, keyPrefix: keyPrefix}
}

func NewFromConfig(cfg config.RedisConfig) *Client {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return NewClient(rdb, defaultKeyPrefix)
}

func IsMiss(err error) bool {
	return err == goredis.Nil
}

func (c *Client) Key(format string, args ...any) string {
	prefix := ""
	if c != nil {
		prefix = c.keyPrefix
	}
	return prefix + fmt.Sprintf(format, args...)
}

func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) GetBytes(ctx context.Context, key string) ([]byte, error) {
	if c == nil || c.rdb == nil {
		return nil, goredis.Nil
	}
	return c.rdb.Get(ctx, key).Bytes()
}

func (c *Client) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c *Client) MGet(ctx context.Context, keys ...string) ([]any, error) {
	if c == nil || c.rdb == nil {
		return nil, goredis.Nil
	}
	return c.rdb.MGet(ctx, keys...).Result()
}

func (c *Client) Del(ctx context.Context, key string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, key).Err()
}

func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (string, bool, error) {
	if c == nil || c.rdb == nil {
		return "", false, nil
	}
	token, err := randToken(16)
	if err != nil {
		return "", false, err
	}
	ok, err := c.rdb.SetNX(ctx, key, token, ttl).Result()
	return token, ok, err
}

func (c *Client) Unlock(ctx context.Context, key string, token string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	_, err := unlockScript.Run(ctx, c.rdb, []string{key}, token).Result()
	return err
}

func (c *Client) IncrementWithExpire(ctx context.Context, key string, expire time.Duration) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	return incrementWithExpireScript.Run(ctx, c.rdb, []string{key}, expire.Milliseconds()).Int64()
}

func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

var unlockScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
else
  return 0
end
`)

var incrementWithExpireScript = goredis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return count
`)
