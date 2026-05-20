package http

import (
	"time"

	"video-feed/internal/account"
	"video-feed/internal/feed"
	authjwt "video-feed/internal/middleware/jwt"
	"video-feed/internal/middleware/ratelimit"
	rediscache "video-feed/internal/middleware/redis"
	"video-feed/internal/social"
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

		likeRepo := video.NewLikeRepository(database)
		likeService := video.NewLikeService(likeRepo, videoRepo, tokenCache)
		likeHandler := video.NewLikeHandler(likeService)
		likeGroup := router.Group("/like")
		likeGroup.Use(authjwt.JWTAuth(accountRepo, tokenCache))
		{
			likeGroup.POST("/like", ratelimit.Limit(tokenCache, "like_write", 30, time.Minute, ratelimit.KeyByAccount), likeHandler.Like)
			likeGroup.POST("/unlike", ratelimit.Limit(tokenCache, "like_write", 30, time.Minute, ratelimit.KeyByAccount), likeHandler.Unlike)
			likeGroup.POST("/isLiked", likeHandler.IsLiked)
			likeGroup.POST("/listMyLikedVideos", likeHandler.ListMyLikedVideos)
		}

		commentRepo := video.NewCommentRepository(database)
		commentService := video.NewCommentService(commentRepo, videoRepo, tokenCache)
		commentHandler := video.NewCommentHandler(commentService, accountService)
		commentGroup := router.Group("/comment")
		{
			commentGroup.POST("/listAll", commentHandler.GetAllComments)
		}
		protectedCommentGroup := commentGroup.Group("")
		protectedCommentGroup.Use(authjwt.JWTAuth(accountRepo, tokenCache))
		{
			protectedCommentGroup.POST("/publish", ratelimit.Limit(tokenCache, "comment_write", 10, time.Minute, ratelimit.KeyByAccount), commentHandler.PublishComment)
			protectedCommentGroup.POST("/delete", ratelimit.Limit(tokenCache, "comment_write", 10, time.Minute, ratelimit.KeyByAccount), commentHandler.DeleteComment)
		}

		socialRepo := social.NewRepository(database)
		socialService := social.NewService(socialRepo, accountRepo)
		socialHandler := social.NewHandler(socialService)
		socialGroup := router.Group("/social")
		socialGroup.Use(authjwt.JWTAuth(accountRepo, tokenCache))
		{
			socialGroup.POST("/follow", ratelimit.Limit(tokenCache, "social_write", 20, time.Minute, ratelimit.KeyByAccount), socialHandler.Follow)
			socialGroup.POST("/unfollow", ratelimit.Limit(tokenCache, "social_write", 20, time.Minute, ratelimit.KeyByAccount), socialHandler.Unfollow)
			socialGroup.POST("/getAllFollowers", socialHandler.GetAllFollowers)
			socialGroup.POST("/getAllVloggers", socialHandler.GetAllVloggers)
			socialGroup.POST("/getCounts", socialHandler.GetCounts)
		}

		feedRepo := feed.NewRepository(database)
		feedService := feed.NewService(feedRepo, likeRepo, tokenCache)
		feedHandler := feed.NewHandler(feedService)
		feedGroup := router.Group("/feed")
		feedGroup.Use(authjwt.SoftJWTAuth(accountRepo, tokenCache))
		{
			feedGroup.POST("/listLatest", feedHandler.ListLatest)
			feedGroup.POST("/listLikesCount", feedHandler.ListLikesCount)
			feedGroup.POST("/listByPopularity", feedHandler.ListByPopularity)
		}
		protectedFeedGroup := feedGroup.Group("")
		protectedFeedGroup.Use(authjwt.JWTAuth(accountRepo, tokenCache))
		{
			protectedFeedGroup.POST("/listByFollowing", feedHandler.ListByFollowing)
		}
	}
	return router
}
