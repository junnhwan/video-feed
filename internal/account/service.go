package account

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"video-feed/internal/auth"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/observability"

	"go.uber.org/zap"
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
	repo  *Repository
	cache *rediscache.Client
}

func NewService(repo *Repository, cache ...*rediscache.Client) *Service {
	service := &Service{repo: repo}
	if len(cache) > 0 {
		service.cache = cache[0]
	}
	return service
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
	s.cacheToken(ctx, account.ID, token)
	s.cacheRefreshToken(ctx, account.ID, refreshToken)
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
	s.cacheToken(ctx, accountID, token)
	return token, nil
}

func (s *Service) Logout(ctx context.Context, accountID uint) error {
	accountInfo, err := s.FindByID(ctx, accountID)
	if err != nil {
		return err
	}
	if err := s.repo.ClearTokens(ctx, accountID); err != nil {
		return err
	}
	if s.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		if err := s.cache.Del(cacheCtx, s.cache.Key("account:%d", accountID)); err != nil {
			observability.WithContext(ctx).Warn("delete account token cache failed", zap.Error(err))
		}
		if err := s.cache.Del(cacheCtx, s.cache.Key("account:%d:refresh", accountID)); err != nil {
			observability.WithContext(ctx).Warn("delete refresh token cache failed", zap.Error(err))
		}
		if accountInfo.RefreshToken != "" {
			if err := s.cache.Del(cacheCtx, s.cache.Key("refresh:%s", accountInfo.RefreshToken)); err != nil {
				observability.WithContext(ctx).Warn("delete refresh lookup cache failed", zap.Error(err))
			}
		}
	}
	return nil
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
	s.cacheToken(ctx, account.ID, token)
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

func (s *Service) UpdateAvatar(ctx context.Context, accountID uint, avatarURL string) error {
	avatarURL = strings.TrimSpace(avatarURL)
	if avatarURL == "" {
		return ErrInvalidInput
	}
	return s.repo.UpdateAvatar(ctx, accountID, avatarURL)
}

func (s *Service) cacheToken(ctx context.Context, accountID uint, token string) {
	if s.cache == nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := s.cache.SetBytes(cacheCtx, s.cache.Key("account:%d", accountID), []byte(token), 24*time.Hour); err != nil {
		observability.WithContext(ctx).Warn("set account token cache failed", zap.Error(err))
	}
}

func (s *Service) cacheRefreshToken(ctx context.Context, accountID uint, refreshToken string) {
	if s.cache == nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := s.cache.SetBytes(cacheCtx, s.cache.Key("account:%d:refresh", accountID), []byte(refreshToken), 7*24*time.Hour); err != nil {
		observability.WithContext(ctx).Warn("set account refresh cache failed", zap.Error(err))
	}
	accountIDBytes := []byte(strconv.FormatUint(uint64(accountID), 10))
	if err := s.cache.SetBytes(cacheCtx, s.cache.Key("refresh:%s", refreshToken), accountIDBytes, 7*24*time.Hour); err != nil {
		observability.WithContext(ctx).Warn("set refresh lookup cache failed", zap.Error(err))
	}
}
