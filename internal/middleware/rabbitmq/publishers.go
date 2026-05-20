package rabbitmq

type Publishers struct {
	Base       *RabbitMQ
	Like       *LikeMQ
	Comment    *CommentMQ
	Popularity *PopularityMQ
	Social     *SocialMQ
}

func NewPublishers(base *RabbitMQ) (*Publishers, error) {
	likeMQ, err := NewLikeMQ(base)
	if err != nil {
		return nil, err
	}
	commentMQ, err := NewCommentMQ(base)
	if err != nil {
		return nil, err
	}
	popularityMQ, err := NewPopularityMQ(base)
	if err != nil {
		return nil, err
	}
	socialMQ, err := NewSocialMQ(base)
	if err != nil {
		return nil, err
	}
	if err := DeclareNotificationTopology(base); err != nil {
		return nil, err
	}
	return &Publishers{
		Base:       base,
		Like:       likeMQ,
		Comment:    commentMQ,
		Popularity: popularityMQ,
		Social:     socialMQ,
	}, nil
}
