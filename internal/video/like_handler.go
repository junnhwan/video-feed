package video

import (
	"errors"
	"net/http"

	authjwt "video-feed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

type LikeHandler struct {
	service *LikeService
}

func NewLikeHandler(service *LikeService) *LikeHandler {
	return &LikeHandler{service: service}
}

type likeRequest struct {
	VideoID uint `json:"video_id"`
}

func (h *LikeHandler) Like(c *gin.Context) {
	videoID, ok := bindVideoID(c)
	if !ok {
		return
	}
	accountID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.Like(c.Request.Context(), videoID, accountID); err != nil {
		c.JSON(statusForInteractionError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "like success"})
}

func (h *LikeHandler) Unlike(c *gin.Context) {
	videoID, ok := bindVideoID(c)
	if !ok {
		return
	}
	accountID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.Unlike(c.Request.Context(), videoID, accountID); err != nil {
		c.JSON(statusForInteractionError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unlike success"})
}

func (h *LikeHandler) IsLiked(c *gin.Context) {
	videoID, ok := bindVideoID(c)
	if !ok {
		return
	}
	accountID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	isLiked, err := h.service.IsLiked(c.Request.Context(), videoID, accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"is_liked": isLiked})
}

func (h *LikeHandler) ListMyLikedVideos(c *gin.Context) {
	accountID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	videos, err := h.service.ListLikedVideos(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if videos == nil {
		videos = []Video{}
	}
	c.JSON(http.StatusOK, videos)
}

func bindVideoID(c *gin.Context) (uint, bool) {
	var req likeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return 0, false
	}
	if req.VideoID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "video_id is required"})
		return 0, false
	}
	return req.VideoID, true
}

func statusForInteractionError(err error) int {
	switch {
	case errors.Is(err, ErrVideoNotFound), errors.Is(err, ErrCommentNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrAlreadyLiked), errors.Is(err, ErrNotLiked):
		return http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}
