package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"video-feed/internal/middleware/rabbitmq"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/observability"
	"video-feed/internal/video"

	amqp "github.com/rabbitmq/amqp091-go"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type TimelinePublisher interface {
	PublishVideo(ctx context.Context, videoID uint, createTime time.Time) error
}

type TimelineConsumer struct {
	cache *rediscache.Client
}

func NewTimelineConsumer(cache *rediscache.Client) *TimelineConsumer {
	return &TimelineConsumer{cache: cache}
}

func (c *TimelineConsumer) Process(ctx context.Context, event rabbitmq.TimelineEvent) error {
	if c == nil || c.cache == nil {
		return errors.New("timeline consumer is not initialized")
	}
	if event.VideoID == 0 || event.CreateTime == 0 {
		return nil
	}
	key := c.cache.Key("feed:global_timeline")
	member := fmt.Sprintf("%d", event.VideoID)
	if err := c.cache.ZAdd(ctx, key, goredis.Z{Score: float64(event.CreateTime), Member: member}); err != nil {
		return err
	}
	return c.cache.ZRemRangeByRank(ctx, key, 0, -1001)
}

type TimelineWorker struct {
	ch       *amqp.Channel
	queue    string
	consumer *TimelineConsumer
}

func NewTimelineWorker(ch *amqp.Channel, cache *rediscache.Client, queue string) *TimelineWorker {
	return &TimelineWorker{ch: ch, queue: queue, consumer: NewTimelineConsumer(cache)}
}

func (w *TimelineWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.consumer == nil {
		return errors.New("timeline worker is not initialized")
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

func (w *TimelineWorker) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	var event rabbitmq.TimelineEvent
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		observability.MQConsumeTotal.WithLabelValues(w.queue, "drop").Inc()
		_ = delivery.Ack(false)
		return
	}
	if err := w.consumer.Process(ctx, event); err != nil {
		retryCount := rabbitmq.GetRetryCount(delivery)
		if retryCount >= rabbitmq.MaxRetryCount {
			observability.WithContext(ctx).Error("timeline worker max retries exceeded",
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

func StartOutboxPoller(ctx context.Context, database *gorm.DB, publisher TimelinePublisher) {
	if database == nil || publisher == nil {
		observability.L().Warn("outbox poller disabled: timeline publisher is not initialized")
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				publishPendingOutbox(ctx, database, publisher)
			}
		}
	}()
}

func publishPendingOutbox(ctx context.Context, database *gorm.DB, publisher TimelinePublisher) {
	var messages []video.OutboxMsg
	if err := database.WithContext(ctx).
		Where("status = ?", "pending").
		Order("create_time ASC, id ASC").
		Limit(100).
		Find(&messages).Error; err != nil {
		observability.WithContext(ctx).Error("outbox poller query failed", zap.Error(err))
		return
	}
	observability.OutboxPendingGauge.Set(float64(len(messages)))
	for _, msg := range messages {
		if err := publisher.PublishVideo(ctx, msg.VideoID, msg.CreateTime); err != nil {
			observability.WithContext(ctx).Error("outbox poller publish failed",
				zap.Uint("video_id", msg.VideoID), zap.Error(err))
			continue
		}
		if err := database.WithContext(ctx).Delete(&msg).Error; err != nil {
			observability.WithContext(ctx).Error("outbox poller delete failed",
				zap.Uint("msg_id", msg.ID), zap.Error(err))
		}
	}
}
