package account

import (
	"context"
	"errors"
	"strings"

	"video-feed/internal/auth"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUsernameTaken       = errors.New("username already exists")
	ErrInvalidCredentials  = errors.New("invalid username or password")
	ErrInvalidInput        = errors.New("username and password are required")
	ErrNewUsernameRequired = errors.New("new_username is required")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (*Account, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" || input.Password == "" {
		return nil, ErrInvalidInput
	}

	if _, err := s.repo.FindByUsername(ctx, username); err == nil {
		return nil, ErrUsernameTaken
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	account := &Account{
		Username: username,
		Password: string(hash),
	}
	if err := s.repo.Create(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	account, err := s.repo.FindByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	token, err := auth.GenerateToken(account.ID, account.Username)
	if err != nil {
		return nil, err
	}
	refreshToken, err := auth.GenerateRefreshToken(account.ID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveTokens(ctx, account.ID, token, refreshToken); err != nil {
		return nil, err
	}
	account.Token = token
	account.RefreshToken = refreshToken
	return &LoginResult{Account: account, Token: token, RefreshToken: refreshToken}, nil
}

func (s *Service) FindByID(ctx context.Context, id uint) (*Account, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) FindByUsername(ctx context.Context, username string) (*Account, error) {
	return s.repo.FindByUsername(ctx, strings.TrimSpace(username))
}

func (s *Service) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	account, err := s.FindByUsername(ctx, username)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.repo.ChangePassword(ctx, account.ID, string(hash)); err != nil {
		return err
	}
	return s.Logout(ctx, account.ID)
}

func (s *Service) Rename(ctx context.Context, accountID uint, newUsername string) (string, error) {
	newUsername = strings.TrimSpace(newUsername)
	if newUsername == "" {
		return "", ErrNewUsernameRequired
	}
	token, err := auth.GenerateToken(accountID, newUsername)
	if err != nil {
		return "", err
	}
	if err := s.repo.RenameWithToken(ctx, accountID, newUsername, token); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) Logout(ctx context.Context, accountID uint) error {
	return s.repo.ClearTokens(ctx, accountID)
}

func (s *Service) RefreshAccessToken(ctx context.Context, refreshToken string) (*LoginResult, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, ErrInvalidCredentials
	}
	account, err := s.repo.FindByRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	token, err := auth.GenerateToken(account.ID, account.Username)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateToken(ctx, account.ID, token); err != nil {
		return nil, err
	}
	account.Token = token
	return &LoginResult{Account: account, Token: token, RefreshToken: account.RefreshToken}, nil
}

func (s *Service) UpdateProfile(ctx context.Context, accountID uint, avatarURL string, bio string) error {
	updates := map[string]any{}
	if strings.TrimSpace(avatarURL) != "" {
		updates["avatar_url"] = strings.TrimSpace(avatarURL)
	}
	if strings.TrimSpace(bio) != "" {
		updates["bio"] = strings.TrimSpace(bio)
	}
	if len(updates) == 0 {
		return ErrInvalidInput
	}
	return s.repo.UpdateFields(ctx, accountID, updates)
}
