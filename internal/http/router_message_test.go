package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"video-feed/internal/account"
	"video-feed/internal/message"
	"video-feed/internal/social"
	"video-feed/internal/video"
	"video-feed/internal/worker"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMessageRoutesSendAndListConversation(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.Like{}, &video.Comment{}, &video.Tag{}, &video.VideoTag{}, &social.Social{}, &worker.Notification{}, &message.Message{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)
	aliceToken, aliceID := registerAndLoginAsWithID(t, router, "alice")
	bobToken, bobID := registerAndLoginAsWithID(t, router, "bob")

	send := postJSONWithToken(t, router, "/message/send", aliceToken, []byte(`{"to_id":`+strconv.FormatUint(uint64(bobID), 10)+`,"content":" hello bob "}`))
	if send.Code != http.StatusOK {
		t.Fatalf("send expected 200, got %d body=%s", send.Code, send.Body.String())
	}
	var sent message.Message
	if err := json.Unmarshal(send.Body.Bytes(), &sent); err != nil {
		t.Fatalf("decode send response: %v", err)
	}
	if sent.FromID != aliceID || sent.ToID != bobID || sent.Content != "hello bob" {
		t.Fatalf("unexpected sent message: %+v", sent)
	}

	reply := postJSONWithToken(t, router, "/message/send", bobToken, []byte(`{"to_id":`+strconv.FormatUint(uint64(aliceID), 10)+`,"content":"reply"}`))
	if reply.Code != http.StatusOK {
		t.Fatalf("reply expected 200, got %d body=%s", reply.Code, reply.Body.String())
	}
	if err := database.Create(&message.Message{FromID: 99, ToID: aliceID, Content: "not in this conversation"}).Error; err != nil {
		t.Fatalf("create unrelated message: %v", err)
	}

	list := postJSONWithToken(t, router, "/message/list", aliceToken, []byte(`{"peer_id":`+strconv.FormatUint(uint64(bobID), 10)+`}`))
	if list.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d body=%s", list.Code, list.Body.String())
	}
	var listResp message.ListResponse
	if err := json.Unmarshal(list.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d: %+v", len(listResp.Messages), listResp.Messages)
	}
	for _, msg := range listResp.Messages {
		if !((msg.FromID == aliceID && msg.ToID == bobID) || (msg.FromID == bobID && msg.ToID == aliceID)) {
			t.Fatalf("unexpected message leaked into conversation: %+v", msg)
		}
	}
}

func TestMessageRoutesRequireJWT(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &message.Message{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)

	req := httptest.NewRequest("POST", "/message/send", bytes.NewReader([]byte(`{"to_id":2,"content":"hello"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func registerAndLoginAsWithID(t *testing.T, router http.Handler, username string) (string, uint) {
	t.Helper()
	body := bytes.NewBufferString(`{"username":"` + username + `","password":"secret123"}`)
	req := httptest.NewRequest("POST", "/account/register", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("register %s expected 200, got %d body=%s", username, rec.Code, rec.Body.String())
	}

	loginBody := bytes.NewBufferString(`{"username":"` + username + `","password":"secret123"}`)
	loginReq := httptest.NewRequest("POST", "/account/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login %s expected 200, got %d body=%s", username, loginRec.Code, loginRec.Body.String())
	}
	var response struct {
		Token     string `json:"token"`
		AccountID uint   `json:"account_id"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if response.Token == "" || response.AccountID == 0 {
		t.Fatalf("expected login token and account id, got %+v", response)
	}
	return response.Token, response.AccountID
}

func postJSONWithToken(t *testing.T, router http.Handler, path string, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
