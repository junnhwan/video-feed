package message

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestServiceSendAndListConversation(t *testing.T) {
	service := newMessageService(t)
	ctx := context.Background()

	first, err := service.Send(ctx, 1, 2, " hello ")
	if err != nil {
		t.Fatalf("send first message: %v", err)
	}
	if first.Content != "hello" {
		t.Fatalf("expected trimmed content, got %q", first.Content)
	}
	if _, err := service.Send(ctx, 2, 1, "reply"); err != nil {
		t.Fatalf("send reply: %v", err)
	}
	if _, err := service.Send(ctx, 3, 1, "intruder"); err != nil {
		t.Fatalf("send unrelated message: %v", err)
	}

	messages, err := service.List(ctx, 1, 2, 50)
	if err != nil {
		t.Fatalf("list conversation: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 conversation messages, got %d: %+v", len(messages), messages)
	}
	for _, msg := range messages {
		if !((msg.FromID == 1 && msg.ToID == 2) || (msg.FromID == 2 && msg.ToID == 1)) {
			t.Fatalf("unexpected message from another conversation: %+v", msg)
		}
	}
}

func TestServiceRejectsInvalidMessageInput(t *testing.T) {
	service := newMessageService(t)
	ctx := context.Background()

	if _, err := service.Send(ctx, 1, 0, "hello"); err != ErrInvalidMessageInput {
		t.Fatalf("expected ErrInvalidMessageInput for missing recipient, got %v", err)
	}
	if _, err := service.Send(ctx, 1, 2, "   "); err != ErrInvalidMessageInput {
		t.Fatalf("expected ErrInvalidMessageInput for blank content, got %v", err)
	}
	if _, err := service.List(ctx, 1, 0, 50); err != ErrInvalidMessageInput {
		t.Fatalf("expected ErrInvalidMessageInput for missing peer, got %v", err)
	}
}

func newMessageService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Message{}); err != nil {
		t.Fatalf("migrate message: %v", err)
	}
	return NewService(NewRepository(db))
}
