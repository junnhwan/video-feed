package rabbitmq

import (
	"context"
	"errors"
	"time"
)

const (
	TimelineExchange   = "video.timeline.events"
	TimelineQueue      = "video.timeline.update.queue"
	TimelineBindingKey = "video.timeline.*"

	timelinePublishRoute = "video.timeline.publish"
)

type TimelineMQ struct {
	*RabbitMQ
}

type TimelineEvent struct {
	EventID    string    `json:"event_id"`
	VideoID    uint      `json:"video_id"`
	CreateTime int64     `json:"create_time"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewTimelineMQ(base *RabbitMQ) (*TimelineMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	if err := base.DeclareTopic(TimelineExchange, TimelineQueue, TimelineBindingKey); err != nil {
		return nil, err
	}
	return &TimelineMQ{RabbitMQ: base}, nil
}

func (m *TimelineMQ) PublishVideo(ctx context.Context, videoID uint, createTime time.Time) error {
	if m == nil || m.RabbitMQ == nil {
		return errors.New("timeline mq is not initialized")
	}
	if videoID == 0 || createTime.IsZero() {
		return errors.New("video_id and create_time are required")
	}
	eventID, err := newEventID(16)
	if err != nil {
		return err
	}
	return m.PublishJSON(ctx, TimelineExchange, timelinePublishRoute, TimelineEvent{
		EventID:    eventID,
		VideoID:    videoID,
		CreateTime: createTime.UnixMilli(),
		OccurredAt: time.Now().UTC(),
	})
}
