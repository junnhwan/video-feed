package video

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	rediscache "video-feed/internal/middleware/redis"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestLikeUsesMQWhenPublishersSucceed(t *testing.T) {
	service, db := newLikeTestService(t)
	video := createServiceTestVideo(t, db)
	likePub := &fakeLikePublisher{}
	popPub := &fakePopularityPublisher{}
	service.SetPublishers(likePub, popPub)

	if err := service.Like(context.Background(), video.ID, 7); err != nil {
		t.Fatalf("like: %v", err)
	}

	if likePub.likeCalls != 1 || likePub.lastVideoID != video.ID || likePub.lastUserID != 7 {
		t.Fatalf("expected like event to be published, got %+v", likePub)
	}
	if popPub.calls != 1 || popPub.lastVideoID != video.ID || popPub.lastChange != 1 {
		t.Fatalf("expected popularity event to be published, got %+v", popPub)
	}
	var likes int64
	if err := db.Model(&Like{}).Where("video_id = ?", video.ID).Count(&likes).Error; err != nil {
		t.Fatalf("count likes: %v", err)
	}
	if likes != 0 {
		t.Fatalf("expected API path to skip direct DB write when MQ succeeds, got %d likes", likes)
	}
}

func TestLikeFallsBackToDBAndRedisWhenMQPublishFails(t *testing.T) {
	cache, _ := newVideoRedisClient(t)
	service, db := newLikeTestService(t)
	service.cache = cache
	video := createServiceTestVideo(t, db)
	likePub := &fakeLikePublisher{err: errors.New("mq down")}
	popPub := &fakePopularityPublisher{err: errors.New("mq down")}
	service.SetPublishers(likePub, popPub)

	if err := service.Like(context.Background(), video.ID, 7); err != nil {
		t.Fatalf("like fallback: %v", err)
	}

	var stored Video
	if err := db.First(&stored, video.ID).Error; err != nil {
		t.Fatalf("find video: %v", err)
	}
	if stored.LikesCount != 1 || stored.Popularity != 1 {
		t.Fatalf("expected DB fallback counters 1/1, got likes=%d popularity=%d", stored.LikesCount, stored.Popularity)
	}
	assertHotCacheMember(t, cache, video.ID)
}

func TestCommentUsesMQWhenPublishersSucceed(t *testing.T) {
	service, db := newCommentTestService(t)
	video := createServiceTestVideo(t, db)
	commentPub := &fakeCommentPublisher{}
	popPub := &fakePopularityPublisher{}
	service.SetPublishers(commentPub, popPub)

	comment, err := service.Publish(context.Background(), PublishCommentInput{
		VideoID:  video.ID,
		AuthorID: 7,
		Username: "alice",
		Content:  "hello",
	})
	if err != nil {
		t.Fatalf("publish comment: %v", err)
	}

	if comment.ID != 0 {
		t.Fatalf("expected async comment placeholder without DB id, got %d", comment.ID)
	}
	if commentPub.publishCalls != 1 || popPub.calls != 1 {
		t.Fatalf("expected comment and popularity events, got comment=%+v popularity=%+v", commentPub, popPub)
	}
	var comments int64
	if err := db.Model(&Comment{}).Where("video_id = ?", video.ID).Count(&comments).Error; err != nil {
		t.Fatalf("count comments: %v", err)
	}
	if comments != 0 {
		t.Fatalf("expected API path to skip direct comment write when MQ succeeds, got %d", comments)
	}
}

type fakeLikePublisher struct {
	err         error
	likeCalls   int
	unlikeCalls int
	lastUserID  uint
	lastVideoID uint
}

func (f *fakeLikePublisher) Like(_ context.Context, userID uint, videoID uint) error {
	f.likeCalls++
	f.lastUserID = userID
	f.lastVideoID = videoID
	return f.err
}

func (f *fakeLikePublisher) Unlike(_ context.Context, userID uint, videoID uint) error {
	f.unlikeCalls++
	f.lastUserID = userID
	f.lastVideoID = videoID
	return f.err
}

type fakePopularityPublisher struct {
	err         error
	calls       int
	lastVideoID uint
	lastChange  int64
}

func (f *fakePopularityPublisher) Update(_ context.Context, videoID uint, change int64) error {
	f.calls++
	f.lastVideoID = videoID
	f.lastChange = change
	return f.err
}

type fakeCommentPublisher struct {
	err          error
	publishCalls int
	deleteCalls  int
}

func (f *fakeCommentPublisher) Publish(_ context.Context, _ string, _ uint, _ uint, _ string) error {
	f.publishCalls++
	return f.err
}

func (f *fakeCommentPublisher) Delete(_ context.Context, _ uint) error {
	f.deleteCalls++
	return f.err
}

func newVideoRedisClient(t *testing.T) (*rediscache.Client, *miniredis.Miniredis) {
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

func assertHotCacheMember(t *testing.T, cache *rediscache.Client, videoID uint) {
	t.Helper()
	before := time.Now().UTC().Truncate(time.Minute)
	after := time.Now().UTC().Truncate(time.Minute)
	expected := strconv.FormatUint(uint64(videoID), 10)
	for _, minute := range []time.Time{before, after} {
		key := cache.Key("hot:video:1m:%s", minute.Format("200601021504"))
		members, err := cache.ZRevRange(context.Background(), key, 0, 0)
		if err != nil {
			t.Fatalf("read hot cache: %v", err)
		}
		if len(members) == 1 && members[0] == expected {
			return
		}
	}
	t.Fatalf("expected hot cache member %s", expected)
}
