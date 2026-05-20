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
	LikesCount  int64     `gorm:"not null;default:0;index:idx_videos_likes_count_id,priority:1,sort:desc" json:"likes_count"`
	Popularity  int64     `gorm:"not null;default:0;index:idx_videos_popularity_created_id,priority:1,sort:desc" json:"popularity"`
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

type deleteVideoRequest struct {
	ID uint `json:"id"`
}
