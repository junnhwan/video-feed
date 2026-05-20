package feed

import (
	"context"
	"strings"
	"time"

	"video-feed/internal/video"
)

type Service struct {
	repo     *Repository
	likeRepo *video.LikeRepository
}

func NewService(repo *Repository, likeRepo *video.LikeRepository, _ any) *Service {
	return &Service{repo: repo, likeRepo: likeRepo}
}

func (s *Service) ListLatest(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (ListLatestResponse, error) {
	limit = normalizeLimit(limit)
	videos, err := s.repo.ListLatest(ctx, limit, latestBefore)
	if err != nil {
		return ListLatestResponse{}, err
	}
	items, err := s.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListLatestResponse{}, err
	}
	return ListLatestResponse{
		VideoList: items,
		NextTime:  nextTime(videos),
		HasMore:   len(videos) == limit,
	}, nil
}

func (s *Service) ListLikesCount(ctx context.Context, limit int, cursor *LikesCountCursor, viewerAccountID uint) (ListLikesCountResponse, error) {
	limit = normalizeLimit(limit)
	videos, err := s.repo.ListLikesCountWithCursor(ctx, limit, cursor)
	if err != nil {
		return ListLikesCountResponse{}, err
	}
	items, err := s.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListLikesCountResponse{}, err
	}
	resp := ListLikesCountResponse{VideoList: items, HasMore: len(videos) == limit}
	if len(videos) > 0 {
		last := videos[len(videos)-1]
		nextLikes := last.LikesCount
		nextID := last.ID
		resp.NextLikesCountBefore = &nextLikes
		resp.NextIDBefore = &nextID
	}
	return resp, nil
}

func (s *Service) ListByFollowing(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (ListByFollowingResponse, error) {
	limit = normalizeLimit(limit)
	videos, err := s.repo.ListByFollowing(ctx, limit, viewerAccountID, latestBefore)
	if err != nil {
		return ListByFollowingResponse{}, err
	}
	items, err := s.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListByFollowingResponse{}, err
	}
	return ListByFollowingResponse{
		VideoList: items,
		NextTime:  nextTime(videos),
		HasMore:   len(videos) == limit,
	}, nil
}

func (s *Service) ListByPopularity(ctx context.Context, limit int, latestPopularity int64, latestBefore time.Time, latestIDBefore uint, viewerAccountID uint) (ListByPopularityResponse, error) {
	limit = normalizeLimit(limit)
	videos, err := s.repo.ListByPopularity(ctx, limit, latestPopularity, latestBefore, latestIDBefore)
	if err != nil {
		return ListByPopularityResponse{}, err
	}
	items, err := s.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListByPopularityResponse{}, err
	}
	resp := ListByPopularityResponse{VideoList: items, HasMore: len(videos) == limit}
	if len(videos) > 0 {
		last := videos[len(videos)-1]
		nextPopularity := last.Popularity
		nextBefore := last.CreatedAt
		nextID := last.ID
		resp.NextLatestPopularity = &nextPopularity
		resp.NextLatestBefore = &nextBefore
		resp.NextLatestIDBefore = &nextID
	}
	return resp, nil
}

func (s *Service) ListByTag(ctx context.Context, tagName string, limit int, viewerAccountID uint) ([]FeedVideoItem, error) {
	tagName = strings.TrimSpace(strings.TrimPrefix(tagName, "#"))
	if tagName == "" {
		return []FeedVideoItem{}, nil
	}
	videos, err := s.repo.ListByTag(ctx, tagName, normalizeLimit(limit))
	if err != nil {
		return nil, err
	}
	return s.buildFeedVideos(ctx, videos, viewerAccountID)
}

func (s *Service) buildFeedVideos(ctx context.Context, videos []*video.Video, viewerAccountID uint) ([]FeedVideoItem, error) {
	videoIDs := make([]uint, 0, len(videos))
	for _, item := range videos {
		videoIDs = append(videoIDs, item.ID)
	}
	likedMap, err := s.likeRepo.BatchGetLiked(ctx, videoIDs, viewerAccountID)
	if err != nil {
		return nil, err
	}
	items := make([]FeedVideoItem, 0, len(videos))
	for _, item := range videos {
		items = append(items, FeedVideoItem{
			ID:          item.ID,
			Author:      FeedAuthor{ID: item.AuthorID, Username: item.Username},
			Title:       item.Title,
			Description: item.Description,
			PlayURL:     item.PlayURL,
			CoverURL:    item.CoverURL,
			CreateTime:  item.CreatedAt.UnixMilli(),
			LikesCount:  item.LikesCount,
			IsLiked:     likedMap[item.ID],
		})
	}
	return items, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 50 {
		return 10
	}
	return limit
}

func nextTime(videos []*video.Video) int64 {
	if len(videos) == 0 {
		return 0
	}
	return videos[len(videos)-1].CreatedAt.UnixMilli()
}
