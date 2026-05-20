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
		if err := db.Preload("User").Preload("User.Plan.PublicChannel").Preload("User.Plan.PollingPool.Accounts").Where("key_hash = ? AND status = ?", utils.HashToken(token), model.APIKeyStatusActive).First(&apiKey).Error; err != nil {
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

		now := time.Now()
		protocol := requestProtocol(c)
		db.Model(&apiKey).Updates(map[string]interface{}{"last_used_at": &now})
		channelMultiplierByName := loadEnabledChannelMultipliers(db)

		c.Set("api_key", apiKey)
		c.Set("protocol", protocol)
		if apiKey.User.Plan != nil && apiKey.User.Plan.PlanType == model.PlanTypePublic {
			if !service.PlanChannelSupportsProtocol(apiKey.User.Plan, protocol) {
				response.Error(c, 403, "protocol not supported by plan")
				c.Abort()
				return
			}
			if apiKey.User.Plan.PublicChannelID != nil {
				var publicChannel model.PublicChannel
				if db.Where("id = ? AND enabled = ? AND remaining_usd_cents > 0", *apiKey.User.Plan.PublicChannelID, true).First(&publicChannel).Error != nil ||
					!service.SupportsProtocol(publicChannel.SupportsGPT, publicChannel.SupportsClaude, protocol) {
					response.Error(c, 403, "public channel sold out")
					c.Abort()
					return
				}
				db.Model(&publicChannel).Updates(map[string]interface{}{"last_used_at": &now})
				c.Set("public_channel", publicChannel)
			} else if apiKey.User.Plan.PollingPoolID != nil {
				var poolAccount model.PollingPoolAccount
				if db.Joins("JOIN polling_pools ON polling_pools.id = polling_pool_accounts.polling_pool_id").
					Where("polling_pool_accounts.polling_pool_id = ? AND polling_pool_accounts.enabled = ? AND polling_pool_accounts.remaining_usd_cents > 0 AND polling_pools.enabled = ?", *apiKey.User.Plan.PollingPoolID, true, true).
					Order("polling_pool_accounts.sort_order asc, polling_pool_accounts.id asc").
					First(&poolAccount).Error != nil {
					response.Error(c, 403, "public channel sold out")
					c.Abort()
					return
				}
				db.Model(&poolAccount).Updates(map[string]interface{}{"last_used_at": &now})
				c.Set("pool_account", poolAccount)
			} else {
				response.Error(c, 403, "public channel sold out")
				c.Abort()
				return
			}
		} else {
			var upstream model.UpstreamAccount
			if err := db.Where("user_id = ? AND status = ?", apiKey.UserID, model.UpstreamStatusActive).First(&upstream).Error; err != nil {
				response.Error(c, 403, "no active upstream account bound")
				c.Abort()
				return
			}
			if !service.SupportsProtocol(upstream.SupportsGPT, upstream.SupportsClaude, protocol) {
				response.Error(c, 403, "protocol not supported by upstream")
				c.Abort()
				return
			}
			if upstream.GroupMultipliers == "" {
				if channelID := upstream.ChannelID; channelID != nil {
					var channel model.UpstreamChannel
					if db.Select("group_multipliers").Where("id = ?", *channelID).First(&channel).Error == nil {
						upstream.GroupMultipliers = channel.GroupMultipliers
					}
				}
				if upstream.GroupMultipliers == "" {
					upstream.GroupMultipliers = channelMultiplierByName[upstream.Channel]
				}
			}
			db.Model(&upstream).Updates(map[string]interface{}{"last_used_at": &now})
			c.Set("upstream", upstream)
		}
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

func allowPlanQuota(db *gorm.DB, user model.User) bool {
	if !service.HasActiveSubscription(user, time.Now()) {
		return false
	}
	if user.Plan == nil {
		return false
	}
	now := time.Now()
	usage, ok := service.UserPlanQuotaUsage(db, user, now)
	if !ok {
		return false
	}
	if !service.QuotaAllowsRequest(usage) {
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
