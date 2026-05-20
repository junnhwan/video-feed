package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"video-feed/internal/account"
	"video-feed/internal/benchkit"
	"video-feed/internal/config"
	"video-feed/internal/db"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/social"
	"video-feed/internal/video"
	"video-feed/internal/worker"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type seedOptions struct {
	configPath       string
	baseURL          string
	outPath          string
	runID            string
	password         string
	userCount        int
	videoCount       int
	likesPerVideo    int
	commentsPerVideo int
	followsPerUser   int
	prewarmEntities  bool
}

func main() {
	opts := parseOptions()
	ctx := context.Background()

	cfg, err := config.Load(opts.configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	database, err := db.Open(cfg.Database)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(database,
		&account.Account{},
		&video.Video{},
		&video.OutboxMsg{},
		&video.Like{},
		&video.Comment{},
		&video.Tag{},
		&video.VideoTag{},
		&social.Social{},
		&worker.Notification{},
	); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	users := seedAccounts(ctx, database, opts)
	videos := seedVideos(ctx, database, users, opts)
	seedTags(ctx, database, videos)
	seedRelations(ctx, database, users, opts.followsPerUser)
	seedLikes(ctx, database, users, videos, opts.likesPerVideo)
	seedComments(ctx, database, users, videos, opts.commentsPerVideo)

	hotAsOf := time.Now().UTC().Truncate(time.Minute)
	if err := seedRedis(ctx, cfg, videos, hotAsOf, opts.prewarmEntities); err != nil {
		log.Printf("seed redis skipped or incomplete: %v", err)
	}

	manifest := buildManifest(opts, users, videos, hotAsOf)
	resultDir := filepath.Dir(opts.outPath)
	manifest.UsersCSV = filepath.ToSlash(filepath.Join(resultDir, "users-"+opts.runID+".csv"))
	manifest.VideosCSV = filepath.ToSlash(filepath.Join(resultDir, "videos-"+opts.runID+".csv"))
	if err := benchkit.WriteUsersCSV(manifest.UsersCSV, manifest.Users); err != nil {
		log.Fatalf("write users csv: %v", err)
	}
	if err := benchkit.WriteVideosCSV(manifest.VideosCSV, manifest.Videos); err != nil {
		log.Fatalf("write videos csv: %v", err)
	}
	if err := benchkit.WriteManifest(opts.outPath, manifest); err != nil {
		log.Fatalf("write manifest: %v", err)
	}
	log.Printf("seeded users=%d videos=%d manifest=%s users_csv=%s videos_csv=%s", len(users), len(videos), opts.outPath, manifest.UsersCSV, manifest.VideosCSV)
}

func parseOptions() seedOptions {
	now := time.Now().Format("20060102-150405")
	opts := seedOptions{}
	flag.StringVar(&opts.configPath, "config", "configs/config.yaml", "config file path")
	flag.StringVar(&opts.baseURL, "base-url", "http://127.0.0.1:8080", "API base URL recorded in manifest")
	flag.StringVar(&opts.runID, "run-id", "bench-"+now, "seed run id")
	flag.StringVar(&opts.password, "password", "benchpass123", "password shared by seeded users")
	flag.IntVar(&opts.userCount, "users", 50, "number of accounts to seed")
	flag.IntVar(&opts.videoCount, "videos", 1000, "number of videos to seed")
	flag.IntVar(&opts.likesPerVideo, "likes-per-video", 12, "likes per video")
	flag.IntVar(&opts.commentsPerVideo, "comments-per-video", 2, "comments per video")
	flag.IntVar(&opts.followsPerUser, "follows-per-user", 8, "following relations per account")
	flag.BoolVar(&opts.prewarmEntities, "prewarm-entities", true, "prewarm Redis feed entity cache")
	flag.StringVar(&opts.outPath, "out", "", "manifest output path")
	flag.Parse()

	if opts.outPath == "" {
		opts.outPath = filepath.Join("bench", "results", "seed-"+opts.runID+".json")
	}
	if opts.userCount < 2 {
		log.Fatal("users must be >= 2")
	}
	if opts.videoCount < 1 {
		log.Fatal("videos must be >= 1")
	}
	if opts.likesPerVideo < 0 || opts.commentsPerVideo < 0 || opts.followsPerUser < 0 {
		log.Fatal("likes/comments/follows counts must be >= 0")
	}
	return opts
}

func seedAccounts(ctx context.Context, database *gorm.DB, opts seedOptions) []account.Account {
	hash, err := bcrypt.GenerateFromPassword([]byte(opts.password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	rows := make([]account.Account, 0, opts.userCount)
	usernames := make([]string, 0, opts.userCount)
	for i := 0; i < opts.userCount; i++ {
		username := fmt.Sprintf("%s_user_%04d", opts.runID, i+1)
		usernames = append(usernames, username)
		rows = append(rows, account.Account{
			Username: username,
			Password: string(hash),
			Bio:      "bench seed account",
		})
	}
	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 200).Error; err != nil {
		log.Fatalf("create accounts: %v", err)
	}
	var users []account.Account
	if err := database.WithContext(ctx).Where("username IN ?", usernames).Order("id ASC").Find(&users).Error; err != nil {
		log.Fatalf("load accounts: %v", err)
	}
	if len(users) != opts.userCount {
		log.Fatalf("seed account count mismatch: got %d want %d", len(users), opts.userCount)
	}
	return users
}

func seedVideos(ctx context.Context, database *gorm.DB, users []account.Account, opts seedOptions) []video.Video {
	now := time.Now().UTC()
	rows := make([]video.Video, 0, opts.videoCount)
	titles := make([]string, 0, opts.videoCount)
	for i := 0; i < opts.videoCount; i++ {
		author := users[i%len(users)]
		title := fmt.Sprintf("%s video %04d #feed", opts.runID, i+1)
		titles = append(titles, title)
		popularity := int64((opts.videoCount-i)*3 + (i % 17))
		likesCount := int64(minInt(opts.likesPerVideo, len(users)))
		rows = append(rows, video.Video{
			AuthorID:    author.ID,
			Username:    author.Username,
			Title:       title,
			Description: fmt.Sprintf("bench video %04d #go #redis", i+1),
			PlayURL:     fmt.Sprintf("https://example.invalid/video/%s/%04d.mp4", opts.runID, i+1),
			CoverURL:    fmt.Sprintf("https://example.invalid/cover/%s/%04d.jpg", opts.runID, i+1),
			LikesCount:  likesCount,
			Popularity:  popularity,
			CreatedAt:   now.Add(-time.Duration(i) * time.Second),
			UpdatedAt:   now.Add(-time.Duration(i) * time.Second),
		})
	}
	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 500).Error; err != nil {
		log.Fatalf("create videos: %v", err)
	}
	var videos []video.Video
	if err := database.WithContext(ctx).Where("title IN ?", titles).Order("id ASC").Find(&videos).Error; err != nil {
		log.Fatalf("load videos: %v", err)
	}
	if len(videos) != opts.videoCount {
		log.Fatalf("seed video count mismatch: got %d want %d", len(videos), opts.videoCount)
	}
	return videos
}

func seedTags(ctx context.Context, database *gorm.DB, videos []video.Video) {
	tags := []string{"feed", "go", "redis", "mq", "video"}
	byName := make(map[string]video.Tag, len(tags))
	for _, name := range tags {
		tag := video.Tag{Name: name}
		if err := database.WithContext(ctx).Where("name = ?", name).FirstOrCreate(&tag).Error; err != nil {
			log.Fatalf("create tag %s: %v", name, err)
		}
		byName[name] = tag
	}
	rows := make([]video.VideoTag, 0, len(videos)*2)
	for i, item := range videos {
		rows = append(rows, video.VideoTag{VideoID: item.ID, TagID: byName["feed"].ID})
		rows = append(rows, video.VideoTag{VideoID: item.ID, TagID: byName[tags[(i%4)+1]].ID})
	}
	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 500).Error; err != nil {
		log.Fatalf("create video tags: %v", err)
	}
}

