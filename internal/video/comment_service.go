package video

import (
	"context"
	"errors"
	"strings"
)

type CommentService struct {
	repo            *CommentRepository
	VideoRepository *VideoRepository
}

func NewCommentService(repo *CommentRepository, videoRepository *VideoRepository) *CommentService {
	return &CommentService{repo: repo, VideoRepository: videoRepository}
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

	if err := s.repo.CreateComment(ctx, comment); err != nil {
		return err
	}

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
