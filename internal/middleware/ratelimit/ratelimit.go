package ratelimit

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	authjwt "video-feed/internal/middleware/jwt"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/observability"

	"github.com/gin-gonic/gin"
)

type KeyFunc func(*gin.Context) (string, bool)

func Limit(cache *rediscache.Client, keyPrefix string, maxRequests int64, window time.Duration, keyFunc KeyFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cache == nil || keyFunc == nil || maxRequests <= 0 || window <= 0 {
			c.Next()
			return
		}
		subject, ok := keyFunc(c)
		if !ok {
			c.Next()
			return
		}

		count, err := cache.IncrementWithExpire(c.Request.Context(), buildKey(keyPrefix, subject), window)
		if err != nil {
			c.Next()
			return
		}
		if count > maxRequests {
			observability.RateLimitRejectTotal.WithLabelValues(keyPrefix).Inc()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	}
}

func KeyByIP(c *gin.Context) (string, bool) {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		return "", false
	}
	return ip, true
}

func KeyByAccount(c *gin.Context) (string, bool) {
	accountID, err := authjwt.GetAccountID(c)
	if err != nil || accountID == 0 {
		return "", false
	}
	return strconv.FormatUint(uint64(accountID), 10), true
}

func buildKey(keyPrefix string, subject string) string {
	keyPrefix = strings.TrimSpace(keyPrefix)
	if keyPrefix == "" {
		keyPrefix = "default"
	}
	return fmt.Sprintf("ratelimit:%s:%s", keyPrefix, strings.TrimSpace(subject))
}
