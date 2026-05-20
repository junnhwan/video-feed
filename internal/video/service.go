package video

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidInput = errors.New("author_id, title, play_url and cover_url are required")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Publish(ctx context.Context, input PublishInput) (*Video, error) {
	title := strings.TrimSpace(input.Title)
	playURL := strings.TrimSpace(input.PlayURL)
	coverURL := strings.TrimSpace(input.CoverURL)
	if input.AuthorID == 0 || title == "" || playURL == "" || coverURL == "" {
		return nil, ErrInvalidInput
	}

	video := &Video{
		AuthorID:    input.AuthorID,
		Title:       title,
		Description: strings.TrimSpace(input.Description),
		PlayURL:     playURL,
		CoverURL:    coverURL,
	}
	if err := s.repo.Create(ctx, video); err != nil {
		return nil, err
	}
	return video, nil
}

func (s *Service) GetDetail(ctx context.Context, id uint) (*Video, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ListByAuthorID(ctx context.Context, authorID uint) ([]Video, error) {
	return s.repo.ListByAuthorID(ctx, authorID)
}
