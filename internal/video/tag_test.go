package video

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestExtractTagsDeduplicatesAndSupportsUnicode(t *testing.T) {
	tags := ExtractTags("hello #Go #后端 #Go #go_redis plain")

	expected := []string{"Go", "后端", "go_redis"}
	if len(tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d: %#v", len(expected), len(tags), tags)
	}
	for i := range expected {
		if tags[i] != expected[i] {
			t.Fatalf("expected tag %q at index %d, got %q", expected[i], i, tags[i])
		}
	}
}

func TestPublishCreatesTagsAndVideoRelations(t *testing.T) {
	service, db := newTagTestService(t)

	published, err := service.Publish(context.Background(), PublishInput{
		AuthorID:    1,
		Username:    "alice",
		Title:       "first #Go vlog",
		Description: "learn #后端 #Go",
		PlayURL:     "1.mp4",
		CoverURL:    "1.jpg",
	})

	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	var tags []Tag
	if err := db.Order("name ASC").Find(&tags).Error; err != nil {
		t.Fatalf("find tags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 unique tags, got %d", len(tags))
	}
	var relations []VideoTag
	if err := db.Where("video_id = ?", published.ID).Find(&relations).Error; err != nil {
		t.Fatalf("find video tags: %v", err)
	}
	if len(relations) != 2 {
		t.Fatalf("expected 2 video tag relations, got %d", len(relations))
	}
}

func newTagTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Video{}, &OutboxMsg{}, &Tag{}, &VideoTag{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	return NewService(NewRepository(db)), db
}
