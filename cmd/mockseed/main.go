package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"video-feed/internal/account"
	"video-feed/internal/config"
	"video-feed/internal/db"
	"video-feed/internal/social"
	"video-feed/internal/video"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Real playable MP4 videos (short, royalty-free).
// Mix of Pexels videos and sample MP4s — all publicly accessible.
var mockVideos = []struct {
	title   string
	desc    string
	playURL string
	cover   string
}{
	{
		"城市夜景延时摄影",
		"霓虹灯光下的城市车流 #城市 #夜景",
		"https://videos.pexels.com/video-files/3571264/3571264-uhd_2560_1440_30fps.mp4",
		"https://images.pexels.com/videos/3571264/city-lights-3571264.jpeg?auto=compress&cs=tinysrgb&w=640",
	},
	{
		"海滩日落",
		"金色阳光洒在波浪上 #海边 #日落",
		"https://videos.pexels.com/video-files/1093662/1093662-hd_1920_1080_30fps.mp4",
		"https://images.pexels.com/videos/1093662/sunset-1093662.jpeg?auto=compress&cs=tinysrgb&w=640",
	},
	{
		"咖啡拉花",
		"手冲咖啡的制作过程 #咖啡 #生活",
		"https://videos.pexels.com/video-files/2795173/2795173-hd_1920_1080_24fps.mp4",
		"https://images.pexels.com/videos/2795173/coffee-2795173.jpeg?auto=compress&cs=tinysrgb&w=640",
	},
	{
		"森林小溪",
		"阳光穿过树叶洒在溪流上 #自然 #治愈",
		"https://videos.pexels.com/video-files/1448735/1448735-hd_1920_1080_24fps.mp4",
		"https://images.pexels.com/videos/1448735/forest-1448735.jpeg?auto=compress&cs=tinysrgb&w=640",
	},
	{
		"猫咪日常",
		"橘猫打盹的午后时光 #猫咪 #萌宠",
		"https://videos.pexels.com/video-files/3209211/3209211-hd_1920_1080_30fps.mp4",
		"https://images.pexels.com/videos/3209211/cat-3209211.jpeg?auto=compress&cs=tinysrgb&w=640",
	},
	{
		"樱花飘落",
		"春风中的粉色浪漫 #樱花 #春天",
		"https://videos.pexels.com/video-files/3571264/3571264-sd_640_360_30fps.mp4",
		"https://images.pexels.com/videos/3571264/city-lights-3571264.jpeg?auto=compress&cs=tinysrgb&w=640",
	},
	{
		"星空银河",
		"延时摄影下的浩瀚星空 #星空 #摄影",
		"https://videos.pexels.com/video-files/1093662/1093662-sd_640_360_30fps.mp4",
		"https://images.pexels.com/videos/1093662/sunset-1093662.jpeg?auto=compress&cs=tinysrgb&w=640",
	},
	{
		"美食制作",
		"精致料理的烹饪全过程 #美食 #料理",
		"https://videos.pexels.com/video-files/2795173/2795173-sd_640_360_24fps.mp4",
		"https://images.pexels.com/videos/2795173/coffee-2795173.jpeg?auto=compress&cs=tinysrgb&w=640",
	},
	{
		"城市天际线",
		"高楼林立的都市风景线 #城市 #建筑",
		"https://videos.pexels.com/video-files/1448735/1448735-sd_640_360_24fps.mp4",
		"https://images.pexels.com/videos/1448735/forest-1448735.jpeg?auto=compress&cs=tinysrgb&w=640",
	},
	{
		"萌宠乐园",
		"可爱的小动物们 #萌宠 #治愈系",
		"https://videos.pexels.com/video-files/3209211/3209211-sd_640_360_30fps.mp4",
		"https://images.pexels.com/videos/3209211/cat-3209211.jpeg?auto=compress&cs=tinysrgb&w=640",
	},
}

var mockUsernames = []string{
	"小明旅行记", "吃货日记", "萌宠星球", "城市探索者", "摄影小达人",
	"生活美学家", "自然之声", "音乐旋律", "运动达人", "代码与咖啡",
	"星空守望者", "美食猎人", "旅行摄影师", "猫咪控", "文艺青年",
}

type options struct {
	configPath string
	password   string
	rounds     int // how many times to repeat the video set
	likes      bool
	comments   bool
	follows    bool
	clean      bool // delete previously seeded mock data
}

func main() {
	opts := options{}
	flag.StringVar(&opts.configPath, "config", "configs/config.yaml", "config file path")
	flag.StringVar(&opts.password, "password", "mock123", "password for mock accounts")
	flag.IntVar(&opts.rounds, "rounds", 3, "number of rounds to repeat the video set")
	flag.BoolVar(&opts.likes, "likes", true, "seed random likes")
	flag.BoolVar(&opts.comments, "comments", true, "seed comments")
	flag.BoolVar(&opts.follows, "follows", true, "seed follow relations")
	flag.BoolVar(&opts.clean, "clean", false, "delete all mock data before seeding")
	flag.Parse()

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
	); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	ctx := context.Background()

	if opts.clean {
		cleanMockData(ctx, database)
	}

	users := seedUsers(ctx, database, opts)
	videos := seedVideos(ctx, database, users, opts)

	if opts.follows {
		seedFollows(ctx, database, users)
	}
	if opts.likes {
		seedLikes(ctx, database, users, videos)
	}
	if opts.comments {
		seedComments(ctx, database, users, videos)
	}

	log.Printf("done! seeded %d users, %d videos", len(users), len(videos))
	log.Printf("login with any user: username=%s password=%s", users[0].Username, opts.password)
}

