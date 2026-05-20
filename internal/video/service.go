package video

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	rediscache "video-feed/internal/middleware/redis"

	"gorm.io/gorm"
)

var ErrInvalidInput = errors.New("author_id, title, play_url and cover_url are required")

type Service struct {
	repo           *Repository
	cache          *rediscache.Client
	detailCache    map[uint]detailCacheEntry
	detailCacheMu  sync.RWMutex
	detailCacheTTL time.Duration
	detailRedisTTL time.Duration
}

type detailCacheEntry struct {
	video     Video
	expiresAt time.Time
}

func NewService(repo *Repository, cache ...*rediscache.Client) *Service {
	service := &Service{
		repo:           repo,
		detailCache:    map[uint]detailCacheEntry{},
		detailCacheTTL: 3 * time.Second,
		detailRedisTTL: 5 * time.Minute,
	}
	if len(cache) > 0 {
		service.cache = cache[0]
	}
	return service
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
	if cached, ok := s.getLocalDetail(id); ok {
		return cached, nil
	}
	if cached, ok := s.getRedisDetail(ctx, id); ok {
		s.setLocalDetail(cached)
		return cached, nil
	}
	if s.cache != nil {
		return s.getDetailWithRedisRebuild(ctx, id)
	}

	video, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.setLocalDetail(video)
	s.setRedisDetail(ctx, video)
	return video, nil
}

func (s *Service) getDetailWithRedisRebuild(ctx context.Context, id uint) (*Video, error) {
	cacheKey := s.cache.Key("video:detail:id=%d", id)
	lockKey := "lock:" + cacheKey
	lockCtx, lockCancel := context.WithTimeout(ctx, 50*time.Millisecond)
	token, locked, lockErr := s.cache.Lock(lockCtx, lockKey, 2*time.Second)
	lockCancel()
	if lockErr == nil && locked {
		defer func() { _ = s.cache.Unlock(context.Background(), lockKey, token) }()
		if cached, ok := s.getRedisDetail(ctx, id); ok {
			s.setLocalDetail(cached)
			return cached, nil
		}
		video, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		s.setLocalDetail(video)
		s.setRedisDetail(ctx, video)
		return video, nil
	}

	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
		if cached, ok := s.getRedisDetail(ctx, id); ok {
			s.setLocalDetail(cached)
			return cached, nil
		}
	}

	video, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.setLocalDetail(video)
	s.setRedisDetail(ctx, video)
	return video, nil
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
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateDetailCache(id)
	return nil
}

func (s *Service) getLocalDetail(id uint) (*Video, bool) {
	if s.detailCache == nil {
		return nil, false
	}
	s.detailCacheMu.RLock()
	entry, ok := s.detailCache[id]
	s.detailCacheMu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		s.detailCacheMu.Lock()
		delete(s.detailCache, id)
		s.detailCacheMu.Unlock()
		return nil, false
	}
	video := entry.video
	return &video, true
}

func (s *Service) setLocalDetail(video *Video) {
	if video == nil || s.detailCache == nil {
		return
	}
	s.detailCacheMu.Lock()
	s.detailCache[video.ID] = detailCacheEntry{
		video:     *video,
		expiresAt: time.Now().Add(s.detailCacheTTL),
	}
	s.detailCacheMu.Unlock()
}

func (s *Service) getRedisDetail(ctx context.Context, id uint) (*Video, bool) {
	if s.cache == nil {
		return nil, false
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	payload, err := s.cache.GetBytes(cacheCtx, s.cache.Key("video:detail:id=%d", id))
	if err != nil {
		return nil, false
	}
	var cached Video
	if err := json.Unmarshal(payload, &cached); err != nil {
		return nil, false
	}
	return &cached, true
}

func (s *Service) setRedisDetail(ctx context.Context, video *Video) {
	if s.cache == nil || video == nil {
		return
	}
	payload, err := json.Marshal(video)
	if err != nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_ = s.cache.SetBytes(cacheCtx, s.cache.Key("video:detail:id=%d", video.ID), payload, s.detailRedisTTL)
}

func (s *Service) invalidateDetailCache(id uint) {
	if s.detailCache != nil {
		s.detailCacheMu.Lock()
		delete(s.detailCache, id)
		s.detailCacheMu.Unlock()
	}
	if s.cache != nil {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_ = s.cache.Del(cacheCtx, s.cache.Key("video:detail:id=%d", id))
	}
}
