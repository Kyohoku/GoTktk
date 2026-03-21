package main

import (
	"context"
	"log"
	"strconv"
	"time"

	"gotik/internal/config"
	"gotik/internal/db"
	apphttp "gotik/internal/http"
	rabbitmq "gotik/internal/middleware/rabbitmq"
	rediscache "gotik/internal/middleware/redis"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	//数据库连接
	sqlDB, err := db.NewDB(cfg.Database)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer func() {
		if err := db.CloseDB(sqlDB); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	if err := db.AutoMigrate(sqlDB); err != nil {
		log.Fatalf("failed to auto migrate database: %v", err)
	}

	// 连接 Redis
	cache, err := rediscache.NewFromEnv(&cfg.Redis)
	if err != nil {
		log.Printf("Redis config error (cache disabled): %v", err)
		cache = nil
	} else {
		pingCtx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		if err := cache.Ping(pingCtx); err != nil {
			log.Printf("Redis not available (cache disabled): %v", err)
			_ = cache.Close()
			cache = nil
		} else {
			defer cache.Close()
			log.Printf("Redis connected (cache enabled)")
		}
	}

	// 连接 RabbitMQ (可选，用于消息队列)
	rmq, err := rabbitmq.NewRabbitMQ(&cfg.RabbitMQ)
	if err != nil {
		log.Printf("RabbitMQ config error (disabled): %v", err)
		rmq = nil
	} else {
		defer rmq.Close()
		log.Printf("RabbitMQ connected")
	}

	//设置路由
	r := apphttp.SetRouter(sqlDB, cache, rmq)
	log.Printf("server is running on port %d", cfg.Server.Port)
	if err := r.Run(":" + strconv.Itoa(cfg.Server.Port)); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
