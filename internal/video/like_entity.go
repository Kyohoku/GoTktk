package video

import "time"

type Like struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	VideoID   uint      `gorm:"uniqueIndex:idx_like_video_account;not null" json:"video_id"` //联合索引，保证一个用户对一个视频不会重复点赞
	AccountID uint      `gorm:"uniqueIndex:idx_like_video_account;not null" json:"account_id"`
	CreatedAt time.Time `json:"created_at"`
}

type LikeRequest struct {
	VideoID uint `json:"video_id"`
}
