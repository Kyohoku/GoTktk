package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	rediscache "gotik/internal/middleware/redis"
	"log"
	"strings"
	"time"
)

type VideoService struct {
	repo     *VideoRepository
	cache    *rediscache.Client
	cacheTTL time.Duration
}

func NewVideoService(repo *VideoRepository, cache *rediscache.Client) *VideoService {

	return &VideoService{repo: repo, cache: cache, cacheTTL: 5 * time.Minute}
}

func (vs *VideoService) Publish(ctx context.Context, video *Video) error {
	if video == nil {
		return errors.New("video is nil")
	}

	video.Title = strings.TrimSpace(video.Title)
	video.Description = strings.TrimSpace(video.Description)
	video.PlayURL = strings.TrimSpace(video.PlayURL)
	video.CoverURL = strings.TrimSpace(video.CoverURL)

	if video.Title == "" {
		return errors.New("title is required")
	}
	if video.PlayURL == "" {
		return errors.New("play url is required")
	}
	if video.CoverURL == "" {
		return errors.New("cover url is required")
	}

	if err := vs.repo.CreateVideo(ctx, video); err != nil {
		return err
	}
	return nil
}

func (vs *VideoService) ListByAuthorID(ctx context.Context, authorID uint) ([]Video, error) {
	videos, err := vs.repo.ListByAuthorID(ctx, authorID)
	if err != nil {
		return nil, err
	}
	return videos, nil
}

func (vs *VideoService) GetDetail(ctx context.Context, id uint) (*Video, error) {
	cacheKey := fmt.Sprintf("video:detail:id=%d", id)

	//定义 get、set  缓存函数
	getCached := func() (*Video, bool) {
		if vs.cache == nil {
			return nil, false
		}

		b, err := vs.cache.GetBytes(ctx, cacheKey)
		if err != nil {
			return nil, false
		}

		var cached Video
		if err := json.Unmarshal(b, &cached); err != nil {
			return nil, false
		}

		return &cached, true
	}

	setCached := func(video *Video) {
		if vs.cache == nil || video == nil {
			return
		}

		b, err := json.Marshal(video)
		if err != nil {
			return
		}

		_ = vs.cache.SetBytes(ctx, cacheKey, b, vs.cacheTTL)
	}

	if video, ok := getCached(); ok {
		log.Printf("video detail cache hit: key=%d", video.ID)
		return video, nil
	}

	//缓存 miss 失效的兜底
	video, err := vs.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	setCached(video)
	return video, nil
}
