package message

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidMessageInput = errors.New("to_id and content are required")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Send(ctx context.Context, fromID uint, toID uint, content string) (*Message, error) {
	content = strings.TrimSpace(content)
	if fromID == 0 || toID == 0 || content == "" {
		return nil, ErrInvalidMessageInput
	}
	message := &Message{
		FromID:  fromID,
		ToID:    toID,
		Content: content,
	}
	if err := s.repo.Send(ctx, message); err != nil {
		return nil, err
	}
	return message, nil
}

func (s *Service) List(ctx context.Context, userID uint, peerID uint, limit int) ([]Message, error) {
	if userID == 0 || peerID == 0 {
		return nil, ErrInvalidMessageInput
	}
	messages, err := s.repo.List(ctx, userID, peerID, limit)
	if messages == nil {
		messages = []Message{}
	}
	return messages, err
}
