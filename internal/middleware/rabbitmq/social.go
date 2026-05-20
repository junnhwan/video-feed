package rabbitmq

import (
	"context"
	"errors"
	"time"
)

const (
	SocialExchange   = "social.events"
	SocialQueue      = "social.events"
	SocialBindingKey = "social.*"

	socialRouteFollow   = "social.follow"
	socialRouteUnfollow = "social.unfollow"
)

type SocialMQ struct {
	*RabbitMQ
}

type SocialEvent struct {
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	FollowerID uint      `json:"follower_id"`
	VloggerID  uint      `json:"vlogger_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewSocialMQ(base *RabbitMQ) (*SocialMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	if err := base.DeclareTopic(SocialExchange, SocialQueue, SocialBindingKey); err != nil {
		return nil, err
	}
	return &SocialMQ{RabbitMQ: base}, nil
}

func (m *SocialMQ) Follow(ctx context.Context, followerID uint, vloggerID uint) error {
	return m.publish(ctx, "follow", socialRouteFollow, followerID, vloggerID)
}

func (m *SocialMQ) Unfollow(ctx context.Context, followerID uint, vloggerID uint) error {
	return m.publish(ctx, "unfollow", socialRouteUnfollow, followerID, vloggerID)
}

func (m *SocialMQ) publish(ctx context.Context, action string, routingKey string, followerID uint, vloggerID uint) error {
	if m == nil || m.RabbitMQ == nil {
		return errors.New("social mq is not initialized")
	}
	if followerID == 0 || vloggerID == 0 {
		return errors.New("follower_id and vlogger_id are required")
	}
	eventID, err := newEventID(16)
	if err != nil {
		return err
	}
	return m.PublishJSON(ctx, SocialExchange, routingKey, SocialEvent{
		EventID:    eventID,
		Action:     action,
		FollowerID: followerID,
		VloggerID:  vloggerID,
		OccurredAt: time.Now().UTC(),
	})
}
