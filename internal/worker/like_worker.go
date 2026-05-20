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
)

type LikeWorker struct {
	ch     *amqp.Channel
	likes  *video.LikeRepository
	videos *video.Repository
	queue  string
}

func NewLikeWorker(ch *amqp.Channel, likes *video.LikeRepository, videos *video.Repository, queue string) *LikeWorker {
	return &LikeWorker{ch: ch, likes: likes, videos: videos, queue: queue}
}

func (w *LikeWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.likes == nil || w.videos == nil {
		return errors.New("like worker is not initialized")
	}
	if w.queue == "" {
		return errors.New("queue is required")
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

func (w *LikeWorker) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	if err := w.process(ctx, delivery.Body); err != nil {
		retryCount := rabbitmq.GetRetryCount(delivery)
		if retryCount >= rabbitmq.MaxRetryCount {
			log.Printf("like worker: max retries exceeded (%d): %v", retryCount, err)
			_ = delivery.Ack(false)
			return
		}
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}

func (w *LikeWorker) process(ctx context.Context, body []byte) error {
	var event rabbitmq.LikeEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil
	}
	if event.UserID == 0 || event.VideoID == 0 {
		return nil
	}
	switch event.Action {
	case "like":
		return w.applyLike(ctx, event.UserID, event.VideoID)
	case "unlike":
		return w.applyUnlike(ctx, event.UserID, event.VideoID)
	default:
		return nil
	}
}

func (w *LikeWorker) applyLike(ctx context.Context, userID uint, videoID uint) error {
	exists, err := w.videos.IsExist(ctx, videoID)
	if err != nil || !exists {
		return err
	}
	created, err := w.likes.LikeIgnoreDuplicate(ctx, &video.Like{
		VideoID:   videoID,
		AccountID: userID,
		CreatedAt: time.Now(),
	})
	if err != nil || !created {
		return err
	}
	if err := w.videos.ChangeLikesCount(ctx, videoID, 1); err != nil {
		return err
	}
	return w.videos.ChangePopularity(ctx, videoID, 1)
}

func (w *LikeWorker) applyUnlike(ctx context.Context, userID uint, videoID uint) error {
	exists, err := w.videos.IsExist(ctx, videoID)
	if err != nil || !exists {
		return err
	}
	deleted, err := w.likes.DeleteByVideoAndAccount(ctx, videoID, userID)
	if err != nil || !deleted {
		return err
	}
	if err := w.videos.ChangeLikesCount(ctx, videoID, -1); err != nil {
		return err
	}
	return w.videos.ChangePopularity(ctx, videoID, -1)
}
