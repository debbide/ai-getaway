package database

import (
	"context"
	"log"
	"strconv"
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
	backfillOrderPaymentRefs(db)
	prepareAccessSourceMigration(db)
	if err := db.AutoMigrate(
		&model.User{},
		&model.OAuthAccount{},
		&model.Plan{},
		&model.Order{},
		&model.RedeemCode{},
		&model.UpstreamAccount{},
		&model.UpstreamChannel{},
		&model.PublicChannel{},
		&model.PollingPool{},
		&model.PollingPoolAccount{},
		&model.ChannelMonitor{},
		&model.ChannelMonitorRecord{},
		&model.DocPage{},
		&model.Announcement{},
		&model.APIKey{},
		&model.ModelPricing{},
		&model.APILog{},
		&model.QuotaReservation{},
		&model.SystemSetting{},
		&model.EmailTemplate{},
		&model.EmailNotificationLog{},
		&model.EmailVerification{},
	); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
	dropLegacyQuotaColumns(db)
	ensureBalanceColumns(db)
	ensureAccessSourceColumns(db)
	backfillOrderPaymentRefs(db)
	backfillOrderNumbers(db)
}

func prepareAccessSourceMigration(db *gorm.DB) {
	if db == nil || !db.Migrator().HasTable(&model.UpstreamAccount{}) {
		return
	}
	if db.Migrator().HasIndex(&model.UpstreamAccount{}, "uni_upstream_accounts_user_id") {
		if err := db.Migrator().DropIndex(&model.UpstreamAccount{}, "uni_upstream_accounts_user_id"); err != nil {
			log.Printf("drop legacy upstream user unique index before migrate failed: %v", err)
		}
	}
}

func ensureBalanceColumns(db *gorm.DB) {
	if db == nil {
		return
	}
	if db.Migrator().HasTable(&model.User{}) && !db.Migrator().HasColumn(&model.User{}, "balance_usd_cents") {
		if err := db.Exec("ALTER TABLE `users` ADD COLUMN `balance_usd_cents` BIGINT DEFAULT 0").Error; err != nil {
			log.Printf("add balance_usd_cents column failed: %v", err)
		}
	}
}

func ensureAccessSourceColumns(db *gorm.DB) {
	if db == nil {
		return
	}
	if db.Migrator().HasTable(&model.UpstreamAccount{}) {
		if !db.Migrator().HasColumn(&model.UpstreamAccount{}, "access_type") {
			if err := db.Exec("ALTER TABLE `upstream_accounts` ADD COLUMN `access_type` VARCHAR(32) DEFAULT 'plan'").Error; err != nil {
				log.Printf("add upstream access_type column failed: %v", err)
			}
		}
		if err := db.Exec("UPDATE `upstream_accounts` SET `access_type` = 'plan' WHERE `access_type` IS NULL OR `access_type` = ''").Error; err != nil {
			log.Printf("backfill upstream access_type failed: %v", err)
		}
		if db.Migrator().HasIndex(&model.UpstreamAccount{}, "uni_upstream_accounts_user_id") {
			if err := db.Migrator().DropIndex(&model.UpstreamAccount{}, "uni_upstream_accounts_user_id"); err != nil {
				log.Printf("drop legacy upstream user unique index failed: %v", err)
			}
		}
		if !db.Migrator().HasIndex(&model.UpstreamAccount{}, "idx_upstream_user_access") {
			if err := db.Migrator().CreateIndex(&model.UpstreamAccount{}, "idx_upstream_user_access"); err != nil {
				log.Printf("create upstream user access index failed: %v", err)
			}
		}
	}
	if db.Migrator().HasTable(&model.QuotaReservation{}) && !db.Migrator().HasColumn(&model.QuotaReservation{}, "access_source") {
		if err := db.Exec("ALTER TABLE `quota_reservations` ADD COLUMN `access_source` VARCHAR(32) DEFAULT 'plan'").Error; err != nil {
			log.Printf("add quota reservation access_source column failed: %v", err)
		}
	}
	if db.Migrator().HasTable(&model.APILog{}) && !db.Migrator().HasColumn(&model.APILog{}, "access_source") {
		if err := db.Exec("ALTER TABLE `api_logs` ADD COLUMN `access_source` VARCHAR(32) DEFAULT 'plan'").Error; err != nil {
			log.Printf("add api log access_source column failed: %v", err)
		}
	}
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

	db.FirstOrCreate(&model.SystemSetting{}, model.SystemSetting{Model: gorm.Model{ID: 1}, AllowRegistration: true})

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

func StartSlideCaptchaCleanup(cfg config.Config, db *gorm.DB, redisClient *redis.Client) {
	if db == nil {
		return
	}
	if !cfg.RunBackgroundJobs {
		log.Printf("slide captcha cleanup disabled by RUN_BACKGROUND_JOBS=false")
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

	runBackgroundJob(cfg, redisClient, "slide-captcha-cleanup", 10*time.Minute, cleanup)
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			runBackgroundJob(cfg, redisClient, "slide-captcha-cleanup", 10*time.Minute, cleanup)
		}
	}()
}

