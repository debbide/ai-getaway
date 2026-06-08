package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-gateway/config"
	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/service"
	"ai-gateway/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func APIKeyAuth(cfg config.Config, db *gorm.DB, redisClient *redis.Client) gin.HandlerFunc {
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
		if err := db.Preload("User.PublicChannel").Preload("User.Plan.PublicChannel").Preload("User.Plan.PollingPool.Accounts").Where("key_hash = ? AND status = ?", utils.HashToken(token), model.APIKeyStatusActive).First(&apiKey).Error; err != nil {
			response.Error(c, 401, "invalid api key")
			c.Abort()
			return
		}
		if apiKey.User.Status != model.UserStatusApproved {
			response.Error(c, 403, "user is not approved")
			c.Abort()
			return
		}
		if apiKey.User.ExpiresAt != nil && time.Now().After(*apiKey.User.ExpiresAt) && !service.HasBalanceAccess(apiKey.User, time.Now()) {
			response.Error(c, 403, "subscription expired")
			c.Abort()
			return
		}
		if !service.HasCallableAccess(apiKey.User, time.Now()) {
			response.Error(c, 403, "no active subscription assigned")
			c.Abort()
			return
		}
		if !allowAPIKey(redisClient, apiKey.ID, cfg.APIKeyRateLimitPerMin) {
			response.Error(c, 429, "rate limit exceeded")
			c.Abort()
			return
		}

		now := time.Now()
		protocol := requestProtocol(c)
		db.Model(&apiKey).Updates(map[string]interface{}{"last_used_at": &now})

		c.Set("api_key", apiKey)
		c.Set("protocol", protocol)
		c.Next()
	}
}

func loadEnabledChannelMultipliers(db *gorm.DB) map[string]string {
	var channels []model.UpstreamChannel
	db.Select("name", "group_multipliers").Where("enabled = ?", true).Find(&channels)
	values := make(map[string]string, len(channels))
	for _, channel := range channels {
		if channel.GroupMultipliers == "" {
			continue
		}
		values[channel.Name] = channel.GroupMultipliers
	}
	return values
}

func requestProtocol(c *gin.Context) string {
	path := strings.ToLower(c.Request.URL.Path)
	if strings.Contains(path, "/messages") || c.GetHeader("Anthropic-Version") != "" || c.GetHeader("Anthropic-Beta") != "" {
		return model.ProtocolClaude
	}
	return model.ProtocolGPT
}

func allowAPIKey(redisClient *redis.Client, apiKeyID uint, limitPerMinute int) bool {
	if limitPerMinute <= 0 {
		return true
	}
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
	return count <= int64(limitPerMinute)
}
