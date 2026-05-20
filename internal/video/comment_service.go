package video

import (
	"context"
	"errors"
	"strings"

	rediscache "video-feed/internal/middleware/redis"

	"gorm.io/gorm"
)

var (
	ErrCommentNotFound = errors.New("comment not found")
	ErrUnauthorized    = errors.New("unauthorized")
)

type CommentService struct {
	repo      *CommentRepository
	videoRepo *Repository
	cache     *rediscache.Client
}

func NewCommentService(repo *CommentRepository, videoRepo *Repository, cache *rediscache.Client) *CommentService {
	return &CommentService{repo: repo, videoRepo: videoRepo, cache: cache}
}

func (s *CommentService) Publish(ctx context.Context, input PublishCommentInput) (*Comment, error) {
	comment := &Comment{
		VideoID:  input.VideoID,
		AuthorID: input.AuthorID,
		Username: strings.TrimSpace(input.Username),
		Content:  strings.TrimSpace(input.Content),
	}
	if comment.VideoID == 0 || comment.AuthorID == 0 {
		return nil, errors.New("video_id and author_id are required")
	}
	if comment.Content == "" {
		return nil, errors.New("content is required")
	}
	if ok, err := s.videoRepo.IsExist(ctx, comment.VideoID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrVideoNotFound
	}

	err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return err
		}
		return tx.Model(&Video{}).Where("id = ?", comment.VideoID).
			UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error
	})
	if err != nil {
		return nil, err
	}
	UpdatePopularityCache(ctx, s.cache, comment.VideoID, 1)
	return comment, nil
}

func (s *CommentService) Delete(ctx context.Context, commentID uint, accountID uint) error {
	comment, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment == nil {
		return ErrCommentNotFound
	}
	if comment.AuthorID != accountID {
		return ErrUnauthorized
	}
	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(comment).Error; err != nil {
			return err
		}
		return tx.Model(&Video{}).Where("id = ?", comment.VideoID).
			UpdateColumn("popularity", gorm.Expr("CASE WHEN popularity > 0 THEN popularity - 1 ELSE 0 END")).Error
	}); err != nil {
		return err
	}
	UpdatePopularityCache(ctx, s.cache, comment.VideoID, -1)
	return nil
}

func (s *CommentService) GetAll(ctx context.Context, videoID uint) ([]Comment, error) {
	if videoID == 0 {
		return nil, errors.New("video_id is required")
	}
	if ok, err := s.videoRepo.IsExist(ctx, videoID); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrVideoNotFound
	}
	return s.repo.GetAllComments(ctx, videoID)
}
