package rabbitmq

import (
	"context"
	"errors"
	"time"
)

const (
	PopularityExchange   = "video.popularity.events"
	PopularityQueue      = "video.popularity.events"
	PopularityBindingKey = "video.popularity.*"

	popularityRouteUpdate = "video.popularity.update"
)

type PopularityMQ struct {
	*RabbitMQ
}

type PopularityEvent struct {
	EventID    string    `json:"event_id"`
	VideoID    uint      `json:"video_id"`
	Change     int64     `json:"change"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewPopularityMQ(base *RabbitMQ) (*PopularityMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	if err := base.DeclareTopic(PopularityExchange, PopularityQueue, PopularityBindingKey); err != nil {
		return nil, err
	}
	return &PopularityMQ{RabbitMQ: base}, nil
}

func (m *PopularityMQ) Update(ctx context.Context, videoID uint, change int64) error {
	if m == nil || m.RabbitMQ == nil {
		return errors.New("popularity mq is not initialized")
	}
	if videoID == 0 || change == 0 {
		return errors.New("video_id and change are required")
	}
	eventID, err := newEventID(16)
	if err != nil {
		return err
	}
	return m.PublishJSON(ctx, PopularityExchange, popularityRouteUpdate, PopularityEvent{
		EventID:    eventID,
		VideoID:    videoID,
		Change:     change,
		OccurredAt: time.Now().UTC(),
	})
}
