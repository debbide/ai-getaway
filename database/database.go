package database

import (
	"context"
	"log"
	"time"

	"ai-gateway/config"
	"ai-gateway/model"
	"ai-gateway/service"
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
		&model.UpstreamChannel{},
		&model.PublicChannel{},
		&model.DocPage{},
		&model.Announcement{},
		&model.APIKey{},
		&model.ModelPricing{},
		&model.APILog{},
		&model.SystemSetting{},
		&model.EmailTemplate{},
		&model.EmailNotificationLog{},
		&model.EmailVerification{},
	); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
	dropLegacyQuotaColumns(db)
}

func Seed(db *gorm.DB, cfg config.Config) {
	plans := []model.Plan{
		{Name: "日卡套餐", Code: "day-pass", BadgeText: "日用特惠", PlanType: "subscription", QuotaPeriod: "daily", PriceCents: 590, SettlementUSDCents: 2000, DurationDays: 1, Description: "灵活应对突发需求", Enabled: true},
		{Name: "月卡（标准）", Code: "monthly", BadgeText: "热卖推荐", PlanType: "subscription", QuotaPeriod: "weekly", PriceCents: 5900, SettlementUSDCents: 6000, DurationDays: 30, Description: "覆盖常规研发工作量", Enabled: true},
		{Name: "月卡（专业）", Code: "team", BadgeText: "高频进阶", PlanType: "subscription", QuotaPeriod: "weekly", PriceCents: 9000, SettlementUSDCents: 9000, DurationDays: 30, Description: "为高频团队保驾护航", Enabled: true},
	}
	for _, plan := range plans {
		db.FirstOrCreate(&plan, model.Plan{Name: plan.Name})
	}

	seedDocs(db)
	service.SeedEmailTemplates(db)
	if _, err := service.SyncOfficialOpenAIModelPrices(db); err != nil {
		log.Printf("seed model pricing failed: %v", err)
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

func StartSlideCaptchaCleanup(db *gorm.DB) {
	if db == nil {
		return
	}
	cleanup := func() {
		if !db.Migrator().HasTable(&model.SlideCaptcha{}) {
			return
		}
		if err := db.Unscoped().
			Where("expires_at < ? OR used_at IS NOT NULL", time.Now()).
			Delete(&model.SlideCaptcha{}).Error; err != nil {
			log.Printf("slide captcha cleanup failed: %v", err)
		}
	}

	cleanup()
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cleanup()
		}
	}()
}

func StartOrderTimeoutCleanup(db *gorm.DB) {
	if db == nil {
		return
	}
	cleanup := func() {
		if !db.Migrator().HasTable(&model.Order{}) {
			return
		}
		if err := db.Model(&model.Order{}).
			Where("status = ? AND created_at <= ?", model.OrderStatusPendingPayment, time.Now().Add(-5*time.Minute)).
			Update("status", model.OrderStatusPaymentTimeout).Error; err != nil {
			log.Printf("order timeout cleanup failed: %v", err)
		}
	}

	cleanup()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cleanup()
		}
	}()
}

func StartSubscriptionExpireEmailReminder(db *gorm.DB) {
	if db == nil {
		return
	}
	remind := func() {
		if !db.Migrator().HasTable(&model.EmailTemplate{}) || !db.Migrator().HasTable(&model.EmailNotificationLog{}) {
			return
		}
		service.SendSubscriptionExpireReminders(db)
	}

	remind()
	go func() {
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			remind()
		}
	}()
}

func dropLegacyQuotaColumns(db *gorm.DB) {
	dropColumnIfExists(db, &model.User{}, "quota_tokens")
	dropColumnIfExists(db, &model.User{}, "used_tokens")
	dropColumnIfExists(db, &model.Plan{}, "quota_tokens")
	dropColumnIfExists(db, &model.Plan{}, "daily_quota_tokens")
	dropColumnIfExists(db, &model.Plan{}, "weekly_quota_tokens")
}

func dropColumnIfExists(db *gorm.DB, value interface{}, name string) {
	if db == nil || !db.Migrator().HasColumn(value, name) {
		return
	}
	if err := db.Migrator().DropColumn(value, name); err != nil {
		log.Printf("drop legacy column %s failed: %v", name, err)
	}
}

