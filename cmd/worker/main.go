package main

import (
	"context"
	"gotik/internal/config"
	"gotik/internal/db"
	rediscache "gotik/internal/middleware/redis"
	"gotik/internal/video"
	"gotik/internal/worker"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	likeExchange   = "like.events"
	likeQueue      = "like.events"
	likeBindingKey = "like.*"
)

func main() {
	// 加载配置
	log.Printf("Loading config from configs/config.yaml")
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	// 连接数据库
	sqlDB, err := db.NewDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer db.CloseDB(sqlDB)

	// 连接 Redis（用于流行度更新）
	cache, err := rediscache.NewFromEnv(&cfg.Redis)
	if err != nil {
		log.Printf("Redis config error (popularity worker disabled): %v", err)
		cache = nil
	} else {
		pingCtx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		if err := cache.Ping(pingCtx); err != nil {
			log.Printf("Redis not available (popularity worker disabled): %v", err)
			_ = cache.Close()
			cache = nil
		} else {
			defer cache.Close()
			log.Printf("Redis connected (popularity worker enabled)")
		}
	}
	// 连接 RabbitMQ
	url := "amqp://" + cfg.RabbitMQ.Username + ":" + cfg.RabbitMQ.Password + "@" + cfg.RabbitMQ.Host + ":" + strconv.Itoa(cfg.RabbitMQ.Port) + "/"
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("Failed to connect rabbitmq: %v", err)
	}
	defer conn.Close()
	// 创建 RabbitMQ 通道
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open rabbitmq channel: %v", err)
	}
	defer ch.Close()

	// 声明 Like 交换机和队列
	if err := declareLikeTopology(ch); err != nil {
		log.Fatalf("Failed to declare like topology: %v", err)
	}

	if err := ch.Qos(50, 0, false); err != nil {
		log.Fatalf("Failed to set qos: %v", err)
	}

	videoRepo := video.NewVideoRepository(sqlDB)
	likeRepo := video.NewLikeRepository(sqlDB)
	likeWorker := worker.NewLikeWorker(ch, likeRepo, videoRepo, likeQueue)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 4)

	log.Printf("Worker started, consuming queue=%s", likeQueue)
	go func() { errCh <- likeWorker.Run(ctx) }()

	err = <-errCh
	if err != nil && err != context.Canceled {
		log.Fatalf("Worker stopped: %v", err)
	}
	log.Printf("Worker stopped")

}

func declareLikeTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(
		likeExchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	q, err := ch.QueueDeclare(
		likeQueue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	return ch.QueueBind(
		q.Name,
		likeBindingKey,
		likeExchange,
		false,
		nil,
	)
}
