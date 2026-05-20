package video

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&Video{}); err != nil {
		t.Fatalf("migrate video: %v", err)
	}

	return NewService(NewRepository(database))
}

func TestPublishCreatesVideo(t *testing.T) {
	service := newTestService(t)

	video, err := service.Publish(context.Background(), PublishInput{
		AuthorID:    7,
		Title:       "first vlog",
		Description: "hello",
		PlayURL:     "http://example.com/video.mp4",
		CoverURL:    "http://example.com/cover.jpg",
	})

	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if video.ID == 0 {
		t.Fatal("expected generated video id")
	}
	if video.AuthorID != 7 {
		t.Fatalf("expected author id 7, got %d", video.AuthorID)
	}
}

func TestPublishRejectsMissingRequiredFields(t *testing.T) {
	service := newTestService(t)

	_, err := service.Publish(context.Background(), PublishInput{AuthorID: 7})

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetDetailReturnsPublishedVideo(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()
	published, err := service.Publish(ctx, PublishInput{
		AuthorID: 7,
		Title:    "first vlog",
		PlayURL:  "http://example.com/video.mp4",
		CoverURL: "http://example.com/cover.jpg",
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	found, err := service.GetDetail(ctx, published.ID)

	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if found.Title != "first vlog" {
		t.Fatalf("expected first vlog, got %q", found.Title)
	}
}

func TestListByAuthorIDReturnsNewestFirst(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()
	first, err := service.Publish(ctx, PublishInput{
		AuthorID: 7,
		Title:    "first vlog",
		PlayURL:  "http://example.com/1.mp4",
		CoverURL: "http://example.com/1.jpg",
	})
	if err != nil {
		t.Fatalf("publish first: %v", err)
	}
	time.Sleep(time.Millisecond)
	second, err := service.Publish(ctx, PublishInput{
		AuthorID: 7,
		Title:    "second vlog",
		PlayURL:  "http://example.com/2.mp4",
		CoverURL: "http://example.com/2.jpg",
	})
	if err != nil {
		t.Fatalf("publish second: %v", err)
	}

	videos, err := service.ListByAuthorID(ctx, 7)

	if err != nil {
		t.Fatalf("list by author: %v", err)
	}
	if len(videos) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(videos))
	}
	if videos[0].ID != second.ID || videos[1].ID != first.ID {
		t.Fatalf("expected newest first, got ids %d then %d", videos[0].ID, videos[1].ID)
	}
}
