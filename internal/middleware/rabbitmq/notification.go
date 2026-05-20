package rabbitmq

import "errors"

const (
	NotificationLikeQueue    = "notification.like"
	NotificationCommentQueue = "notification.comment"
	NotificationSocialQueue  = "notification.social"
)

func DeclareNotificationTopology(base *RabbitMQ) error {
	if base == nil {
		return errors.New("rabbitmq base is nil")
	}
	if err := base.DeclareTopic(LikeExchange, NotificationLikeQueue, likeRouteLike); err != nil {
		return err
	}
	if err := base.DeclareTopic(CommentExchange, NotificationCommentQueue, commentRoutePublish); err != nil {
		return err
	}
	return base.DeclareTopic(SocialExchange, NotificationSocialQueue, socialRouteFollow)
}
