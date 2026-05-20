package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"video-feed/internal/middleware/rabbitmq"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/video"

	amqp "github.com/rabbitmq/amqp091-go"
)

type PopularityWorker struct {
	ch    *amqp.Channel
	cache *rediscache.Client
	queue string
}

func NewPopularityWorker(ch *amqp.Channel, cache *rediscache.Client, queue string) *PopularityWorker {
	return &PopularityWorker{ch: ch, cache: cache, queue: queue}
}

func (w *PopularityWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.cache == nil {
		return errors.New("popularity worker is not initialized")
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

func (w *PopularityWorker) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	if err := w.process(ctx, delivery.Body); err != nil {
		retryCount := rabbitmq.GetRetryCount(delivery)
		if retryCount >= rabbitmq.MaxRetryCount {
			log.Printf("popularity worker: max retries exceeded (%d): %v", retryCount, err)
			_ = delivery.Ack(false)
			return
		}
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}

func (w *PopularityWorker) process(ctx context.Context, body []byte) error {
	var event rabbitmq.PopularityEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil
	}
	if event.VideoID == 0 || event.Change == 0 {
		return nil
	}
	video.UpdatePopularityCache(ctx, w.cache, event.VideoID, event.Change)
	return nil
}
