package http

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"video-feed/internal/account"
	"video-feed/internal/social"
	"video-feed/internal/video"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestVideoDeleteRequiresOwnerAndRemovesTaggedVideo(t *testing.T) {
	database := newVideoRouterDB(t)
	router := NewRouter(database)
	aliceToken := registerAndLoginAs(t, router, "alice")
	bobToken := registerAndLoginAs(t, router, "bob")

	publishBody := bytes.NewBufferString(`{"title":"first #Go vlog","play_url":"1.mp4","cover_url":"1.jpg"}`)
	publishReq := httptest.NewRequest("POST", "/video/publish", publishBody)
	publishReq.Header.Set("Content-Type", "application/json")
	publishReq.Header.Set("Authorization", "Bearer "+aliceToken)
	publishRec := httptest.NewRecorder()
	router.ServeHTTP(publishRec, publishReq)
	if publishRec.Code != http.StatusOK {
		t.Fatalf("publish expected 200, got %d body=%s", publishRec.Code, publishRec.Body.String())
	}
	var published struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(publishRec.Body.Bytes(), &published); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}

	deleteBody := bytes.NewBufferString(`{"id":` + jsonUint(published.ID) + `}`)
	deleteReq := httptest.NewRequest("POST", "/video/delete", deleteBody)
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteReq.Header.Set("Authorization", "Bearer "+bobToken)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusUnauthorized {
		t.Fatalf("non-owner delete expected 401, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	deleteBody = bytes.NewBufferString(`{"id":` + jsonUint(published.ID) + `}`)
	deleteReq = httptest.NewRequest("POST", "/video/delete", deleteBody)
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteReq.Header.Set("Authorization", "Bearer "+aliceToken)
	deleteRec = httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("owner delete expected 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	tagReq := httptest.NewRequest("POST", "/feed/listByTag", bytes.NewBufferString(`{"tag_name":"Go","limit":10}`))
	tagReq.Header.Set("Content-Type", "application/json")
	tagRec := httptest.NewRecorder()
	router.ServeHTTP(tagRec, tagReq)
	if tagRec.Code != http.StatusOK {
		t.Fatalf("tag feed expected 200, got %d body=%s", tagRec.Code, tagRec.Body.String())
	}
	var tagResp struct {
		VideoList []struct {
			ID uint `json:"id"`
		} `json:"video_list"`
	}
	if err := json.Unmarshal(tagRec.Body.Bytes(), &tagResp); err != nil {
		t.Fatalf("decode tag response: %v", err)
	}
	if len(tagResp.VideoList) != 0 {
		t.Fatalf("expected deleted video removed from tag feed, got %+v", tagResp.VideoList)
	}
}

func TestVideoUploadRoutesValidateTypeAndReturnStaticURL(t *testing.T) {
	restore := chdirTemp(t)
	defer restore()

	database := newVideoRouterDB(t)
	router := NewRouter(database)
	token := registerAndLoginAs(t, router, "alice")

	invalidVideoRec := postMultipartFile(t, router, "/video/uploadVideo", token, "note.txt", "plain text")
	if invalidVideoRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid video expected 400, got %d body=%s", invalidVideoRec.Code, invalidVideoRec.Body.String())
	}

	videoRec := postMultipartFile(t, router, "/video/uploadVideo", token, "clip.mp4", "fake mp4 bytes")
	if videoRec.Code != http.StatusOK {
		t.Fatalf("upload video expected 200, got %d body=%s", videoRec.Code, videoRec.Body.String())
	}
	var videoResp struct {
		PlayURL string `json:"play_url"`
	}
	if err := json.Unmarshal(videoRec.Body.Bytes(), &videoResp); err != nil {
		t.Fatalf("decode video upload response: %v", err)
	}
	if !strings.Contains(videoResp.PlayURL, "/static/videos/1/") || !strings.HasPrefix(videoResp.PlayURL, "http://example.com") {
		t.Fatalf("unexpected play_url %q", videoResp.PlayURL)
	}
	assertOneUploadedFile(t, filepath.Join(".run", "uploads", "videos"))

	invalidCoverRec := postMultipartFile(t, router, "/video/uploadCover", token, "cover.gif", "gif")
	if invalidCoverRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid cover expected 400, got %d body=%s", invalidCoverRec.Code, invalidCoverRec.Body.String())
	}

	coverRec := postMultipartFile(t, router, "/video/uploadCover", token, "cover.jpg", "jpeg")
	if coverRec.Code != http.StatusOK {
		t.Fatalf("upload cover expected 200, got %d body=%s", coverRec.Code, coverRec.Body.String())
	}
	var coverResp struct {
		CoverURL string `json:"cover_url"`
	}
	if err := json.Unmarshal(coverRec.Body.Bytes(), &coverResp); err != nil {
		t.Fatalf("decode cover upload response: %v", err)
	}
	if !strings.Contains(coverResp.CoverURL, "/static/covers/1/") || !strings.HasPrefix(coverResp.CoverURL, "http://example.com") {
		t.Fatalf("unexpected cover_url %q", coverResp.CoverURL)
	}
	assertOneUploadedFile(t, filepath.Join(".run", "uploads", "covers"))
}

func TestVideoDetailUsesSourceCreateTimeJSONField(t *testing.T) {
	database := newVideoRouterDB(t)
	createdAt := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	seed := video.Video{
		AuthorID:  1,
		Username:  "alice",
		Title:     "first",
		PlayURL:   "1.mp4",
		CoverURL:  "1.jpg",
		CreatedAt: createdAt,
	}
	if err := database.Create(&seed).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	router := NewRouter(database)

	req := httptest.NewRequest("POST", "/video/getDetail", bytes.NewBufferString(`{"id":`+jsonUint(seed.ID)+`}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get detail expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if _, ok := response["create_time"]; !ok {
		t.Fatalf("expected source-compatible create_time field, got %s", rec.Body.String())
	}
	if _, ok := response["created_at"]; ok {
		t.Fatalf("did not expect created_at field in video detail response: %s", rec.Body.String())
	}
}

func newVideoRouterDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &video.Tag{}, &video.VideoTag{}, &social.Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	return database
}

func registerAndLoginAs(t *testing.T, router http.Handler, username string) string {
	t.Helper()
	body := `{"username":"` + username + `","password":"secret123"}`
	registerReq := httptest.NewRequest("POST", "/account/register", bytes.NewBufferString(body))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusOK {
		t.Fatalf("register %s expected 200, got %d body=%s", username, registerRec.Code, registerRec.Body.String())
	}

	loginReq := httptest.NewRequest("POST", "/account/login", bytes.NewBufferString(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login %s expected 200, got %d body=%s", username, loginRec.Code, loginRec.Body.String())
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if response.Token == "" {
		t.Fatalf("expected token for %s", username)
	}
	return response.Token
}

func postMultipartFile(t *testing.T, router http.Handler, path string, token string, filename string, content string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader(content)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest("POST", "http://example.com"+path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func chdirTemp(t *testing.T) func() {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	temp := t.TempDir()
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	return func() {
		if err := os.Chdir(original); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}
}

func assertOneUploadedFile(t *testing.T, root string) {
	t.Helper()
	count := 0
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	}); err != nil {
		t.Fatalf("walk uploads: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 uploaded file under %s, got %d", root, count)
	}
}

func jsonUint(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
