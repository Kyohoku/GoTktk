package ai

import (
	"context"

	rediscache "gotik/internal/middleware/redis"
)

type PythonClient interface {
	GenerateSummary(ctx context.Context, req *PythonSummaryRequest) (*SummaryResult, error)
	GenerateCommentSuggestions(ctx context.Context, req *PythonCommentSuggestionsRequest) (*CommentSuggestionsResult, error)
	AnswerQuestion(ctx context.Context, req *PythonQARequest) (*QAResult, error)
}

type Service struct {
	repo   *Repository
	cache  *rediscache.Client
	client PythonClient
}

func NewService(repo *Repository, cache *rediscache.Client, client PythonClient) *Service {
	return &Service{
		repo:   repo,
		cache:  cache,
		client: client,
	}
}
