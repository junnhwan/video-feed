package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	rediscache "video-feed/internal/middleware/redis"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
)

func TestLimitRejectsRequestsAfterWindowLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer server.Close()

	cache := rediscache.NewClient(goredis.NewClient(&goredis.Options{Addr: server.Addr()}), "test:")
	defer cache.Close()

	router := gin.New()
	router.GET("/limited", Limit(cache, "test", 2, time.Minute, KeyByIP), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/limited", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest("GET", "/limited", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLimitAllowsWhenCacheIsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/limited", Limit(nil, "test", 1, time.Minute, KeyByIP), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/limited", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d", i+1, rec.Code)
		}
	}
}
