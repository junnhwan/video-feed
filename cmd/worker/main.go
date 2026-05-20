package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"video-feed/internal/account"
	"video-feed/internal/config"
	"video-feed/internal/db"
	"video-feed/internal/middleware/rabbitmq"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/observability"
	"video-feed/internal/social"
	"video-feed/internal/video"
	"video-feed/internal/worker"
)

func main() {
	cfg, err := config.Load(os.Getenv("CONFIG_PATH"))
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	database, err := db.Open(cfg.Database)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(database, &account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &video.Tag{}, &video.VideoTag{}, &social.Social{}, &worker.Notification{}); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	cache := rediscache.NewFromConfig(cfg.Redis)
	if err := cache.Ping(context.Background()); err != nil {
		log.Printf("redis unavailable, popularity worker disabled: %v", err)
		_ = cache.Close()
		cache = nil
	}
	if cache != nil {
		defer cache.Close()
	}

	broker, err := rabbitmq.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("connect rabbitmq: %v", err)
	}
	defer broker.Close()
	if _, err := rabbitmq.NewPublishers(broker); err != nil {
		log.Fatalf("declare rabbitmq topology: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pprofServer, err := observability.NewPprofServer("worker", cfg.Observability.Pprof.Enabled, cfg.Observability.Pprof.WorkerAddr)
	if err != nil {
		log.Printf("pprof unavailable: %v", err)
	}
	if pprofServer != nil {
		defer pprofServer.Close()
	}

	likeRepo := video.NewLikeRepository(database)
	videoRepo := video.NewRepository(database)
	commentRepo := video.NewCommentRepository(database)
	socialRepo := social.NewRepository(database)

	run(ctx, "like", worker.NewLikeWorker(broker.Ch, likeRepo, videoRepo, rabbitmq.LikeQueue).Run)
	run(ctx, "comment", worker.NewCommentWorker(broker.Ch, commentRepo, videoRepo, rabbitmq.CommentQueue).Run)
	run(ctx, "social", worker.NewSocialWorker(broker.Ch, socialRepo, rabbitmq.SocialQueue).Run)
	if cache != nil {
		run(ctx, "popularity", worker.NewPopularityWorker(broker.Ch, cache, rabbitmq.PopularityQueue).Run)
		run(ctx, "timeline", worker.NewTimelineWorker(broker.Ch, cache, rabbitmq.TimelineQueue).Run)
	}

	<-ctx.Done()
	log.Printf("worker shutting down: %v", ctx.Err())
}

func run(ctx context.Context, name string, fn func(context.Context) error) {
	go func() {
		if err := fn(ctx); err != nil && ctx.Err() == nil {
			log.Printf("%s worker stopped: %v", name, err)
		}
	}()
}
