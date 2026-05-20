package rabbitmq

import (
	"context"
	"errors"
	"time"
)

const (
	CommentExchange   = "comment.events"
	CommentQueue      = "comment.events"
	CommentBindingKey = "comment.*"

	commentRoutePublish = "comment.publish"
	commentRouteDelete  = "comment.delete"
)

type CommentMQ struct {
	*RabbitMQ
}

type CommentEvent struct {
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	CommentID  uint      `json:"comment_id,omitempty"`
	Username   string    `json:"username,omitempty"`
	VideoID    uint      `json:"video_id,omitempty"`
	AuthorID   uint      `json:"author_id,omitempty"`
	Content    string    `json:"content,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewCommentMQ(base *RabbitMQ) (*CommentMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	if err := base.DeclareTopic(CommentExchange, CommentQueue, CommentBindingKey); err != nil {
		return nil, err
	}
	return &CommentMQ{RabbitMQ: base}, nil
}

func (m *CommentMQ) Publish(ctx context.Context, username string, videoID uint, authorID uint, content string) error {
	return m.publish(ctx, "publish", commentRoutePublish, CommentEvent{
		Username: username,
		VideoID:  videoID,
		AuthorID: authorID,
		Content:  content,
	})
}

func (m *CommentMQ) Delete(ctx context.Context, commentID uint) error {
	return m.publish(ctx, "delete", commentRouteDelete, CommentEvent{CommentID: commentID})
}

func (m *CommentMQ) publish(ctx context.Context, action string, routingKey string, event CommentEvent) error {
	if m == nil || m.RabbitMQ == nil {
		return errors.New("comment mq is not initialized")
	}
	eventID, err := newEventID(16)
	if err != nil {
		return err
	}
	event.EventID = eventID
	event.Action = action
	event.OccurredAt = time.Now().UTC()
	return m.PublishJSON(ctx, CommentExchange, routingKey, event)
}
