package database

import (
	"context"
	"log"
	"time"

	"ai-gateway/config"
	"ai-gateway/model"
	"ai-gateway/utils"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitMariaDB(cfg config.Config) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(cfg.DBDSN), &gorm.Config{})
}

func InitRedis(cfg config.Config) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("redis unavailable, redis-backed features degrade gracefully: %v", err)
	}

	return client
}

func AutoMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(
		&model.User{},
		&model.Plan{},
		&model.Order{},
		&model.UpstreamAccount{},
		&model.APIKey{},
		&model.APILog{},
	); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
}

func Seed(db *gorm.DB, cfg config.Config) {
	plans := []model.Plan{
		{Name: "Basic", PriceCents: 9900, QuotaTokens: 1000000, DurationDays: 30, Description: "Starter subscription for personal usage", Enabled: true},
		{Name: "Pro", PriceCents: 29900, QuotaTokens: 5000000, DurationDays: 30, Description: "Higher quota for production workloads", Enabled: true},
		{Name: "Enterprise", PriceCents: 99900, QuotaTokens: 25000000, DurationDays: 30, Description: "Dedicated account and priority support", Enabled: true},
	}
	for _, plan := range plans {
		db.FirstOrCreate(&plan, model.Plan{Name: plan.Name})
	}

	var count int64
	db.Model(&model.User{}).Where("role = ?", model.RoleAdmin).Count(&count)
	if count == 0 {
		passwordHash, _ := utils.HashPassword(cfg.DefaultAdminPass)
		db.Create(&model.User{
			Username:     "admin",
			Email:        cfg.DefaultAdminMail,
			PasswordHash: passwordHash,
			Role:         model.RoleAdmin,
			Status:       model.UserStatusApproved,
		})
	}
}
