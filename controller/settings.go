package controller

import (
	"encoding/json"
	"net/mail"
	"strings"

	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/service"

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
	SiteTitle                      string  `json:"site_title"`
	ContactEmail                   string  `json:"contact_email"`
	APIEndpoints                   string  `json:"api_endpoints"`
	NavigationItems                string  `json:"navigation_items"`
	PricingTitle                   string  `json:"pricing_title"`
	PricingSubtitle                string  `json:"pricing_subtitle"`
	PricingNotice                  string  `json:"pricing_notice"`
	AllowRegistration              bool    `json:"allow_registration"`
	EmailWhitelist                 string  `json:"email_whitelist"`
	SMTPHost                       string  `json:"smtp_host"`
	SMTPPort                       int     `json:"smtp_port"`
	SMTPUsername                   string  `json:"smtp_username"`
	SMTPPassword                   string  `json:"smtp_password"`
	SMTPFromEmail                  string  `json:"smtp_from_email"`
	SMTPFromName                   string  `json:"smtp_from_name"`
	SMTPUseTLS                     bool    `json:"smtp_use_tls"`
	OrderPaymentAdminEmailEnabled  bool    `json:"order_payment_admin_email_enabled"`
	OrderApprovedUserEmailEnabled  bool    `json:"order_approved_user_email_enabled"`
	SubscriptionExpireEmailEnabled bool    `json:"subscription_expire_email_enabled"`
	SubscriptionExpireRemindDays   int     `json:"subscription_expire_remind_days"`
	EpayPID                        string  `json:"epay_pid"`
	EpayKey                        string  `json:"epay_key"`
	EpayNotifyURL                  string  `json:"epay_notify_url"`
	EpayReturnURL                  string  `json:"epay_return_url"`
	EpaySubmitURL                  string  `json:"epay_submit_url"`
	OnlinePaymentEnabled           bool    `json:"online_payment_enabled"`
	ManualPaymentEnabled           bool    `json:"manual_payment_enabled"`
	ManualPaymentQRCode            string  `json:"manual_payment_qr_code"`
	BalanceRechargeRateRMBPerUSD   float64 `json:"balance_recharge_rate_rmb_per_usd"`
	MockAPIOnlineEnabled           bool    `json:"mock_api_online_enabled"`
	MockAPIOnlineBase              int     `json:"mock_api_online_base"`
	GitHubOAuthEnabled             bool    `json:"github_oauth_enabled"`
	GitHubOAuthClientID            string  `json:"github_oauth_client_id"`
	GitHubOAuthClientSecret        string  `json:"github_oauth_client_secret"`
	GoogleOAuthEnabled             bool    `json:"google_oauth_enabled"`
	GoogleOAuthClientID            string  `json:"google_oauth_client_id"`
	GoogleOAuthClientSecret        string  `json:"google_oauth_client_secret"`
}

type testSMTPRequest struct {
	updateSettingsRequest
	ToEmail string `json:"to_email" binding:"required,email"`
}

type apiEndpointSetting struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

