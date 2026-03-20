package feed

import (
	"context"
	"encoding/json"
	"fmt"
	rediscache "gotik/internal/middleware/redis"
	"gotik/internal/video"
	"log"
	"strconv"
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
	if viewerAccountID == 0 && f.cache != nil { //匿名流
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
		} else if rediscache.IsMiss(err) { //未命中
			lockKey := "lock:" + cacheKey //锁的 key
			log.Printf("feed latest cache miss: key=%s", cacheKey)

			lockCtx, lockCancel := context.WithTimeout(ctx, 50*time.Millisecond)
			token, locked, lockErr := f.cache.Lock(lockCtx, lockKey, 1*time.Second)
			lockCancel()
			if lockErr == nil && locked { // get lock
				log.Printf("feed latest lock acquired: lockKey=%s", lockKey)

				defer func() { _ = f.cache.Unlock(context.Background(), lockKey, token) }()

				if b, err := f.cache.GetBytes(ctx, cacheKey); err == nil { //check cache again
					var cached ListLatestResponse
					if err := json.Unmarshal(b, &cached); err == nil {
						return cached, nil
					}
				}

				//database
				resp, err := doListLatestFromDB()
				if err != nil {
					return ListLatestResponse{}, err
				}
				if b, err := json.Marshal(resp); err == nil {
					_ = f.cache.SetBytes(ctx, cacheKey, b, f.cacheTTL)
				}
				return resp, nil

			}

			// no lock
			for i := 0; i < 5; i++ {
				select {
				case <-ctx.Done():
					return ListLatestResponse{}, ctx.Err()
				case <-time.After(20 * time.Millisecond):
				}

				if b, err := f.cache.GetBytes(ctx, cacheKey); err == nil {
					var cached ListLatestResponse
					if err := json.Unmarshal(b, &cached); err == nil {
						return cached, nil
					}
				}
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

// 按热度推送
func (f *FeedService) ListByPopularity(
	ctx context.Context,
	limit int,
	reqAsOf int64,
	offset int,
	viewerAccountID uint,
	latestPopularity int64,
	latestBefore time.Time,
	latestIDBefore uint,
) (ListByPopularityResponse, error) {
	if f.cache != nil {
		asOf := time.Now().UTC().Truncate(time.Minute)
		if reqAsOf > 0 {
			asOf = time.Unix(reqAsOf, 0).UTC().Truncate(time.Minute)
		}

		const win = 60
		keys := make([]string, 0, win)
		for i := 0; i < win; i++ {
			keys = append(keys, "hot:video:1m:"+asOf.Add(-time.Duration(i)*time.Minute).Format("200601021504"))
		}

		dest := "hot:video:merge:1m:" + asOf.Format("200601021504")

		exists, err := f.cache.Exists(ctx, dest)
		if err == nil && !exists {
			_ = f.cache.ZUnionStore(ctx, dest, keys, "SUM")
			_ = f.cache.Expire(ctx, dest, 2*time.Minute)
		}

		start := int64(offset)
		stop := start + int64(limit) - 1

		members, err := f.cache.ZRevRange(ctx, dest, start, stop)
		if err == nil {
			log.Printf("hot ranking page: dest=%s offset=%d limit=%d members=%v", dest, offset, limit, members)
			if len(members) == 0 {
				return ListByPopularityResponse{
					VideoList:  []FeedVideoItem{},
					AsOf:       asOf.Unix(),
					NextOffset: offset,
					HasMore:    false,
				}, nil
			}

			ids := make([]uint, 0, len(members))
			for _, m := range members {
				u, err := strconv.ParseUint(m, 10, 64)
				if err == nil && u > 0 {
					ids = append(ids, uint(u))
				}
			}

			videos, err := f.repo.GetByIDs(ctx, ids)
			if err == nil {
				byID := make(map[uint]*video.Video, len(videos))
				for _, v := range videos {
					byID[v.ID] = v
				}

				ordered := make([]*video.Video, 0, len(ids))
				for _, id := range ids {
					if v := byID[id]; v != nil {
						ordered = append(ordered, v)
					}
				}

				items, err := f.buildFeedVideos(ctx, ordered, viewerAccountID)
				if err != nil {
					return ListByPopularityResponse{}, err
				}

				resp := ListByPopularityResponse{
					VideoList:  items,
					AsOf:       asOf.Unix(),
					NextOffset: offset + len(items),
					HasMore:    len(items) == limit,
				}

				if len(ordered) > 0 {
					last := ordered[len(ordered)-1]
					nextPopularity := last.Popularity
					nextBefore := last.CreateTime
					nextID := last.ID
					resp.NextLatestPopularity = &nextPopularity
					resp.NextLatestBefore = &nextBefore
					resp.NextLatestIDBefore = &nextID
				}

				return resp, nil
			}
		}
	}

	//缓存不可用

	videos, err := f.repo.ListByPopularity(ctx, limit, latestPopularity, latestBefore, latestIDBefore)
	if err != nil {
		return ListByPopularityResponse{}, err
	}

	items, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListByPopularityResponse{}, err
	}

	resp := ListByPopularityResponse{
		VideoList:  items,
		AsOf:       0,
		NextOffset: 0,
		HasMore:    len(items) == limit,
	}

	if len(videos) > 0 {
		last := videos[len(videos)-1]
		nextPopularity := last.Popularity
		nextBefore := last.CreateTime
		nextID := last.ID
		resp.NextLatestPopularity = &nextPopularity
		resp.NextLatestBefore = &nextBefore
		resp.NextLatestIDBefore = &nextID
	}

	return resp, nil
}
