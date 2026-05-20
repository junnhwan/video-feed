package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"video-feed/internal/middleware/rabbitmq"
	"video-feed/internal/video"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

type Notification struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	RecipientID uint      `gorm:"index;not null" json:"recipient_id"`
	SenderID    uint      `gorm:"not null" json:"sender_id"`
	Type        string    `gorm:"size:50;not null" json:"type"`
	TargetID    uint      `json:"target_id"`
	Content     string    `gorm:"size:255" json:"content"`
	IsRead      bool      `gorm:"not null;default:false" json:"is_read"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type NotificationHub interface {
	Push(userID uint, notification *Notification)
}

type NotificationWorker struct {
	ch    *amqp.Channel
	db    *gorm.DB
	queue string
	hub   NotificationHub
}

func NewNotificationWorker(ch *amqp.Channel, db *gorm.DB, queue string, hub NotificationHub) *NotificationWorker {
	return &NotificationWorker{ch: ch, db: db, queue: queue, hub: hub}
}

func (w *NotificationWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.db == nil {
		return errors.New("notification worker is not initialized")
	}
	if w.queue == "" {
		return errors.New("queue is required")
	}
	if err := w.db.WithContext(ctx).AutoMigrate(&Notification{}); err != nil {
		return err
	}

	deliveries, err := w.ch.Consume(w.queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("deliveries channel closed")
			}
			w.handleDelivery(ctx, delivery)
		}
	}
}

func (w *NotificationWorker) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	if err := w.process(ctx, delivery); err != nil {
		retryCount := rabbitmq.GetRetryCount(delivery)
		if retryCount >= rabbitmq.MaxRetryCount {
			log.Printf("notification worker: max retries exceeded (%d): %v", retryCount, err)
			_ = delivery.Ack(false)
			return
		}
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}

func (w *NotificationWorker) process(ctx context.Context, delivery amqp.Delivery) error {
	if len(delivery.Body) == 0 {
		return nil
	}

	notification, err := w.notificationFromDelivery(ctx, delivery)
	if err != nil || notification == nil {
		return err
	}
	if err := w.db.WithContext(ctx).Create(notification).Error; err != nil {
		return err
	}
	if w.hub != nil {
		w.hub.Push(notification.RecipientID, notification)
	}
	return nil
}

func (w *NotificationWorker) notificationFromDelivery(ctx context.Context, delivery amqp.Delivery) (*Notification, error) {
	switch delivery.RoutingKey {
	case "like.like":
		var event rabbitmq.LikeEvent
		if err := json.Unmarshal(delivery.Body, &event); err != nil {
			return nil, nil
		}
		if event.UserID == 0 || event.VideoID == 0 {
			return nil, nil
		}
		authorID, err := w.videoAuthorID(ctx, event.VideoID)
		if err != nil || authorID == 0 || authorID == event.UserID {
			return nil, err
		}
		return &Notification{
			RecipientID: authorID,
			SenderID:    event.UserID,
			Type:        "like",
			TargetID:    event.VideoID,
			Content:     "点赞了你的视频",
		}, nil
	case "comment.publish":
		var event rabbitmq.CommentEvent
		if err := json.Unmarshal(delivery.Body, &event); err != nil {
			return nil, nil
		}
		if event.AuthorID == 0 || event.VideoID == 0 {
			return nil, nil
		}
		authorID, err := w.videoAuthorID(ctx, event.VideoID)
		if err != nil || authorID == 0 || authorID == event.AuthorID {
			return nil, err
		}
		return &Notification{
			RecipientID: authorID,
			SenderID:    event.AuthorID,
			Type:        "comment",
			TargetID:    event.VideoID,
			Content:     "评论了你的视频",
		}, nil
	case "social.follow":
		var event rabbitmq.SocialEvent
		if err := json.Unmarshal(delivery.Body, &event); err != nil {
			return nil, nil
		}
		if event.FollowerID == 0 || event.VloggerID == 0 || event.FollowerID == event.VloggerID {
			return nil, nil
		}
		return &Notification{
			RecipientID: event.VloggerID,
			SenderID:    event.FollowerID,
			Type:        "follow",
			TargetID:    event.FollowerID,
			Content:     "关注了你",
		}, nil
	default:
		return nil, nil
	}
}

func (w *NotificationWorker) videoAuthorID(ctx context.Context, videoID uint) (uint, error) {
	var row struct {
		AuthorID uint
	}
	err := w.db.WithContext(ctx).
		Model(&video.Video{}).
		Select("author_id").
		Where("id = ?", videoID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.AuthorID, nil
}
