package feed

import (
	"net/http"
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
	resp, err := h.service.ListByFollowing(c.Request.Context(), req.Limit, unixMilliCursor(req.LatestTime), viewerAccountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.VideoList = nonNilFeedItems(resp.VideoList)
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListByPopularity(c *gin.Context) {
	var req ListByPopularityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		req.LatestPopularity,
		req.LatestBefore,
		latestIDBefore,
		viewerAccountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.VideoList = nonNilFeedItems(resp.VideoList)
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListByTag(c *gin.Context) {
	var req struct {
		TagName string `json:"tag_name"`
		Limit   int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

func nonNilFeedItems(items []FeedVideoItem) []FeedVideoItem {
	if items == nil {
		return []FeedVideoItem{}
	}
	return items
}
