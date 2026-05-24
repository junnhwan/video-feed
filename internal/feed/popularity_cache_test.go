package feed

import (
	"context"
	"strconv"
	"testing"
	"time"

	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/social"
	"video-feed/internal/video"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func TestListByPopularityUsesRedisHotSnapshotAndStableOffset(t *testing.T) {
	service, db, cache := newRedisFeedService(t)
	ctx := context.Background()
	asOf := time.Now().UTC().Truncate(time.Minute)

	lowDBHighRedis := createVideo(t, db, video.Video{AuthorID: 1, Username: "alice", Title: "redis first", PlayURL: "1.mp4", CoverURL: "1.jpg", Popularity: 1})
	highDBLowRedis := createVideo(t, db, video.Video{AuthorID: 2, Username: "bob", Title: "redis last", PlayURL: "2.mp4", CoverURL: "2.jpg", Popularity: 99})
	middle := createVideo(t, db, video.Video{AuthorID: 3, Username: "carol", Title: "redis second", PlayURL: "3.mp4", CoverURL: "3.jpg", Popularity: 10})

	seedHotScore(t, cache, asOf, lowDBHighRedis.ID, 9)
	seedHotScore(t, cache, asOf, middle.ID, 7)
	seedHotScore(t, cache, asOf, highDBLowRedis.ID, 1)

	firstPage, err := service.ListByPopularity(ctx, 2, asOf.Unix(), 0, 0, 0, time.Time{}, 0)
	if err != nil {
		t.Fatalf("list first hot page: %v", err)
	}
	if firstPage.AsOf != asOf.Unix() {
		t.Fatalf("expected as_of %d, got %d", asOf.Unix(), firstPage.AsOf)
	}
	if firstPage.NextOffset != 2 {
		t.Fatalf("expected next_offset 2, got %d", firstPage.NextOffset)
	}
	if len(firstPage.VideoList) != 2 {
		t.Fatalf("expected 2 hot videos, got %d", len(firstPage.VideoList))
	}
	if firstPage.VideoList[0].ID != lowDBHighRedis.ID || firstPage.VideoList[1].ID != middle.ID {
		t.Fatalf("expected Redis score order, got %+v", firstPage.VideoList)
	}

	secondPage, err := service.ListByPopularity(ctx, 2, firstPage.AsOf, firstPage.NextOffset, 0, 0, time.Time{}, 0)
	if err != nil {
		t.Fatalf("list second hot page: %v", err)
	}
	if len(secondPage.VideoList) != 1 {
		t.Fatalf("expected 1 hot video on second page, got %d", len(secondPage.VideoList))
	}
	if secondPage.VideoList[0].ID != highDBLowRedis.ID {
		t.Fatalf("expected Redis offset to keep third item, got %+v", secondPage.VideoList)
	}
}

func TestUpdatePopularityCacheWritesMinuteHotWindow(t *testing.T) {
	cache, _ := newFeedRedisClient(t)
	ctx := context.Background()
	before := time.Now().UTC().Truncate(time.Minute)

	video.UpdatePopularityCache(ctx, cache, 42, 3)

	after := time.Now().UTC().Truncate(time.Minute)
	var members []string
	for _, minute := range []time.Time{before, after} {
		key := cache.Key("hot:video:1m:%s", minute.Format("200601021504"))
		got, err := cache.ZRevRange(ctx, key, 0, 0)
		if err != nil {
			t.Fatalf("zrevrange hot window: %v", err)
		}
		if len(got) > 0 {
			members = got
			break
		}
	}
	if len(members) != 1 || members[0] != "42" {
		t.Fatalf("expected hot member 42, got %#v", members)
	}
}

func newRedisFeedService(t *testing.T) (*Service, *gorm.DB, *rediscache.Client) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&video.Video{}, &video.Like{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	cache, _ := newFeedRedisClient(t)
	return NewService(NewRepository(database), video.NewLikeRepository(database), cache, nil), database, cache
}

func newFeedRedisClient(t *testing.T) (*rediscache.Client, *miniredis.Miniredis) {
	t.Helper()
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	client := rediscache.NewClient(goredis.NewClient(&goredis.Options{Addr: server.Addr()}), "test:")
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return client, server
}

func seedHotScore(t *testing.T, cache *rediscache.Client, asOf time.Time, videoID uint, score float64) {
	t.Helper()
	key := cache.Key("hot:video:1m:%s", asOf.Format("200601021504"))
	if err := cache.ZIncrBy(context.Background(), key, strconv.FormatUint(uint64(videoID), 10), score); err != nil {
		t.Fatalf("seed hot score: %v", err)
	}
}
