package social

import (
	"errors"
	"net/http"

	authjwt "video-feed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type followRequest struct {
	VloggerID uint `json:"vlogger_id"`
}

type followersRequest struct {
	VloggerID uint `json:"vlogger_id"`
}

type vloggersRequest struct {
	FollowerID uint `json:"follower_id"`
}

func (h *Handler) Follow(c *gin.Context) {
	followerID, vloggerID, ok := h.bindRelation(c)
	if !ok {
		return
	}
	if err := h.service.Follow(c.Request.Context(), followerID, vloggerID); err != nil {
		c.JSON(statusForSocialError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "followed"})
}

func (h *Handler) Unfollow(c *gin.Context) {
	followerID, vloggerID, ok := h.bindRelation(c)
	if !ok {
		return
	}
	if err := h.service.Unfollow(c.Request.Context(), followerID, vloggerID); err != nil {
		c.JSON(statusForSocialError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unfollowed"})
}

func (h *Handler) GetAllFollowers(c *gin.Context) {
	var req followersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	vloggerID := req.VloggerID
	if vloggerID == 0 {
		accountID, err := authjwt.GetAccountID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		vloggerID = accountID
	}
	followers, err := h.service.GetAllFollowers(c.Request.Context(), vloggerID)
	if err != nil {
		c.JSON(statusForSocialError(err), gin.H{"error": err.Error()})
		return
	}
	count, _ := h.service.CountFollowers(c.Request.Context(), vloggerID)
	c.JSON(http.StatusOK, gin.H{"followers": followers, "follower_count": count})
}

func (h *Handler) GetAllVloggers(c *gin.Context) {
	var req vloggersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	followerID := req.FollowerID
	if followerID == 0 {
		accountID, err := authjwt.GetAccountID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		followerID = accountID
	}
	vloggers, err := h.service.GetAllVloggers(c.Request.Context(), followerID)
	if err != nil {
		c.JSON(statusForSocialError(err), gin.H{"error": err.Error()})
		return
	}
	count, _ := h.service.CountVloggers(c.Request.Context(), followerID)
	c.JSON(http.StatusOK, gin.H{"vloggers": vloggers, "vlogger_count": count})
}

func (h *Handler) GetCounts(c *gin.Context) {
	accountID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	followers, _ := h.service.CountFollowers(c.Request.Context(), accountID)
	vloggers, _ := h.service.CountVloggers(c.Request.Context(), accountID)
	c.JSON(http.StatusOK, gin.H{"follower_count": followers, "vlogger_count": vloggers})
}

func (h *Handler) bindRelation(c *gin.Context) (uint, uint, bool) {
	var req followRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return 0, 0, false
	}
	if req.VloggerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vlogger_id is required"})
		return 0, 0, false
	}
	followerID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return 0, 0, false
	}
	return followerID, req.VloggerID, true
}

func statusForSocialError(err error) int {
	switch {
	case errors.Is(err, ErrCannotFollowSelf), errors.Is(err, ErrAlreadyFollowed), errors.Is(err, ErrNotFollowed):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
