package account

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, account *Account) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *Repository) RenameWithToken(ctx context.Context, id uint, newUsername string, token string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Account{}).Where("id = ?", id).Updates(map[string]any{
			"username": newUsername,
			"token":    token,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *Repository) ChangePassword(ctx context.Context, id uint, newPassword string) error {
	return r.db.WithContext(ctx).Model(&Account{}).
		Where("id = ?", id).
		Update("password", newPassword).Error
}

func (r *Repository) FindByID(ctx context.Context, id uint) (*Account, error) {
	var account Account
	if err := r.db.WithContext(ctx).First(&account, id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) FindByUsername(ctx context.Context, username string) (*Account, error) {
	var account Account
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) FindByRefreshToken(ctx context.Context, refreshToken string) (*Account, error) {
	var account Account
	if err := r.db.WithContext(ctx).Where("refresh_token = ?", refreshToken).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) SaveTokens(ctx context.Context, id uint, token string, refreshToken string) error {
	return r.db.WithContext(ctx).Model(&Account{}).
		Where("id = ?", id).
		Updates(map[string]any{"token": token, "refresh_token": refreshToken}).Error
}

func (r *Repository) UpdateToken(ctx context.Context, id uint, token string) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Update("token", token).Error
}

func (r *Repository) UpdateAvatar(ctx context.Context, id uint, avatarURL string) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Update("avatar_url", avatarURL).Error
}

func (r *Repository) ClearTokens(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&Account{}).
		Where("id = ?", id).
		Updates(map[string]any{"token": "", "refresh_token": ""}).Error
}

func (r *Repository) UpdateFields(ctx context.Context, id uint, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Updates(updates).Error
}
