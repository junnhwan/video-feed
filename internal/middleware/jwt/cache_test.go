package jwt

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"video-feed/internal/account"
	rediscache "video-feed/internal/middleware/redis"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func newTestCache(t *testing.T) (*rediscache.Client, *miniredis.Miniredis) {
	t.Helper()
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	client := rediscache.NewClient(goredis.NewClient(&goredis.Options{Addr: server.Addr()}), "test:")
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return client, server
}

func TestJWTAuthUsesCacheWhenTokenMatches(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	cache, _ := newTestCache(t)
	router, service := newCachedTestRouter(t, cache)
	if _, err := service.Register(t.Context(), account.RegisterInput{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := service.Login(t.Context(), "alice", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := cache.SetBytes(t.Context(), cache.Key("account:%d", login.Account.ID), []byte(login.Token), time.Minute); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestJWTAuthRejectsCachedTokenMismatch(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	cache, _ := newTestCache(t)
	router, service := newCachedTestRouter(t, cache)
	if _, err := service.Register(t.Context(), account.RegisterInput{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := service.Login(t.Context(), "alice", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := cache.SetBytes(t.Context(), cache.Key("account:%d", login.Account.ID), []byte("revoked-token"), time.Minute); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestJWTAuthBackfillsCacheAfterDBFallback(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	cache, _ := newTestCache(t)
	router, service := newCachedTestRouter(t, cache)
	if _, err := service.Register(t.Context(), account.RegisterInput{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := service.Login(t.Context(), "alice", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	key := cache.Key("account:%d", login.Account.ID)
	if err := cache.Del(t.Context(), key); err != nil {
		t.Fatalf("delete cache: %v", err)
	}
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	cached, err := cache.GetBytes(t.Context(), key)
	if err != nil {
		t.Fatalf("get backfilled token: %v", err)
	}
	if string(cached) != login.Token {
		t.Fatal("expected DB fallback to backfill token cache")
	}
}

func newCachedTestRouter(t *testing.T, cache *rediscache.Client) (*gin.Engine, *account.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open account db: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}); err != nil {
		t.Fatalf("migrate account: %v", err)
	}
	repo := account.NewRepository(database)
	service := account.NewService(repo, cache)

	router := gin.New()
	router.GET("/protected", JWTAuth(repo, cache), func(c *gin.Context) {
		accountID, err := GetAccountID(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		username, err := GetUsername(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"account_id": accountID, "username": username})
	})

	return router, service
}
