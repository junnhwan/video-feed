package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"video-feed/internal/account"
	"video-feed/internal/social"
	"video-feed/internal/video"
	"video-feed/internal/worker"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestNotificationRoutesListUnreadAndMarkRead(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}, &video.Video{}, &video.Like{}, &video.Comment{}, &video.Tag{}, &video.VideoTag{}, &social.Social{}, &worker.Notification{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	router := NewRouter(database)
	token := registerAndLogin(t, router)

	notification := worker.Notification{
		RecipientID: 1,
		SenderID:    2,
		Type:        "follow",
		TargetID:    2,
		Content:     "关注了你",
	}
	if err := database.Create(&notification).Error; err != nil {
		t.Fatalf("create notification: %v", err)
	}

	count := postNotification(t, router, "/notification/unreadCount", token, nil)
	if count.Code != http.StatusOK {
		t.Fatalf("unread count expected 200, got %d body=%s", count.Code, count.Body.String())
	}
	var countResp struct {
		Count int64 `json:"count"`
	}
	if err := json.Unmarshal(count.Body.Bytes(), &countResp); err != nil {
		t.Fatalf("decode unread count: %v", err)
	}
	if countResp.Count != 1 {
		t.Fatalf("expected unread count 1, got %d", countResp.Count)
	}

	list := postNotification(t, router, "/notification/list", token, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d body=%s", list.Code, list.Body.String())
	}
	var listResp struct {
		Notifications []worker.Notification `json:"notifications"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Notifications) != 1 || listResp.Notifications[0].ID != notification.ID {
		t.Fatalf("unexpected notifications: %+v", listResp.Notifications)
	}

	markRead := postNotification(t, router, "/notification/markRead", token, []byte(`{"id":`+strconv.FormatUint(uint64(notification.ID), 10)+`}`))
	if markRead.Code != http.StatusOK {
		t.Fatalf("mark read expected 200, got %d body=%s", markRead.Code, markRead.Body.String())
	}

	count = postNotification(t, router, "/notification/unreadCount", token, nil)
	if count.Code != http.StatusOK {
		t.Fatalf("unread count after mark expected 200, got %d body=%s", count.Code, count.Body.String())
	}
	if err := json.Unmarshal(count.Body.Bytes(), &countResp); err != nil {
		t.Fatalf("decode unread count after mark: %v", err)
	}
	if countResp.Count != 0 {
		t.Fatalf("expected unread count 0, got %d", countResp.Count)
	}
}

func postNotification(t *testing.T, router http.Handler, path string, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	if body == nil {
		body = []byte(`{}`)
	}
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
