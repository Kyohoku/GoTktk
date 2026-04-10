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
	log.Printf("INFO video_detail request received: video_id=%d", id)

	cacheKey := fmt.Sprintf("video:detail:id=%d", id)

	// 定义 get、set 缓存函数
	getCached := func() (*Video, bool) {
		if vs.cache == nil {
			return nil, false
		}

		b, err := vs.cache.GetBytes(ctx, cacheKey)
		if err != nil {
			if !rediscache.IsMiss(err) {
				log.Printf("ERROR video_detail cache read failed: video_id=%d key=%s err=%v", id, cacheKey, err)
			}
			return nil, false
		}

		var cached Video
		if err := json.Unmarshal(b, &cached); err != nil {
			log.Printf("ERROR video_detail cache unmarshal failed: video_id=%d key=%s err=%v", id, cacheKey, err)
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
			log.Printf("ERROR video_detail cache marshal failed: video_id=%d key=%s err=%v", id, cacheKey, err)
			return
		}

		if err := vs.cache.SetBytes(ctx, cacheKey, b, vs.cacheTTL); err != nil {
			log.Printf("ERROR video_detail cache write failed: video_id=%d key=%s err=%v", id, cacheKey, err)
			return
		}

		log.Printf("INFO video_detail cache_write success: video_id=%d key=%s ttl=%s", id, cacheKey, vs.cacheTTL)
	}

	if vs.cache != nil { // redis 可用
		if v, ok := getCached(); ok { // 缓存命中
			log.Printf("INFO video_detail cache_result=hit video_id=%d key=%s", id, cacheKey)
			return v, nil
		}

		log.Printf("INFO video_detail cache_result=miss video_id=%d key=%s", id, cacheKey)

		opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		b, err := vs.cache.GetBytes(opCtx, cacheKey)
		cancel()
		if err == nil {
			var cached Video
			if err := json.Unmarshal(b, &cached); err == nil {
				log.Printf("INFO video_detail cache_result=hit_after_recheck video_id=%d key=%s", id, cacheKey)
				return &cached, nil
			}
			log.Printf("ERROR video_detail cache recheck unmarshal failed: video_id=%d key=%s err=%v", id, cacheKey, err)
		} else if rediscache.IsMiss(err) { // 缓存 miss
			lockKey := "lock:" + cacheKey // lock key 设计

			lockCtx, lockCancel := context.WithTimeout(ctx, 50*time.Millisecond)
			token, locked, lockErr := vs.cache.Lock(lockCtx, lockKey, 2*time.Second) // 锁 2 秒有效
			lockCancel()

			if lockErr != nil {
				log.Printf("ERROR video_detail lock failed: video_id=%d lock_key=%s err=%v", id, lockKey, lockErr)
			}

			if lockErr == nil && locked {
				// defer 保证锁可以被释放
				defer func() {
					if err := vs.cache.Unlock(context.Background(), lockKey, token); err != nil {
						log.Printf("ERROR video_detail unlock failed: video_id=%d lock_key=%s err=%v", id, lockKey, err)
					}
				}()

				log.Printf("INFO video_detail lock acquired: video_id=%d lock_key=%s token_prefix=%s", id, lockKey, token[:8])

				if v, ok := getCached(); ok { // 拿到锁后再先查一次缓存
					log.Printf("INFO video_detail cache_result=filled_after_lock video_id=%d key=%s", id, cacheKey)
					return v, nil
				}

				log.Printf("INFO video_detail db_fallback start: video_id=%d", id)
				video, err := vs.repo.GetByID(ctx, id)
				if err != nil {
					log.Printf("ERROR video_detail db_fallback failed: video_id=%d err=%v", id, err)
					return nil, err
				}
				setCached(video)
				return video, nil
			}

			if lockErr == nil && !locked {
				log.Printf("INFO video_detail lock skipped_wait_cache_fill: video_id=%d lock_key=%s", id, lockKey)
			}

			// 没拿到锁：等待别人回填缓存
			for i := 0; i < 5; i++ { // 等待 100ms
				select {
				case <-ctx.Done():
					log.Printf("ERROR video_detail context canceled while waiting cache fill: video_id=%d err=%v", id, ctx.Err())
					return nil, ctx.Err()
				case <-time.After(20 * time.Millisecond):
				}
				if v, ok := getCached(); ok {
					log.Printf("INFO video_detail cache_result=hit_after_wait video_id=%d key=%s retry=%d", id, cacheKey, i+1)
					return v, nil
				}
			}

			log.Printf("INFO video_detail wait_cache_fill_timeout_fallback_db: video_id=%d key=%s", id, cacheKey)
		} else {
			log.Printf("ERROR video_detail cache recheck failed: video_id=%d key=%s err=%v", id, cacheKey, err)
		}
	}

	// 查数据库兜底
	log.Printf("INFO video_detail db_fallback start: video_id=%d", id)
	video, err := vs.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("ERROR video_detail db_fallback failed: video_id=%d err=%v", id, err)
		return nil, err
	}
	if vs.cache != nil {
		setCached(video) // 回填 redis
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
