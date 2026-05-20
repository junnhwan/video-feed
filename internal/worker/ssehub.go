package worker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"video-feed/internal/auth"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SSEHub struct {
	mu      sync.RWMutex
	clients map[uint][]chan *Notification
	db      *gorm.DB
}

func NewSSEHub(db *gorm.DB) *SSEHub {
	return &SSEHub{
		clients: make(map[uint][]chan *Notification),
		db:      db,
	}
}

func (h *SSEHub) Push(userID uint, notification *Notification) {
	if h == nil || notification == nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.clients[userID] {
		select {
		case ch <- notification:
		default:
		}
	}
}

func (h *SSEHub) Subscribe(userID uint) chan *Notification {
	ch := make(chan *Notification, 20)
	h.mu.Lock()
	h.clients[userID] = append(h.clients[userID], ch)
	h.mu.Unlock()
	return ch
}

func (h *SSEHub) Unsubscribe(userID uint, ch chan *Notification) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.clients[userID]
	for i, candidate := range clients {
		if candidate == ch {
			h.clients[userID] = append(clients[:i], clients[i+1:]...)
			if len(h.clients[userID]) == 0 {
				delete(h.clients, userID)
			}
			close(candidate)
			return
		}
	}
}

func (h *SSEHub) SSERequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.Query("token")
		if tokenString == "" {
			tokenString = bearerToken(c.GetHeader("Authorization"))
		}
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("accountID", claims.AccountID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func (h *SSEHub) SSEHandler(c *gin.Context) {
	userID, ok := accountIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	ch := h.Subscribe(userID)
	defer h.Unsubscribe(userID, ch)

	flusher, _ := c.Writer.(http.Flusher)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case notification, ok := <-ch:
			if !ok {
				return
			}
			body, _ := json.Marshal(notification)
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", body)
			if flusher != nil {
				flusher.Flush()
			}
		case <-ticker.C:
			_, _ = fmt.Fprint(c.Writer, ": keepalive\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

func (h *SSEHub) ListHandler(c *gin.Context) {
	userID, ok := accountIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}

	var notifications []Notification
	if err := h.db.WithContext(c.Request.Context()).
		Where("recipient_id = ?", userID).
		Order("created_at DESC, id DESC").
		Limit(50).
		Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if notifications == nil {
		notifications = []Notification{}
	}
	c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}

func (h *SSEHub) MarkReadHandler(c *gin.Context) {
	userID, ok := accountIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}

	var req struct {
		ID *uint `json:"id"`
	}
	_ = c.ShouldBindJSON(&req)

	query := h.db.WithContext(c.Request.Context()).Model(&Notification{})
	if req.ID != nil {
		query = query.Where("id = ? AND recipient_id = ?", *req.ID, userID)
	} else {
		query = query.Where("recipient_id = ?", userID)
	}
	if err := query.Update("is_read", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (h *SSEHub) UnreadCountHandler(c *gin.Context) {
	userID, ok := accountIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}

	var count int64
	if err := h.db.WithContext(c.Request.Context()).
		Model(&Notification{}).
		Where("recipient_id = ? AND is_read = ?", userID, false).
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *SSEHub) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/stream", h.SSERequireAuth(), h.SSEHandler)
	group.POST("/list", h.ListHandler)
	group.POST("/markRead", h.MarkReadHandler)
	group.POST("/unreadCount", h.UnreadCountHandler)
}

func accountIDFromContext(c *gin.Context) (uint, bool) {
	value, exists := c.Get("accountID")
	if !exists {
		return 0, false
	}
	accountID, ok := value.(uint)
	return accountID, ok
}

func bearerToken(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

var _ NotificationHub = (*SSEHub)(nil)
