package video

import (
	"context"
	"fmt"
	rediscache "gotik/internal/middleware/redis"
	"strconv"
)

func UpdatePopularityCache(ctx context.Context, cache *rediscache.Client, id uint, change int64) {
	if cache == nil || id == 0 || change == 0 {
		return
	}

	_ = cache.Del(context.Background(), fmt.Sprintf("video:detail:id=%d", id))

	member := strconv.FormatUint(uint64(id), 10)
	_ = cache.ZincrBy(ctx, "hot:video", member, float64(change))
}
