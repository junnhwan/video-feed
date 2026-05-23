package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"video-feed/internal/account"
	"video-feed/internal/config"
	"video-feed/internal/db"
	"video-feed/internal/middleware/rabbitmq"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/observability"
	"video-feed/internal/social"
	"video-feed/internal/video"
	"video-feed/internal/worker"

	"go.uber.org/zap"
)

func main() {
	logger := observability.InitLogger("worker")
	defer observability.Sync()

	shutdownTracer, err := observability.InitTracer("worker")
	if err != nil {
		logger.Warn("init tracer", zap.Error(err))
	}
	if shutdownTracer != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = shutdownTracer(ctx)
		}()
	}

	cfg, err := config.Load(os.Getenv("CONFIG_PATH"))
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}
	database, err := db.Open(cfg.Database)
	if err != nil {
		logger.Fatal("open database", zap.Error(err))
	}
	if err := db.AutoMigrate(database, &account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &video.Tag{}, &video.VideoTag{}, &social.Social{}, &worker.Notification{}); err != nil {
		logger.Fatal("auto migrate", zap.Error(err))
	}

	cache := rediscache.NewFromConfig(cfg.Redis)
	if err := cache.Ping(context.Background()); err != nil {
		logger.Warn("redis unavailable, popularity worker disabled", zap.Error(err))
		_ = cache.Close()
		cache = nil
	}
	if cache != nil {
		defer cache.Close()
	}

	broker, err := rabbitmq.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		logger.Fatal("connect rabbitmq", zap.Error(err))
	}
	defer broker.Close()
	if _, err := rabbitmq.NewPublishers(broker); err != nil {
		logger.Fatal("declare rabbitmq topology", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pprofServer, err := observability.NewPprofServer("worker", cfg.Observability.Pprof.Enabled, cfg.Observability.Pprof.WorkerAddr)
	if err != nil {
		logger.Warn("pprof unavailable", zap.Error(err))
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

	logger.Info("worker started, waiting for events")
	<-ctx.Done()
	logger.Info("worker shutting down", zap.Error(ctx.Err()))
}

func run(ctx context.Context, name string, fn func(context.Context) error) {
	go func() {
		if err := fn(ctx); err != nil && ctx.Err() == nil {
			observability.L().Error("worker stopped", zap.String("name", name), zap.Error(err))
		}
	}()
}
