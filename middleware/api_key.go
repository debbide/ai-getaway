package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func APIKeyAuth(db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-API-Key")
		if token == "" {
			token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		}
		if token == "" {
			response.Error(c, 401, "missing api key")
			c.Abort()
			return
		}

		var apiKey model.APIKey
		if err := db.Preload("User").Where("key_hash = ? AND status = ?", utils.HashToken(token), model.APIKeyStatusActive).First(&apiKey).Error; err != nil {
			response.Error(c, 401, "invalid api key")
			c.Abort()
			return
		}
		if apiKey.User.Status != model.UserStatusApproved {
			response.Error(c, 403, "user is not approved")
			c.Abort()
			return
		}

		if !allowAPIKey(redisClient, apiKey.ID) {
			response.Error(c, 429, "rate limit exceeded")
			c.Abort()
			return
		}

		var upstream model.UpstreamAccount
		if err := db.Where("user_id = ? AND status = ?", apiKey.UserID, model.UpstreamStatusActive).First(&upstream).Error; err != nil {
			response.Error(c, 403, "no active upstream account bound")
			c.Abort()
			return
		}

		now := time.Now()
		db.Model(&apiKey).Updates(map[string]interface{}{"last_used_at": &now})
		db.Model(&upstream).Updates(map[string]interface{}{"last_used_at": &now})

		c.Set("api_key", apiKey)
		c.Set("upstream", upstream)
		c.Next()
	}
}

func allowAPIKey(redisClient *redis.Client, apiKeyID uint) bool {
	if redisClient == nil {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	key := fmt.Sprintf("rate_limit:api_key:%d:%d", apiKeyID, time.Now().Unix()/60)
	count, err := redisClient.Incr(ctx, key).Result()
	if err != nil {
		return true
	}
	if count == 1 {
		redisClient.Expire(ctx, key, time.Minute)
	}
	return count <= 120
}
