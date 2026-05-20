package message

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) AutoMigrate(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&Message{})
}

func (r *Repository) Send(ctx context.Context, message *Message) error {
	message.CreatedAt = time.Now()
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *Repository) List(ctx context.Context, userID uint, peerID uint, limit int) ([]Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	var messages []Message
	err := r.db.WithContext(ctx).
		Where("(from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)", userID, peerID, peerID, userID).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}
