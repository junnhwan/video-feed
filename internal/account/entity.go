package account

import "time"

type Account struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Password     string    `gorm:"size:255;not null" json:"-"`
	Token        string    `gorm:"size:512" json:"-"`
	RefreshToken string    `gorm:"size:128" json:"-"`
	AvatarURL    string    `gorm:"type:varchar(512)" json:"avatar_url,omitempty"`
	Bio          string    `gorm:"type:varchar(255)" json:"bio,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RegisterInput struct {
	Username string
	Password string
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type findByIDRequest struct {
	ID uint `json:"id"`
}

type findByUsernameRequest struct {
	Username string `json:"username"`
}

type changePasswordRequest struct {
	Username    string `json:"username"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type renameRequest struct {
	NewUsername string `json:"new_username"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type updateProfileRequest struct {
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
}

type getProfileRequest struct {
	AccountID uint `json:"account_id"`
}

type accountResponse struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Bio       string `json:"bio,omitempty"`
}

type getProfileResponse struct {
	Account       accountResponse `json:"account"`
	VideoCount    int64           `json:"video_count"`
	TotalLikes    int64           `json:"total_likes"`
	FollowerCount int64           `json:"follower_count"`
	VloggerCount  int64           `json:"vlogger_count"`
}

type LoginResult struct {
	Account      *Account
	Token        string
	RefreshToken string
}

type loginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	AccountID    uint   `json:"account_id"`
	Username     string `json:"username"`
}
