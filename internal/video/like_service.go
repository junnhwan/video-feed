package video

import (
	"context"
	"errors"

	rediscache "video-feed/internal/middleware/redis"

	"gorm.io/gorm"
)

var (
	ErrAlreadyLiked  = errors.New("user has liked this video")
	ErrNotLiked      = errors.New("user has not liked this video")
	ErrVideoNotFound = errors.New("video not found")
)

type LikeService struct {
	repo          *LikeRepository
	videoRepo     *Repository
	cache         *rediscache.Client
	likePublisher LikeEventPublisher
	popPublisher  PopularityEventPublisher
}

type LikeEventPublisher interface {
	Like(ctx context.Context, userID uint, videoID uint) error
	Unlike(ctx context.Context, userID uint, videoID uint) error
}

type PopularityEventPublisher interface {
	Update(ctx context.Context, videoID uint, change int64) error
}

func NewLikeService(repo *LikeRepository, videoRepo *Repository, cache *rediscache.Client) *LikeService {
	return &LikeService{repo: repo, videoRepo: videoRepo, cache: cache}
}

func (s *LikeService) SetPublishers(likePublisher LikeEventPublisher, popularityPublisher PopularityEventPublisher) {
	s.likePublisher = likePublisher
	s.popPublisher = popularityPublisher
}

func (s *LikeService) Like(ctx context.Context, videoID uint, accountID uint) error {
	if videoID == 0 || accountID == 0 {
		return errors.New("video_id and account_id are required")
	}
	if ok, err := s.videoRepo.IsExist(ctx, videoID); err != nil {
		return err
	} else if !ok {
		return ErrVideoNotFound
	}
	liked, err := s.repo.IsLiked(ctx, videoID, accountID)
	if err != nil {
		return err
	}
	if liked {
		return ErrAlreadyLiked
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.likePublisher != nil {
		if err := s.likePublisher.Like(ctx, accountID, videoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popPublisher != nil {
		if err := s.popPublisher.Update(ctx, videoID, 1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	if !mysqlEnqueued {
		if err := s.applyLike(ctx, videoID, accountID); err != nil {
			return err
		}
	}
	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, videoID, 1)
	}
	return nil
}

func (s *LikeService) Unlike(ctx context.Context, videoID uint, accountID uint) error {
	if videoID == 0 || accountID == 0 {
		return errors.New("video_id and account_id are required")
	}
	if ok, err := s.videoRepo.IsExist(ctx, videoID); err != nil {
		return err
	} else if !ok {
		return ErrVideoNotFound
	}
	liked, err := s.repo.IsLiked(ctx, videoID, accountID)
	if err != nil {
		return err
	}
	if !liked {
		return ErrNotLiked
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.likePublisher != nil {
		if err := s.likePublisher.Unlike(ctx, accountID, videoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popPublisher != nil {
		if err := s.popPublisher.Update(ctx, videoID, -1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	if !mysqlEnqueued {
		if err := s.applyUnlike(ctx, videoID, accountID); err != nil {
			return err
		}
	}
	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, videoID, -1)
	}
	return nil
}

func (s *LikeService) applyLike(ctx context.Context, videoID uint, accountID uint) error {
	return s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&Like{VideoID: videoID, AccountID: accountID}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Video{}).Where("id = ?", videoID).
			UpdateColumn("likes_count", gorm.Expr("likes_count + 1")).Error; err != nil {
			return err
		}
		return tx.Model(&Video{}).Where("id = ?", videoID).
			UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error
	})
}

func (s *LikeService) applyUnlike(ctx context.Context, videoID uint, accountID uint) error {
	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		del := tx.Where("video_id = ? AND account_id = ?", videoID, accountID).Delete(&Like{})
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected == 0 {
			return ErrNotLiked
		}
		if err := tx.Model(&Video{}).Where("id = ?", videoID).
			UpdateColumn("likes_count", gorm.Expr("CASE WHEN likes_count > 0 THEN likes_count - 1 ELSE 0 END")).Error; err != nil {
			return err
		}
		return tx.Model(&Video{}).Where("id = ?", videoID).
			UpdateColumn("popularity", gorm.Expr("CASE WHEN popularity > 0 THEN popularity - 1 ELSE 0 END")).Error
	}); err != nil {
		return err
	}
	return nil
}

func (s *LikeService) IsLiked(ctx context.Context, videoID uint, accountID uint) (bool, error) {
	return s.repo.IsLiked(ctx, videoID, accountID)
}

func (s *LikeService) ListLikedVideos(ctx context.Context, accountID uint) ([]Video, error) {
	return s.repo.ListLikedVideos(ctx, accountID)
}
