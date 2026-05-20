package social

import (
	"context"
	"errors"
	"testing"
)

func TestFollowPersistsRelationAndPublishesEventWhenPublisherSucceeds(t *testing.T) {
	service, db := newSocialTestService(t)
	createAccount(t, db, 1, "alice")
	createAccount(t, db, 2, "bob")
	pub := &fakeSocialPublisher{}
	service.SetPublisher(pub)

	if err := service.Follow(context.Background(), 1, 2); err != nil {
		t.Fatalf("follow: %v", err)
	}

	if pub.followCalls != 1 || pub.lastFollowerID != 1 || pub.lastVloggerID != 2 {
		t.Fatalf("expected follow event, got %+v", pub)
	}
	isFollowed, err := service.repo.IsFollowed(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("is followed: %v", err)
	}
	if !isFollowed {
		t.Fatal("expected relation to be persisted immediately even when MQ succeeds")
	}
}

func TestFollowFallsBackToDBWhenPublisherFails(t *testing.T) {
	service, db := newSocialTestService(t)
	createAccount(t, db, 1, "alice")
	createAccount(t, db, 2, "bob")
	service.SetPublisher(&fakeSocialPublisher{err: errors.New("mq down")})

	if err := service.Follow(context.Background(), 1, 2); err != nil {
		t.Fatalf("follow fallback: %v", err)
	}

	isFollowed, err := service.repo.IsFollowed(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("is followed: %v", err)
	}
	if !isFollowed {
		t.Fatal("expected DB fallback relation")
	}
}

type fakeSocialPublisher struct {
	err            error
	followCalls    int
	unfollowCalls  int
	lastFollowerID uint
	lastVloggerID  uint
}

func (f *fakeSocialPublisher) Follow(_ context.Context, followerID uint, vloggerID uint) error {
	f.followCalls++
	f.lastFollowerID = followerID
	f.lastVloggerID = vloggerID
	return f.err
}

func (f *fakeSocialPublisher) Unfollow(_ context.Context, followerID uint, vloggerID uint) error {
	f.unfollowCalls++
	f.lastFollowerID = followerID
	f.lastVloggerID = vloggerID
	return f.err
}
