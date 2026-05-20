package feed

import (
	"context"
	"testing"

	"video-feed/internal/social"
	"video-feed/internal/video"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestListByTagReturnsTaggedVideos(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&video.Video{}, &video.Like{}, &video.Tag{}, &video.VideoTag{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	videoService := video.NewService(video.NewRepository(db))
	tagged, err := videoService.Publish(context.Background(), video.PublishInput{
		AuthorID: 1,
		Username: "alice",
		Title:    "first #Go",
		PlayURL:  "1.mp4",
		CoverURL: "1.jpg",
	})
	if err != nil {
		t.Fatalf("publish tagged: %v", err)
	}
	if _, err := videoService.Publish(context.Background(), video.PublishInput{
		AuthorID: 2,
		Username: "bob",
		Title:    "second #Java",
		PlayURL:  "2.mp4",
		CoverURL: "2.jpg",
	}); err != nil {
		t.Fatalf("publish untagged: %v", err)
	}
	service := NewService(NewRepository(db), video.NewLikeRepository(db), nil)

	items, err := service.ListByTag(context.Background(), "Go", 10, 0)

	if err != nil {
		t.Fatalf("list by tag: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != tagged.ID {
		t.Fatalf("expected tagged video %d, got %d", tagged.ID, items[0].ID)
	}
}
