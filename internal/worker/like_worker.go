package worker

import (
	"context"
	"encoding/json"
	"errors"
	"gotik/internal/middleware/rabbitmq"
	"gotik/internal/video"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// 从 like 队列中消费消息，解析为 LikeEvent，并执行数据库更新

type LikeWorker struct {
	ch     *amqp.Channel
	likes  *video.LikeRepository
	videos *video.VideoRepository
	queue  string // 消费的队列名
}

func NewLikeWorker(ch *amqp.Channel, likes *video.LikeRepository, videos *video.VideoRepository, queue string) *LikeWorker {
	return &LikeWorker{ch: ch, likes: likes, videos: videos, queue: queue}
}

func (w *LikeWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.likes == nil || w.videos == nil {
		log.Printf("ERROR like_worker init_check failed: reason=worker_not_initialized")
		return errors.New("like worker is not initialized")
	}
	if w.queue == "" {
		log.Printf("ERROR like_worker init_check failed: reason=queue_required")
		return errors.New("queue is required")
	}

	deliveries, err := w.ch.Consume(
		w.queue,
		"",
		false, // 关闭自动确认，避免业务未处理成功消息就被删除
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("ERROR like_worker consume init failed: queue=%s err=%v", w.queue, err)
		return err
	}

	log.Printf("INFO like_worker consume started: queue=%s", w.queue)

	for { // 循环读取消息
		select {
		case <-ctx.Done(): // 收到退出信号
			log.Printf("INFO like_worker context canceled: queue=%s err=%v", w.queue, ctx.Err())
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				log.Printf("ERROR like_worker deliveries channel closed: queue=%s", w.queue)
				return errors.New("deliveries channel closed")
			}
			w.handleDelivery(ctx, d)
		}
	}
}

func (w *LikeWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	log.Printf("INFO like_worker delivery received: queue=%s routing_key=%s body_size=%d", w.queue, d.RoutingKey, len(d.Body))

	if err := w.process(ctx, d.Body); err != nil {
		log.Printf("ERROR like_worker process failed: queue=%s routing_key=%s requeue=%t err=%v", w.queue, d.RoutingKey, true, err)

		// 处理失败重新回队列
		if err := d.Nack(false, true); err != nil {
			log.Printf("ERROR like_worker nack failed: queue=%s routing_key=%s err=%v", w.queue, d.RoutingKey, err)
		}
		return
	}

	// 处理成功返回 ACK
	if err := d.Ack(false); err != nil {
		log.Printf("ERROR like_worker ack failed: queue=%s routing_key=%s err=%v", w.queue, d.RoutingKey, err)
		return
	}

	log.Printf("INFO like_worker delivery acked: queue=%s routing_key=%s", w.queue, d.RoutingKey)
}

// 解析事件
func (w *LikeWorker) process(ctx context.Context, body []byte) error {
	var evt rabbitmq.LikeEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		// 解析事件失败，直接丢弃
		log.Printf("ERROR like_worker event_unmarshal failed: queue=%s err=%v body=%s", w.queue, err, string(body))
		return nil
	}
	if evt.UserID == 0 || evt.VideoID == 0 {
		log.Printf("ERROR like_worker invalid_event: queue=%s action=%s user_id=%d video_id=%d", w.queue, evt.Action, evt.UserID, evt.VideoID)
		return nil
	}

	log.Printf("INFO like_worker process start: queue=%s action=%s user_id=%d video_id=%d", w.queue, evt.Action, evt.UserID, evt.VideoID)

	switch evt.Action {
	case "like":
		return w.applyLike(ctx, evt.UserID, evt.VideoID)
	case "unlike":
		return w.applyUnlike(ctx, evt.UserID, evt.VideoID)
	default:
		log.Printf("ERROR like_worker unknown_action: queue=%s action=%s user_id=%d video_id=%d", w.queue, evt.Action, evt.UserID, evt.VideoID)
		return nil
	}
}

func (w *LikeWorker) applyLike(ctx context.Context, userID, videoID uint) error {
	ok, err := w.videos.IsExist(ctx, videoID)
	if err != nil {
		log.Printf("ERROR like_worker apply_like video_exist_check_failed: user_id=%d video_id=%d err=%v", userID, videoID, err)
		return err
	}
	if !ok {
		log.Printf("ERROR like_worker apply_like video_not_found: user_id=%d video_id=%d", userID, videoID)
		return nil
	}

	created, err := w.likes.LikeIgnoreDuplicate(ctx, &video.Like{
		VideoID:   videoID,
		AccountID: userID,
		CreatedAt: time.Now(),
	})
	if err != nil {
		log.Printf("ERROR like_worker apply_like insert_like_failed: user_id=%d video_id=%d err=%v", userID, videoID, err)
		return err
	}
	if !created {
		log.Printf("INFO like_worker apply_like duplicate_ignored: user_id=%d video_id=%d", userID, videoID)
		return nil
	}

	if err := w.videos.ChangeLikesCount(ctx, videoID, 1); err != nil {
		log.Printf("ERROR like_worker apply_like change_likes_count_failed: user_id=%d video_id=%d err=%v", userID, videoID, err)
		return err
	}
	if err := w.videos.UpdatePopularity(ctx, videoID, 1); err != nil {
		log.Printf("ERROR like_worker apply_like update_popularity_failed: user_id=%d video_id=%d err=%v", userID, videoID, err)
		return err
	}

	log.Printf("INFO like_worker apply_like success: user_id=%d video_id=%d", userID, videoID)
	return nil
}

func (w *LikeWorker) applyUnlike(ctx context.Context, userID, videoID uint) error {
	ok, err := w.videos.IsExist(ctx, videoID)
	if err != nil {
		log.Printf("ERROR like_worker apply_unlike video_exist_check_failed: user_id=%d video_id=%d err=%v", userID, videoID, err)
		return err
	}
	if !ok {
		log.Printf("ERROR like_worker apply_unlike video_not_found: user_id=%d video_id=%d", userID, videoID)
		return nil
	}

	deleted, err := w.likes.DeleteByVideoAndAccount(ctx, videoID, userID)
	if err != nil {
		log.Printf("ERROR like_worker apply_unlike delete_like_failed: user_id=%d video_id=%d err=%v", userID, videoID, err)
		return err
	}
	if !deleted {
		log.Printf("INFO like_worker apply_unlike no_like_deleted: user_id=%d video_id=%d", userID, videoID)
		return nil
	}

	if err := w.videos.ChangeLikesCount(ctx, videoID, -1); err != nil {
		log.Printf("ERROR like_worker apply_unlike change_likes_count_failed: user_id=%d video_id=%d err=%v", userID, videoID, err)
		return err
	}
	if err := w.videos.UpdatePopularity(ctx, videoID, -1); err != nil {
		log.Printf("ERROR like_worker apply_unlike update_popularity_failed: user_id=%d video_id=%d err=%v", userID, videoID, err)
		return err
	}

	log.Printf("INFO like_worker apply_unlike success: user_id=%d video_id=%d", userID, videoID)
	return nil
}