func (s *SettingsController) Public(c *gin.Context) {
	if err := ensureSystemSettingColumns(s.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(s.db)
	response.OK(c, gin.H{
		"site_title":                        setting.SiteTitle,
		"contact_email":                     setting.ContactEmail,
		"api_endpoints":                     setting.APIEndpoints,
		"navigation_items":                  setting.NavigationItems,
		"pricing_title":                     setting.PricingTitle,
		"pricing_subtitle":                  setting.PricingSubtitle,
		"pricing_notice":                    setting.PricingNotice,
		"allow_registration":                setting.AllowRegistration,
		"email_whitelist":                   setting.EmailWhitelist,
		"online_payment_enabled":            setting.OnlinePaymentEnabled,
		"manual_payment_enabled":            setting.ManualPaymentEnabled,
		"balance_recharge_rate_rmb_per_usd": normalizeBalanceRechargeRate(setting.BalanceRechargeRateRMBPerUSD),
		"mock_api_online_enabled":           setting.MockAPIOnlineEnabled,
		"mock_api_online_base":              normalizeMockAPIOnlineBase(setting.MockAPIOnlineBase),
		"oauth_providers":                   publicOAuthProviders(setting),
	})
}

func (s *SettingsController) ManualPayment(c *gin.Context) {
	if err := ensureSystemSettingColumns(s.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(s.db)
	if !setting.ManualPaymentEnabled {
		response.Error(c, 400, "manual payment disabled")
		return
	}
	response.OK(c, gin.H{
		"manual_payment_qr_code": setting.ManualPaymentQRCode,
		"manual_payment_enabled": setting.ManualPaymentEnabled,
	})
}

func (s *SettingsController) Get(c *gin.Context) {
	if err := ensureSystemSettingColumns(s.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(s.db)
	response.OK(c, gin.H{
		"id":                                    setting.ID,
		"site_title":                            setting.SiteTitle,
		"contact_email":                         setting.ContactEmail,
		"api_endpoints":                         setting.APIEndpoints,
		"navigation_items":                      setting.NavigationItems,
		"pricing_title":                         setting.PricingTitle,
		"pricing_subtitle":                      setting.PricingSubtitle,
		"pricing_notice":                        setting.PricingNotice,
		"allow_registration":                    setting.AllowRegistration,
		"email_whitelist":                       setting.EmailWhitelist,
		"smtp_host":                             setting.SMTPHost,
		"smtp_port":                             setting.SMTPPort,
		"smtp_username":                         setting.SMTPUsername,
		"smtp_from_email":                       setting.SMTPFromEmail,
		"smtp_from_name":                        setting.SMTPFromName,
		"smtp_use_tls":                          setting.SMTPUseTLS,
		"order_payment_admin_email_enabled":     setting.OrderPaymentAdminEmailEnabled,
		"order_approved_user_email_enabled":     setting.OrderApprovedUserEmailEnabled,
		"subscription_expire_email_enabled":     setting.SubscriptionExpireEmailEnabled,
		"subscription_expire_remind_days":       setting.SubscriptionExpireRemindDays,
		"smtp_password_configured":              setting.SMTPPassword != "",
		"epay_pid":                              setting.EpayPID,
		"epay_notify_url":                       setting.EpayNotifyURL,
		"epay_return_url":                       setting.EpayReturnURL,
		"epay_submit_url":                       setting.EpaySubmitURL,
		"online_payment_enabled":                setting.OnlinePaymentEnabled,
		"manual_payment_enabled":                setting.ManualPaymentEnabled,
		"epay_key_configured":                   setting.EpayKey != "",
		"manual_payment_qr_code":                setting.ManualPaymentQRCode,
		"balance_recharge_rate_rmb_per_usd":     normalizeBalanceRechargeRate(setting.BalanceRechargeRateRMBPerUSD),
		"mock_api_online_enabled":               setting.MockAPIOnlineEnabled,
		"mock_api_online_base":                  normalizeMockAPIOnlineBase(setting.MockAPIOnlineBase),
		"github_oauth_enabled":                  setting.GitHubOAuthEnabled,
		"github_oauth_client_id":                setting.GitHubOAuthClientID,
		"github_oauth_client_secret_configured": setting.GitHubOAuthClientSecret != "",
		"google_oauth_enabled":                  setting.GoogleOAuthEnabled,
		"google_oauth_client_id":                setting.GoogleOAuthClientID,
		"google_oauth_client_secret_configured": setting.GoogleOAuthClientSecret != "",
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
		"site_title":                        req.SiteTitle,
		"contact_email":                     req.ContactEmail,
		"api_endpoints":                     normalizeAPIEndpointsJSON(req.APIEndpoints),
		"navigation_items":                  req.NavigationItems,
		"pricing_title":                     req.PricingTitle,
		"pricing_subtitle":                  req.PricingSubtitle,
		"pricing_notice":                    req.PricingNotice,
		"allow_registration":                req.AllowRegistration,
		"email_whitelist":                   normalizeEmailWhitelistJSON(req.EmailWhitelist),
		"smtp_host":                         req.SMTPHost,
		"smtp_port":                         req.SMTPPort,
		"smtp_username":                     req.SMTPUsername,
		"smtp_from_email":                   req.SMTPFromEmail,
		"smtp_from_name":                    req.SMTPFromName,
		"smtp_use_tls":                      req.SMTPUseTLS,
		"order_payment_admin_email_enabled": req.OrderPaymentAdminEmailEnabled,
		"order_approved_user_email_enabled": req.OrderApprovedUserEmailEnabled,
		"subscription_expire_email_enabled": req.SubscriptionExpireEmailEnabled,
		"subscription_expire_remind_days":   normalizeRemindDays(req.SubscriptionExpireRemindDays),
		"epay_pid":                          req.EpayPID,
		"epay_notify_url":                   req.EpayNotifyURL,
		"epay_return_url":                   req.EpayReturnURL,
		"epay_submit_url":                   req.EpaySubmitURL,
		"online_payment_enabled":            req.OnlinePaymentEnabled,
		"manual_payment_enabled":            req.ManualPaymentEnabled,
		"manual_payment_qr_code":            req.ManualPaymentQRCode,
		"balance_recharge_rate_rmb_per_usd": normalizeBalanceRechargeRate(req.BalanceRechargeRateRMBPerUSD),
		"mock_api_online_enabled":           req.MockAPIOnlineEnabled,
		"mock_api_online_base":              normalizeMockAPIOnlineBase(req.MockAPIOnlineBase),
		"github_oauth_enabled":              req.GitHubOAuthEnabled,
		"github_oauth_client_id":            strings.TrimSpace(req.GitHubOAuthClientID),
		"google_oauth_enabled":              req.GoogleOAuthEnabled,
		"google_oauth_client_id":            strings.TrimSpace(req.GoogleOAuthClientID),
	}
	if req.SMTPPassword != "" {
		updates["smtp_password"] = req.SMTPPassword
	}
	if req.EpayKey != "" {
		updates["epay_key"] = req.EpayKey
	}
	if strings.TrimSpace(req.GitHubOAuthClientSecret) != "" {
		updates["github_oauth_client_secret"] = strings.TrimSpace(req.GitHubOAuthClientSecret)
	}
	if strings.TrimSpace(req.GoogleOAuthClientSecret) != "" {
		updates["google_oauth_client_secret"] = strings.TrimSpace(req.GoogleOAuthClientSecret)
	}
	if err := s.db.Model(&setting).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update settings")
		return
	}
	response.OK(c, nil)
}

func (s *SettingsController) TestSMTP(c *gin.Context) {
	var req testSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if _, err := mail.ParseAddress(req.ToEmail); err != nil {
		response.Error(c, 400, "invalid test email")
		return
	}
	if err := ensureSystemSettingColumns(s.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}

	setting := loadSettings(s.db)
	setting.SiteTitle = req.SiteTitle
	setting.SMTPHost = strings.TrimSpace(req.SMTPHost)
	setting.SMTPPort = req.SMTPPort
	setting.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	setting.SMTPFromEmail = strings.TrimSpace(req.SMTPFromEmail)
	setting.SMTPFromName = strings.TrimSpace(req.SMTPFromName)
	setting.SMTPUseTLS = req.SMTPUseTLS
	if strings.TrimSpace(req.SMTPPassword) != "" {
		setting.SMTPPassword = req.SMTPPassword
	}
	if setting.SMTPPort == 0 {
		setting.SMTPPort = 587
	}

	if err := service.NewMailer(setting).SendSMTPTest(req.ToEmail); err != nil {
		response.Error(c, 500, "failed to send test email: "+err.Error())
		return
	}
	response.OK(c, gin.H{"sent": true})
}

func ensureSystemSettingColumns(db *gorm.DB) error {
	if err := cleanupLegacySystemSettingColumns(db); err != nil {
		return err
	}
	columns := map[string]string{
		"navigation_items":                  "TEXT",
		"api_endpoints":                     "TEXT",
		"contact_email":                     "VARCHAR(128)",
		"pricing_title":                     "VARCHAR(128)",
		"pricing_subtitle":                  "VARCHAR(255)",
		"pricing_notice":                    "VARCHAR(512)",
		"allow_registration":                "BOOLEAN DEFAULT TRUE",
		"email_whitelist":                   "TEXT",
		"epay_pid":                          "VARCHAR(128)",
		"epay_key":                          "VARCHAR(255)",
		"epay_notify_url":                   "VARCHAR(512)",
		"epay_return_url":                   "VARCHAR(512)",
		"epay_submit_url":                   "VARCHAR(512)",
		"online_payment_enabled":            "BOOLEAN DEFAULT TRUE",
		"manual_payment_enabled":            "BOOLEAN DEFAULT TRUE",
		"manual_payment_qr_code":            "LONGTEXT",
		"balance_recharge_rate_rmb_per_usd": "DOUBLE DEFAULT 0.7",
		"mock_api_online_enabled":           "BOOLEAN DEFAULT FALSE",
		"mock_api_online_base":              "INT DEFAULT 0",
		"github_oauth_enabled":              "BOOLEAN DEFAULT FALSE",
		"github_oauth_client_id":            "VARCHAR(191)",
		"github_oauth_client_secret":        "VARCHAR(255)",
		"google_oauth_enabled":              "BOOLEAN DEFAULT FALSE",
		"google_oauth_client_id":            "VARCHAR(191)",
		"google_oauth_client_secret":        "VARCHAR(255)",
		"order_payment_admin_email_enabled": "BOOLEAN DEFAULT FALSE",
		"order_approved_user_email_enabled": "BOOLEAN DEFAULT FALSE",
		"subscription_expire_email_enabled": "BOOLEAN DEFAULT FALSE",
		"subscription_expire_remind_days":   "INT DEFAULT 3",
	}
	for column, definition := range columns {
		if systemSettingColumnExists(db, column) {
			continue
		}
		if err := db.Exec("ALTER TABLE `system_settings` ADD COLUMN `" + column + "` " + definition).Error; err != nil {
			return err
		}
	}
	if systemSettingColumnExists(db, "api_endpoint") {
		if err := migrateLegacyAPIEndpoint(db); err != nil {
			return err
		}
		if err := db.Exec("ALTER TABLE `system_settings` DROP COLUMN `api_endpoint`").Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyAPIEndpoint(db *gorm.DB) error {
	var rows []struct {
		ID           uint
		APIEndpoint  string
		APIEndpoints string
	}
	if err := db.Raw("SELECT `id`, `api_endpoint`, `api_endpoints` FROM `system_settings`").Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if strings.TrimSpace(row.APIEndpoints) != "" || strings.TrimSpace(row.APIEndpoint) == "" {
			continue
		}
		endpoints := mustMarshalAPIEndpoints([]apiEndpointSetting{{
			Label:       "默认",
			Description: "主线路",
			URL:         row.APIEndpoint,
		}})
		if err := db.Exec("UPDATE `system_settings` SET `api_endpoints` = ? WHERE `id` = ?", endpoints, row.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

func cleanupLegacySystemSettingColumns(db *gorm.DB) error {
	if systemSettingColumnExists(db, "tutorial_video_url") {
		if err := db.Exec("ALTER TABLE `system_settings` DROP COLUMN `tutorial_video_url`").Error; err != nil {
			return err
		}
	}
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
		setting = model.SystemSetting{Model: gorm.Model{ID: 1}, SiteTitle: "星空 AI", AllowRegistration: true, SMTPPort: 587, SMTPUseTLS: true}
		db.FirstOrCreate(&setting, model.SystemSetting{Model: gorm.Model{ID: 1}})
	}
	if setting.SiteTitle == "" {
		setting.SiteTitle = "星空AI"
	}
	if setting.ContactEmail == "" {
		setting.ContactEmail = "support@example.com"
	}
	if strings.TrimSpace(setting.APIEndpoints) == "" {
		setting.APIEndpoints = defaultAPIEndpointsJSON()
	}
	if setting.NavigationItems == "" {
		setting.NavigationItems = `[{"label":"首页","path":"/"},{"label":"教程 ↗","path":"/docs"},{"label":"定价","path":"/plans"},{"label":"模型","path":"/models"},{"label":"常见问题","path":"/faq"},{"label":"更多中转↱","path":"#","children":[{"label":"Claude Code 中转","path":"/claude"}]}]`
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
	if setting.SubscriptionExpireRemindDays <= 0 {
		setting.SubscriptionExpireRemindDays = 3
	}
	setting.EmailWhitelist = normalizeEmailWhitelistJSON(setting.EmailWhitelist)
	setting.BalanceRechargeRateRMBPerUSD = normalizeBalanceRechargeRate(setting.BalanceRechargeRateRMBPerUSD)
	setting.MockAPIOnlineBase = normalizeMockAPIOnlineBase(setting.MockAPIOnlineBase)
	return setting
}

func publicOAuthProviders(setting model.SystemSetting) []gin.H {
	providers := []gin.H{}
	if setting.GitHubOAuthEnabled {
		providers = append(providers, gin.H{"provider": model.OAuthProviderGitHub, "label": "GitHub"})
	}
	if setting.GoogleOAuthEnabled {
		providers = append(providers, gin.H{"provider": model.OAuthProviderGoogle, "label": "Google"})
	}
	return providers
}

func normalizeRemindDays(value int) int {
	if value < 1 {
		return 1
	}
	if value > 365 {
		return 365
	}
	return value
}

func normalizeMockAPIOnlineBase(value int) int {
	if value < 0 {
		return 0
	}
	if value > 1000000 {
		return 1000000
	}
	return value
}

func normalizeBalanceRechargeRate(value float64) float64 {
	if value <= 0 {
		return 0.7
	}
	return value
}

func defaultAPIEndpointsJSON() string {
	return mustMarshalAPIEndpoints([]apiEndpointSetting{{
		Label:       "默认",
		Description: "主线路",
		URL:         "https://ai.itzkb.cn",
	}})
}

func normalizeAPIEndpointsJSON(value string) string {
	var endpoints []apiEndpointSetting
	if err := json.Unmarshal([]byte(value), &endpoints); err != nil {
		return defaultAPIEndpointsJSON()
	}
	normalized := make([]apiEndpointSetting, 0, len(endpoints))
	for _, endpoint := range endpoints {
		item := apiEndpointSetting{
			Label:       strings.TrimSpace(endpoint.Label),
			Description: strings.TrimSpace(endpoint.Description),
			URL:         strings.TrimSpace(endpoint.URL),
		}
		if item.URL == "" {
			continue
		}
		if item.Label == "" {
			item.Label = "API"
		}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return defaultAPIEndpointsJSON()
	}
	return mustMarshalAPIEndpoints(normalized)
}

func normalizeEmailWhitelistJSON(value string) string {
	domains := parseEmailWhitelist(value)
	if len(domains) == 0 {
		return "[]"
	}
	body, err := json.Marshal(domains)
	if err != nil {
		return "[]"
	}
	return string(body)
}

func parseEmailWhitelist(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var rawItems []string
	if err := json.Unmarshal([]byte(value), &rawItems); err != nil {
		rawItems = strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '\n' || r == ';' || r == ' '
		})
	}
	seen := map[string]bool{}
	domains := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		domain := normalizeEmailDomain(item)
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		domains = append(domains, domain)
	}
	return domains
}

func normalizeEmailDomain(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "@")
	if value == "" || strings.Contains(value, "@") || strings.ContainsAny(value, "/\\") || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || !strings.Contains(value, ".") {
		return ""
	}
	return value
}

func emailAllowedByWhitelist(email string, whitelistJSON string) bool {
	domains := parseEmailWhitelist(whitelistJSON)
	if len(domains) == 0 {
		return true
	}
	_, domain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@")
	if !ok {
		return false
	}
	domain = normalizeEmailDomain(domain)
	if domain == "" {
		return false
	}
	for _, allowed := range domains {
		if domain == allowed {
			return true
		}
	}
	return false
}

func mustMarshalAPIEndpoints(endpoints []apiEndpointSetting) string {
	body, err := json.Marshal(endpoints)
	if err != nil {
		return "[]"
	}
	return string(body)
}
