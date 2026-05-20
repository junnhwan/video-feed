package http

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"video-feed/internal/account"
	"video-feed/internal/video"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestNewRouterRespondsToHealth(t *testing.T) {
	router := NewRouter(nil)
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestNewRouterRegistersAccountRoutes(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}); err != nil {
		t.Fatalf("migrate account: %v", err)
	}
	router := NewRouter(database)
	body := bytes.NewBufferString(`{"username":"alice","password":"secret123"}`)
	req := httptest.NewRequest("POST", "/account/register", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNewRouterRegistersVideoRoutes(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)
	body := bytes.NewBufferString(`{"author_id":1,"title":"first vlog","play_url":"http://example.com/video.mp4","cover_url":"http://example.com/cover.jpg"}`)
	req := httptest.NewRequest("POST", "/video/publish", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}
