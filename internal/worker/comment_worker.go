package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"

	"video-feed/internal/middleware/rabbitmq"
	"video-feed/internal/video"

	amqp "github.com/rabbitmq/amqp091-go"
)

type CommentWorker struct {
	ch       *amqp.Channel
	comments *video.CommentRepository
	videos   *video.Repository
	queue    string
}

func NewCommentWorker(ch *amqp.Channel, comments *video.CommentRepository, videos *video.Repository, queue string) *CommentWorker {
	return &CommentWorker{ch: ch, comments: comments, videos: videos, queue: queue}
}

func (w *CommentWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.comments == nil || w.videos == nil {
		return errors.New("comment worker is not initialized")
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

func (w *CommentWorker) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	if err := w.process(ctx, delivery.Body); err != nil {
		retryCount := rabbitmq.GetRetryCount(delivery)
		if retryCount >= rabbitmq.MaxRetryCount {
			log.Printf("comment worker: max retries exceeded (%d): %v", retryCount, err)
			_ = delivery.Ack(false)
			return
		}
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}

func (w *CommentWorker) process(ctx context.Context, body []byte) error {
	var event rabbitmq.CommentEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil
	}
	switch event.Action {
	case "publish":
		return w.applyPublish(ctx, event)
	case "delete":
		return w.applyDelete(ctx, event)
	default:
		return nil
	}
}

func (w *CommentWorker) applyPublish(ctx context.Context, event rabbitmq.CommentEvent) error {
	content := strings.TrimSpace(event.Content)
	if event.VideoID == 0 || event.AuthorID == 0 || content == "" {
		return nil
	}
	exists, err := w.videos.IsExist(ctx, event.VideoID)
	if err != nil || !exists {
		return err
	}
	comment := &video.Comment{
		Username: strings.TrimSpace(event.Username),
		VideoID:  event.VideoID,
		AuthorID: event.AuthorID,
		Content:  content,
	}
	if err := w.comments.CreateComment(ctx, comment); err != nil {
		return err
	}
	return w.videos.ChangePopularity(ctx, event.VideoID, 1)
}

func (w *CommentWorker) applyDelete(ctx context.Context, event rabbitmq.CommentEvent) error {
	if event.CommentID == 0 {
		return nil
	}
	comment, err := w.comments.GetByID(ctx, event.CommentID)
	if err != nil || comment == nil {
		return err
	}
	if err := w.comments.DeleteComment(ctx, comment); err != nil {
		return err
	}
	return w.videos.ChangePopularity(ctx, comment.VideoID, -1)
}
