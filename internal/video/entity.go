package video

import "time"

type Video struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AuthorID    uint      `gorm:"index;not null" json:"author_id"`
	Username    string    `gorm:"size:64;not null" json:"username"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Description string    `gorm:"size:500" json:"description,omitempty"`
	PlayURL     string    `gorm:"size:512;not null" json:"play_url"`
	CoverURL    string    `gorm:"size:512;not null" json:"cover_url"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PublishInput struct {
	AuthorID    uint
	Username    string
	Title       string
	Description string
	PlayURL     string
	CoverURL    string
}

type publishRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	PlayURL     string `json:"play_url"`
	CoverURL    string `json:"cover_url"`
}

type getDetailRequest struct {
	ID uint `json:"id"`
}

type listByAuthorIDRequest struct {
	AuthorID uint `json:"author_id"`
}
