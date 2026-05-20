package rabbitmq

type Publishers struct {
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
	return &Publishers{
		Like:       likeMQ,
		Comment:    commentMQ,
		Popularity: popularityMQ,
		Social:     socialMQ,
	}, nil
}
