package worker

import (
	"context"
	"encoding/json"
	"errors"

	"video-feed/internal/middleware/rabbitmq"
	"video-feed/internal/observability"
	"video-feed/internal/social"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type SocialWorker struct {
	ch    *amqp.Channel
	repo  *social.Repository
	queue string
}

func NewSocialWorker(ch *amqp.Channel, repo *social.Repository, queue string) *SocialWorker {
	return &SocialWorker{ch: ch, repo: repo, queue: queue}
}

func (w *SocialWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.repo == nil {
		return errors.New("social worker is not initialized")
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

func (w *SocialWorker) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	if err := w.process(ctx, delivery.Body); err != nil {
		retryCount := rabbitmq.GetRetryCount(delivery)
		if retryCount >= rabbitmq.MaxRetryCount {
			observability.WithContext(ctx).Error("social worker max retries exceeded",
				zap.Int("retry", retryCount), zap.Error(err))
			observability.MQConsumeTotal.WithLabelValues(w.queue, "drop").Inc()
			_ = delivery.Ack(false)
			return
		}
		observability.MQConsumeTotal.WithLabelValues(w.queue, "retry").Inc()
		_ = delivery.Nack(false, true)
		return
	}
	observability.MQConsumeTotal.WithLabelValues(w.queue, "success").Inc()
	_ = delivery.Ack(false)
}

func (w *SocialWorker) process(ctx context.Context, body []byte) error {
	var event rabbitmq.SocialEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil
	}
	if event.FollowerID == 0 || event.VloggerID == 0 {
		return nil
	}
	switch event.Action {
	case "follow":
		return w.repo.FollowIgnoreDuplicate(ctx, event.FollowerID, event.VloggerID)
	case "unfollow":
		return w.repo.Unfollow(ctx, event.FollowerID, event.VloggerID)
	default:
		return nil
	}
}
