package video

import (
	"context"
	"errors"
	"gotik/internal/middleware/rabbitmq"
	rediscache "gotik/internal/middleware/redis"
	"log"
	"strings"

	"gorm.io/gorm"
)

type CommentService struct {
	repo            *CommentRepository
	VideoRepository *VideoRepository
	cache           *rediscache.Client
	commentMQ       *rabbitmq.CommentMQ
}

func NewCommentService(repo *CommentRepository, videoRepository *VideoRepository, cache *rediscache.Client, commentMQ *rabbitmq.CommentMQ) *CommentService {
	return &CommentService{repo: repo, VideoRepository: videoRepository, cache: cache, commentMQ: commentMQ}
}

func (s *CommentService) Publish(ctx context.Context, comment *Comment) error {
	if comment == nil {
		return errors.New("comment is nil")
	}

	comment.Username = strings.TrimSpace(comment.Username)
	comment.Content = strings.TrimSpace(comment.Content)

	if comment.VideoID == 0 || comment.AuthorID == 0 {
		return errors.New("video_id and author_id are required")
	}
	if comment.Content == "" {
		return errors.New("content is required")
	}

	exists, err := s.VideoRepository.IsExist(ctx, comment.VideoID)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("video not found")
	}

	mysqlEnqueued := false
	if s.commentMQ != nil {
		if err := s.commentMQ.Publish(ctx, comment.Username, comment.VideoID, comment.AuthorID, comment.Content); err == nil {
			log.Printf("comment request enqueued to rabbitmq: user_name=%v video_id=%d", comment.Username, comment.VideoID)
			mysqlEnqueued = true
		}
	}
	if mysqlEnqueued {
		return nil
	}

	// fallback: MQ 不可用时直接写数据库  直接写sql语句，确保评论和热度同步更新
	err = s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("id").First(&Video{}, comment.VideoID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("video not found")
			}
			return err
		}

		if err := tx.Create(comment).Error; err != nil {
			return err
		}

		return tx.Model(&Video{}).
			Where("id = ?", comment.VideoID).
			UpdateColumn("popularity", gorm.Expr("popularity + 2")).Error
	})
	if err != nil {
		return err
	}

	UpdatePopularityCache(ctx, s.cache, comment.VideoID, 2)
	return nil
}

func (s *CommentService) Delete(ctx context.Context, commentID uint, accountID uint) error {
	comment, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment == nil {
		return errors.New("comment not found")
	}
	if comment.AuthorID != accountID {
		return errors.New("permission denied")
	}

	if s.commentMQ != nil { //异步化
		if err := s.commentMQ.Delete(ctx, commentID); err == nil {
			return nil
		}
	}
	//直接写数据库兜底
	return s.repo.DeleteComment(ctx, comment)
}

func (s *CommentService) GetAll(ctx context.Context, videoID uint) ([]Comment, error) {
	exists, err := s.VideoRepository.IsExist(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("video not found")
	}
	return s.repo.GetAllComments(ctx, videoID)
}
