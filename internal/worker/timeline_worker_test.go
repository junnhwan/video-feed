package worker

import (
	"context"
	"testing"
	"time"

	"video-feed/internal/middleware/rabbitmq"
	rediscache "video-feed/internal/middleware/redis"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestTimelineConsumerWritesAndTrimsGlobalTimeline(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	cache := rediscache.NewClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()}), "")
	defer cache.Close()

	consumer := NewTimelineConsumer(cache)
	base := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 1005; i++ {
		if err := consumer.Process(context.Background(), rabbitmq.TimelineEvent{
			VideoID:    uint(i + 1),
			CreateTime: base.Add(time.Duration(i) * time.Second).UnixMilli(),
		}); err != nil {
			t.Fatalf("process timeline event %d: %v", i, err)
		}
	}

	members, err := cache.ZRevRange(context.Background(), cache.Key("feed:global_timeline"), 0, -1)
	if err != nil {
		t.Fatalf("read timeline: %v", err)
	}
	if len(members) != 1000 {
		t.Fatalf("expected trimmed timeline of 1000, got %d", len(members))
	}
	if members[0] != "1005" || members[len(members)-1] != "6" {
		t.Fatalf("unexpected timeline bounds: first=%s last=%s", members[0], members[len(members)-1])
	}
}
