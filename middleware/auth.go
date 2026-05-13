package middleware

import (
	"strings"

	"ai-gateway/config"
	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Auth(cfg config.Config, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if tokenString == "" {
			response.Error(c, 401, "missing authorization token")
			c.Abort()
			return
		}

		claims, err := utils.ParseJWT(tokenString, cfg.JWTSecret)
		if err != nil {
			response.Error(c, 401, "invalid authorization token")
			c.Abort()
			return
		}

		var user model.User
		if err := db.First(&user, claims.UserID).Error; err != nil {
			response.Error(c, 401, "user not found")
			c.Abort()
			return
		}
		if user.Status == model.UserStatusDisabled {
			response.Error(c, 403, "user disabled")
			c.Abort()
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get("user")
		if !ok || user.(model.User).Role != model.RoleAdmin {
			response.Error(c, 403, "admin permission required")
			c.Abort()
			return
		}
		c.Next()
	}
}
