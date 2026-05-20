package worker

import (
	"context"
	"encoding/json"
	"testing"

	"video-feed/internal/middleware/rabbitmq"
	"video-feed/internal/video"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestLikeWorkerProcessIsIdempotent(t *testing.T) {
	worker, db, seed := newLikeWorkerForTest(t)
	event := rabbitmq.LikeEvent{Action: "like", UserID: 7, VideoID: seed.ID}
	body := mustJSON(t, event)

	if err := worker.process(context.Background(), body); err != nil {
		t.Fatalf("process first like: %v", err)
	}
	if err := worker.process(context.Background(), body); err != nil {
		t.Fatalf("process duplicate like: %v", err)
	}

	var stored video.Video
	if err := db.First(&stored, seed.ID).Error; err != nil {
		t.Fatalf("find video: %v", err)
	}
	if stored.LikesCount != 1 || stored.Popularity != 1 {
		t.Fatalf("expected idempotent counters 1/1, got likes=%d popularity=%d", stored.LikesCount, stored.Popularity)
	}
}

func TestLikeWorkerProcessUnlikeIsIdempotent(t *testing.T) {
	worker, db, seed := newLikeWorkerForTest(t)
	if err := db.Create(&video.Like{VideoID: seed.ID, AccountID: 7}).Error; err != nil {
		t.Fatalf("create like: %v", err)
	}
	if err := db.Model(&video.Video{}).Where("id = ?", seed.ID).Updates(map[string]any{"likes_count": 1, "popularity": 1}).Error; err != nil {
		t.Fatalf("seed counters: %v", err)
	}
	event := rabbitmq.LikeEvent{Action: "unlike", UserID: 7, VideoID: seed.ID}
	body := mustJSON(t, event)

	if err := worker.process(context.Background(), body); err != nil {
		t.Fatalf("process unlike: %v", err)
	}
	if err := worker.process(context.Background(), body); err != nil {
		t.Fatalf("process duplicate unlike: %v", err)
	}

	var stored video.Video
	if err := db.First(&stored, seed.ID).Error; err != nil {
		t.Fatalf("find video: %v", err)
	}
	if stored.LikesCount != 0 || stored.Popularity != 0 {
		t.Fatalf("expected counters 0/0 after idempotent unlike, got likes=%d popularity=%d", stored.LikesCount, stored.Popularity)
	}
}

func newLikeWorkerForTest(t *testing.T) (*LikeWorker, *gorm.DB, video.Video) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&video.Video{}, &video.Like{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	seed := video.Video{AuthorID: 1, Username: "alice", Title: "first", PlayURL: "1.mp4", CoverURL: "1.jpg"}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	return NewLikeWorker(nil, video.NewLikeRepository(db), video.NewRepository(db), rabbitmq.LikeQueue), db, seed
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return body
}
