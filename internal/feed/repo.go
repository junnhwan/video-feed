package feed

import (
	"context"
	"time"

	"video-feed/internal/social"
	"video-feed/internal/video"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListLatest(ctx context.Context, limit int, latestBefore time.Time) ([]*video.Video, error) {
	var videos []*video.Video
	query := r.db.WithContext(ctx).Model(&video.Video{}).Order("created_at DESC, id DESC")
	if !latestBefore.IsZero() {
		query = query.Where("created_at < ?", latestBefore.UTC())
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *Repository) ListLikesCountWithCursor(ctx context.Context, limit int, cursor *LikesCountCursor) ([]*video.Video, error) {
	var videos []*video.Video
	query := r.db.WithContext(ctx).Model(&video.Video{}).Order("likes_count DESC, id DESC")
	if cursor != nil {
		query = query.Where("(likes_count < ?) OR (likes_count = ? AND id < ?)", cursor.LikesCount, cursor.LikesCount, cursor.ID)
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *Repository) ListByFollowing(ctx context.Context, limit int, viewerAccountID uint, latestBefore time.Time) ([]*video.Video, error) {
	var videos []*video.Video
	query := r.db.WithContext(ctx).Model(&video.Video{}).Order("created_at DESC, id DESC")
	followingSubQuery := r.db.WithContext(ctx).
		Model(&social.Social{}).
		Select("vlogger_id").
		Where("follower_id = ?", viewerAccountID)
	query = query.Where("author_id IN (?)", followingSubQuery)
	if !latestBefore.IsZero() {
		query = query.Where("created_at < ?", latestBefore.UTC())
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *Repository) ListByPopularity(ctx context.Context, limit int, popularityBefore int64, timeBefore time.Time, idBefore uint) ([]*video.Video, error) {
	var videos []*video.Video
	query := r.db.WithContext(ctx).Model(&video.Video{}).Order("popularity DESC, created_at DESC, id DESC")
	if !timeBefore.IsZero() && idBefore > 0 {
		query = query.Where(
			"(popularity < ?) OR (popularity = ? AND created_at < ?) OR (popularity = ? AND created_at = ? AND id < ?)",
			popularityBefore,
			popularityBefore, timeBefore,
			popularityBefore, timeBefore, idBefore,
		)
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}
