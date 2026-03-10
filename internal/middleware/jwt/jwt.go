package jwt

import (
	"errors"
	"net/http"
	"strings"

	"gotik/internal/account"
	"gotik/internal/auth"

	"github.com/gin-gonic/gin"
)

func JWTAuth(accountRepo *account.AccountRepository) gin.HandlerFunc {
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

		accountInfo, err := accountRepo.FindByID(c.Request.Context(), claims.AccountID)
		if err != nil || accountInfo.Token == "" || accountInfo.Token != tokenString {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
			return
		}

		// token is legal ,so put it into context ，后续不需要重复解析token
		c.Set("accountID", claims.AccountID)
		c.Set("username", claims.Username)
		c.Next()
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