func cleanMockData(ctx context.Context, database *gorm.DB) {
	log.Println("cleaning previous mock data...")
	tables := []string{"notifications", "comments", "likes", "video_tags", "socials", "videos", "accounts"}
	for _, t := range tables {
		database.WithContext(ctx).Exec("DELETE FROM " + t)
	}
	log.Println("clean done")
}

func seedUsers(ctx context.Context, database *gorm.DB, opts options) []account.Account {
	hash, err := bcrypt.GenerateFromPassword([]byte(opts.password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	rows := make([]account.Account, 0, len(mockUsernames))
	for _, name := range mockUsernames {
		rows = append(rows, account.Account{
			Username: name,
			Password: string(hash),
			Bio:      "这是一个模拟账号",
		})
	}
	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 100).Error; err != nil {
		log.Fatalf("create accounts: %v", err)
	}

	var users []account.Account
	if err := database.WithContext(ctx).Where("username IN ?", mockUsernames).Order("id ASC").Find(&users).Error; err != nil {
		log.Fatalf("load accounts: %v", err)
	}
	log.Printf("seeded %d accounts", len(users))
	return users
}

func seedVideos(ctx context.Context, database *gorm.DB, users []account.Account, opts options) []video.Video {
	total := len(mockVideos) * opts.rounds
	now := time.Now().UTC()
	rows := make([]video.Video, 0, total)
	titles := make([]string, 0, total)

	for r := 0; r < opts.rounds; r++ {
		for i, mv := range mockVideos {
			idx := r*len(mockVideos) + i
			author := users[idx%len(users)]
			title := mv.title
			if r > 0 {
				title = fmt.Sprintf("%s (%d)", mv.title, r+1)
			}
			titles = append(titles, title)
			rows = append(rows, video.Video{
				AuthorID:    author.ID,
				Username:    author.Username,
				Title:       title,
				Description: mv.desc,
				PlayURL:     mv.playURL,
				CoverURL:    mv.cover,
				LikesCount:  int64(10 + idx*3%200),
				Popularity:  int64(total - idx + (idx % 17)),
				CreatedAt:   now.Add(-time.Duration(idx) * 5 * time.Minute),
				UpdatedAt:   now.Add(-time.Duration(idx) * 5 * time.Minute),
			})
		}
	}

	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 100).Error; err != nil {
		log.Fatalf("create videos: %v", err)
	}

	var videos []video.Video
	if err := database.WithContext(ctx).Where("title IN ?", titles).Order("id ASC").Find(&videos).Error; err != nil {
		log.Fatalf("load videos: %v", err)
	}
	log.Printf("seeded %d videos", len(videos))
	return videos
}

func seedFollows(ctx context.Context, database *gorm.DB, users []account.Account) {
	rows := make([]social.Social, 0)
	for i, u := range users {
		for j := 1; j <= 3 && j < len(users); j++ {
			peer := users[(i+j)%len(users)]
			if u.ID == peer.ID {
				continue
			}
			rows = append(rows, social.Social{FollowerID: u.ID, VloggerID: peer.ID})
		}
	}
	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 500).Error; err != nil {
		log.Fatalf("create follows: %v", err)
	}
	log.Printf("seeded %d follow relations", len(rows))
}

func seedLikes(ctx context.Context, database *gorm.DB, users []account.Account, videos []video.Video) {
	rows := make([]video.Like, 0)
	for _, v := range videos {
		numLikes := 3 + int(v.ID)%8
		for j := 0; j < numLikes && j < len(users); j++ {
			u := users[(int(v.ID)+j)%len(users)]
			rows = append(rows, video.Like{VideoID: v.ID, AccountID: u.ID})
		}
	}
	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 1000).Error; err != nil {
		log.Fatalf("create likes: %v", err)
	}
	log.Printf("seeded %d likes", len(rows))
}

func seedComments(ctx context.Context, database *gorm.DB, users []account.Account, videos []video.Video) {
	commentTexts := []string{
		"太棒了！", "拍得好好看", "收藏了", "这是在哪里拍的？",
		"好治愈啊", "绝了！", "想去了", "同款！",
		"画质好清晰", "已关注，期待更多", "这个角度绝了", "真的假的？",
	}

	rows := make([]video.Comment, 0)
	for _, v := range videos {
		numComments := 2 + int(v.ID)%4
		for j := 0; j < numComments && j < len(users); j++ {
			u := users[(int(v.ID)+j)%len(users)]
			text := commentTexts[(int(v.ID)+j)%len(commentTexts)]
			rows = append(rows, video.Comment{
				VideoID:  v.ID,
				AuthorID: u.ID,
				Username: u.Username,
				Content:  text,
			})
		}
	}
	if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 1000).Error; err != nil {
		log.Fatalf("create comments: %v", err)
	}
	log.Printf("seeded %d comments", len(rows))
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ltime)
}
