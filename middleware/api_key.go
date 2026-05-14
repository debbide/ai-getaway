package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/service"
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
		if err := db.Preload("User").Preload("User.Plan").Where("key_hash = ? AND status = ?", utils.HashToken(token), model.APIKeyStatusActive).First(&apiKey).Error; err != nil {
			response.Error(c, 401, "invalid api key")
			c.Abort()
			return
		}
		if apiKey.User.Status != model.UserStatusApproved {
			response.Error(c, 403, "user is not approved")
			c.Abort()
			return
		}
		if apiKey.User.ExpiresAt != nil && time.Now().After(*apiKey.User.ExpiresAt) {
			response.Error(c, 403, "subscription expired")
			c.Abort()
			return
		}
		if !service.HasActiveSubscription(apiKey.User, time.Now()) || apiKey.User.Plan == nil {
			response.Error(c, 403, "no active subscription assigned")
			c.Abort()
			return
		}
		if !allowPlanQuota(db, apiKey.User) {
			response.Error(c, 429, "subscription quota exceeded")
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

func allowPlanQuota(db *gorm.DB, user model.User) bool {
	if !service.HasActiveSubscription(user, time.Now()) {
		return false
	}
	if user.Plan == nil {
		return false
	}
	usage := service.PlanQuotaUsage(db, user.ID, user.Plan, time.Now())
	if usage.LimitUSDCents > 0 && usage.UsedUSDCents >= usage.LimitUSDCents {
		return false
	}
	return true
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
