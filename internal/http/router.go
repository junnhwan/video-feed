package http

import (
	"time"

	"video-feed/internal/account"
	authjwt "video-feed/internal/middleware/jwt"
	"video-feed/internal/middleware/ratelimit"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/video"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(database *gorm.DB, cache ...*rediscache.Client) *gin.Engine {
	var tokenCache *rediscache.Client
	if len(cache) > 0 {
		tokenCache = cache[0]
	}

	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	if database != nil {
		accountRepo := account.NewRepository(database)
		accountService := account.NewService(accountRepo, tokenCache)
		accountHandler := account.NewHandler(accountService)

		accountGroup := router.Group("/account")
		{
			accountGroup.POST("/register", ratelimit.Limit(tokenCache, "account_register", 5, time.Hour, ratelimit.KeyByIP), accountHandler.Register)
			accountGroup.POST("/login", ratelimit.Limit(tokenCache, "account_login", 10, time.Minute, ratelimit.KeyByIP), accountHandler.Login)
			accountGroup.POST("/refresh", accountHandler.Refresh)
			accountGroup.POST("/changePassword", accountHandler.ChangePassword)
			accountGroup.POST("/findByID", accountHandler.FindByID)
			accountGroup.POST("/findByUsername", accountHandler.FindByUsername)
		}
		protectedAccountGroup := accountGroup.Group("")
		protectedAccountGroup.Use(authjwt.JWTAuth(accountRepo, tokenCache))
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
		protectedVideoGroup.Use(authjwt.JWTAuth(accountRepo, tokenCache))
		{
			protectedVideoGroup.POST("/publish", ratelimit.Limit(tokenCache, "video_publish", 30, time.Minute, ratelimit.KeyByAccount), videoHandler.Publish)
		}
	}
	return router
}
