package video

import (
	"context"
	"testing"
	"time"

	rediscache "video-feed/internal/middleware/redis"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func TestGetDetailUsesRedisCacheAcrossServiceInstances(t *testing.T) {
	db, cache := newDetailCacheDeps(t)
	first := NewService(NewRepository(db), cache)
	second := NewService(NewRepository(db), cache)
	seed := createServiceTestVideo(t, db)

	got, err := first.GetDetail(context.Background(), seed.ID)
	if err != nil {
		t.Fatalf("first get detail: %v", err)
	}
	if got.Title != "first" {
		t.Fatalf("expected first title, got %q", got.Title)
	}
	if err := db.Model(&Video{}).Where("id = ?", seed.ID).Update("title", "changed in db").Error; err != nil {
		t.Fatalf("update db title: %v", err)
	}

	cached, err := second.GetDetail(context.Background(), seed.ID)
	if err != nil {
		t.Fatalf("second get detail: %v", err)
	}
	if cached.Title != "first" {
		t.Fatalf("expected Redis cached title, got %q", cached.Title)
	}
}

func TestGetDetailUsesLocalCacheBeforeRedis(t *testing.T) {
	db, cache := newDetailCacheDeps(t)
	service := NewService(NewRepository(db), cache)
	seed := createServiceTestVideo(t, db)

	if _, err := service.GetDetail(context.Background(), seed.ID); err != nil {
		t.Fatalf("first get detail: %v", err)
	}
	if err := cache.Del(context.Background(), cache.Key("video:detail:id=%d", seed.ID)); err != nil {
		t.Fatalf("delete redis detail cache: %v", err)
	}
	if err := db.Model(&Video{}).Where("id = ?", seed.ID).Update("title", "changed in db").Error; err != nil {
		t.Fatalf("update db title: %v", err)
	}

	cached, err := service.GetDetail(context.Background(), seed.ID)
	if err != nil {
		t.Fatalf("second get detail: %v", err)
	}
	if cached.Title != "first" {
		t.Fatalf("expected local cached title, got %q", cached.Title)
	}
}

func TestDeleteInvalidatesDetailCaches(t *testing.T) {
	db, cache := newDetailCacheDeps(t)
	service := NewService(NewRepository(db), cache)
	seed := createServiceTestVideo(t, db)

	if _, err := service.GetDetail(context.Background(), seed.ID); err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if err := service.Delete(context.Background(), seed.ID, seed.AuthorID); err != nil {
		t.Fatalf("delete video: %v", err)
	}

	if _, err := cache.GetBytes(context.Background(), cache.Key("video:detail:id=%d", seed.ID)); !rediscache.IsMiss(err) {
		t.Fatalf("expected Redis cache miss after delete, got %v", err)
	}
	if _, err := service.GetDetail(context.Background(), seed.ID); err == nil {
		t.Fatal("expected get detail to fail after delete")
	}
}

func newDetailCacheDeps(t *testing.T) (*gorm.DB, *rediscache.Client) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Video{}, &Tag{}, &VideoTag{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	client := rediscache.NewClient(goredis.NewClient(&goredis.Options{Addr: server.Addr()}), "test:")
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return db, client
}

func TestDetailCacheEntryExpires(t *testing.T) {
	db, cache := newDetailCacheDeps(t)
	service := NewService(NewRepository(db), cache)
	service.detailCacheTTL = 10 * time.Millisecond
	seed := createServiceTestVideo(t, db)

	if _, err := service.GetDetail(context.Background(), seed.ID); err != nil {
		t.Fatalf("first get detail: %v", err)
	}
	if err := db.Model(&Video{}).Where("id = ?", seed.ID).Update("title", "changed after ttl").Error; err != nil {
		t.Fatalf("update db title: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := cache.Del(context.Background(), cache.Key("video:detail:id=%d", seed.ID)); err != nil {
		t.Fatalf("delete redis detail cache: %v", err)
	}

	got, err := service.GetDetail(context.Background(), seed.ID)
	if err != nil {
		t.Fatalf("get detail after ttl: %v", err)
	}
	if got.Title != "changed after ttl" {
		t.Fatalf("expected refreshed title, got %q", got.Title)
	}
}
