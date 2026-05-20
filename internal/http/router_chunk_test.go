package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"video-feed/internal/account"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/social"
	"video-feed/internal/video"
	"video-feed/internal/worker"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func TestChunkUploadRoutesInitRequiresJWTAndUsesRedisSession(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &video.Tag{}, &video.VideoTag{}, &social.Social{}, &worker.Notification{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	cache := rediscache.NewClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()}), "")
	defer cache.Close()

	router := NewRouter(database, cache)
	body := []byte(`{"filename":"clip.mp4","file_size":128,"chunk_size":64,"total_chunks":2,"file_hash":"abc123"}`)

	unauthReq := httptest.NewRequest("POST", "/video/chunk/init", bytes.NewReader(body))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthRec := httptest.NewRecorder()
	router.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth init expected 401, got %d body=%s", unauthRec.Code, unauthRec.Body.String())
	}

	token := registerAndLoginAs(t, router, "alice")
	authReq := httptest.NewRequest("POST", "/video/chunk/init", bytes.NewReader(body))
	authReq.Header.Set("Content-Type", "application/json")
	authReq.Header.Set("Authorization", "Bearer "+token)
	authRec := httptest.NewRecorder()
	router.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusOK {
		t.Fatalf("auth init expected 200, got %d body=%s", authRec.Code, authRec.Body.String())
	}
	var resp struct {
		UploadID       string `json:"upload_id"`
		UploadedChunks []int  `json:"uploaded_chunks"`
	}
	if err := json.Unmarshal(authRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode init response: %v", err)
	}
	if resp.UploadID == "" || len(resp.UploadedChunks) != 0 {
		t.Fatalf("unexpected init response: %+v", resp)
	}
}
