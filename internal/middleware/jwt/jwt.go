package jwt

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"video-feed/internal/account"
	"video-feed/internal/auth"
	rediscache "video-feed/internal/middleware/redis"

	"github.com/gin-gonic/gin"
)

func JWTAuth(accountRepo *account.Repository, cache ...*rediscache.Client) gin.HandlerFunc {
	var tokenCache *rediscache.Client
	if len(cache) > 0 {
		tokenCache = cache[0]
	}
	return func(c *gin.Context) {
		tokenString, ok := bearerToken(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}

		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		check(c, claims, tokenString, accountRepo, tokenCache)
	}
}

func SoftJWTAuth(accountRepo *account.Repository, cache ...*rediscache.Client) gin.HandlerFunc {
	var tokenCache *rediscache.Client
	if len(cache) > 0 {
		tokenCache = cache[0]
	}
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenString, ok := bearerToken(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}

		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		check(c, claims, tokenString, accountRepo, tokenCache)
	}
}

func GetAccountID(c *gin.Context) (uint, error) {
	value, exists := c.Get("accountID")
	if !exists {
		return 0, errors.New("accountID not found")
	}
	accountID, ok := value.(uint)
	if !ok {
		return 0, errors.New("accountID has invalid type")
	}
	return accountID, nil
}

func GetUsername(c *gin.Context) (string, error) {
	value, exists := c.Get("username")
	if !exists {
		return "", errors.New("username not found")
	}
	username, ok := value.(string)
	if !ok {
		return "", errors.New("username has invalid type")
	}
	return username, nil
}

func bearerToken(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return parts[1], true
}

func check(c *gin.Context, claims *auth.Claims, tokenString string, accountRepo *account.Repository, cache *rediscache.Client) {
	if accountRepo == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token validation unavailable"})
		return
	}

	if cache != nil {
		key := cache.Key("account:%d", claims.AccountID)
		cacheCtx, cancel := contextWithShortTimeout(c)
		defer cancel()
		cached, err := cache.GetBytes(cacheCtx, key)
		if err == nil {
			if string(cached) != tokenString {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
				return
			}
			setIdentity(c, claims)
			c.Next()
			return
		}
	}

	accountInfo, err := accountRepo.FindByID(c.Request.Context(), claims.AccountID)
	if err != nil || accountInfo.Token == "" || accountInfo.Token != tokenString {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
		return
	}

	if cache != nil {
		cacheCtx, cancel := contextWithShortTimeout(c)
		defer cancel()
		if err := cache.SetBytes(cacheCtx, cache.Key("account:%d", claims.AccountID), []byte(tokenString), 24*time.Hour); err != nil {
			log.Printf("failed to backfill account token cache: %v", err)
		}
	}

	setIdentity(c, claims)
	c.Next()
}

func setIdentity(c *gin.Context, claims *auth.Claims) {
	c.Set("accountID", claims.AccountID)
	c.Set("username", claims.Username)
}

func contextWithShortTimeout(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 50*time.Millisecond)
}
