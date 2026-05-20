package video

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LikeRepository struct {
	db *gorm.DB
}

func NewLikeRepository(db *gorm.DB) *LikeRepository {
	return &LikeRepository{db: db}
}

func (r *LikeRepository) IsLiked(ctx context.Context, videoID uint, accountID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Like{}).
		Where("video_id = ? AND account_id = ?", videoID, accountID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *LikeRepository) LikeIgnoreDuplicate(ctx context.Context, like *Like) (bool, error) {
	if like == nil || like.VideoID == 0 || like.AccountID == 0 {
		return false, nil
	}
	res := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(like)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *LikeRepository) DeleteByVideoAndAccount(ctx context.Context, videoID uint, accountID uint) (bool, error) {
	if videoID == 0 || accountID == 0 {
		return false, nil
	}
	res := r.db.WithContext(ctx).
		Where("video_id = ? AND account_id = ?", videoID, accountID).
		Delete(&Like{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *LikeRepository) BatchGetLiked(ctx context.Context, videoIDs []uint, accountID uint) (map[uint]bool, error) {
	liked := make(map[uint]bool)
	if accountID == 0 || len(videoIDs) == 0 {
		return liked, nil
	}

	var likes []Like
	if err := r.db.WithContext(ctx).Model(&Like{}).
		Where("video_id IN ? AND account_id = ?", videoIDs, accountID).
		Find(&likes).Error; err != nil {
		return nil, err
	}
	for _, like := range likes {
		liked[like.VideoID] = true
	}
	return liked, nil
}

func (r *LikeRepository) ListLikedVideos(ctx context.Context, accountID uint) ([]Video, error) {
	var videos []Video
	if accountID == 0 {
		return videos, nil
	}
	err := r.db.WithContext(ctx).
		Model(&Video{}).
		Joins("JOIN likes ON likes.video_id = videos.id").
		Where("likes.account_id = ?", accountID).
		Order("likes.created_at DESC").
		Limit(200).
		Find(&videos).Error
	return videos, err
}
