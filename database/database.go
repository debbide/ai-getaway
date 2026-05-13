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
		{Name: "日卡套餐", Code: "day-pass", BadgeText: "日用特惠", PlanType: "subscription", PriceCents: 590, SettlementUSDCents: 2000, QuotaTokens: 0, DailyQuotaTokens: 0, WeeklyQuotaTokens: 0, DurationDays: 1, Description: "灵活应对突发需求", Enabled: true},
		{Name: "月卡（标准）", Code: "monthly", BadgeText: "热卖推荐", PlanType: "subscription", PriceCents: 5900, SettlementUSDCents: 6000, QuotaTokens: 0, DailyQuotaTokens: 0, WeeklyQuotaTokens: 0, DurationDays: 30, Description: "覆盖常规研发工作量", Enabled: true},
		{Name: "月卡（专业）", Code: "team", BadgeText: "高频进阶", PlanType: "subscription", PriceCents: 9000, SettlementUSDCents: 9000, QuotaTokens: 0, DailyQuotaTokens: 0, WeeklyQuotaTokens: 0, DurationDays: 30, Description: "为高频团队保驾护航", Enabled: true},
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
