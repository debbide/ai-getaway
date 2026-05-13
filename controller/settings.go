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
	})
}

func (s *SettingsController) Get(c *gin.Context) {
	setting := loadSettings(s.db)
	response.OK(c, gin.H{
		"id":                       setting.ID,
		"site_title":               setting.SiteTitle,
		"tutorial_video_url":       setting.TutorialVideoURL,
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

	setting := loadSettings(s.db)
	updates := map[string]interface{}{
		"site_title":         req.SiteTitle,
		"tutorial_video_url": req.TutorialVideoURL,
		"smtp_host":          req.SMTPHost,
		"smtp_port":          req.SMTPPort,
		"smtp_username":      req.SMTPUsername,
		"smtp_from_email":    req.SMTPFromEmail,
		"smtp_from_name":     req.SMTPFromName,
		"smtp_use_tls":       req.SMTPUseTLS,
		"epay_pid":           req.EpayPID,
		"epay_notify_url":    req.EpayNotifyURL,
		"epay_return_url":    req.EpayReturnURL,
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

func loadSettings(db *gorm.DB) model.SystemSetting {
	var setting model.SystemSetting
	if err := db.First(&setting, 1).Error; err != nil {
		setting = model.SystemSetting{Model: gorm.Model{ID: 1}, SiteTitle: "AI Gateway", SMTPPort: 587, SMTPUseTLS: true}
		db.FirstOrCreate(&setting, model.SystemSetting{Model: gorm.Model{ID: 1}})
	}
	if setting.SiteTitle == "" {
		setting.SiteTitle = "AI Gateway"
	}
	if setting.SMTPPort == 0 {
		setting.SMTPPort = 587
	}
	return setting
}
