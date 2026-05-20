package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"video-feed/internal/account"
	"video-feed/internal/social"
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
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.Like{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)
	token := registerAndLogin(t, router)
	body := bytes.NewBufferString(`{"title":"first vlog","play_url":"http://example.com/video.mp4","cover_url":"http://example.com/cover.jpg"}`)
	req := httptest.NewRequest("POST", "/video/publish", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVideoPublishRequiresJWT(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.Like{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)
	body := bytes.NewBufferString(`{"title":"first vlog","play_url":"http://example.com/video.mp4","cover_url":"http://example.com/cover.jpg"}`)
	req := httptest.NewRequest("POST", "/video/publish", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLogoutRevokesTokenForProtectedRoutes(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.Like{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)
	token := registerAndLogin(t, router)

	logoutReq := httptest.NewRequest("POST", "/account/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout expected 200, got %d body=%s", logoutRec.Code, logoutRec.Body.String())
	}

	body := bytes.NewBufferString(`{"title":"first vlog","play_url":"http://example.com/video.mp4","cover_url":"http://example.com/cover.jpg"}`)
	publishReq := httptest.NewRequest("POST", "/video/publish", body)
	publishReq.Header.Set("Content-Type", "application/json")
	publishReq.Header.Set("Authorization", "Bearer "+token)
	publishRec := httptest.NewRecorder()

	router.ServeHTTP(publishRec, publishReq)

	if publishRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body=%s", publishRec.Code, publishRec.Body.String())
	}
}

func TestRefreshReturnsNewAccessToken(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.Like{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)
	registerAndLogin(t, router)

	loginBody := bytes.NewBufferString(`{"username":"alice","password":"secret123"}`)
	loginReq := httptest.NewRequest("POST", "/account/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}
	var loginResponse struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResponse); err != nil {
		t.Fatalf("decode login response: %v", err)
	}

	refreshBody := bytes.NewBufferString(`{"refresh_token":"` + loginResponse.RefreshToken + `"}`)
	refreshReq := httptest.NewRequest("POST", "/account/refresh", refreshBody)
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshRec := httptest.NewRecorder()

	router.ServeHTTP(refreshRec, refreshReq)

	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh expected 200, got %d body=%s", refreshRec.Code, refreshRec.Body.String())
	}
	var refreshResponse struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(refreshRec.Body.Bytes(), &refreshResponse); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if refreshResponse.Token == "" {
		t.Fatal("expected refreshed access token")
	}
}

func TestNewRouterRegistersFeedRoutes(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.Like{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	if err := database.Create(&video.Video{
		AuthorID: 1,
		Username: "alice",
		Title:    "first vlog",
		PlayURL:  "http://example.com/video.mp4",
		CoverURL: "http://example.com/cover.jpg",
	}).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	router := NewRouter(database)
	body := bytes.NewBufferString(`{"limit":10}`)
	req := httptest.NewRequest("POST", "/feed/listLatest", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		VideoList []struct {
			Title string `json:"title"`
		} `json:"video_list"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode feed response: %v", err)
	}
	if len(resp.VideoList) != 1 || resp.VideoList[0].Title != "first vlog" {
		t.Fatalf("unexpected feed response: %+v", resp.VideoList)
	}
}

func registerAndLogin(t *testing.T, router http.Handler) string {
	t.Helper()

	registerBody := bytes.NewBufferString(`{"username":"alice","password":"secret123"}`)
	registerReq := httptest.NewRequest("POST", "/account/register", registerBody)
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusOK {
		t.Fatalf("register expected 200, got %d body=%s", registerRec.Code, registerRec.Body.String())
	}

	loginBody := bytes.NewBufferString(`{"username":"alice","password":"secret123"}`)
	loginReq := httptest.NewRequest("POST", "/account/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}

	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if response.Token == "" {
		t.Fatal("expected login token")
	}
	return response.Token
}