func seedRelations(ctx context.Context, database *gorm.DB, users []account.Account, followsPerUser int) {
	if followsPerUser == 0 {
		return
	}
	rows := make([]social.Social, 0, len(users)*followsPerUser)
	for i, follower := range users {
		for j := 1; j <= followsPerUser && j < len(users); j++ {
			vlogger := users[(i+j)%len(users)]
			if follower.ID == vlogger.ID {
				continue
			}
			rows = append(rows, social.Social{FollowerID: follower.ID, VloggerID: vlogger.ID})
		}
	}
	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 500).Error; err != nil {
		log.Fatalf("create social relations: %v", err)
	}
}

func seedLikes(ctx context.Context, database *gorm.DB, users []account.Account, videos []video.Video, likesPerVideo int) {
	if likesPerVideo == 0 {
		return
	}
	rows := make([]video.Like, 0, len(videos)*likesPerVideo)
	limit := minInt(likesPerVideo, len(users))
	for i, item := range videos {
		for j := 0; j < limit; j++ {
			user := users[(i+j)%len(users)]
			rows = append(rows, video.Like{VideoID: item.ID, AccountID: user.ID})
		}
	}
	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 1000).Error; err != nil {
		log.Fatalf("create likes: %v", err)
	}
}

func seedComments(ctx context.Context, database *gorm.DB, users []account.Account, videos []video.Video, commentsPerVideo int) {
	if commentsPerVideo == 0 {
		return
	}
	rows := make([]video.Comment, 0, len(videos)*commentsPerVideo)
	for i, item := range videos {
		for j := 0; j < commentsPerVideo; j++ {
			user := users[(i+j)%len(users)]
			rows = append(rows, video.Comment{
				VideoID:  item.ID,
				AuthorID: user.ID,
				Username: user.Username,
				Content:  fmt.Sprintf("bench comment %d for video %d", j+1, item.ID),
			})
		}
	}
	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 1000).Error; err != nil {
		log.Fatalf("create comments: %v", err)
	}
}

