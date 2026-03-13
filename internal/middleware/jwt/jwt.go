package jwt

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gotik/internal/account"
	"gotik/internal/auth"
	rediscache "gotik/internal/middleware/redis"

	"github.com/gin-gonic/gin"
)

func JWTAuth(accountRepo *account.AccountRepository, cache *rediscache.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}

		tokenString := parts[1]

		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		check(c, claims, tokenString, accountRepo, cache)
	}
}

func GetAccountID(c *gin.Context) (uint, error) {
	uidValue, exists := c.Get("accountID")
	if !exists {
		return 0, errors.New("accountID not found")
	}

	accountID, ok := uidValue.(uint)
	if !ok {
		return 0, errors.New("accountID has invalid type")
	}

	return accountID, nil
}

func GetUsername(c *gin.Context) (string, error) {
	usernameValue, exists := c.Get("username")
	if !exists {
		return "", errors.New("username not found")
	}

	username, ok := usernameValue.(string)
	if !ok {
		return "", errors.New("username has invalid type")
	}

	return username, nil
}

func check(c *gin.Context, claims *auth.Claims, tokenString string, accountRepo *account.AccountRepository, cache *rediscache.Client) {
	key := fmt.Sprintf("account:%d", claims.AccountID) //key is account id

	// 先查 Redis
	if cache != nil {
		cacheCtx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Millisecond)
		defer cancel()

		b, err := cache.GetBytes(cacheCtx, key)
		if err == nil {
			log.Printf("redis hit: key=%s", key)
			if string(b) != tokenString {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
				return
			}
			c.Set("accountID", claims.AccountID)
			c.Set("username", claims.Username)
			c.Next()
			return
		}
	}

	// Redis 故障/未启用：查 DB 兜底
	accountInfo, err := accountRepo.FindByID(c.Request.Context(), claims.AccountID)
	if err != nil || accountInfo.Token == "" || accountInfo.Token != tokenString {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
		return
	}

	if cache != nil { //自愈
		cacheCtx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Millisecond)
		defer cancel()

		if err := cache.SetBytes(cacheCtx, key, []byte(tokenString), 24*time.Hour); err != nil {
			log.Printf("failed to set cache: %v", err)
		}
	}

	c.Set("accountID", claims.AccountID)
	c.Set("username", claims.Username)
	c.Next()

}
