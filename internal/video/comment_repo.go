package video

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type CommentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) CreateComment(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *CommentRepository) DeleteComment(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Delete(comment).Error
}

func (r *CommentRepository) GetByID(ctx context.Context, id uint) (*Comment, error) {
	var comment Comment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &comment, nil
}

func (r *CommentRepository) GetAllComments(ctx context.Context, videoID uint) ([]Comment, error) {
	var comments []Comment
	err := r.db.WithContext(ctx).
		Where("video_id = ?", videoID).
		Order("created_at ASC, id ASC").
		Limit(200).
		Find(&comments).Error
	return comments, err
}
