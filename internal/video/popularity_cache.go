package video

import (
	"context"
	"fmt"
	rediscache "gotik/internal/middleware/redis"
	"strconv"
	"time"
)

func UpdatePopularityCache(ctx context.Context, cache *rediscache.Client, id uint, change int64) {
	if cache == nil || id == 0 || change == 0 {
		return
	}

	_ = cache.Del(context.Background(), fmt.Sprintf("video:detail:id=%d", id))

	now := time.Now().UTC().Truncate(time.Minute)
	windowKey := "hot:video:1m:" + now.Format("200601021504")
	member := strconv.FormatUint(uint64(id), 10)

	_ = cache.ZincrBy(ctx, windowKey, member, float64(change))
	_ = cache.Expire(ctx, windowKey, 2*time.Hour)
}
