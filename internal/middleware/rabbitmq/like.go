package rabbitmq

import (
	"context"
	"errors"
	"time"
)

const (
	LikeExchange   = "like.events"
	LikeQueue      = "like.events"
	LikeBindingKey = "like.*"

	likeRouteLike   = "like.like"
	likeRouteUnlike = "like.unlike"
)

type LikeMQ struct {
	*RabbitMQ
}

type LikeEvent struct {
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	UserID     uint      `json:"user_id"`
	VideoID    uint      `json:"video_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewLikeMQ(base *RabbitMQ) (*LikeMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	if err := base.DeclareTopic(LikeExchange, LikeQueue, LikeBindingKey); err != nil {
		return nil, err
	}
	return &LikeMQ{RabbitMQ: base}, nil
}

func (m *LikeMQ) Like(ctx context.Context, userID uint, videoID uint) error {
	return m.publish(ctx, "like", likeRouteLike, userID, videoID)
}

func (m *LikeMQ) Unlike(ctx context.Context, userID uint, videoID uint) error {
	return m.publish(ctx, "unlike", likeRouteUnlike, userID, videoID)
}

func (m *LikeMQ) publish(ctx context.Context, action string, routingKey string, userID uint, videoID uint) error {
	if m == nil || m.RabbitMQ == nil {
		return errors.New("like mq is not initialized")
	}
	if userID == 0 || videoID == 0 {
		return errors.New("user_id and video_id are required")
	}
	eventID, err := newEventID(16)
	if err != nil {
		return err
	}
	return m.PublishJSON(ctx, LikeExchange, routingKey, LikeEvent{
		EventID:    eventID,
		Action:     action,
		UserID:     userID,
		VideoID:    videoID,
		OccurredAt: time.Now().UTC(),
	})
}
