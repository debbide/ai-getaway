package controller

import (
	"ai-gateway/model"
	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SettingsController struct {
	db *gorm.DB
}

func NewSettingsController(db *gorm.DB) *SettingsController {
	return &SettingsController{db: db}
}

type updateSettingsRequest struct {
	SiteTitle        string `json:"site_title"`
	TutorialVideoURL string `json:"tutorial_video_url"`
	NavigationItems  string `json:"navigation_items"`
	PricingTitle     string `json:"pricing_title"`
	PricingSubtitle  string `json:"pricing_subtitle"`
	PricingNotice    string `json:"pricing_notice"`
	SMTPHost         string `json:"smtp_host"`
	SMTPPort         int    `json:"smtp_port"`
	SMTPUsername     string `json:"smtp_username"`
	SMTPPassword     string `json:"smtp_password"`
	SMTPFromEmail    string `json:"smtp_from_email"`
	SMTPFromName     string `json:"smtp_from_name"`
	SMTPUseTLS       bool   `json:"smtp_use_tls"`
	EpayPID          string `json:"epay_pid"`
	EpayKey          string `json:"epay_key"`
	EpayNotifyURL    string `json:"epay_notify_url"`
	EpayReturnURL    string `json:"epay_return_url"`
	EpaySubmitURL    string `json:"epay_submit_url"`
}

func (s *SettingsController) Public(c *gin.Context) {
	setting := loadSettings(s.db)
	response.OK(c, gin.H{
		"site_title":         setting.SiteTitle,
		"tutorial_video_url": setting.TutorialVideoURL,
		"navigation_items":   setting.NavigationItems,
		"pricing_title":      setting.PricingTitle,
		"pricing_subtitle":   setting.PricingSubtitle,
		"pricing_notice":     setting.PricingNotice,
	})
}

func (s *SettingsController) Get(c *gin.Context) {
	if err := ensureSystemSettingColumns(s.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(s.db)
	response.OK(c, gin.H{
		"id":                       setting.ID,
		"site_title":               setting.SiteTitle,
		"tutorial_video_url":       setting.TutorialVideoURL,
		"navigation_items":         setting.NavigationItems,
		"pricing_title":            setting.PricingTitle,
		"pricing_subtitle":         setting.PricingSubtitle,
		"pricing_notice":           setting.PricingNotice,
		"smtp_host":                setting.SMTPHost,
		"smtp_port":                setting.SMTPPort,
		"smtp_username":            setting.SMTPUsername,
		"smtp_from_email":          setting.SMTPFromEmail,
		"smtp_from_name":           setting.SMTPFromName,
		"smtp_use_tls":             setting.SMTPUseTLS,
		"smtp_password_configured": setting.SMTPPassword != "",
		"epay_pid":                 setting.EpayPID,
		"epay_notify_url":          setting.EpayNotifyURL,
		"epay_return_url":          setting.EpayReturnURL,
		"epay_submit_url":          setting.EpaySubmitURL,
		"epay_key_configured":      setting.EpayKey != "",
	})
}

func (s *SettingsController) Update(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if err := ensureSystemSettingColumns(s.db); err != nil {
		response.Error(c, 500, "failed to update settings")
		return
	}

	setting := loadSettings(s.db)
	updates := map[string]interface{}{
		"site_title":         req.SiteTitle,
		"tutorial_video_url": req.TutorialVideoURL,
		"navigation_items":   req.NavigationItems,
		"pricing_title":      req.PricingTitle,
		"pricing_subtitle":   req.PricingSubtitle,
		"pricing_notice":     req.PricingNotice,
		"smtp_host":          req.SMTPHost,
		"smtp_port":          req.SMTPPort,
		"smtp_username":      req.SMTPUsername,
		"smtp_from_email":    req.SMTPFromEmail,
		"smtp_from_name":     req.SMTPFromName,
		"smtp_use_tls":       req.SMTPUseTLS,
		"epay_pid":           req.EpayPID,
		"epay_submit_url":    req.EpaySubmitURL,
	}
	if req.SMTPPassword != "" {
		updates["smtp_password"] = req.SMTPPassword
	}
	if req.EpayKey != "" {
		updates["epay_key"] = req.EpayKey
	}
	if err := s.db.Model(&setting).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update settings")
		return
	}
	response.OK(c, nil)
}

func ensureSystemSettingColumns(db *gorm.DB) error {
	if err := cleanupLegacySystemSettingColumns(db); err != nil {
		return err
	}
	columns := map[string]string{
		"navigation_items": "TEXT",
		"pricing_title":    "VARCHAR(128)",
		"pricing_subtitle": "VARCHAR(255)",
		"pricing_notice":   "VARCHAR(512)",
		"epay_pid":         "VARCHAR(128)",
		"epay_key":         "VARCHAR(255)",
		"epay_notify_url":  "VARCHAR(512)",
		"epay_return_url":  "VARCHAR(512)",
		"epay_submit_url":  "VARCHAR(512)",
	}
	for column, definition := range columns {
		if systemSettingColumnExists(db, column) {
			continue
		}
		if err := db.Exec("ALTER TABLE `system_settings` ADD COLUMN `" + column + "` " + definition).Error; err != nil {
			return err
		}
	}
	return nil
}

func cleanupLegacySystemSettingColumns(db *gorm.DB) error {
	if !systemSettingColumnExists(db, "epay_p_id") {
		return nil
	}
	if !systemSettingColumnExists(db, "epay_pid") {
		if err := db.Exec("ALTER TABLE `system_settings` ADD COLUMN `epay_pid` VARCHAR(128)").Error; err != nil {
			return err
		}
	}
	if err := db.Exec("UPDATE `system_settings` SET `epay_pid` = `epay_p_id` WHERE (`epay_pid` IS NULL OR `epay_pid` = '') AND `epay_p_id` IS NOT NULL AND `epay_p_id` <> ''").Error; err != nil {
		return err
	}
	if err := db.Exec("ALTER TABLE `system_settings` DROP COLUMN `epay_p_id`").Error; err != nil {
		return err
	}
	return nil
}

func systemSettingColumnExists(db *gorm.DB, column string) bool {
	var count int64
	db.Raw(
		"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?",
		"system_settings",
		column,
	).Scan(&count)
	return count > 0
}

func loadSettings(db *gorm.DB) model.SystemSetting {
	var setting model.SystemSetting
	if err := db.First(&setting, 1).Error; err != nil {
		setting = model.SystemSetting{Model: gorm.Model{ID: 1}, SiteTitle: "AI Gateway", SMTPPort: 587, SMTPUseTLS: true}
		db.FirstOrCreate(&setting, model.SystemSetting{Model: gorm.Model{ID: 1}})
	}
	if setting.SiteTitle == "" {
		setting.SiteTitle = "CodexZH"
	}
	if setting.NavigationItems == "" {
		setting.NavigationItems = `[{"label":"首页","path":"/"},{"label":"教程 ↗","path":"#tutorial","external":true},{"label":"定价","path":"/plans"},{"label":"模型","path":"/models"},{"label":"常见问题","path":"/faq"},{"label":"更多中转⌄","path":"#","children":[{"label":"Claude Code 中转","path":"/claude"}]}]`
	}
	if setting.PricingTitle == "" {
		setting.PricingTitle = "简单透明的定价"
	}
	if setting.PricingSubtitle == "" {
		setting.PricingSubtitle = "保质保量无降智不掺假"
	}
	if setting.PricingNotice == "" {
		setting.PricingNotice = "本站仅支持 GPT 模型使用，具体型号请查看 /models 页面；如需使用 Claude 模型，请前往顶部菜单更多中转 → Claude Code 中转"
	}
	if setting.SMTPPort == 0 {
		setting.SMTPPort = 587
	}
	return setting
}
