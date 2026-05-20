package video

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newLikeTestService(t *testing.T) (*LikeService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Video{}, &Like{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	return NewLikeService(NewLikeRepository(db), NewRepository(db), nil), db
}

func TestLikeCreatesRowAndIncrementsCounters(t *testing.T) {
	service, db := newLikeTestService(t)
	video := createServiceTestVideo(t, db)

	err := service.Like(context.Background(), video.ID, 7)

	if err != nil {
		t.Fatalf("like: %v", err)
	}
	var stored Video
	if err := db.First(&stored, video.ID).Error; err != nil {
		t.Fatalf("find video: %v", err)
	}
	if stored.LikesCount != 1 {
		t.Fatalf("expected likes_count 1, got %d", stored.LikesCount)
	}
	if stored.Popularity != 1 {
		t.Fatalf("expected popularity 1, got %d", stored.Popularity)
	}
	liked, err := service.IsLiked(context.Background(), video.ID, 7)
	if err != nil {
		t.Fatalf("is liked: %v", err)
	}
	if !liked {
		t.Fatal("expected video to be liked")
	}
}

func TestLikeRejectsDuplicateWithoutDoubleCounting(t *testing.T) {
	service, db := newLikeTestService(t)
	video := createServiceTestVideo(t, db)
	if err := service.Like(context.Background(), video.ID, 7); err != nil {
		t.Fatalf("first like: %v", err)
	}

	err := service.Like(context.Background(), video.ID, 7)

	if !errors.Is(err, ErrAlreadyLiked) {
		t.Fatalf("expected ErrAlreadyLiked, got %v", err)
	}
	var stored Video
	if err := db.First(&stored, video.ID).Error; err != nil {
		t.Fatalf("find video: %v", err)
	}
	if stored.LikesCount != 1 {
		t.Fatalf("expected likes_count to stay 1, got %d", stored.LikesCount)
	}
}

func TestUnlikeDeletesRowAndDecrementsCounters(t *testing.T) {
	service, db := newLikeTestService(t)
	video := createServiceTestVideo(t, db)
	if err := service.Like(context.Background(), video.ID, 7); err != nil {
		t.Fatalf("like: %v", err)
	}

	if err := service.Unlike(context.Background(), video.ID, 7); err != nil {
		t.Fatalf("unlike: %v", err)
	}

	var stored Video
	if err := db.First(&stored, video.ID).Error; err != nil {
		t.Fatalf("find video: %v", err)
	}
	if stored.LikesCount != 0 {
		t.Fatalf("expected likes_count 0, got %d", stored.LikesCount)
	}
	if stored.Popularity != 0 {
		t.Fatalf("expected popularity 0, got %d", stored.Popularity)
	}
	liked, err := service.IsLiked(context.Background(), video.ID, 7)
	if err != nil {
		t.Fatalf("is liked: %v", err)
	}
	if liked {
		t.Fatal("expected video to be unliked")
	}
}

func createServiceTestVideo(t *testing.T, db *gorm.DB) Video {
	t.Helper()
	video := Video{AuthorID: 1, Username: "alice", Title: "first", PlayURL: "1.mp4", CoverURL: "1.jpg"}
	if err := db.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	return video
}
