package jwt

import (
	"errors"
	"net/http"
	"strings"

	"video-feed/internal/account"
	"video-feed/internal/auth"

	"github.com/gin-gonic/gin"
)

func JWTAuth(accountRepo *account.Repository) gin.HandlerFunc {
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
		check(c, claims, tokenString, accountRepo)
	}
}

func SoftJWTAuth(accountRepo *account.Repository) gin.HandlerFunc {
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
		check(c, claims, tokenString, accountRepo)
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

func check(c *gin.Context, claims *auth.Claims, tokenString string, accountRepo *account.Repository) {
	if accountRepo == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token validation unavailable"})
		return
	}

	accountInfo, err := accountRepo.FindByID(c.Request.Context(), claims.AccountID)
	if err != nil || accountInfo.Token == "" || accountInfo.Token != tokenString {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
		return
	}

	c.Set("accountID", claims.AccountID)
	c.Set("username", claims.Username)
	c.Next()
}
