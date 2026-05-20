package main

import (
	"context"
	"log"
	"strconv"
	"time"

	"video-feed/internal/account"
	"video-feed/internal/config"
	"video-feed/internal/db"
	apphttp "video-feed/internal/http"
	"video-feed/internal/middleware/rabbitmq"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/social"
	"video-feed/internal/video"
)

func main() {
	cfg := config.Default()
	database, err := db.Open(cfg.Database)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(database, &account.Account{}, &video.Video{}, &video.Like{}, &video.Comment{}, &video.Tag{}, &video.VideoTag{}, &social.Social{}); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	cache := rediscache.NewFromConfig(cfg.Redis)
	pingCtx, cancel := contextWithTimeout()
	if err := cache.Ping(pingCtx); err != nil {
		log.Printf("redis unavailable, cache disabled: %v", err)
		_ = cache.Close()
		cache = nil
	}
	cancel()
	if cache != nil {
		defer cache.Close()
	}

	var publishers *rabbitmq.Publishers
	broker, err := rabbitmq.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Printf("rabbitmq unavailable, async workers disabled: %v", err)
	} else {
		defer broker.Close()
		publishers, err = rabbitmq.NewPublishers(broker)
		if err != nil {
			log.Printf("rabbitmq publisher init failed, async workers disabled: %v", err)
			publishers = nil
		}
	}

	router := apphttp.NewRouterWithPublishers(database, cache, publishers)
	if err := router.Run(":" + strconv.Itoa(cfg.Server.Port)); err != nil {
		log.Fatalf("run server: %v", err)
	}
}

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 300*time.Millisecond)
}
