package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"video-feed/internal/account"
	"video-feed/internal/config"
	"video-feed/internal/db"
	_ "video-feed/internal/docs"
	apphttp "video-feed/internal/http"
	"video-feed/internal/message"
	"video-feed/internal/middleware/rabbitmq"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/observability"
	"video-feed/internal/social"
	"video-feed/internal/video"
	"video-feed/internal/worker"

	"go.uber.org/zap"
)

// @title           Video Feed API
// @version         1.0
// @description     短视频社区后端 API:账号、视频、Feed、互动、通知。
// @host            127.0.0.1:8080
// @BasePath        /
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Bearer JWT token.
func main() {
	logger := observability.InitLogger("api")
	defer observability.Sync()

	shutdownTracer, err := observability.InitTracer("api")
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
	if err := db.AutoMigrate(database, &account.Account{}, &video.Video{}, &video.OutboxMsg{}, &video.Like{}, &video.Comment{}, &video.Tag{}, &video.VideoTag{}, &social.Social{}, &message.Message{}, &worker.Notification{}); err != nil {
		logger.Fatal("auto migrate", zap.Error(err))
	}

	cache := rediscache.NewFromConfig(cfg.Redis)
	pingCtx, cancel := contextWithTimeout()
	if err := cache.Ping(pingCtx); err != nil {
		logger.Warn("redis unavailable, cache disabled", zap.Error(err))
		_ = cache.Close()
		cache = nil
	}
	cancel()
	if cache != nil {
		defer cache.Close()
	}

	pprofServer, err := observability.NewPprofServer("api", cfg.Observability.Pprof.Enabled, cfg.Observability.Pprof.APIAddr)
	if err != nil {
		logger.Warn("pprof unavailable", zap.Error(err))
	}
	if pprofServer != nil {
		defer pprofServer.Close()
	}

	var publishers *rabbitmq.Publishers
	broker, err := rabbitmq.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		logger.Warn("rabbitmq unavailable, async workers disabled", zap.Error(err))
	} else {
		defer broker.Close()
		publishers, err = rabbitmq.NewPublishers(broker)
		if err != nil {
			logger.Warn("rabbitmq publisher init failed, async workers disabled", zap.Error(err))
			publishers = nil
		}
	}

	router := apphttp.NewRouterWithPublishers(database, cache, publishers)
	logger.Info("api server starting", zap.Int("port", cfg.Server.Port))
	if err := router.Run(":" + strconv.Itoa(cfg.Server.Port)); err != nil {
		logger.Fatal("run server", zap.Error(err))
	}
}

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 300*time.Millisecond)
}
