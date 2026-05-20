package video

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
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
	username := strings.TrimSpace(input.Username)
	playURL := strings.TrimSpace(input.PlayURL)
	coverURL := strings.TrimSpace(input.CoverURL)
	if input.AuthorID == 0 || username == "" || title == "" || playURL == "" || coverURL == "" {
		return nil, ErrInvalidInput
	}

	video := &Video{
		AuthorID:    input.AuthorID,
		Username:    username,
		Title:       title,
		Description: strings.TrimSpace(input.Description),
		PlayURL:     playURL,
		CoverURL:    coverURL,
	}
	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(video).Error; err != nil {
			return err
		}
		for _, tagName := range ExtractTags(video.Title + " " + video.Description) {
			var tag Tag
			if err := tx.Where("name = ?", tagName).FirstOrCreate(&tag, Tag{Name: tagName}).Error; err != nil {
				return err
			}
			if err := tx.FirstOrCreate(&VideoTag{}, VideoTag{VideoID: video.ID, TagID: tag.ID}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
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

func (s *Service) Delete(ctx context.Context, id uint, authorID uint) error {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item.AuthorID != authorID {
		return ErrUnauthorized
	}
	return s.repo.Delete(ctx, id)
}
