package redis

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	client := NewClient(goredis.NewClient(&goredis.Options{Addr: server.Addr()}), "test:")
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return client, server
}

func TestGetBytesReportsCacheMiss(t *testing.T) {
	client, _ := newTestClient(t)

	_, err := client.GetBytes(context.Background(), client.Key("missing"))

	if !IsMiss(err) {
		t.Fatalf("expected cache miss, got %v", err)
	}
}

func TestSetGetAndDeleteBytes(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()
	key := client.Key("account:%d", 7)

	if err := client.SetBytes(ctx, key, []byte("token"), time.Minute); err != nil {
		t.Fatalf("set bytes: %v", err)
	}
	got, err := client.GetBytes(ctx, key)
	if err != nil {
		t.Fatalf("get bytes: %v", err)
	}
	if string(got) != "token" {
		t.Fatalf("expected token, got %q", string(got))
	}
	if err := client.Del(ctx, key); err != nil {
		t.Fatalf("delete key: %v", err)
	}
	if _, err := client.GetBytes(ctx, key); !IsMiss(err) {
		t.Fatalf("expected cache miss after delete, got %v", err)
	}
}

func TestIncrementWithExpireDoesNotExtendWindow(t *testing.T) {
	client, server := newTestClient(t)
	ctx := context.Background()
	key := client.Key("ratelimit:login:127.0.0.1")
	window := 30 * time.Second

	count, err := client.IncrementWithExpire(ctx, key, window)
	if err != nil {
		t.Fatalf("first increment: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	server.FastForward(5 * time.Second)
	ttlBeforeSecond := server.TTL(key)
	count, err = client.IncrementWithExpire(ctx, key, window)
	if err != nil {
		t.Fatalf("second increment: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
	if ttlAfterSecond := server.TTL(key); ttlAfterSecond != ttlBeforeSecond {
		t.Fatalf("expected ttl to stay at %s, got %s", ttlBeforeSecond, ttlAfterSecond)
	}
}
