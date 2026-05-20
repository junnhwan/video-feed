package video

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newCommentTestService(t *testing.T) (*CommentService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Video{}, &Comment{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	return NewCommentService(NewCommentRepository(db), NewRepository(db), nil), db
}

func TestPublishCommentCreatesCommentAndIncrementsPopularity(t *testing.T) {
	service, db := newCommentTestService(t)
	video := createServiceTestVideo(t, db)

	comment, err := service.Publish(context.Background(), PublishCommentInput{
		VideoID:  video.ID,
		AuthorID: 7,
		Username: "alice",
		Content:  "hello",
	})

	if err != nil {
		t.Fatalf("publish comment: %v", err)
	}
	if comment.ID == 0 {
		t.Fatal("expected generated comment id")
	}
	var stored Video
	if err := db.First(&stored, video.ID).Error; err != nil {
		t.Fatalf("find video: %v", err)
	}
	if stored.Popularity != 1 {
		t.Fatalf("expected popularity 1, got %d", stored.Popularity)
	}
}

func TestGetAllCommentsReturnsAscendingOrder(t *testing.T) {
	service, db := newCommentTestService(t)
	video := createServiceTestVideo(t, db)
	first, err := service.Publish(context.Background(), PublishCommentInput{VideoID: video.ID, AuthorID: 7, Username: "alice", Content: "first"})
	if err != nil {
		t.Fatalf("publish first: %v", err)
	}
	second, err := service.Publish(context.Background(), PublishCommentInput{VideoID: video.ID, AuthorID: 8, Username: "bob", Content: "second"})
	if err != nil {
		t.Fatalf("publish second: %v", err)
	}

	comments, err := service.GetAll(context.Background(), video.ID)

	if err != nil {
		t.Fatalf("get all comments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].ID != first.ID || comments[1].ID != second.ID {
		t.Fatalf("expected ascending order, got ids %d and %d", comments[0].ID, comments[1].ID)
	}
}

func TestDeleteCommentRequiresAuthor(t *testing.T) {
	service, db := newCommentTestService(t)
	video := createServiceTestVideo(t, db)
	comment, err := service.Publish(context.Background(), PublishCommentInput{VideoID: video.ID, AuthorID: 7, Username: "alice", Content: "hello"})
	if err != nil {
		t.Fatalf("publish comment: %v", err)
	}

	err = service.Delete(context.Background(), comment.ID, 8)

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}
