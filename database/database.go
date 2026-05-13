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
		&model.SystemSetting{},
		&model.EmailVerification{},
		&model.SlideCaptcha{},
	); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
}

func Seed(db *gorm.DB, cfg config.Config) {
	plans := []model.Plan{
		{Name: "日卡套餐", Code: "day-pass", PlanType: "subscription", PriceCents: 990, SettlementUSDCents: 100, QuotaTokens: 200000, DailyQuotaTokens: 200000, WeeklyQuotaTokens: 0, DurationDays: 1, Description: "适合短期测试的一日订阅", Enabled: true},
		{Name: "月卡套餐", Code: "monthly", PlanType: "subscription", PriceCents: 2990, SettlementUSDCents: 500, QuotaTokens: 5000000, DailyQuotaTokens: 300000, WeeklyQuotaTokens: 1500000, DurationDays: 30, Description: "适合个人长期使用的订阅套餐", Enabled: true},
		{Name: "团队套餐", Code: "team", PlanType: "subscription", PriceCents: 9990, SettlementUSDCents: 1800, QuotaTokens: 25000000, DailyQuotaTokens: 1500000, WeeklyQuotaTokens: 8000000, DurationDays: 30, Description: "团队额度与独立上游账号", Enabled: true},
	}
	for _, plan := range plans {
		db.FirstOrCreate(&plan, model.Plan{Name: plan.Name})
	}

	db.FirstOrCreate(&model.SystemSetting{}, model.SystemSetting{Model: gorm.Model{ID: 1}})

	var count int64
	db.Model(&model.User{}).Where("role = ?", model.RoleAdmin).Count(&count)
	if count == 0 {
		passwordHash, _ := utils.HashPassword(cfg.DefaultAdminPass)
		db.Create(&model.User{
			Username:      "admin",
			Email:         cfg.DefaultAdminMail,
			PasswordHash:  passwordHash,
			Role:          model.RoleAdmin,
			Status:        model.UserStatusApproved,
			EmailVerified: true,
		})
	}
}
