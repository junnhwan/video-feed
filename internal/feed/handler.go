package feed

import (
	"net/http"
	"strings"
	"time"

	authjwt "video-feed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListLatest godoc
// @Summary      最新视频流
// @Description  按发布时间倒序的最新 Feed,游标分页(time + id 复合)
// @Tags         feed
// @Accept       json
// @Produce      json
// @Param        body  body      ListLatestRequest  true  "分页参数"
// @Success      200   {object}  ListLatestResponse
// @Router       /feed/listLatest [post]
func (h *Handler) ListLatest(c *gin.Context) {
	var req ListLatestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	viewerAccountID, _ := authjwt.GetAccountID(c)
	resp, err := h.service.ListLatest(c.Request.Context(), req.Limit, unixMilliCursor(req.LatestTime), viewerAccountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.VideoList = nonNilFeedItems(resp.VideoList)
	c.JSON(http.StatusOK, resp)
}

// ListLikesCount godoc
// @Summary      点赞榜
// @Description  按点赞数倒序的视频榜单,复合游标 (likes_count + id) 避免相同点赞数排序漂移
// @Tags         feed
// @Accept       json
// @Produce      json
// @Param        body  body      ListLikesCountRequest  true  "分页参数"
// @Success      200   {object}  ListLikesCountResponse
// @Router       /feed/listLikesCount [post]
func (h *Handler) ListLikesCount(c *gin.Context) {
	var req ListLikesCountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var cursor *LikesCountCursor
	if req.LikesCountBefore != nil || req.IDBefore != nil {
		if req.LikesCountBefore == nil || req.IDBefore == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "likes_count_before and id_before must be provided together"})
			return
		}
		if *req.LikesCountBefore < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor: likes_count_before must be >= 0"})
			return
		}
		if *req.IDBefore == 0 && *req.LikesCountBefore != 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor: id_before must be > 0"})
			return
		}
		cursor = &LikesCountCursor{LikesCount: *req.LikesCountBefore, ID: *req.IDBefore}
	}

	viewerAccountID, _ := authjwt.GetAccountID(c)
	resp, err := h.service.ListLikesCount(c.Request.Context(), req.Limit, cursor, viewerAccountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.VideoList = nonNilFeedItems(resp.VideoList)
	c.JSON(http.StatusOK, resp)
}

// ListByFollowing godoc
// @Summary      关注流
// @Description  仅返回当前登录用户关注的作者发布的视频,时间游标分页
// @Tags         feed
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      ListByFollowingRequest  true  "分页参数"
// @Success      200   {object}  ListByFollowingResponse
// @Failure      401   {object}  map[string]string
// @Router       /feed/listByFollowing [post]
func (h *Handler) ListByFollowing(c *gin.Context) {
	var req ListByFollowingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	viewerAccountID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.service.ListByFollowing(c.Request.Context(), req.Limit, unixSecondCursor(req.LatestTime), viewerAccountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.VideoList = nonNilFeedItems(resp.VideoList)
	c.JSON(http.StatusOK, resp)
}

// ListByPopularity godoc
// @Summary      热榜
// @Description  基于 Redis 时间窗口分片 ZSET 的热度榜单,as_of 快照分页
// @Tags         feed
// @Accept       json
// @Produce      json
// @Param        body  body      ListByPopularityRequest  true  "分页参数"
// @Success      200   {object}  ListByPopularityResponse
// @Router       /feed/listByPopularity [post]
func (h *Handler) ListByPopularity(c *gin.Context) {
	var req ListByPopularityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.LatestPopularity < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "latest_popularity must be >= 0"})
		return
	}
	anyCursor := !req.LatestBefore.IsZero() || req.LatestIDBefore != nil
	if anyCursor && (req.LatestBefore.IsZero() || req.LatestIDBefore == nil || *req.LatestIDBefore == 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "latest_before and latest_id_before must be provided together"})
		return
	}
	viewerAccountID, _ := authjwt.GetAccountID(c)
	var latestIDBefore uint
	if req.LatestIDBefore != nil {
		latestIDBefore = *req.LatestIDBefore
	}
	resp, err := h.service.ListByPopularity(
		c.Request.Context(),
		req.Limit,
		req.AsOf,
		req.Offset,
		viewerAccountID,
		req.LatestPopularity,
		req.LatestBefore,
		latestIDBefore,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.VideoList = nonNilFeedItems(resp.VideoList)
	c.JSON(http.StatusOK, resp)
}

// ListByTag godoc
// @Summary      按标签筛选
// @Description  根据标签名筛选视频列表
// @Tags         feed
// @Accept       json
// @Produce      json
// @Param        body  body      object{tag_name=string,limit=int}  true  "标签筛选参数"
// @Success      200   {object}  map[string]interface{}
// @Router       /feed/listByTag [post]
func (h *Handler) ListByTag(c *gin.Context) {
	var req struct {
		TagName string `json:"tag_name"`
		Limit   int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.TagName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_name is required"})
		return
	}
	viewerAccountID, _ := authjwt.GetAccountID(c)
	items, err := h.service.ListByTag(c.Request.Context(), req.TagName, req.Limit, viewerAccountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"video_list": nonNilFeedItems(items)})
}

func unixMilliCursor(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}

func unixSecondCursor(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(value, 0)
}

func nonNilFeedItems(items []FeedVideoItem) []FeedVideoItem {
	if items == nil {
		return []FeedVideoItem{}
	}
	return items
}
