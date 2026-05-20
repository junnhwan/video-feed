package social

import (
	"context"
	"errors"
	"testing"

	"video-feed/internal/account"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newSocialTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&account.Account{}, &Social{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	return NewService(NewRepository(db), account.NewRepository(db)), db
}

func TestFollowCreatesRelationAndCounts(t *testing.T) {
	service, db := newSocialTestService(t)
	createAccount(t, db, 1, "alice")
	createAccount(t, db, 2, "bob")

	if err := service.Follow(context.Background(), 1, 2); err != nil {
		t.Fatalf("follow: %v", err)
	}

	isFollowed, err := service.IsFollowed(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("is followed: %v", err)
	}
	if !isFollowed {
		t.Fatal("expected relation to exist")
	}
	followerCount, err := service.CountFollowers(context.Background(), 2)
	if err != nil {
		t.Fatalf("count followers: %v", err)
	}
	if followerCount != 1 {
		t.Fatalf("expected follower count 1, got %d", followerCount)
	}
}

func TestFollowRejectsSelfAndDuplicate(t *testing.T) {
	service, db := newSocialTestService(t)
	createAccount(t, db, 1, "alice")
	createAccount(t, db, 2, "bob")

	if err := service.Follow(context.Background(), 1, 1); !errors.Is(err, ErrCannotFollowSelf) {
		t.Fatalf("expected ErrCannotFollowSelf, got %v", err)
	}
	if err := service.Follow(context.Background(), 1, 2); err != nil {
		t.Fatalf("follow: %v", err)
	}
	if err := service.Follow(context.Background(), 1, 2); !errors.Is(err, ErrAlreadyFollowed) {
		t.Fatalf("expected ErrAlreadyFollowed, got %v", err)
	}
}

func TestUnfollowRemovesRelation(t *testing.T) {
	service, db := newSocialTestService(t)
	createAccount(t, db, 1, "alice")
	createAccount(t, db, 2, "bob")
	if err := service.Follow(context.Background(), 1, 2); err != nil {
		t.Fatalf("follow: %v", err)
	}

	if err := service.Unfollow(context.Background(), 1, 2); err != nil {
		t.Fatalf("unfollow: %v", err)
	}

	isFollowed, err := service.IsFollowed(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("is followed: %v", err)
	}
	if isFollowed {
		t.Fatal("expected relation to be removed")
	}
}

func createAccount(t *testing.T, db *gorm.DB, id uint, username string) {
	t.Helper()
	if err := db.Create(&account.Account{ID: id, Username: username, Password: "hash"}).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
}
