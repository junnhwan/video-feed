package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/observability"
	"video-feed/internal/video"

	localcache "github.com/patrickmn/go-cache"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type Service struct {
	repo           *Repository
	likeRepo       *video.LikeRepository
	cache          *rediscache.Client
	localCache     *localcache.Cache
	entityCacheTTL time.Duration
	followingTTL   time.Duration
	requestGroup   singleflight.Group
}

func NewService(repo *Repository, likeRepo *video.LikeRepository, cache *rediscache.Client) *Service {
	return &Service{
		repo:           repo,
		likeRepo:       likeRepo,
		cache:          cache,
		localCache:     localcache.New(3*time.Second, 5*time.Second),
		entityCacheTTL: time.Hour,
		followingTTL:   24 * time.Hour,
	}
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
		NextTime:  nextTimeMilli(videos),
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
		sfKey := s.cache.Key("sf:fallback:global_timeline_rebuild")
		_, rebuildErr, _ := s.requestGroup.Do(sfKey, func() (any, error) {
			return nil, s.rebuildGlobalTimeline(ctx, key)
		})
		if rebuildErr != nil {
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
		sfKey := s.cache.Key("sf:cold:listLatest:%d:%d", limit, reqTime)
		value, err, _ := s.requestGroup.Do(sfKey, func() (any, error) {
			return s.repo.ListLatest(ctx, limit, latestBefore)
		})
		if err != nil {
			return nil, true, err
		}
		videos, _ := value.([]*video.Video)
		return videos, true, nil
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
		remainLimit := limit - len(videos)
		sfKey := s.cache.Key("sf:stitch:listLatest:%d:%d", remainLimit, coldCursor.UnixMilli())
		value, err, _ := s.requestGroup.Do(sfKey, func() (any, error) {
			return s.repo.ListLatest(ctx, remainLimit, coldCursor)
		})
		if err != nil {
			return nil, true, err
		}
		coldVideos, _ := value.([]*video.Video)
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
	byID := make(map[uint]*video.Video, len(ids))
	missedL1 := s.loadVideoEntitiesFromLocal(ids, byID)
	missedL2 := s.loadVideoEntitiesFromRedis(ctx, missedL1, byID)
	if err := s.loadVideoEntitiesFromDB(ctx, missedL2, byID); err != nil {
		return nil, err
	}
	ordered := make([]*video.Video, 0, len(ids))
	for _, id := range ids {
		if item := byID[id]; item != nil {
			ordered = append(ordered, item)
		}
	}
	return ordered, nil
}

func (s *Service) loadVideoEntitiesFromLocal(ids []uint, byID map[uint]*video.Video) []uint {
	if s.localCache == nil {
		return ids
	}
	missed := make([]uint, 0, len(ids))
	for _, id := range ids {
		value, ok := s.localCache.Get(s.videoEntityCacheKey(id))
		if !ok {
			missed = append(missed, id)
			observability.CacheMissTotal.WithLabelValues("video_entity", "local").Inc()
			continue
		}
		switch cached := value.(type) {
		case video.Video:
			item := cached
			byID[id] = &item
			observability.CacheHitTotal.WithLabelValues("video_entity", "local").Inc()
		case *video.Video:
			if cached != nil {
				item := *cached
				byID[id] = &item
				observability.CacheHitTotal.WithLabelValues("video_entity", "local").Inc()
			}
		default:
			missed = append(missed, id)
			observability.CacheMissTotal.WithLabelValues("video_entity", "local").Inc()
		}
	}
	return missed
}

func (s *Service) loadVideoEntitiesFromRedis(ctx context.Context, ids []uint, byID map[uint]*video.Video) []uint {
	if len(ids) == 0 || s.cache == nil {
		return ids
	}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, s.videoEntityCacheKey(id))
	}
	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	results, err := s.cache.MGet(opCtx, keys...)
	cancel()
	if err != nil {
		return ids
	}

	missed := make([]uint, 0, len(ids))
	for index, raw := range results {
		id := ids[index]
		if raw == nil {
			missed = append(missed, id)
			observability.CacheMissTotal.WithLabelValues("video_entity", "redis").Inc()
			continue
		}
		var payload []byte
		switch value := raw.(type) {
		case string:
			payload = []byte(value)
		case []byte:
			payload = value
		default:
			missed = append(missed, id)
			observability.CacheMissTotal.WithLabelValues("video_entity", "redis").Inc()
			continue
		}
		var item video.Video
		if err := json.Unmarshal(payload, &item); err != nil {
			missed = append(missed, id)
			observability.CacheMissTotal.WithLabelValues("video_entity", "redis").Inc()
			continue
		}
		byID[id] = &item
		observability.CacheHitTotal.WithLabelValues("video_entity", "redis").Inc()
		s.setLocalVideoEntity(&item)
	}
	return missed
}

func (s *Service) loadVideoEntitiesFromDB(ctx context.Context, ids []uint, byID map[uint]*video.Video) error {
	if len(ids) == 0 {
		return nil
	}
	sfKey := s.singleflightKey(ids)
	value, err, _ := s.requestGroup.Do(sfKey, func() (any, error) {
		return s.repo.GetByIDs(ctx, ids)
	})
	if err != nil {
		return err
	}
	videos, ok := value.([]*video.Video)
	if !ok {
		return nil
	}
	for _, item := range videos {
		if item == nil {
			continue
		}
		copied := *item
		byID[copied.ID] = &copied
		s.setVideoEntityCaches(ctx, &copied)
	}
	return nil
}

