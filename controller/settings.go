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
	if err := s.ensureSettingColumns(); err != nil {
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
	updates = s.existingSettingColumns(updates)
	if err := s.db.Model(&setting).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update settings")
		return
	}
	response.OK(c, nil)
}

func (s *SettingsController) existingSettingColumns(updates map[string]interface{}) map[string]interface{} {
	filtered := map[string]interface{}{}
	for column, value := range updates {
		if s.db.Migrator().HasColumn(&model.SystemSetting{}, column) {
			filtered[column] = value
		}
	}
	return filtered
}

func (s *SettingsController) ensureSettingColumns() error {
	fields := []string{
		"NavigationItems",
		"PricingTitle",
		"PricingSubtitle",
		"PricingNotice",
		"EpayPID",
		"EpayKey",
		"EpayNotifyURL",
		"EpayReturnURL",
		"EpaySubmitURL",
	}
	for _, field := range fields {
		if s.db.Migrator().HasColumn(&model.SystemSetting{}, field) {
			continue
		}
		if err := s.db.Migrator().AddColumn(&model.SystemSetting{}, field); err != nil {
			return err
		}
	}
	return nil
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
