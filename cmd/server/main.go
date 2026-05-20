package main

import (
	"log"
	"strconv"

	"video-feed/internal/account"
	"video-feed/internal/config"
	"video-feed/internal/db"
	apphttp "video-feed/internal/http"
	"video-feed/internal/video"
)

func main() {
	cfg := config.Default()
	database, err := db.Open(cfg.Database)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(database, &account.Account{}, &video.Video{}); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	router := apphttp.NewRouter(database)
	if err := router.Run(":" + strconv.Itoa(cfg.Server.Port)); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