func seedDocs(db *gorm.DB) {
	docs := []model.DocPage{
		{
			Title:       "官方 API Base URL",
			Slug:        "api-base-url",
			GroupName:   "快速开始",
			Description: "根据客户端或工具要求选择 API Base URL，并查看可用模型 ID。",
			SortOrder:   10,
			Enabled:     true,
			Content: `# 官方 API Base URL

根据客户端或工具的兼容要求，选择对应的 API Base URL 填入配置文件或配置向导。

## 推荐填写

如果工具要求填写 OpenAI 兼容地址，优先使用：

` + "```text" + `
https://你的域名/v1
` + "```" + `

例如你的网关部署在 ` + "`https://ai.itzkb.cn`" + `，Codex 中通常填写：

` + "```text" + `
https://ai.itzkb.cn/v1
` + "```" + `

## 可用模型

| 模型 | 模型 ID | 推理强度 |
| --- | --- | --- |
| GPT-5.2 | ` + "`gpt-5.2`" + ` | low / medium / high / xhigh |
| GPT-5.2 Pro | ` + "`gpt-5.2-pro`" + ` | low / medium / high / xhigh |
| GPT-5.3 Codex | ` + "`gpt-5.3-codex`" + ` | low / medium / high / xhigh |
| GPT-5.3 Codex Spark | ` + "`gpt-5.3-codex-spark`" + ` | low / medium / high / xhigh |
| GPT-5.4 | ` + "`gpt-5.4`" + ` | low / medium / high / xhigh |
| GPT-5.4 Mini | ` + "`gpt-5.4-mini`" + ` | low / medium / high / xhigh |
| GPT-5.5 | ` + "`gpt-5.5`" + ` | low / medium / high / xhigh |
`,
		},
		{
			Title:       "如何获取 APIKey",
			Slug:        "get-api-key",
			GroupName:   "快速开始",
			Description: "在用户控制台创建或复制平台 API Key。",
			SortOrder:   20,
			Enabled:     true,
			Content: `# 如何获取 APIKey

1. 登录当前站点账号。
2. 进入控制台。
3. 在 API 密钥管理区域点击「创建 Key」。
4. 创建后复制完整密钥，并妥善保存。

调用网关时使用平台生成的 API Key，不需要填写后台绑定的上游 API Key。

` + "```http" + `
Authorization: Bearer ag_xxx
` + "```" + `
`,
		},
		{
			Title:       "如何获取配置",
			Slug:        "get-config",
			GroupName:   "快速开始",
			Description: "整理客户端需要的 Base URL、API Key 和模型 ID。",
			SortOrder:   30,
			Enabled:     true,
			Content: `# 如何获取配置

你需要准备三项配置：

| 配置项 | 填写内容 |
| --- | --- |
| API Base URL | ` + "`https://你的域名/v1`" + ` |
| API Key | 用户控制台生成的 ` + "`ag_xxx`" + ` |
| Model | 文档中列出的模型 ID |

示例：

` + "```text" + `
API Base URL: https://ai.itzkb.cn/v1
API Key: ag_xxx
Model: gpt-5.3-codex
` + "```" + `
`,
		},
		{
			Title:       "配置 Codex CLI / Claude Code CLI",
			Slug:        "codex-claude-cli",
			GroupName:   "客户端配置",
			Description: "在 Codex 或 Claude Code 兼容配置中使用当前网关。",
			SortOrder:   40,
			Enabled:     true,
			Content: `# 配置 Codex CLI / Claude Code CLI

Codex 中使用 OpenAI 兼容配置时，填写当前站点的 API Base URL 和平台 API Key。

` + "```toml" + `
base_url = "https://ai.itzkb.cn/v1"
api_key = "ag_xxx"
model = "gpt-5.3-codex"
reasoning_effort = "medium"
` + "```" + `

如果工具会自动拼接 /v1，则 Base URL 可填写裸域名：

` + "```text" + `
https://ai.itzkb.cn
` + "```" + `

无法确定时，优先使用 ` + "`https://ai.itzkb.cn/v1`" + `。
`,
		},
		{
			Title:       "如何配置 VSCode",
			Slug:        "vscode",
			GroupName:   "客户端配置",
			Description: "在 VSCode 插件中配置 OpenAI 兼容接口。",
			SortOrder:   50,
			Enabled:     true,
			Content: `# 如何配置 VSCode

在支持 OpenAI Compatible Provider 的 VSCode 插件中选择自定义接口。

| 配置项 | 示例 |
| --- | --- |
| Provider | OpenAI Compatible |
| Base URL | ` + "`https://ai.itzkb.cn/v1`" + ` |
| API Key | ` + "`ag_xxx`" + ` |
| Model | ` + "`gpt-5.3-codex`" + ` |

保存后发送一条简单消息测试连通性。
`,
		},
		{
			Title:       "如何配置 OpenClaw【小龙虾】",
			Slug:        "openclaw",
			GroupName:   "客户端配置",
			Description: "OpenClaw 工具的 OpenAI 兼容配置说明。",
			SortOrder:   60,
			Enabled:     true,
			Content: `# 如何配置 OpenClaw【小龙虾】

1. 打开 OpenClaw 设置。
2. Provider 选择 OpenAI Compatible 或自定义 OpenAI 接口。
3. Base URL 填写 ` + "`https://ai.itzkb.cn/v1`" + `。
4. API Key 填写用户控制台生成的密钥。
5. 模型填写 ` + "`gpt-5.3-codex`" + ` 或其它可用模型 ID。

配置完成后建议先发送 ` + "`hello`" + ` 进行测试。
`,
		},
	}

	for _, doc := range docs {
		var existing model.DocPage
		if err := db.Where("slug = ?", doc.Slug).First(&existing).Error; err == nil {
			continue
		}
		db.Create(&doc)
	}
}
