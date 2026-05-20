package video

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, video *Video) error {
	return r.db.WithContext(ctx).Create(video).Error
}

func (r *Repository) GetByID(ctx context.Context, id uint) (*Video, error) {
	var video Video
	if err := r.db.WithContext(ctx).First(&video, id).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

func (r *Repository) ListByAuthorID(ctx context.Context, authorID uint) ([]Video, error) {
	var videos []Video
	if err := r.db.WithContext(ctx).
		Where("author_id = ?", authorID).
		Order("created_at DESC, id DESC").
		Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *Repository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("video_id = ?", id).Delete(&VideoTag{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Video{}, id).Error
	})
}

func (r *Repository) IsExist(ctx context.Context, id uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Video{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) ChangeLikesCount(ctx context.Context, id uint, change int64) error {
	return r.db.WithContext(ctx).Model(&Video{}).Where("id = ?", id).
		UpdateColumn("likes_count", gorm.Expr("CASE WHEN likes_count + ? > 0 THEN likes_count + ? ELSE 0 END", change, change)).Error
}

func (r *Repository) ChangePopularity(ctx context.Context, id uint, change int64) error {
	return r.db.WithContext(ctx).Model(&Video{}).Where("id = ?", id).
		UpdateColumn("popularity", gorm.Expr("CASE WHEN popularity + ? > 0 THEN popularity + ? ELSE 0 END", change, change)).Error
}
