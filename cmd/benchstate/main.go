package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"video-feed/internal/benchkit"
	"video-feed/internal/config"
	rediscache "video-feed/internal/middleware/redis"

	goredis "github.com/redis/go-redis/v9"
)

func main() {
	var configPath string
	var manifestPath string
	var mode string
	flag.StringVar(&configPath, "config", "configs/config.yaml", "config file path")
	flag.StringVar(&manifestPath, "manifest", "", "seed manifest path")
	flag.StringVar(&mode, "mode", "", "state mode: hot, db, detail-cold")
	flag.Parse()
	if manifestPath == "" {
		log.Fatal("-manifest is required")
	}
	if mode == "" {
		log.Fatal("-mode is required")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	manifest, err := benchkit.ReadManifest(manifestPath)
	if err != nil {
		log.Fatalf("read manifest: %v", err)
	}
	cache := rediscache.NewFromConfig(cfg.Redis)
	ctx := context.Background()
	if err := cache.Ping(ctx); err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer cache.Close()

	switch mode {
	case "hot":
		seedHotKeys(ctx, cache, manifest)
	case "db":
		clearHotKeys(ctx, cache, manifest.HotAsOf)
	case "detail-cold":
		clearDetailKeys(ctx, cache, manifest.Videos)
	default:
		log.Fatalf("unsupported mode %q", mode)
	}
	log.Printf("bench state applied: mode=%s manifest=%s", mode, manifestPath)
}

func clearHotKeys(ctx context.Context, cache *rediscache.Client, asOf int64) {
	base := hotBase(asOf)
	for i := 0; i < 60; i++ {
		window := base.Add(-time.Duration(i) * time.Minute)
		_ = cache.Del(ctx, cache.Key("hot:video:1m:%s", window.Format("200601021504")))
	}
	_ = cache.Del(ctx, cache.Key("hot:video:merge:1m:%s", base.Format("200601021504")))
}

func seedHotKeys(ctx context.Context, cache *rediscache.Client, manifest benchkit.Manifest) {
	base := hotBase(manifest.HotAsOf)
	_ = cache.Del(ctx, cache.Key("hot:video:merge:1m:%s", base.Format("200601021504")))
	members := make([]goredis.Z, 0, len(manifest.Videos))
	for _, item := range manifest.Videos {
		members = append(members, goredis.Z{
			Score:  float64(item.Popularity),
			Member: fmt.Sprintf("%d", item.ID),
		})
	}
	must(cache.ZAdd(ctx, cache.Key("hot:video:1m:%s", base.Format("200601021504")), members...))
	must(cache.Expire(ctx, cache.Key("hot:video:1m:%s", base.Format("200601021504")), 2*time.Hour))
}

func clearDetailKeys(ctx context.Context, cache *rediscache.Client, videos []benchkit.ManifestVideo) {
	for _, item := range videos {
		_ = cache.Del(ctx, cache.Key("video:detail:id=%d", item.ID))
	}
}

func hotBase(asOf int64) time.Time {
	if asOf <= 0 {
		return time.Now().UTC().Truncate(time.Minute)
	}
	return time.Unix(asOf, 0).UTC().Truncate(time.Minute)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	log.SetOutput(os.Stdout)
}
