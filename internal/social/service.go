package social

import (
	"context"
	"errors"
	"gotik/internal/account"
)

type Service struct {
	repository        *Repository
	accountRepository *account.AccountRepository
}

func NewSocialService(repository *Repository, accountRepository *account.AccountRepository) *Service {
	return &Service{
		repository:        repository,
		accountRepository: accountRepository,
	}
}

func (s *Service) Follow(ctx context.Context, relation *Social) error {
	if relation.FollowerID == relation.VloggerID {
		return errors.New("can not follow self")
	}

	if _, err := s.accountRepository.FindByID(ctx, relation.FollowerID); err != nil {
		return err
	}
	if _, err := s.accountRepository.FindByID(ctx, relation.VloggerID); err != nil {
		return err
	}

	isFollowed, err := s.repository.IsFollowed(ctx, relation)
	if err != nil {
		return err
	}
	if isFollowed {
		return errors.New("already followed")
	}

	return s.repository.Follow(ctx, relation)
}

func (s *Service) Unfollow(ctx context.Context, relation *Social) error {
	if _, err := s.accountRepository.FindByID(ctx, relation.FollowerID); err != nil {
		return err
	}
	if _, err := s.accountRepository.FindByID(ctx, relation.VloggerID); err != nil {
		return err
	}

	isFollowed, err := s.repository.IsFollowed(ctx, relation)
	if err != nil {
		return err
	}
	if !isFollowed {
		return errors.New("not followed")
	}

	return s.repository.Unfollow(ctx, relation)
}

func (s *Service) GetAllFollowers(ctx context.Context, vloggerID uint) ([]*account.Account, error) {
	if _, err := s.accountRepository.FindByID(ctx, vloggerID); err != nil {
		return nil, err
	}
	return s.repository.GetAllFollowers(ctx, vloggerID)
}

func (s *Service) GetAllVloggers(ctx context.Context, followerID uint) ([]*account.Account, error) {
	if _, err := s.accountRepository.FindByID(ctx, followerID); err != nil {
		return nil, err
	}
	return s.repository.GetAllVloggers(ctx, followerID)
}
