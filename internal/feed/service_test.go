package feed

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/social"
	"video-feed/internal/video"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&video.Video{}, &video.OutboxMsg{}, &video.Like{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	return NewService(NewRepository(database), video.NewLikeRepository(database), nil, nil), database
}

func createVideo(t *testing.T, db *gorm.DB, v video.Video) video.Video {
	t.Helper()
	if err := db.Create(&v).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	return v
}

func TestListLatestUsesTimeCursor(t *testing.T) {
	service, db := newTestService(t)
	base := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	oldest := createVideo(t, db, video.Video{AuthorID: 1, Username: "alice", Title: "oldest", PlayURL: "1.mp4", CoverURL: "1.jpg", CreatedAt: base})
	middle := createVideo(t, db, video.Video{AuthorID: 1, Username: "alice", Title: "middle", PlayURL: "2.mp4", CoverURL: "2.jpg", CreatedAt: base.Add(time.Minute)})
	newest := createVideo(t, db, video.Video{AuthorID: 2, Username: "bob", Title: "newest", PlayURL: "3.mp4", CoverURL: "3.jpg", CreatedAt: base.Add(2 * time.Minute)})
	_ = oldest

	firstPage, err := service.ListLatest(context.Background(), 2, time.Time{}, 0)
	if err != nil {
		t.Fatalf("list latest first page: %v", err)
	}
	if len(firstPage.VideoList) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(firstPage.VideoList))
	}
	if firstPage.VideoList[0].ID != newest.ID || firstPage.VideoList[1].ID != middle.ID {
		t.Fatalf("expected newest then middle, got %+v", firstPage.VideoList)
	}
	if firstPage.VideoList[0].CreateTime != newest.CreatedAt.Unix() {
		t.Fatalf("expected feed item create_time in Unix seconds, got %d", firstPage.VideoList[0].CreateTime)
	}
	if !firstPage.HasMore {
		t.Fatal("expected has_more true for full page")
	}

	nextCursor := time.UnixMilli(firstPage.NextTime)
	secondPage, err := service.ListLatest(context.Background(), 2, nextCursor, 0)
	if err != nil {
		t.Fatalf("list latest second page: %v", err)
	}
	if len(secondPage.VideoList) != 1 {
		t.Fatalf("expected 1 video, got %d", len(secondPage.VideoList))
	}
	if secondPage.VideoList[0].Title != "oldest" {
		t.Fatalf("expected oldest, got %q", secondPage.VideoList[0].Title)
	}
}

func TestListLatestUsesRedisGlobalTimelineAndStitchesColdData(t *testing.T) {
	service, db := newTestService(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	cache := rediscache.NewClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()}), "")
	defer cache.Close()
	service.cache = cache

	base := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	oldest := createVideo(t, db, video.Video{AuthorID: 1, Username: "alice", Title: "oldest", PlayURL: "1.mp4", CoverURL: "1.jpg", CreatedAt: base})
	middle := createVideo(t, db, video.Video{AuthorID: 1, Username: "alice", Title: "middle", PlayURL: "2.mp4", CoverURL: "2.jpg", CreatedAt: base.Add(time.Minute)})
	newest := createVideo(t, db, video.Video{AuthorID: 1, Username: "alice", Title: "newest", PlayURL: "3.mp4", CoverURL: "3.jpg", CreatedAt: base.Add(2 * time.Minute)})

	if err := cache.ZAdd(context.Background(), cache.Key("feed:global_timeline"), goredis.Z{
		Score:  float64(middle.CreatedAt.UnixMilli()),
		Member: strconv.FormatUint(uint64(middle.ID), 10),
	}); err != nil {
		t.Fatalf("seed timeline: %v", err)
	}

	resp, err := service.ListLatest(context.Background(), 2, time.Time{}, 0)
	if err != nil {
		t.Fatalf("list latest: %v", err)
	}
	if len(resp.VideoList) != 2 {
		t.Fatalf("expected redis hot item stitched with cold DB item, got %d: %+v", len(resp.VideoList), resp.VideoList)
	}
	if resp.VideoList[0].ID != middle.ID || resp.VideoList[1].ID != oldest.ID {
		t.Fatalf("unexpected stitched order: %+v", resp.VideoList)
	}
	_ = newest
}

