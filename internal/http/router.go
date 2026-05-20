package http

import (
	"video-feed/internal/account"
	authjwt "video-feed/internal/middleware/jwt"
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
			accountGroup.POST("/refresh", accountHandler.Refresh)
			accountGroup.POST("/changePassword", accountHandler.ChangePassword)
			accountGroup.POST("/findByID", accountHandler.FindByID)
			accountGroup.POST("/findByUsername", accountHandler.FindByUsername)
		}
		protectedAccountGroup := accountGroup.Group("")
		protectedAccountGroup.Use(authjwt.JWTAuth(accountRepo))
		{
			protectedAccountGroup.POST("/logout", accountHandler.Logout)
			protectedAccountGroup.POST("/rename", accountHandler.Rename)
			protectedAccountGroup.POST("/updateProfile", accountHandler.UpdateProfile)
		}

		videoRepo := video.NewRepository(database)
		videoService := video.NewService(videoRepo)
		videoHandler := video.NewHandler(videoService)

		videoGroup := router.Group("/video")
		{
			videoGroup.POST("/getDetail", videoHandler.GetDetail)
			videoGroup.POST("/listByAuthorID", videoHandler.ListByAuthorID)
		}
		protectedVideoGroup := videoGroup.Group("")
		protectedVideoGroup.Use(authjwt.JWTAuth(accountRepo))
		{
			protectedVideoGroup.POST("/publish", videoHandler.Publish)
		}
	}
	return router
}
