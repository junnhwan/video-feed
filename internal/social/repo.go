package social

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

func (r *Repository) Follow(ctx context.Context, followerID uint, vloggerID uint) error {
	return r.db.WithContext(ctx).Create(&Social{FollowerID: followerID, VloggerID: vloggerID}).Error
}

func (r *Repository) Unfollow(ctx context.Context, followerID uint, vloggerID uint) error {
	return r.db.WithContext(ctx).
		Where("follower_id = ? AND vlogger_id = ?", followerID, vloggerID).
		Delete(&Social{}).Error
}

func (r *Repository) IsFollowed(ctx context.Context, followerID uint, vloggerID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Social{}).
		Where("follower_id = ? AND vlogger_id = ?", followerID, vloggerID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
