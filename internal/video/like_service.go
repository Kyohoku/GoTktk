package video

import (
	"context"
	"errors"
	"gotik/internal/middleware/rabbitmq"
	rediscache "gotik/internal/middleware/redis"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type LikeService struct {
	repo         *LikeRepository
	VideoRepo    *VideoRepository
	cache        *rediscache.Client
	likeMQ       *rabbitmq.LikeMQ
	popularityMQ *rabbitmq.PopularityMQ
}

func NewLikeService(repo *LikeRepository, videoRepo *VideoRepository, cache *rediscache.Client, likeMQ *rabbitmq.LikeMQ, popularityMQ *rabbitmq.PopularityMQ) *LikeService {
	return &LikeService{repo: repo, VideoRepo: videoRepo, cache: cache, likeMQ: likeMQ, popularityMQ: popularityMQ}
}

func isDupKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func (s *LikeService) Like(ctx context.Context, like *Like) error {
	if like == nil {
		log.Printf("ERROR video_like invalid_request: reason=like_is_nil")
		return errors.New("like is nil")
	}
	if like.VideoID == 0 || like.AccountID == 0 {
		log.Printf("ERROR video_like invalid_request: account_id=%d video_id=%d reason=missing_required_fields", like.AccountID, like.VideoID)
		return errors.New("video_id and account_id are required")
	}

	log.Printf("INFO video_like request received: account_id=%d video_id=%d", like.AccountID, like.VideoID)

	ok, err := s.VideoRepo.IsExist(ctx, like.VideoID)
	if err != nil {
		log.Printf("ERROR video_like video_exist_check failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
		return err
	}
	if !ok {
		log.Printf("ERROR video_like video_not_found: account_id=%d video_id=%d", like.AccountID, like.VideoID)
		return errors.New("video not found")
	}

	isLiked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		log.Printf("ERROR video_like liked_check failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
		return err
	}
	if isLiked {
		log.Printf("ERROR video_like duplicate_like: account_id=%d video_id=%d", like.AccountID, like.VideoID)
		return errors.New("user has liked this video")
	}

	like.CreatedAt = time.Now()

	mysqlEnqueued := false
	redisEnqueued := false

	if s.likeMQ != nil {
		if err := s.likeMQ.Like(ctx, like.AccountID, like.VideoID); err == nil {
			log.Printf("INFO video_like mq_enqueue success: account_id=%d video_id=%d queue=like.events", like.AccountID, like.VideoID)
			mysqlEnqueued = true
		} else {
			log.Printf("ERROR video_like mq_enqueue failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
		}
	}

	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, like.VideoID, 1); err == nil {
			log.Printf("INFO video_like popularity_mq_enqueue success: account_id=%d video_id=%d change=%d", like.AccountID, like.VideoID, 1)
			redisEnqueued = true
		} else {
			log.Printf("ERROR video_like popularity_mq_enqueue failed: account_id=%d video_id=%d change=%d err=%v", like.AccountID, like.VideoID, 1, err)
		}
	}

	// like MQ 成功时，不再同步写 like 表，避免重复点赞
	if mysqlEnqueued {
		if !redisEnqueued {
			log.Printf("INFO video_like popularity_cache_fallback: account_id=%d video_id=%d change=%d", like.AccountID, like.VideoID, 1)
			UpdatePopularityCache(ctx, s.cache, like.VideoID, 1)
		}
		log.Printf("INFO video_like request finished_async: account_id=%d video_id=%d", like.AccountID, like.VideoID)
		return nil
	}

	log.Printf("INFO video_like fallback_to_db: account_id=%d video_id=%d reason=mq_unavailable_or_publish_failed", like.AccountID, like.VideoID)

	// fallback: like MQ 失败时，同步写 MySQL 和数据库热度
	err = s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("id").First(&Video{}, like.VideoID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("ERROR video_like db_video_not_found_during_fallback: account_id=%d video_id=%d", like.AccountID, like.VideoID)
				return errors.New("video not found")
			}
			log.Printf("ERROR video_like db_video_check_failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
			return err
		}

		if err := tx.Create(like).Error; err != nil {
			if isDupKey(err) {
				log.Printf("ERROR video_like duplicate_like_during_fallback: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
				return errors.New("user has liked this video")
			}
			log.Printf("ERROR video_like db_insert_like_failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
			return err
		}

		if err := tx.Model(&Video{}).
			Where("id = ?", like.VideoID).
			UpdateColumn("likes_count", gorm.Expr("likes_count + 1")).Error; err != nil {
			log.Printf("ERROR video_like db_update_likes_count_failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
			return err
		}

		if err := tx.Model(&Video{}).
			Where("id = ?", like.VideoID).
			UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error; err != nil {
			log.Printf("ERROR video_like db_update_popularity_failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
			return err
		}

		return nil
	})
	if err != nil {
		log.Printf("ERROR video_like db_fallback_failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
		return err
	}

	// fallback: popularity MQ 失败时，补 Redis 热度
	if !redisEnqueued {
		log.Printf("INFO video_like popularity_cache_fallback: account_id=%d video_id=%d change=%d", like.AccountID, like.VideoID, 1)
		UpdatePopularityCache(ctx, s.cache, like.VideoID, 1)
	}

	log.Printf("INFO video_like request finished_sync_fallback: account_id=%d video_id=%d", like.AccountID, like.VideoID)
	return nil
}

func (s *LikeService) Unlike(ctx context.Context, like *Like) error {
	if like == nil {
		log.Printf("ERROR video_unlike invalid_request: reason=like_is_nil")
		return errors.New("like is nil")
	}
	if like.VideoID == 0 || like.AccountID == 0 {
		log.Printf("ERROR video_unlike invalid_request: account_id=%d video_id=%d reason=missing_required_fields", like.AccountID, like.VideoID)
		return errors.New("video_id and account_id are required")
	}

	log.Printf("INFO video_unlike request received: account_id=%d video_id=%d", like.AccountID, like.VideoID)

	ok, err := s.VideoRepo.IsExist(ctx, like.VideoID)
	if err != nil {
		log.Printf("ERROR video_unlike video_exist_check failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
		return err
	}
	if !ok {
		log.Printf("ERROR video_unlike video_not_found: account_id=%d video_id=%d", like.AccountID, like.VideoID)
		return errors.New("video not found")
	}

	isLiked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		log.Printf("ERROR video_unlike liked_check failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
		return err
	}
	if !isLiked {
		log.Printf("ERROR video_unlike unlike_without_like: account_id=%d video_id=%d", like.AccountID, like.VideoID)
		return errors.New("user has not liked this video")
	}

	mysqlEnqueued := false
	redisEnqueued := false

	if s.likeMQ != nil {
		if err := s.likeMQ.Unlike(ctx, like.AccountID, like.VideoID); err == nil {
			log.Printf("INFO video_unlike mq_enqueue success: account_id=%d video_id=%d queue=like.events", like.AccountID, like.VideoID)
			mysqlEnqueued = true
		} else {
			log.Printf("ERROR video_unlike mq_enqueue failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
		}
	}

	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, like.VideoID, -1); err == nil {
			log.Printf("INFO video_unlike popularity_mq_enqueue success: account_id=%d video_id=%d change=%d", like.AccountID, like.VideoID, -1)
			redisEnqueued = true
		} else {
			log.Printf("ERROR video_unlike popularity_mq_enqueue failed: account_id=%d video_id=%d change=%d err=%v", like.AccountID, like.VideoID, -1, err)
		}
	}

	// unlike MQ 成功时，不再同步删 like 表，避免重复扣减
	if mysqlEnqueued {
		if !redisEnqueued {
			log.Printf("INFO video_unlike popularity_cache_fallback: account_id=%d video_id=%d change=%d", like.AccountID, like.VideoID, -1)
			UpdatePopularityCache(ctx, s.cache, like.VideoID, -1)
		}
		log.Printf("INFO video_unlike request finished_async: account_id=%d video_id=%d", like.AccountID, like.VideoID)
		return nil
	}

	log.Printf("INFO video_unlike fallback_to_db: account_id=%d video_id=%d reason=mq_unavailable_or_publish_failed", like.AccountID, like.VideoID)

	// fallback: like MQ 失败时，同步写 MySQL 和数据库热度
	err = s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		del := tx.Where("video_id = ? AND account_id = ?", like.VideoID, like.AccountID).Delete(&Like{})
		if del.Error != nil {
			log.Printf("ERROR video_unlike db_delete_like_failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, del.Error)
			return del.Error
		}
		if del.RowsAffected == 0 {
			log.Printf("ERROR video_unlike unlike_without_like_during_fallback: account_id=%d video_id=%d", like.AccountID, like.VideoID)
			return errors.New("user has not liked this video")
		}

		if err := tx.Model(&Video{}).
			Where("id = ?", like.VideoID).
			UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count - 1, 0)")).Error; err != nil {
			log.Printf("ERROR video_unlike db_update_likes_count_failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
			return err
		}

		if err := tx.Model(&Video{}).
			Where("id = ?", like.VideoID).
			UpdateColumn("popularity", gorm.Expr("GREATEST(popularity - 1, 0)")).Error; err != nil {
			log.Printf("ERROR video_unlike db_update_popularity_failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
			return err
		}

		return nil
	})
	if err != nil {
		log.Printf("ERROR video_unlike db_fallback_failed: account_id=%d video_id=%d err=%v", like.AccountID, like.VideoID, err)
		return err
	}

	// fallback: popularity MQ 失败时，补 Redis 热度
	if !redisEnqueued {
		log.Printf("INFO video_unlike popularity_cache_fallback: account_id=%d video_id=%d change=%d", like.AccountID, like.VideoID, -1)
		UpdatePopularityCache(ctx, s.cache, like.VideoID, -1)
	}

	log.Printf("INFO video_unlike request finished_sync_fallback: account_id=%d video_id=%d", like.AccountID, like.VideoID)
	return nil
}

func (s *LikeService) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	return s.repo.IsLiked(ctx, videoID, accountID)
}

func (s *LikeService) ListLikedVideos(ctx context.Context, accountID uint) ([]Video, error) {
	return s.repo.ListLikedVideos(ctx, accountID)
}
