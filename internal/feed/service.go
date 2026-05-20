package feed

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/video"

	goredis "github.com/redis/go-redis/v9"
)

type Service struct {
	repo     *Repository
	likeRepo *video.LikeRepository
	cache    *rediscache.Client
}

func NewService(repo *Repository, likeRepo *video.LikeRepository, cache *rediscache.Client) *Service {
	return &Service{repo: repo, likeRepo: likeRepo, cache: cache}
}

func (s *Service) ListLatest(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (ListLatestResponse, error) {
	limit = normalizeLimit(limit)
	videos, fromTimeline, err := s.listLatestFromGlobalTimeline(ctx, limit, latestBefore)
	if err != nil {
		return ListLatestResponse{}, err
	}
	if !fromTimeline {
		videos, err = s.repo.ListLatest(ctx, limit, latestBefore)
	}
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

func (s *Service) listLatestFromGlobalTimeline(ctx context.Context, limit int, latestBefore time.Time) ([]*video.Video, bool, error) {
	if s.cache == nil {
		return nil, false, nil
	}
	key := s.cache.Key("feed:global_timeline")
	tail, err := s.cache.ZRangeWithScores(ctx, key, 0, 0)
	if err != nil {
		return nil, false, nil
	}
	if len(tail) == 0 {
		if err := s.rebuildGlobalTimeline(ctx, key); err != nil {
			return nil, false, nil
		}
		tail, err = s.cache.ZRangeWithScores(ctx, key, 0, 0)
		if err != nil {
			return nil, false, nil
		}
		if len(tail) == 0 {
			return []*video.Video{}, true, nil
		}
	}

	reqTime := time.Now().UnixMilli()
	if !latestBefore.IsZero() {
		reqTime = latestBefore.UnixMilli()
	}
	watermark := int64(tail[0].Score)
	if reqTime <= watermark {
		videos, err := s.repo.ListLatest(ctx, limit, latestBefore)
		return videos, true, err
	}

	maxScore := "+inf"
	if !latestBefore.IsZero() {
		maxScore = strconv.FormatInt(reqTime-1, 10)
	}
	members, err := s.cache.ZRevRangeByScore(ctx, key, maxScore, "-inf", 0, int64(limit))
	if err != nil {
		return nil, false, nil
	}
	ids := make([]uint, 0, len(members))
	for _, member := range members {
		id, err := strconv.ParseUint(member, 10, 64)
		if err == nil && id > 0 {
			ids = append(ids, uint(id))
		}
	}
	videos, err := s.videosByIDsInOrder(ctx, ids)
	if err != nil {
		return nil, true, err
	}
	if len(videos) < limit {
		coldCursor := latestBefore
		if len(videos) > 0 {
			coldCursor = videos[len(videos)-1].CreatedAt
		}
		coldVideos, err := s.repo.ListLatest(ctx, limit-len(videos), coldCursor)
		if err != nil {
			return nil, true, err
		}
		videos = append(videos, coldVideos...)
	}
	return videos, true, nil
}

func (s *Service) rebuildGlobalTimeline(ctx context.Context, key string) error {
	videos, err := s.repo.ListLatest(ctx, 1000, time.Time{})
	if err != nil {
		return err
	}
	if len(videos) == 0 {
		return nil
	}
	members := make([]goredis.Z, 0, len(videos))
	for _, item := range videos {
		members = append(members, goredis.Z{
			Score:  float64(item.CreatedAt.UnixMilli()),
			Member: fmt.Sprintf("%d", item.ID),
		})
	}
	return s.cache.ZAdd(ctx, key, members...)
}

func (s *Service) videosByIDsInOrder(ctx context.Context, ids []uint) ([]*video.Video, error) {
	if len(ids) == 0 {
		return []*video.Video{}, nil
	}
	videos, err := s.repo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]*video.Video, len(videos))
	for _, item := range videos {
		byID[item.ID] = item
	}
	ordered := make([]*video.Video, 0, len(ids))
	for _, id := range ids {
		if item := byID[id]; item != nil {
			ordered = append(ordered, item)
		}
	}
	return ordered, nil
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

func (s *Service) ListByPopularity(ctx context.Context, limit int, reqAsOf int64, offset int, viewerAccountID uint, latestPopularity int64, latestBefore time.Time, latestIDBefore uint) (ListByPopularityResponse, error) {
	limit = normalizeLimit(limit)
	if offset < 0 {
		offset = 0
	}
	if s.cache != nil {
		if resp, ok, err := s.listPopularityFromHotSnapshot(ctx, limit, reqAsOf, offset, viewerAccountID); ok || err != nil {
			return resp, err
		}
	}

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

func (s *Service) listPopularityFromHotSnapshot(ctx context.Context, limit int, reqAsOf int64, offset int, viewerAccountID uint) (ListByPopularityResponse, bool, error) {
	asOf := time.Now().UTC().Truncate(time.Minute)
	if reqAsOf > 0 {
		asOf = time.Unix(reqAsOf, 0).UTC().Truncate(time.Minute)
	}

	keys := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		window := asOf.Add(-time.Duration(i) * time.Minute)
		keys = append(keys, s.cache.Key("hot:video:1m:%s", window.Format("200601021504")))
	}
	dest := s.cache.Key("hot:video:merge:1m:%s", asOf.Format("200601021504"))

	opCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()

	exists, err := s.cache.Exists(opCtx, dest)
	if err != nil {
		return ListByPopularityResponse{}, false, nil
	}
	if !exists {
		if err := s.cache.ZUnionStore(opCtx, dest, keys, "SUM"); err != nil {
			return ListByPopularityResponse{}, false, nil
		}
		_ = s.cache.Expire(opCtx, dest, 2*time.Minute)
	}

	start := int64(offset)
	stop := start + int64(limit) - 1
	members, err := s.cache.ZRevRange(opCtx, dest, start, stop)
	if err != nil {
		return ListByPopularityResponse{}, false, nil
	}
	if len(members) == 0 {
		if offset > 0 {
			return ListByPopularityResponse{
				VideoList:  []FeedVideoItem{},
				AsOf:       asOf.Unix(),
				NextOffset: offset,
				HasMore:    false,
			}, true, nil
		}
		return ListByPopularityResponse{}, false, nil
	}

	ids := make([]uint, 0, len(members))
	for _, member := range members {
		id, err := strconv.ParseUint(member, 10, 64)
		if err == nil && id > 0 {
			ids = append(ids, uint(id))
		}
	}
	videos, err := s.repo.GetByIDs(ctx, ids)
	if err != nil {
		return ListByPopularityResponse{}, false, nil
	}
	byID := make(map[uint]*video.Video, len(videos))
	for _, item := range videos {
		byID[item.ID] = item
	}
	ordered := make([]*video.Video, 0, len(ids))
	for _, id := range ids {
		if item := byID[id]; item != nil {
			ordered = append(ordered, item)
		}
	}
	items, err := s.buildFeedVideos(ctx, ordered, viewerAccountID)
	if err != nil {
		return ListByPopularityResponse{}, true, err
	}
	resp := ListByPopularityResponse{
		VideoList:  items,
		AsOf:       asOf.Unix(),
		NextOffset: offset + len(items),
		HasMore:    len(items) == limit,
	}
	if len(ordered) > 0 {
		last := ordered[len(ordered)-1]
		nextPopularity := last.Popularity
		nextBefore := last.CreatedAt
		nextID := last.ID
		resp.NextLatestPopularity = &nextPopularity
		resp.NextLatestBefore = &nextBefore
		resp.NextLatestIDBefore = &nextID
	}
	return resp, true, nil
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
