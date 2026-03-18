package video

import (
	"context"
	"errors"
	rediscache "gotik/internal/middleware/redis"
	"time"
)

type LikeService struct {
	repo      *LikeRepository
	VideoRepo *VideoRepository
	cache     *rediscache.Client
}

func NewLikeService(repo *LikeRepository, videoRepo *VideoRepository, cache *rediscache.Client) *LikeService {
	return &LikeService{repo: repo, VideoRepo: videoRepo, cache: cache}
}

func (s *LikeService) Like(ctx context.Context, like *Like) error {
	if like == nil {
		return errors.New("like is nil")
	}
	if like.VideoID == 0 || like.AccountID == 0 {
		return errors.New("video_id and account_id are required")
	}

	ok, err := s.VideoRepo.IsExist(ctx, like.VideoID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("video not found")
	}

	isLiked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if isLiked {
		return errors.New("user has liked this video")
	}

	like.CreatedAt = time.Now()

	if err := s.repo.Like(ctx, like); err != nil {
		return err
	}

	if err := s.VideoRepo.ChangeLikesCount(ctx, like.VideoID, 1); err != nil {
		return err
	}

	if err := s.VideoRepo.UpdatePopularity(ctx, like.VideoID, 1); err != nil {
		return err
	}

	//更新热榜
	UpdatePopularityCache(ctx, s.cache, like.VideoID, 1)

	return nil
}

func (s *LikeService) Unlike(ctx context.Context, like *Like) error {
	if like == nil {
		return errors.New("like is nil")
	}
	if like.VideoID == 0 || like.AccountID == 0 {
		return errors.New("video_id and account_id are required")
	}

	ok, err := s.VideoRepo.IsExist(ctx, like.VideoID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("video not found")
	}

	isLiked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if !isLiked {
		return errors.New("user has not liked this video")
	}

	deleted, err := s.repo.DeleteByVideoAndAccount(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if !deleted {
		return errors.New("user has not liked this video")
	}

	if err := s.VideoRepo.ChangeLikesCount(ctx, like.VideoID, -1); err != nil {
		return err
	}

	if err := s.VideoRepo.UpdatePopularity(ctx, like.VideoID, -1); err != nil {
		return err
	}

	UpdatePopularityCache(ctx, s.cache, like.VideoID, -1)

	return nil
}

func (s *LikeService) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	return s.repo.IsLiked(ctx, videoID, accountID)
}

func (s *LikeService) ListLikedVideos(ctx context.Context, accountID uint) ([]Video, error) {
	return s.repo.ListLikedVideos(ctx, accountID)
}