func seedRedis(ctx context.Context, cfg config.Config, videos []video.Video, hotAsOf time.Time, prewarmEntities bool) error {
	cache := rediscache.NewFromConfig(cfg.Redis)
	if err := cache.Ping(ctx); err != nil {
		_ = cache.Close()
		return err
	}
	defer cache.Close()

	hotKey := cache.Key("hot:video:1m:%s", hotAsOf.Format("200601021504"))
	timelineKey := cache.Key("feed:global_timeline")
	hotMembers := make([]goredis.Z, 0, len(videos))
	timelineMembers := make([]goredis.Z, 0, len(videos))
	for _, item := range videos {
		member := fmt.Sprintf("%d", item.ID)
		hotMembers = append(hotMembers, goredis.Z{Score: float64(item.Popularity), Member: member})
		timelineMembers = append(timelineMembers, goredis.Z{Score: float64(item.CreatedAt.UnixMilli()), Member: member})
		if prewarmEntities {
			payload, err := json.Marshal(item)
			if err != nil {
				return err
			}
			if err := cache.SetBytes(ctx, cache.Key("video:entity:%d", item.ID), payload, time.Hour); err != nil {
				return err
			}
		}
	}
	if err := cache.ZAdd(ctx, hotKey, hotMembers...); err != nil {
		return err
	}
	if err := cache.Expire(ctx, hotKey, 2*time.Hour); err != nil {
		return err
	}
	if err := cache.ZAdd(ctx, timelineKey, timelineMembers...); err != nil {
		return err
	}
	return nil
}

func buildManifest(opts seedOptions, users []account.Account, videos []video.Video, hotAsOf time.Time) benchkit.Manifest {
	manifest := benchkit.Manifest{
		RunID:     opts.runID,
		BaseURL:   opts.baseURL,
		Password:  opts.password,
		CreatedAt: time.Now().UTC(),
		Tags:      []string{"feed", "go", "redis", "mq", "video"},
		HotAsOf:   hotAsOf.Unix(),
		Users:     make([]benchkit.ManifestUser, 0, len(users)),
		Videos:    make([]benchkit.ManifestVideo, 0, len(videos)),
	}
	for _, user := range users {
		manifest.Users = append(manifest.Users, benchkit.ManifestUser{ID: user.ID, Username: user.Username})
	}
	for _, item := range videos {
		manifest.Videos = append(manifest.Videos, benchkit.ManifestVideo{
			ID:         item.ID,
			AuthorID:   item.AuthorID,
			Title:      item.Title,
			Popularity: item.Popularity,
			LikesCount: item.LikesCount,
		})
	}
	return manifest
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	log.SetOutput(os.Stdout)
}