func TestListLatestWarmsVideoEntityCacheFromTimeline(t *testing.T) {
	service, db := newTestService(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	cache := rediscache.NewClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()}), "")
	defer cache.Close()
	service.cache = cache

	createdAt := time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC)
	cached := createVideo(t, db, video.Video{AuthorID: 1, Username: "alice", Title: "cached title", PlayURL: "1.mp4", CoverURL: "1.jpg", CreatedAt: createdAt})
	if err := cache.ZAdd(context.Background(), cache.Key("feed:global_timeline"), goredis.Z{
		Score:  float64(cached.CreatedAt.UnixMilli()),
		Member: strconv.FormatUint(uint64(cached.ID), 10),
	}); err != nil {
		t.Fatalf("seed timeline: %v", err)
	}

	first, err := service.ListLatest(context.Background(), 1, time.Time{}, 0)
	if err != nil {
		t.Fatalf("list latest first: %v", err)
	}
	if len(first.VideoList) != 1 || first.VideoList[0].Title != "cached title" {
		t.Fatalf("unexpected first response: %+v", first.VideoList)
	}
	payload, err := cache.GetBytes(context.Background(), cache.Key("video:entity:%d", cached.ID))
	if err != nil {
		t.Fatalf("expected redis entity cache warmed: %v", err)
	}
	if !strings.Contains(string(payload), "cached title") {
		t.Fatalf("expected cached entity payload to include original title, got %s", string(payload))
	}

	if err := db.Model(&video.Video{}).Where("id = ?", cached.ID).Update("title", "updated title").Error; err != nil {
		t.Fatalf("update video title: %v", err)
	}
	second, err := service.ListLatest(context.Background(), 1, time.Time{}, 0)
	if err != nil {
		t.Fatalf("list latest second: %v", err)
	}
	if len(second.VideoList) != 1 || second.VideoList[0].Title != "cached title" {
		t.Fatalf("expected second response from entity cache, got %+v", second.VideoList)
	}
}

func TestListLikesCountUsesCompositeCursor(t *testing.T) {
	service, db := newTestService(t)
	first := createVideo(t, db, video.Video{AuthorID: 1, Username: "alice", Title: "first", PlayURL: "1.mp4", CoverURL: "1.jpg", LikesCount: 10})
	second := createVideo(t, db, video.Video{AuthorID: 1, Username: "alice", Title: "second", PlayURL: "2.mp4", CoverURL: "2.jpg", LikesCount: 10})
	third := createVideo(t, db, video.Video{AuthorID: 1, Username: "alice", Title: "third", PlayURL: "3.mp4", CoverURL: "3.jpg", LikesCount: 8})

	page, err := service.ListLikesCount(context.Background(), 2, nil, 0)
	if err != nil {
		t.Fatalf("list likes count: %v", err)
	}
	if len(page.VideoList) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(page.VideoList))
	}
	if page.VideoList[0].ID != second.ID || page.VideoList[1].ID != first.ID {
		t.Fatalf("expected same-like ties by id desc, got ids %d and %d", page.VideoList[0].ID, page.VideoList[1].ID)
	}
	if page.NextLikesCountBefore == nil || page.NextIDBefore == nil {
		t.Fatal("expected composite cursor")
	}

	next, err := service.ListLikesCount(context.Background(), 2, &LikesCountCursor{
		LikesCount: *page.NextLikesCountBefore,
		ID:         *page.NextIDBefore,
	}, 0)
	if err != nil {
		t.Fatalf("list next likes page: %v", err)
	}
	if len(next.VideoList) != 1 {
		t.Fatalf("expected 1 video, got %d", len(next.VideoList))
	}
	if next.VideoList[0].ID != third.ID {
		t.Fatalf("expected third video, got %d", next.VideoList[0].ID)
	}
}

