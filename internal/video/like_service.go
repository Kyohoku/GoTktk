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
	repo      *LikeRepository
	VideoRepo *VideoRepository
	cache     *rediscache.Client
	likeMQ    *rabbitmq.LikeMQ
}

func NewLikeService(repo *LikeRepository, videoRepo *VideoRepository, cache *rediscache.Client, likeMQ *rabbitmq.LikeMQ) *LikeService {
	return &LikeService{repo: repo, VideoRepo: videoRepo, cache: cache, likeMQ: likeMQ}
}

func isDupKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
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
	mysqlEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Like(ctx, like.AccountID, like.VideoID); err == nil { //成功异步执行
			log.Printf("like request enqueued to rabbitmq: user_id=%d video_id=%d", like.AccountID, like.VideoID)
			mysqlEnqueued = true
		}
	}
	if mysqlEnqueued {
		return nil
	}

	// 回退，mq挂了的话直接写数据库
	if !mysqlEnqueued {
		log.Printf("like mq publish failed, fallback to db")
		err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Select("id").First(&Video{}, like.VideoID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("video not found")
				}
				return err
			}
			if err := tx.Create(like).Error; err != nil {
				if isDupKey(err) {
					return errors.New("user has liked this video")
				}
				return err
			}
			if err := tx.Model(&Video{}).Where("id = ?", like.VideoID).
				UpdateColumn("likes_count", gorm.Expr("likes_count + 1")).Error; err != nil {
				return err
			}
			return tx.Model(&Video{}).Where("id = ?", like.VideoID).
				UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error
		})
		if err != nil {
			return err
		}
	}

	// 直接写缓存的热度

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

	mysqlEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Unlike(ctx, like.AccountID, like.VideoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if mysqlEnqueued {
		return nil
	}

	// fallback: MQ 不可用时直接写数据库
	err = s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		del := tx.Where("video_id = ? AND account_id = ?", like.VideoID, like.AccountID).Delete(&Like{})
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected == 0 {
			return errors.New("user has not liked this video")
		}

		if err := tx.Model(&Video{}).Where("id = ?", like.VideoID).
			UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count - 1, 0)")).Error; err != nil {
			return err
		}

		return tx.Model(&Video{}).Where("id = ?", like.VideoID).
			UpdateColumn("popularity", gorm.Expr("GREATEST(popularity - 1, 0)")).Error
	})
	if err != nil {
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
