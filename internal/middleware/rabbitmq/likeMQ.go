package rabbitmq

import (
	"context"
	"errors"
	"time"
)

// 把点赞业务转换成 MQ 消息发出去
type LikeMQ struct {
	*RabbitMQ
}

const (
	likeExchange   = "like.events" // 点赞业务的交换机名
	likeQueue      = "like.events" // 点赞业务的队列名
	likeBindingKey = "like.*"      // 队列接收哪些路由键

	likeLikeRK   = "like.like"   // 点赞事件的路由键
	likeUnlikeRK = "like.unlike" // 取消点赞事件的路由键
)

// 消息体，发送到mq里的JSON结构
type LikeEvent struct {
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	UserID     uint      `json:"user_id"`
	VideoID    uint      `json:"video_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewLikeMQ(base *RabbitMQ) (*LikeMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}

	//声明话题为 点赞
	if err := base.DeclareTopic(likeExchange, likeQueue, likeBindingKey); err != nil {
		return nil, err
	}
	return &LikeMQ{RabbitMQ: base}, nil
}

// 对外开放接口
func (l *LikeMQ) Like(ctx context.Context, userID, videoID uint) error {
	return l.publish(ctx, "like", likeLikeRK, userID, videoID)
}

func (l *LikeMQ) Unlike(ctx context.Context, userID, videoID uint) error {
	return l.publish(ctx, "unlike", likeUnlikeRK, userID, videoID)
}

// 发消息
func (l *LikeMQ) publish(ctx context.Context, action, routingKey string, userID, videoID uint) error {
	if l == nil || l.RabbitMQ == nil {
		return errors.New("like mq is not initialized")
	}
	if userID == 0 || videoID == 0 {
		return errors.New("userID and videoID are required")
	}
	id, err := newEventID(16)
	if err != nil {
		return err
	}
	event := LikeEvent{
		EventID:    id,
		Action:     action,
		UserID:     userID,
		VideoID:    videoID,
		OccurredAt: time.Now(),
	}
	return l.PublishJSON(ctx, likeExchange, routingKey, event)
}
