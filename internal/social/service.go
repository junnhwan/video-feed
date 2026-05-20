package social

import (
	"context"
	"errors"

	"video-feed/internal/account"
)

var (
	ErrCannotFollowSelf = errors.New("can not follow self")
	ErrAlreadyFollowed  = errors.New("already followed")
	ErrNotFollowed      = errors.New("not followed")
)

type Service struct {
	repo        *Repository
	accountRepo *account.Repository
	publisher   SocialEventPublisher
}

type SocialEventPublisher interface {
	Follow(ctx context.Context, followerID uint, vloggerID uint) error
	Unfollow(ctx context.Context, followerID uint, vloggerID uint) error
}

func NewService(repo *Repository, accountRepo *account.Repository) *Service {
	return &Service{repo: repo, accountRepo: accountRepo}
}

func (s *Service) SetPublisher(publisher SocialEventPublisher) {
	s.publisher = publisher
}

func (s *Service) Follow(ctx context.Context, followerID uint, vloggerID uint) error {
	if followerID == vloggerID {
		return ErrCannotFollowSelf
	}
	if err := s.ensureAccountsExist(ctx, followerID, vloggerID); err != nil {
		return err
	}
	isFollowed, err := s.repo.IsFollowed(ctx, followerID, vloggerID)
	if err != nil {
		return err
	}
	if isFollowed {
		return ErrAlreadyFollowed
	}
	if s.publisher != nil {
		if err := s.publisher.Follow(ctx, followerID, vloggerID); err == nil {
			return nil
		}
	}
	return s.repo.Follow(ctx, followerID, vloggerID)
}

func (s *Service) Unfollow(ctx context.Context, followerID uint, vloggerID uint) error {
	if err := s.ensureAccountsExist(ctx, followerID, vloggerID); err != nil {
		return err
	}
	isFollowed, err := s.repo.IsFollowed(ctx, followerID, vloggerID)
	if err != nil {
		return err
	}
	if !isFollowed {
		return ErrNotFollowed
	}
	if s.publisher != nil {
		if err := s.publisher.Unfollow(ctx, followerID, vloggerID); err == nil {
			return nil
		}
	}
	return s.repo.Unfollow(ctx, followerID, vloggerID)
}

func (s *Service) IsFollowed(ctx context.Context, followerID uint, vloggerID uint) (bool, error) {
	if err := s.ensureAccountsExist(ctx, followerID, vloggerID); err != nil {
		return false, err
	}
	return s.repo.IsFollowed(ctx, followerID, vloggerID)
}

func (s *Service) GetAllFollowers(ctx context.Context, vloggerID uint) ([]*account.Account, error) {
	if _, err := s.accountRepo.FindByID(ctx, vloggerID); err != nil {
		return nil, err
	}
	return s.repo.GetAllFollowers(ctx, vloggerID)
}

func (s *Service) GetAllVloggers(ctx context.Context, followerID uint) ([]*account.Account, error) {
	if _, err := s.accountRepo.FindByID(ctx, followerID); err != nil {
		return nil, err
	}
	return s.repo.GetAllVloggers(ctx, followerID)
}

func (s *Service) CountFollowers(ctx context.Context, vloggerID uint) (int64, error) {
	return s.repo.CountFollowers(ctx, vloggerID)
}

func (s *Service) CountVloggers(ctx context.Context, followerID uint) (int64, error) {
	return s.repo.CountVloggers(ctx, followerID)
}

func (s *Service) ensureAccountsExist(ctx context.Context, followerID uint, vloggerID uint) error {
	if _, err := s.accountRepo.FindByID(ctx, followerID); err != nil {
		return err
	}
	if _, err := s.accountRepo.FindByID(ctx, vloggerID); err != nil {
		return err
	}
	return nil
}
