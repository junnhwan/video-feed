package worker

import (
	"context"
	"encoding/json"
	"testing"

	"video-feed/internal/middleware/rabbitmq"
	"video-feed/internal/video"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestNotificationWorkerCreatesLikeNotificationAndPushes(t *testing.T) {
	db, hub := newNotificationWorkerDeps(t)
	seed := video.Video{AuthorID: 10, Username: "creator", Title: "first", PlayURL: "1.mp4", CoverURL: "1.jpg"}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	worker := NewNotificationWorker(nil, db, "notification.like", hub)
	body := mustNotificationJSON(t, rabbitmq.LikeEvent{Action: "like", UserID: 7, VideoID: seed.ID})

	if err := worker.process(context.Background(), amqp.Delivery{RoutingKey: "like.like", Body: body}); err != nil {
		t.Fatalf("process like notification: %v", err)
	}

	var stored Notification
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("find notification: %v", err)
	}
	if stored.RecipientID != 10 || stored.SenderID != 7 || stored.Type != "like" || stored.TargetID != seed.ID {
		t.Fatalf("unexpected notification: %+v", stored)
	}
	if len(hub.pushed) != 1 || hub.pushed[0].RecipientID != 10 {
		t.Fatalf("expected pushed notification, got %+v", hub.pushed)
	}
}

func TestNotificationWorkerSkipsSelfLike(t *testing.T) {
	db, hub := newNotificationWorkerDeps(t)
	seed := video.Video{AuthorID: 7, Username: "creator", Title: "first", PlayURL: "1.mp4", CoverURL: "1.jpg"}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	worker := NewNotificationWorker(nil, db, "notification.like", hub)
	body := mustNotificationJSON(t, rabbitmq.LikeEvent{Action: "like", UserID: 7, VideoID: seed.ID})

	if err := worker.process(context.Background(), amqp.Delivery{RoutingKey: "like.like", Body: body}); err != nil {
		t.Fatalf("process self like: %v", err)
	}

	var count int64
	if err := db.Model(&Notification{}).Count(&count).Error; err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 0 || len(hub.pushed) != 0 {
		t.Fatalf("expected no self notification, count=%d pushed=%d", count, len(hub.pushed))
	}
}

func TestNotificationWorkerCreatesCommentAndFollowNotifications(t *testing.T) {
	db, hub := newNotificationWorkerDeps(t)
	seed := video.Video{AuthorID: 10, Username: "creator", Title: "first", PlayURL: "1.mp4", CoverURL: "1.jpg"}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	worker := NewNotificationWorker(nil, db, "notification", hub)

	commentBody := mustNotificationJSON(t, rabbitmq.CommentEvent{Action: "publish", AuthorID: 8, VideoID: seed.ID, Content: "hello"})
	if err := worker.process(context.Background(), amqp.Delivery{RoutingKey: "comment.publish", Body: commentBody}); err != nil {
		t.Fatalf("process comment notification: %v", err)
	}
	followBody := mustNotificationJSON(t, rabbitmq.SocialEvent{Action: "follow", FollowerID: 8, VloggerID: 10})
	if err := worker.process(context.Background(), amqp.Delivery{RoutingKey: "social.follow", Body: followBody}); err != nil {
		t.Fatalf("process follow notification: %v", err)
	}

	var notifications []Notification
	if err := db.Order("id ASC").Find(&notifications).Error; err != nil {
		t.Fatalf("find notifications: %v", err)
	}
	if len(notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(notifications))
	}
	if notifications[0].Type != "comment" || notifications[1].Type != "follow" {
		t.Fatalf("unexpected notification types: %+v", notifications)
	}
}

func newNotificationWorkerDeps(t *testing.T) (*gorm.DB, *fakeNotificationHub) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&video.Video{}, &Notification{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	return db, &fakeNotificationHub{}
}

func mustNotificationJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return body
}

type fakeNotificationHub struct {
	pushed []*Notification
}

func (h *fakeNotificationHub) Push(_ uint, notification *Notification) {
	h.pushed = append(h.pushed, notification)
}
