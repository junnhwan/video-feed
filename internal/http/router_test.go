package http

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
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
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
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
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
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
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
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

func TestAccountUploadAvatarStoresProfileURL(t *testing.T) {
	workDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)
	token := registerAndLogin(t, router)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "avatar.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("fake png bytes")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/account/uploadAvatar", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("upload avatar expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode avatar response: %v", err)
	}
	if response.AvatarURL == "" {
		t.Fatal("expected avatar_url")
	}
	var stored account.Account
	if err := database.First(&stored, 1).Error; err != nil {
		t.Fatalf("find account: %v", err)
	}
	if stored.AvatarURL != response.AvatarURL {
		t.Fatalf("expected avatar stored as %q, got %q", response.AvatarURL, stored.AvatarURL)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".run", "uploads", "avatars", "1")); err != nil {
		t.Fatalf("expected avatar upload directory: %v", err)
	}
}

func TestAccountGetProfileAggregatesCounts(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	if err := database.Create(&account.Account{ID: 1, Username: "alice", Password: "hash", AvatarURL: "http://example.com/a.png", Bio: "creator"}).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := database.Create(&account.Account{ID: 2, Username: "bob", Password: "hash"}).Error; err != nil {
		t.Fatalf("create follower: %v", err)
	}
	if err := database.Create(&video.Video{AuthorID: 1, Username: "alice", Title: "v1", PlayURL: "1.mp4", CoverURL: "1.jpg", LikesCount: 3}).Error; err != nil {
		t.Fatalf("create first video: %v", err)
	}
	if err := database.Create(&video.Video{AuthorID: 1, Username: "alice", Title: "v2", PlayURL: "2.mp4", CoverURL: "2.jpg", LikesCount: 5}).Error; err != nil {
		t.Fatalf("create second video: %v", err)
	}
	if err := database.Create(&social.Social{FollowerID: 2, VloggerID: 1}).Error; err != nil {
		t.Fatalf("create follower relation: %v", err)
	}
	if err := database.Create(&social.Social{FollowerID: 1, VloggerID: 2}).Error; err != nil {
		t.Fatalf("create vlogger relation: %v", err)
	}
	router := NewRouter(database)

	req := httptest.NewRequest("POST", "/account/getProfile", bytes.NewBufferString(`{"account_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get profile expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Account struct {
			ID        uint   `json:"id"`
			Username  string `json:"username"`
			AvatarURL string `json:"avatar_url"`
			Bio       string `json:"bio"`
		} `json:"account"`
		VideoCount    int64 `json:"video_count"`
		TotalLikes    int64 `json:"total_likes"`
		FollowerCount int64 `json:"follower_count"`
		VloggerCount  int64 `json:"vlogger_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if response.Account.ID != 1 || response.Account.Username != "alice" {
		t.Fatalf("unexpected profile account: %+v", response.Account)
	}
	if response.VideoCount != 2 || response.TotalLikes != 8 || response.FollowerCount != 1 || response.VloggerCount != 1 {
		t.Fatalf("unexpected profile counts: %+v", response)
	}
}

func TestNewRouterRegistersFeedRoutes(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
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

func TestLikeRouteUpdatesFeedState(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	seed := video.Video{AuthorID: 99, Username: "creator", Title: "first vlog", PlayURL: "1.mp4", CoverURL: "1.jpg"}
	if err := database.Create(&seed).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	router := NewRouter(database)
	token := registerAndLogin(t, router)

	likeBody := bytes.NewBufferString(`{"video_id":` + strconv.Itoa(int(seed.ID)) + `}`)
	likeReq := httptest.NewRequest("POST", "/like/like", likeBody)
	likeReq.Header.Set("Content-Type", "application/json")
	likeReq.Header.Set("Authorization", "Bearer "+token)
	likeRec := httptest.NewRecorder()
	router.ServeHTTP(likeRec, likeReq)
	if likeRec.Code != http.StatusOK {
		t.Fatalf("like expected 200, got %d body=%s", likeRec.Code, likeRec.Body.String())
	}

	feedReq := httptest.NewRequest("POST", "/feed/listLatest", bytes.NewBufferString(`{"limit":10}`))
	feedReq.Header.Set("Content-Type", "application/json")
	feedReq.Header.Set("Authorization", "Bearer "+token)
	feedRec := httptest.NewRecorder()
	router.ServeHTTP(feedRec, feedReq)
	if feedRec.Code != http.StatusOK {
		t.Fatalf("feed expected 200, got %d body=%s", feedRec.Code, feedRec.Body.String())
	}
	var resp struct {
		VideoList []struct {
			ID         uint  `json:"id"`
			LikesCount int64 `json:"likes_count"`
			IsLiked    bool  `json:"is_liked"`
		} `json:"video_list"`
	}
	if err := json.Unmarshal(feedRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode feed response: %v", err)
	}
	if len(resp.VideoList) != 1 {
		t.Fatalf("expected 1 feed item, got %d", len(resp.VideoList))
	}
	if resp.VideoList[0].LikesCount != 1 || !resp.VideoList[0].IsLiked {
		t.Fatalf("expected liked feed item with count 1, got %+v", resp.VideoList[0])
	}
}

func TestCommentRoutesPublishAndList(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	seed := video.Video{AuthorID: 99, Username: "creator", Title: "first vlog", PlayURL: "1.mp4", CoverURL: "1.jpg"}
	if err := database.Create(&seed).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	router := NewRouter(database)
	token := registerAndLogin(t, router)

	publishBody := bytes.NewBufferString(`{"video_id":` + strconv.Itoa(int(seed.ID)) + `,"content":"hello"}`)
	publishReq := httptest.NewRequest("POST", "/comment/publish", publishBody)
	publishReq.Header.Set("Content-Type", "application/json")
	publishReq.Header.Set("Authorization", "Bearer "+token)
	publishRec := httptest.NewRecorder()
	router.ServeHTTP(publishRec, publishReq)
	if publishRec.Code != http.StatusOK {
		t.Fatalf("comment publish expected 200, got %d body=%s", publishRec.Code, publishRec.Body.String())
	}

	listBody := bytes.NewBufferString(`{"video_id":` + strconv.Itoa(int(seed.ID)) + `}`)
	listReq := httptest.NewRequest("POST", "/comment/listAll", listBody)
	listReq.Header.Set("Content-Type", "application/json")
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("comment list expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	var comments []struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &comments); err != nil {
		t.Fatalf("decode comments: %v", err)
	}
	if len(comments) != 1 || comments[0].Content != "hello" {
		t.Fatalf("unexpected comments: %+v", comments)
	}
}

func TestSocialRoutesFollowAndCounts(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)
	token := registerAndLogin(t, router)
	if err := database.Create(&account.Account{ID: 2, Username: "bob", Password: "hash"}).Error; err != nil {
		t.Fatalf("create bob: %v", err)
	}

	followReq := httptest.NewRequest("POST", "/social/follow", bytes.NewBufferString(`{"vlogger_id":2}`))
	followReq.Header.Set("Content-Type", "application/json")
	followReq.Header.Set("Authorization", "Bearer "+token)
	followRec := httptest.NewRecorder()
	router.ServeHTTP(followRec, followReq)
	if followRec.Code != http.StatusOK {
		t.Fatalf("follow expected 200, got %d body=%s", followRec.Code, followRec.Body.String())
	}

	countReq := httptest.NewRequest("POST", "/social/getCounts", nil)
	countReq.Header.Set("Authorization", "Bearer "+token)
	countRec := httptest.NewRecorder()
	router.ServeHTTP(countRec, countReq)
	if countRec.Code != http.StatusOK {
		t.Fatalf("counts expected 200, got %d body=%s", countRec.Code, countRec.Body.String())
	}
	var counts struct {
		VloggerCount int64 `json:"vlogger_count"`
	}
	if err := json.Unmarshal(countRec.Body.Bytes(), &counts); err != nil {
		t.Fatalf("decode counts: %v", err)
	}
	if counts.VloggerCount != 1 {
		t.Fatalf("expected vlogger_count 1, got %d", counts.VloggerCount)
	}
}

func TestListByFollowingRouteUsesUnixSecondCursor(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)
	token := registerAndLogin(t, router)
	if err := database.Create(&account.Account{ID: 2, Username: "bob", Password: "hash"}).Error; err != nil {
		t.Fatalf("create bob: %v", err)
	}
	if err := database.Create(&social.Social{FollowerID: 1, VloggerID: 2}).Error; err != nil {
		t.Fatalf("create follow relation: %v", err)
	}
	base := time.Date(2026, 5, 20, 13, 0, 0, 0, time.UTC)
	oldVideo := video.Video{AuthorID: 2, Username: "bob", Title: "old", PlayURL: "old.mp4", CoverURL: "old.jpg", CreatedAt: base}
	newVideo := video.Video{AuthorID: 2, Username: "bob", Title: "new", PlayURL: "new.mp4", CoverURL: "new.jpg", CreatedAt: base.Add(time.Minute)}
	if err := database.Create(&oldVideo).Error; err != nil {
		t.Fatalf("create old video: %v", err)
	}
	if err := database.Create(&newVideo).Error; err != nil {
		t.Fatalf("create new video: %v", err)
	}

	body := bytes.NewBufferString(`{"limit":10,"latest_time":` + strconv.FormatInt(newVideo.CreatedAt.Unix(), 10) + `}`)
	req := httptest.NewRequest("POST", "/feed/listByFollowing", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("following feed expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		VideoList []struct {
			Title string `json:"title"`
		} `json:"video_list"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode following response: %v", err)
	}
	if len(response.VideoList) != 1 || response.VideoList[0].Title != "old" {
		t.Fatalf("expected second page old video, got %+v", response.VideoList)
	}
}

func TestFeedRoutesValidateSourceCursorContracts(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &social.Social{}, &video.Tag{}, &video.VideoTag{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)

	cases := []struct {
		name string
		path string
		body string
	}{
		{name: "empty tag", path: "/feed/listByTag", body: `{"tag_name":"","limit":10}`},
		{name: "negative likes cursor", path: "/feed/listLikesCount", body: `{"limit":10,"likes_count_before":-1,"id_before":1}`},
		{name: "popularity negative cursor", path: "/feed/listByPopularity", body: `{"limit":10,"latest_popularity":-1}`},
		{name: "popularity partial cursor", path: "/feed/listByPopularity", body: `{"limit":10,"latest_before":"2026-05-20T00:00:00Z"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
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
