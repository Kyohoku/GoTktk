package feed

import (
	"context"
	"encoding/json"
	"fmt"
	rediscache "gotik/internal/middleware/redis"
	"gotik/internal/video"
	"log"
	"time"
)

type FeedService struct {
	repo     *FeedRepository
	likeRepo *video.LikeRepository
	cache    *rediscache.Client
	cacheTTL time.Duration
}

func NewFeedService(repo *FeedRepository, likeRepo *video.LikeRepository, cache *rediscache.Client) *FeedService {
	return &FeedService{repo: repo, likeRepo: likeRepo, cache: cache, cacheTTL: 5 * time.Second}
}

func (f *FeedService) ListLatest(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (ListLatestResponse, error) {
	doListLatestFromDB := func() (ListLatestResponse, error) {
		videos, err := f.repo.ListLatest(ctx, limit, latestBefore)
		if err != nil {
			return ListLatestResponse{}, err
		}

		var nextTime int64
		if len(videos) > 0 {
			nextTime = videos[len(videos)-1].CreateTime.Unix()
		}

		hasMore := len(videos) == limit

		feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
		if err != nil {
			return ListLatestResponse{}, err
		}

		return ListLatestResponse{
			VideoList: feedVideos,
			NextTime:  nextTime,
			HasMore:   hasMore,
		}, nil
	}

	var cacheKey string
	if viewerAccountID == 0 && f.cache != nil {
		before := int64(0)
		if !latestBefore.IsZero() {
			before = latestBefore.Unix()
		}
		cacheKey = fmt.Sprintf("feed:listLatest:limit=%d:before=%d", limit, before) //key 中包含分页

		b, err := f.cache.GetBytes(ctx, cacheKey)
		if err == nil { //缓存命中
			log.Printf("redis hit feed:listLatest:limit=%d:before=%d", limit, before)
			var cached ListLatestResponse
			if err := json.Unmarshal(b, &cached); err == nil {
				return cached, nil
			}
		}
	}

	//查数据库兜底
	resp, err := doListLatestFromDB()
	if err != nil {
		return ListLatestResponse{}, err
	}

	//key 不为空，写入缓存
	if cacheKey != "" {
		if b, err := json.Marshal(resp); err == nil {
			_ = f.cache.SetBytes(ctx, cacheKey, b, f.cacheTTL)
		}
	}

	return resp, nil
}

// 按照点赞数查询视频
func (f *FeedService) ListLikesCount(ctx context.Context, limit int, cursor *LikesCountCursor, viewerAccountID uint) (ListLikesCountResponse, error) {
	videos, err := f.repo.ListLikesCountWithCursor(ctx, limit, cursor)
	if err != nil {
		return ListLikesCountResponse{}, err
	}
	hasMore := len(videos) == limit
	feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListLikesCountResponse{}, err
	}
	resp := ListLikesCountResponse{
		VideoList: feedVideos,
		HasMore:   hasMore,
	}
	if len(videos) > 0 {
		last := videos[len(videos)-1]
		nextLikesCountBefore := last.LikesCount
		nextIDBefore := last.ID
		resp.NextLikesCountBefore = &nextLikesCountBefore
		resp.NextIDBefore = &nextIDBefore
	}
	return resp, nil
}

func (f *FeedService) buildFeedVideos(ctx context.Context, videos []*video.Video, viewerAccountID uint) ([]FeedVideoItem, error) {
	feedVideos := make([]FeedVideoItem, 0, len(videos))
	videoIDs := make([]uint, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	likedMap, err := f.likeRepo.BatchGetLiked(ctx, videoIDs, viewerAccountID) //调用查看推流的视频是否有被当前用户点赞过
	if err != nil {
		return nil, err
	}
	for _, video := range videos {
		feedVideos = append(feedVideos, FeedVideoItem{
			ID:          video.ID,
			Author:      FeedAuthor{ID: video.AuthorID, Username: video.Username},
			Title:       video.Title,
			Description: video.Description,
			PlayURL:     video.PlayURL,
			CoverURL:    video.CoverURL,
			CreateTime:  video.CreateTime.Unix(),
			LikesCount:  video.LikesCount,
			IsLiked:     likedMap[video.ID],
		})
	}
	return feedVideos, nil
}
