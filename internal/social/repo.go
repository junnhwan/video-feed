package social

import (
	"context"

	"video-feed/internal/account"

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

func (r *Repository) GetAllFollowers(ctx context.Context, vloggerID uint) ([]*account.Account, error) {
	var relations []Social
	if err := r.db.WithContext(ctx).Where("vlogger_id = ?", vloggerID).Limit(200).Find(&relations).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(relations))
	for _, relation := range relations {
		ids = append(ids, relation.FollowerID)
	}
	if len(ids) == 0 {
		return []*account.Account{}, nil
	}
	var followers []*account.Account
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&followers).Error; err != nil {
		return nil, err
	}
	return followers, nil
}

func (r *Repository) GetAllVloggers(ctx context.Context, followerID uint) ([]*account.Account, error) {
	var relations []Social
	if err := r.db.WithContext(ctx).Where("follower_id = ?", followerID).Limit(200).Find(&relations).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(relations))
	for _, relation := range relations {
		ids = append(ids, relation.VloggerID)
	}
	if len(ids) == 0 {
		return []*account.Account{}, nil
	}
	var vloggers []*account.Account
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&vloggers).Error; err != nil {
		return nil, err
	}
	return vloggers, nil
}

func (r *Repository) CountFollowers(ctx context.Context, vloggerID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Social{}).Where("vlogger_id = ?", vloggerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) CountVloggers(ctx context.Context, followerID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Social{}).Where("follower_id = ?", followerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
