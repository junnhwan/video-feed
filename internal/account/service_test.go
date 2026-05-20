package account

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&Account{}); err != nil {
		t.Fatalf("migrate account: %v", err)
	}

	return NewService(NewRepository(database))
}

func TestRegisterCreatesAccountWithHashedPassword(t *testing.T) {
	service := newTestService(t)

	account, err := service.Register(context.Background(), RegisterInput{
		Username: "alice",
		Password: "secret123",
	})

	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if account.ID == 0 {
		t.Fatal("expected generated account id")
	}
	if account.Username != "alice" {
		t.Fatalf("expected username alice, got %q", account.Username)
	}
	if account.Password == "secret123" {
		t.Fatal("expected password to be hashed")
	}
}

func TestRegisterRejectsDuplicateUsername(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if _, err := service.Register(ctx, RegisterInput{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, err := service.Register(ctx, RegisterInput{Username: "alice", Password: "secret456"})

	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
}

func TestLoginVerifiesPassword(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()
	if _, err := service.Register(ctx, RegisterInput{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	login, err := service.Login(ctx, "alice", "secret123")

	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.Account.Username != "alice" {
		t.Fatalf("expected username alice, got %q", login.Account.Username)
	}
	if login.Token == "" {
		t.Fatal("expected access token")
	}
	if login.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}

	stored, err := service.FindByID(ctx, login.Account.ID)
	if err != nil {
		t.Fatalf("find stored account: %v", err)
	}
	if stored.Token != login.Token {
		t.Fatal("expected login token to be stored on account")
	}
	if stored.RefreshToken != login.RefreshToken {
		t.Fatal("expected refresh token to be stored on account")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()
	if _, err := service.Register(ctx, RegisterInput{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := service.Login(ctx, "alice", "wrong-password")

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogoutRevokesStoredTokens(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()
	if _, err := service.Register(ctx, RegisterInput{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := service.Login(ctx, "alice", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := service.Logout(ctx, login.Account.ID); err != nil {
		t.Fatalf("logout: %v", err)
	}

	stored, err := service.FindByID(ctx, login.Account.ID)
	if err != nil {
		t.Fatalf("find stored account: %v", err)
	}
	if stored.Token != "" || stored.RefreshToken != "" {
		t.Fatalf("expected tokens to be cleared, got token=%q refresh=%q", stored.Token, stored.RefreshToken)
	}
}

func TestRefreshAccessTokenRotatesStoredAccessToken(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()
	if _, err := service.Register(ctx, RegisterInput{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := service.Login(ctx, "alice", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	refreshed, err := service.RefreshAccessToken(ctx, login.RefreshToken)

	if err != nil {
		t.Fatalf("refresh access token: %v", err)
	}
	if refreshed.Token == "" {
		t.Fatal("expected refreshed access token")
	}
	if refreshed.RefreshToken != login.RefreshToken {
		t.Fatal("expected refresh token to remain stable")
	}

	stored, err := service.FindByID(ctx, login.Account.ID)
	if err != nil {
		t.Fatalf("find stored account: %v", err)
	}
	if stored.Token != refreshed.Token {
		t.Fatal("expected refreshed token to be stored")
	}
}

func TestFindByIDReturnsAccount(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()
	created, err := service.Register(ctx, RegisterInput{Username: "alice", Password: "secret123"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	found, err := service.FindByID(ctx, created.ID)

	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found.Username != "alice" {
		t.Fatalf("expected username alice, got %q", found.Username)
	}
}
