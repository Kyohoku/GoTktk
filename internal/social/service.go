package social

import (
	"context"
	"errors"
	"gotik/internal/account"
	"gotik/internal/middleware/rabbitmq"
	"log"
)

type Service struct {
	repository        *Repository
	accountRepository *account.AccountRepository
	socialMQ          *rabbitmq.SocialMQ
}

func NewSocialService(repository *Repository, accountRepository *account.AccountRepository, socialMQ *rabbitmq.SocialMQ) *Service {
	return &Service{
		repository:        repository,
		accountRepository: accountRepository,
		socialMQ:          socialMQ,
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

	enqueued := false
	if s.socialMQ != nil {
		//异步执行关注
		if err := s.socialMQ.Follow(ctx, relation.FollowerID, relation.VloggerID); err == nil {
			log.Printf("follow request enqueued to rabbitmq: follower_id=%d vlogger_id=%d", relation.FollowerID, relation.VloggerID)
			enqueued = true
		}
	}
	if enqueued {
		return nil
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

	enqueued := false
	if s.socialMQ != nil {
		if err := s.socialMQ.UnFollow(ctx, relation.FollowerID, relation.VloggerID); err == nil {
			log.Printf("unfollow request enqueued to rabbitmq: follower_id=%d vlogger_id=%d", relation.FollowerID, relation.VloggerID)
			enqueued = true
		}
	}
	if enqueued {
		return nil
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