func StartOrderTimeoutCleanup(cfg config.Config, db *gorm.DB, redisClient *redis.Client) {
	if db == nil {
		return
	}
	if !cfg.RunBackgroundJobs {
		log.Printf("order timeout cleanup disabled by RUN_BACKGROUND_JOBS=false")
		return
	}
	cleanup := func() {
		if !db.Migrator().HasTable(&model.Order{}) {
			return
		}
		if err := db.Model(&model.Order{}).
			Where(
				"status = ? AND ((payment_method = ? AND created_at <= ?) OR ((payment_method IS NULL OR payment_method <> ?) AND created_at <= ?))",
				model.OrderStatusPendingPayment,
				model.PaymentMethodManual,
				time.Now().Add(-2*time.Hour),
				model.PaymentMethodManual,
				time.Now().Add(-5*time.Minute),
			).
			Update("status", model.OrderStatusPaymentTimeout).Error; err != nil {
			log.Printf("order timeout cleanup failed: %v", err)
		}
	}

	runBackgroundJob(cfg, redisClient, "order-timeout-cleanup", 2*time.Minute, cleanup)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			runBackgroundJob(cfg, redisClient, "order-timeout-cleanup", 2*time.Minute, cleanup)
		}
	}()
}

func StartSubscriptionExpireEmailReminder(cfg config.Config, db *gorm.DB, redisClient *redis.Client) {
	if db == nil {
		return
	}
	if !cfg.RunBackgroundJobs {
		log.Printf("subscription expire email reminder disabled by RUN_BACKGROUND_JOBS=false")
		return
	}
	remind := func() {
		if !db.Migrator().HasTable(&model.EmailTemplate{}) || !db.Migrator().HasTable(&model.EmailNotificationLog{}) {
			return
		}
		service.SendSubscriptionExpireReminders(db)
	}

	go func() {
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			runBackgroundJob(cfg, redisClient, "subscription-expire-email-reminder", time.Hour, remind)
		}
	}()
}

func runBackgroundJob(cfg config.Config, redisClient *redis.Client, name string, ttl time.Duration, fn func()) {
	service.RunWithClusterLock(redisClient, cfg.ClusterMode, name, cfg.InstanceID, ttl, fn)
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

func backfillOrderPaymentRefs(db *gorm.DB) {
	if db == nil || !db.Migrator().HasTable(&model.Order{}) || !db.Migrator().HasColumn(&model.Order{}, "payment_ref") {
		return
	}
	var orders []model.Order
	if err := db.Unscoped().Where("payment_ref IS NULL OR payment_ref = ''").Find(&orders).Error; err != nil {
		log.Printf("load empty order payment refs failed: %v", err)
		return
	}
	for _, order := range orders {
		ref := "ORDERLEGACY" + strconv.FormatUint(uint64(order.ID), 10)
		if err := db.Unscoped().Model(&model.Order{}).Where("id = ?", order.ID).Update("payment_ref", ref).Error; err != nil {
			log.Printf("backfill order payment ref failed: %v", err)
		}
	}
}

func backfillOrderNumbers(db *gorm.DB) {
	if db == nil || !db.Migrator().HasTable(&model.Order{}) || !db.Migrator().HasColumn(&model.Order{}, "order_no") {
		return
	}
	var orders []model.Order
	if err := db.Unscoped().Where("order_no IS NULL OR order_no = ''").Find(&orders).Error; err != nil {
		log.Printf("load empty order numbers failed: %v", err)
		return
	}
	for _, order := range orders {
		createdAt := order.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		orderNo := model.GenerateOrderNo(order.UserID, createdAt) + strconv.FormatUint(uint64(order.ID%1000000), 10)
		if err := db.Unscoped().Model(&model.Order{}).Where("id = ?", order.ID).Update("order_no", orderNo).Error; err != nil {
			log.Printf("backfill order number failed: %v", err)
		}
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