func TestListByFollowingOnlyReturnsFollowedAuthors(t *testing.T) {
	service, db := newTestService(t)
	createVideo(t, db, video.Video{AuthorID: 2, Username: "bob", Title: "followed", PlayURL: "1.mp4", CoverURL: "1.jpg"})
	createVideo(t, db, video.Video{AuthorID: 3, Username: "carol", Title: "not followed", PlayURL: "2.mp4", CoverURL: "2.jpg"})
	if err := db.Create(&social.Social{FollowerID: 1, VloggerID: 2}).Error; err != nil {
		t.Fatalf("create follow relation: %v", err)
	}

	resp, err := service.ListByFollowing(context.Background(), 10, time.Time{}, 1)
	if err != nil {
		t.Fatalf("list by following: %v", err)
	}
	if len(resp.VideoList) != 1 {
		t.Fatalf("expected 1 video, got %d", len(resp.VideoList))
	}
	if resp.VideoList[0].Author.ID != 2 {
		t.Fatalf("expected author 2, got %d", resp.VideoList[0].Author.ID)
	}
	if resp.NextTime != resp.VideoList[0].CreateTime {
		t.Fatalf("expected following next_time to use Unix seconds cursor, got next_time=%d item_time=%d", resp.NextTime, resp.VideoList[0].CreateTime)
	}
}

func TestListByFollowingCachesResponseByViewerAndCursor(t *testing.T) {
	service, db := newTestService(t)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	cache := rediscache.NewClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()}), "")
	defer cache.Close()
	service.cache = cache

	followed := createVideo(t, db, video.Video{AuthorID: 2, Username: "bob", Title: "followed", PlayURL: "1.mp4", CoverURL: "1.jpg"})
	if err := db.Create(&social.Social{FollowerID: 1, VloggerID: 2}).Error; err != nil {
		t.Fatalf("create follow relation: %v", err)
	}

	first, err := service.ListByFollowing(context.Background(), 10, time.Time{}, 1)
	if err != nil {
		t.Fatalf("list by following first: %v", err)
	}
	if len(first.VideoList) != 1 || first.VideoList[0].Title != "followed" {
		t.Fatalf("unexpected first following response: %+v", first.VideoList)
	}
	if err := db.Model(&video.Video{}).Where("id = ?", followed.ID).Update("title", "updated").Error; err != nil {
		t.Fatalf("update followed video: %v", err)
	}

	second, err := service.ListByFollowing(context.Background(), 10, time.Time{}, 1)
	if err != nil {
		t.Fatalf("list by following second: %v", err)
	}
	if len(second.VideoList) != 1 || second.VideoList[0].Title != "followed" {
		t.Fatalf("expected cached following response, got %+v", second.VideoList)
	}
}

func TestFeedItemsIncludeViewerLikeState(t *testing.T) {
	service, db := newTestService(t)
	liked := createVideo(t, db, video.Video{AuthorID: 2, Username: "bob", Title: "liked", PlayURL: "1.mp4", CoverURL: "1.jpg"})
	unliked := createVideo(t, db, video.Video{AuthorID: 3, Username: "carol", Title: "unliked", PlayURL: "2.mp4", CoverURL: "2.jpg"})
	if err := db.Create(&video.Like{VideoID: liked.ID, AccountID: 1}).Error; err != nil {
		t.Fatalf("create like: %v", err)
	}
	_ = unliked

	resp, err := service.ListLatest(context.Background(), 10, time.Time{}, 1)
	if err != nil {
		t.Fatalf("list latest: %v", err)
	}
	likedByID := map[uint]bool{}
	for _, item := range resp.VideoList {
		likedByID[item.ID] = item.IsLiked
	}
	if !likedByID[liked.ID] {
		t.Fatal("expected liked video to be marked liked")
	}
	if likedByID[unliked.ID] {
		t.Fatal("expected unliked video to remain false")
	}
}
