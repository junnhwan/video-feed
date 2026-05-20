package message

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

func (h *Handler) Send(c *gin.Context) {
	fromID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req sendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	message, err := h.service.Send(c.Request.Context(), fromID, req.ToID, req.Content)
	if err != nil {
		c.JSON(statusForError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, message)
}

func (h *Handler) List(c *gin.Context) {
	userID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	var req listRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	messages, err := h.service.List(c.Request.Context(), userID, req.PeerID, 50)
	if err != nil {
		c.JSON(statusForError(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ListResponse{Messages: messages})
}

func statusForError(err error) int {
	if errors.Is(err, ErrInvalidMessageInput) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
