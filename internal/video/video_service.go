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

	if vs.cache != nil { //redis 运行中
		if v, ok := getCached(); ok { //缓存命中
			log.Printf("video detail cache hit: key=%s", cacheKey)
			return v, nil
		}

		log.Printf("video detail cache miss: key=%s", cacheKey)

		opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		b, err := vs.cache.GetBytes(opCtx, cacheKey)
		cancel()
		if err == nil {
			var cached Video
			if err := json.Unmarshal(b, &cached); err == nil {
				return &cached, nil
			}
		} else if rediscache.IsMiss(err) { //缓存 miss
			lockKey := "lock:" + cacheKey //lock key 设计

			lockCtx, lockCancel := context.WithTimeout(ctx, 50*time.Millisecond)
			token, locked, lockErr := vs.cache.Lock(lockCtx, lockKey, 2*time.Second) //锁2秒有效
			lockCancel()

			if lockErr == nil && locked {
				//defer 保证锁可以被释放
				defer func() { _ = vs.cache.Unlock(context.Background(), lockKey, token) }()

				log.Printf("video detail lock acquired: lockKey=%s token_prefix=%s", lockKey, token[:8])

				if v, ok := getCached(); ok { //拿到锁后再先查一次缓存
					log.Printf("video detail cache filled before db fallback: key=%s", cacheKey) //拿到锁后发现缓存已经被填写了

					return v, nil
				}

				video, err := vs.repo.GetByID(ctx, id)
				if err != nil {
					return nil, err
				}
				setCached(video)
				return video, nil
			}

			// 没拿到锁：等待别人回填缓存
			for i := 0; i < 5; i++ { //等待100ms
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(20 * time.Millisecond):
				}
				if v, ok := getCached(); ok {
					return v, nil
				}
			}
		}
	}
	//查数据库兜底
	video, err := vs.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if vs.cache != nil {
		setCached(video) //回填 redis
	}
	return video, nil
}

//func (vs *VideoService) UpdatePopularity(ctx context.Context, id uint, change int64) error {
//	//先更新数据库
//	if err := vs.repo.UpdatePopularity(ctx, id, change); err != nil {
//		return err
//	}
//
//	if vs.cache != nil {
//		//详情缓存 key 失效
//		_ = vs.cache.Del(context.Background(), fmt.Sprintf("video:detail:id=%d", id))
//
//		member := strconv.FormatUint(uint64(id), 10)
//		_ = vs.cache.ZincrBy(ctx, "hot:video", member, float64(change))
//	}
//
//	return nil
//}
