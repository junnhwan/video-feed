package http

import (
	"video-feed/internal/account"
	"video-feed/internal/video"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(database *gorm.DB) *gin.Engine {
	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	if database != nil {
		accountRepo := account.NewRepository(database)
		accountService := account.NewService(accountRepo)
		accountHandler := account.NewHandler(accountService)

		accountGroup := router.Group("/account")
		{
			accountGroup.POST("/register", accountHandler.Register)
			accountGroup.POST("/login", accountHandler.Login)
			accountGroup.POST("/findByID", accountHandler.FindByID)
		}

		videoRepo := video.NewRepository(database)
		videoService := video.NewService(videoRepo)
		videoHandler := video.NewHandler(videoService)

		videoGroup := router.Group("/video")
		{
			videoGroup.POST("/publish", videoHandler.Publish)
			videoGroup.POST("/getDetail", videoHandler.GetDetail)
			videoGroup.POST("/listByAuthorID", videoHandler.ListByAuthorID)
		}
	}
	return router
}
