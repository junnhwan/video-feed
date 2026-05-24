package video

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	rediscache "video-feed/internal/middleware/redis"
)

const entityCacheTTL = time.Hour

// RefreshEntityCache re-reads the video from DB and updates the Redis entity cache.
// Called by workers after mutating likes_count or popularity so that feed reads
// see fresh data instead of waiting for the 1-hour TTL to expire.
func RefreshEntityCache(ctx context.Context, cache *rediscache.Client, repo *Repository, videoID uint) {
	if cache == nil || repo == nil || videoID == 0 {
		return
	}
	v, err := repo.GetByID(ctx, videoID)
	if err != nil {
		return
	}
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_ = cache.SetBytes(opCtx, cache.Key("video:entity:%d", videoID), payload, entityCacheTTL)
}

func UpdatePopularityCache(ctx context.Context, cache *rediscache.Client, id uint, change int64) {
	if cache == nil || id == 0 || change == 0 {
		return
	}

	_ = cache.Del(context.Background(), cache.Key("video:detail:id=%d", id))

	now := time.Now().UTC().Truncate(time.Minute)
	windowKey := cache.Key("hot:video:1m:%s", now.Format("200601021504"))
	member := strconv.FormatUint(uint64(id), 10)

	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_ = cache.ZIncrBy(opCtx, windowKey, member, float64(change))
	_ = cache.Expire(opCtx, windowKey, 2*time.Hour)
}