func (s *Service) setVideoEntityCaches(ctx context.Context, item *video.Video) {
	if item == nil {
		return
	}
	s.setLocalVideoEntity(item)
	if s.cache == nil {
		return
	}
	payload, err := json.Marshal(item)
	if err != nil {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_ = s.cache.SetBytes(opCtx, s.videoEntityCacheKey(item.ID), payload, s.entityCacheTTL)
}

func (s *Service) setLocalVideoEntity(item *video.Video) {
	if s.localCache == nil || item == nil {
		return
	}
	s.localCache.Set(s.videoEntityCacheKey(item.ID), *item, localcache.DefaultExpiration)
}

func (s *Service) videoEntityCacheKey(id uint) string {
	if s.cache != nil {
		return s.cache.Key("video:entity:%d", id)
	}
	return fmt.Sprintf("video:entity:%d", id)
}

func (s *Service) singleflightKey(ids []uint) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
	}
	return "feed:video-entities:" + strings.Join(parts, ",")
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
	cacheKey := ""
	if s.cache != nil && viewerAccountID != 0 {
		cacheKey = s.followingCacheKey(limit, latestBefore, viewerAccountID)
		if cached, ok := s.getCachedFollowing(ctx, cacheKey); ok {
			return cached, nil
		}
		if resp, handled, err := s.rebuildFollowingWithLock(ctx, cacheKey, limit, latestBefore, viewerAccountID); handled || err != nil {
			return resp, err
		}
	}
	resp, err := s.listByFollowingFromDB(ctx, limit, latestBefore, viewerAccountID)
	if err != nil {
		return ListByFollowingResponse{}, err
	}
	if cacheKey != "" {
		s.setCachedFollowing(ctx, cacheKey, resp)
	}
	return resp, nil
}

func (s *Service) listByFollowingFromDB(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (ListByFollowingResponse, error) {
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
		NextTime:  nextTimeSecond(videos),
		HasMore:   len(videos) == limit,
	}, nil
}

func (s *Service) getCachedFollowing(ctx context.Context, cacheKey string) (ListByFollowingResponse, bool) {
	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	payload, err := s.cache.GetBytes(opCtx, cacheKey)
	cancel()
	if err != nil {
		return ListByFollowingResponse{}, false
	}
	var cached ListByFollowingResponse
	if err := json.Unmarshal(payload, &cached); err != nil {
		return ListByFollowingResponse{}, false
	}
	return cached, true
}

func (s *Service) rebuildFollowingWithLock(ctx context.Context, cacheKey string, limit int, latestBefore time.Time, viewerAccountID uint) (ListByFollowingResponse, bool, error) {
	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	token, locked, _ := s.cache.Lock(opCtx, "lock:"+cacheKey, 500*time.Millisecond)
	cancel()
	if locked {
		defer func() { _ = s.cache.Unlock(context.Background(), "lock:"+cacheKey, token) }()
		if cached, ok := s.getCachedFollowing(ctx, cacheKey); ok {
			return cached, true, nil
		}
		resp, err := s.listByFollowingFromDB(ctx, limit, latestBefore, viewerAccountID)
		if err != nil {
			return ListByFollowingResponse{}, true, err
		}
		s.setCachedFollowing(ctx, cacheKey, resp)
		return resp, true, nil
	}

	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			return ListByFollowingResponse{}, true, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
		if cached, ok := s.getCachedFollowing(ctx, cacheKey); ok {
			return cached, true, nil
		}
	}
	return ListByFollowingResponse{}, false, nil
}

func (s *Service) setCachedFollowing(ctx context.Context, cacheKey string, resp ListByFollowingResponse) {
	payload, err := json.Marshal(resp)
	if err != nil {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_ = s.cache.SetBytes(opCtx, cacheKey, payload, s.followingTTL)
}

func (s *Service) followingCacheKey(limit int, latestBefore time.Time, viewerAccountID uint) string {
	before := int64(0)
	if !latestBefore.IsZero() {
		before = latestBefore.Unix()
	}
	return s.cache.Key("feed:listByFollowing:limit=%d:accountID=%d:before=%d", limit, viewerAccountID, before)
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
		observability.CacheMissTotal.WithLabelValues("hot_rank", "redis_merge").Inc()
		if err := s.cache.ZUnionStore(opCtx, dest, keys, "SUM"); err != nil {
			return ListByPopularityResponse{}, false, nil
		}
		_ = s.cache.Expire(opCtx, dest, 2*time.Minute)
	} else {
		observability.CacheHitTotal.WithLabelValues("hot_rank", "redis_merge").Inc()
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
			CreateTime:  item.CreatedAt.Unix(),
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

func nextTimeMilli(videos []*video.Video) int64 {
	if len(videos) == 0 {
		return 0
	}
	return videos[len(videos)-1].CreatedAt.UnixMilli()
}

func nextTimeSecond(videos []*video.Video) int64 {
	if len(videos) == 0 {
		return 0
	}
	return videos[len(videos)-1].CreatedAt.Unix()
}
