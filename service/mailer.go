package service

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"html/template"
	"log"
	"mime"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"ai-gateway/model"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const verificationTemplate = `
<!doctype html>
<html>
  <body style="margin:0;background:#f6faf7;font-family:Arial,'Microsoft YaHei',sans-serif;color:#173f35;">
    <div style="max-width:560px;margin:0 auto;padding:32px 16px;">
      <div style="background:#ffffff;border:1px solid #d7e5db;border-radius:8px;padding:28px;">
        <h1 style="margin:0 0 12px;font-size:24px;color:#173f35;">{{.SiteTitle}} 邮箱验证</h1>
        <p style="margin:0 0 20px;line-height:1.7;color:#60746a;">请使用下面的验证码完成账号注册。验证码 10 分钟内有效。</p>
        <div style="margin:24px 0;padding:18px;border-radius:8px;background:#eef8ef;text-align:center;font-size:32px;font-weight:700;letter-spacing:6px;color:#169b7b;">
          {{.Code}}
        </div>
        <p style="margin:20px 0 0;font-size:13px;line-height:1.7;color:#60746a;">如果不是你本人操作，可以忽略这封邮件。</p>
      </div>
    </div>
  </body>
</html>`

const smtpTestTemplate = `
<!doctype html>
<html>
  <body style="margin:0;background:#f6faf7;font-family:Arial,'Microsoft YaHei',sans-serif;color:#173f35;">
    <div style="max-width:560px;margin:0 auto;padding:32px 16px;">
      <div style="background:#ffffff;border:1px solid #d7e5db;border-radius:8px;padding:28px;">
        <h1 style="margin:0 0 12px;font-size:22px;color:#173f35;">{{.SiteTitle}} SMTP 测试邮件</h1>
        <p style="margin:0 0 18px;line-height:1.7;color:#60746a;">如果你收到这封邮件，说明当前 SMTP 配置可以正常发送邮件。</p>
        <div style="margin:18px 0;padding:14px;border-radius:8px;background:#eef8ef;color:#169b7b;font-weight:700;">
          测试时间：{{.SentAt}}
        </div>
        <p style="margin:20px 0 0;font-size:13px;line-height:1.7;color:#60746a;">这封邮件由管理后台的 SMTP 配置测试功能发送。</p>
      </div>
    </div>
  </body>
</html>`

type Mailer struct {
	settings model.SystemSetting
}

func NewMailer(settings model.SystemSetting) *Mailer {
	return &Mailer{settings: settings}
}

func (m *Mailer) SendVerification(email, code string) error {
	if m.settings.SMTPHost == "" || m.settings.SMTPFromEmail == "" {
		return fmt.Errorf("smtp is not configured")
	}

	var body bytes.Buffer
	tpl, err := template.New("verification").Parse(verificationTemplate)
	if err != nil {
		return err
	}
	if err := tpl.Execute(&body, map[string]string{
		"SiteTitle": fallback(m.settings.SiteTitle, "星空 AI"),
		"Code":      code,
	}); err != nil {
		return err
	}
	return m.SendHTML(email, "邮箱验证码", body.String())
}

func (m *Mailer) SendSMTPTest(email string) error {
	var body bytes.Buffer
	tpl, err := template.New("smtp-test").Parse(smtpTestTemplate)
	if err != nil {
		return err
	}
	if err := tpl.Execute(&body, map[string]string{
		"SiteTitle": fallback(m.settings.SiteTitle, "星空 AI"),
		"SentAt":    time.Now().Format("2006-01-02 15:04:05"),
	}); err != nil {
		return err
	}
	return m.SendHTML(email, "SMTP 测试邮件", body.String())
}

func (m *Mailer) SendHTML(email, subject, html string) error {
	if m.settings.SMTPHost == "" || m.settings.SMTPFromEmail == "" {
		return fmt.Errorf("smtp is not configured")
	}

	fromName := fallback(m.settings.SMTPFromName, m.settings.SiteTitle)
	headers := map[string]string{
		"From":         fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("UTF-8", fromName), m.settings.SMTPFromEmail),
		"To":           email,
		"Subject":      mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version": "1.0",
		"Content-Type": `text/html; charset="UTF-8"`,
	}

	var message strings.Builder
	for key, value := range headers {
		message.WriteString(key + ": " + value + "\r\n")
	}
	message.WriteString("\r\n")
	message.WriteString(html)

	addr := net.JoinHostPort(m.settings.SMTPHost, fmt.Sprintf("%d", m.settings.SMTPPort))
	auth := smtp.PlainAuth("", m.settings.SMTPUsername, m.settings.SMTPPassword, m.settings.SMTPHost)
	if !m.settings.SMTPUseTLS {
		return smtp.SendMail(addr, auth, m.settings.SMTPFromEmail, []string{email}, []byte(message.String()))
	}

	var client *smtp.Client
	if m.settings.SMTPPort == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.settings.SMTPHost, MinVersion: tls.VersionTLS12})
		if err != nil {
			return err
		}
		defer conn.Close()
		client, err = smtp.NewClient(conn, m.settings.SMTPHost)
		if err != nil {
			return err
		}
	} else {
		var err error
		client, err = smtp.Dial(addr)
		if err != nil {
			return err
		}
		if err := client.StartTLS(&tls.Config{ServerName: m.settings.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
			client.Close()
			return err
		}
	}
	defer client.Quit()

	if m.settings.SMTPUsername != "" {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(m.settings.SMTPFromEmail); err != nil {
		return err
	}
	if err := client.Rcpt(email); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(message.String())); err != nil {
		return err
	}
	return writer.Close()
}

type EmailTemplateInput struct {
	User      *model.User
	Order     *model.Order
	Plan      *model.Plan
	Settings  model.SystemSetting
	AdminNote string
	ExpiresAt *time.Time
}

func SeedEmailTemplates(db *gorm.DB) {
	if db == nil {
		return
	}
	for _, item := range DefaultEmailTemplates() {
		template := item
		db.Where(model.EmailTemplate{Type: template.Type}).Attrs(template).FirstOrCreate(&template)
	}
}

func DefaultEmailTemplates() []model.EmailTemplate {
	return []model.EmailTemplate{
		{
			Type:        model.EmailTemplateOrderPaymentAdmin,
			Name:        "订单支付待审核通知",
			Description: "用户支付成功且订单进入待审核时发送给管理员。",
			Subject:     "{site_title} 有新的待审核订单 #{order_id}",
			Body: `<p>{admin_name}你好：</p>
<p>用户 {username}（{email}）已支付订单 #{order_id}，套餐为 {plan_name}，金额 {amount} 元，请登录管理后台审核。</p>
<p>支付流水：{payment_ref}</p>
<p>{site_title}</p>`,
			Enabled: true,
		},
		{
			Type:        model.EmailTemplateOrderApprovedUser,
			Name:        "订单审核通过通知",
			Description: "管理员审核通过并开通套餐后发送给用户。",
			Subject:     "{site_title} 套餐已开通",
			Body: `<p>{username}你好：</p>
<p>你的订单 #{order_id} 已审核通过，套餐 {plan_name} 已开通。</p>
<p>有效期至：{expires_at}</p>
<p>{admin_note}</p>
<p>感谢使用 {site_title}。</p>`,
			Enabled: true,
		},
		{
			Type:        model.EmailTemplateSubscriptionExpiring,
			Name:        "套餐到期提醒",
			Description: "用户套餐将在指定天数内到期时发送提醒。",
			Subject:     "{site_title} 套餐将在 {days_left} 天后到期",
			Body: `<p>{username}你好：</p>
<p>你的 {plan_name} 套餐将在 {days_left} 天后到期，到期时间为 {expires_at}。</p>
<p>如需继续使用，请及时续费。</p>
<p>{site_title}</p>`,
			Enabled: true,
		},
	}
}

func SendOrderPaymentAdminNotification(db *gorm.DB, orderID uint) {
	if db == nil {
		return
	}
	var setting model.SystemSetting
	if err := db.First(&setting, 1).Error; err != nil || !setting.OrderPaymentAdminEmailEnabled {
		return
	}

	var order model.Order
	if err := db.Preload("User").Preload("Plan").First(&order, orderID).Error; err != nil {
		log.Printf("load paid order email notification failed: %v", err)
		return
	}

	var admins []model.User
	db.Where("role = ? AND email <> ''", model.RoleAdmin).Find(&admins)
	for _, admin := range orderPaymentAdminRecipients(setting, admins) {
		adminUser := admin
		input := EmailTemplateInput{
			User:     &order.User,
			Order:    &order,
			Plan:     &order.Plan,
			Settings: setting,
		}
		extra := map[string]string{"admin_name": fallback(admin.Username, admin.Email)}
		if err := sendTemplateEmail(db, setting, model.EmailTemplateOrderPaymentAdmin, admin.Email, input, extra, notificationFingerprint(model.EmailTemplateOrderPaymentAdmin, &order.UserID, &order.ID, admin.Email, "")); err != nil {
			log.Printf("send paid order admin notification to %s failed: %v", adminUser.Email, err)
		}
	}
}

func orderPaymentAdminRecipients(setting model.SystemSetting, admins []model.User) []model.User {
	recipients := make([]model.User, 0, len(admins))
	seen := map[string]bool{}
	for _, admin := range admins {
		if admin.Status == model.UserStatusDisabled || (admin.Status != "" && admin.Status != model.UserStatusApproved) {
			continue
		}
		email := normalizeRecipientEmail(admin.Email)
		if email == "" || seen[email] {
			continue
		}
		admin.Email = email
		recipients = append(recipients, admin)
		seen[email] = true
	}

	contactEmail := normalizeRecipientEmail(setting.ContactEmail)
	if len(recipients) == 0 && contactEmail != "" && contactEmail != "support@example.com" {
		recipients = append(recipients, model.User{
			Username: "admin",
			Email:    contactEmail,
			Role:     model.RoleAdmin,
			Status:   model.UserStatusApproved,
		})
	}
	return recipients
}

func normalizeRecipientEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return ""
	}
	return email
}

func SendOrderApprovedUserNotification(db *gorm.DB, orderID uint, adminNote string) {
	if db == nil {
		return
	}
	var setting model.SystemSetting
	if err := db.First(&setting, 1).Error; err != nil || !setting.OrderApprovedUserEmailEnabled {
		return
	}

	var order model.Order
	if err := db.Preload("User").Preload("Plan").First(&order, orderID).Error; err != nil {
		log.Printf("load approved order email notification failed: %v", err)
		return
	}
	if order.User.Email == "" {
		return
	}
	expiresAt := order.User.ExpiresAt
	input := EmailTemplateInput{
		User:      &order.User,
		Order:     &order,
		Plan:      &order.Plan,
		Settings:  setting,
		AdminNote: adminNote,
		ExpiresAt: expiresAt,
	}
	if err := sendTemplateEmail(db, setting, model.EmailTemplateOrderApprovedUser, order.User.Email, input, nil, notificationFingerprint(model.EmailTemplateOrderApprovedUser, &order.UserID, &order.ID, order.User.Email, "")); err != nil {
		log.Printf("send approved order user notification to %s failed: %v", order.User.Email, err)
	}
}

func SendSubscriptionExpireReminders(db *gorm.DB) {
	if db == nil {
		return
	}
	var setting model.SystemSetting
	if err := db.First(&setting, 1).Error; err != nil || !setting.SubscriptionExpireEmailEnabled {
		return
	}
	remindDays := setting.SubscriptionExpireRemindDays
	if remindDays < 1 {
		remindDays = 1
	}

	now := time.Now()
	deadline := now.AddDate(0, 0, remindDays)
	var users []model.User
	if err := db.Preload("Plan").
		Where("status = ? AND email <> '' AND plan_id IS NOT NULL AND expires_at IS NOT NULL AND expires_at > ? AND expires_at <= ?", model.UserStatusApproved, now, deadline).
		Find(&users).Error; err != nil {
		log.Printf("load subscription expire reminders failed: %v", err)
		return
	}

	for _, user := range users {
		if user.ExpiresAt == nil || user.Plan == nil {
			continue
		}
		input := EmailTemplateInput{
			User:      &user,
			Plan:      user.Plan,
			Settings:  setting,
			ExpiresAt: user.ExpiresAt,
		}
		fingerprintSuffix := user.ExpiresAt.Format("20060102150405")
		if err := sendTemplateEmail(db, setting, model.EmailTemplateSubscriptionExpiring, user.Email, input, nil, notificationFingerprint(model.EmailTemplateSubscriptionExpiring, &user.ID, nil, user.Email, fingerprintSuffix)); err != nil {
			log.Printf("send subscription expire reminder to %s failed: %v", user.Email, err)
		}
	}
}

func sendTemplateEmail(db *gorm.DB, setting model.SystemSetting, templateType, to string, input EmailTemplateInput, extra map[string]string, fingerprint string) error {
	if strings.TrimSpace(to) == "" {
		return nil
	}
	var existing model.EmailNotificationLog
	if err := db.Where("fingerprint = ?", fingerprint).First(&existing).Error; err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	logItem := model.EmailNotificationLog{
		EventType:   templateType,
		SentTo:      to,
		Fingerprint: fingerprint,
	}
	if input.User != nil {
		logItem.UserID = &input.User.ID
	}
	if input.Order != nil {
		logItem.OrderID = &input.Order.ID
	}
	if err := db.Create(&logItem).Error; err != nil {
		if isDuplicateFingerprintError(err) {
			return nil
		}
		return err
	}

	var tpl model.EmailTemplate
	if err := db.Where("type = ? AND enabled = ?", templateType, true).First(&tpl).Error; err != nil {
		deleteEmailNotificationLog(db, logItem.ID)
		return err
	}
	variables := emailTemplateVariables(input, extra)
	subject := renderEmailTemplateText(tpl.Subject, variables, false)
	body := renderEmailTemplateText(tpl.Body, variables, true)
	if strings.TrimSpace(body) == "" {
		body = renderEmailTemplateText(defaultTemplateBody(templateType), variables, true)
	}

	if err := NewMailer(setting).SendHTML(to, subject, body); err != nil {
		deleteEmailNotificationLog(db, logItem.ID)
		return err
	}
	return nil
}

func deleteEmailNotificationLog(db *gorm.DB, id uint) {
	if id == 0 {
		return
	}
	db.Unscoped().Delete(&model.EmailNotificationLog{}, id)
}

func isDuplicateFingerprintError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func emailTemplateVariables(input EmailTemplateInput, extra map[string]string) map[string]string {
	vars := map[string]string{
		"site_title":    fallback(input.Settings.SiteTitle, "星空 AI"),
		"contact_email": input.Settings.ContactEmail,
		"username":      "",
		"email":         "",
		"order_id":      "",
		"payment_ref":   "",
		"amount":        "0.00",
		"plan_name":     "",
		"duration_days": "0",
		"expires_at":    "",
		"days_left":     "0",
		"admin_note":    "",
	}
	if input.User != nil {
		vars["username"] = fallback(input.User.Username, input.User.Email)
		vars["email"] = input.User.Email
	}
	if input.Order != nil {
		vars["order_id"] = fmt.Sprintf("%d", input.Order.ID)
		vars["payment_ref"] = input.Order.PaymentRef
		vars["amount"] = fmt.Sprintf("%.2f", float64(input.Order.AmountCents)/100)
	}
	if input.Plan != nil {
		vars["plan_name"] = input.Plan.Name
		vars["duration_days"] = fmt.Sprintf("%d", input.Plan.DurationDays)
	}
	if input.ExpiresAt != nil {
		vars["expires_at"] = input.ExpiresAt.Format("2006-01-02 15:04:05")
		daysLeft := int(time.Until(*input.ExpiresAt).Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}
		vars["days_left"] = fmt.Sprintf("%d", daysLeft)
	}
	if strings.TrimSpace(input.AdminNote) != "" {
		vars["admin_note"] = input.AdminNote
	}
	for key, value := range extra {
		vars[key] = value
	}
	return vars
}

var emailVariablePattern = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

func renderEmailTemplateText(text string, vars map[string]string, escapeHTML bool) string {
	return emailVariablePattern.ReplaceAllStringFunc(text, func(match string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
		if value, ok := vars[key]; ok {
			if escapeHTML {
				return html.EscapeString(value)
			}
			return value
		}
		return match
	})
}

func defaultTemplateBody(templateType string) string {
	for _, item := range DefaultEmailTemplates() {
		if item.Type == templateType {
			return item.Body
		}
	}
	return ""
}

func notificationFingerprint(eventType string, userID *uint, orderID *uint, to string, suffix string) string {
	parts := []string{eventType, strings.ToLower(strings.TrimSpace(to))}
	if userID != nil {
		parts = append(parts, fmt.Sprintf("u%d", *userID))
	}
	if orderID != nil {
		parts = append(parts, fmt.Sprintf("o%d", *orderID))
	}
	if suffix != "" {
		parts = append(parts, suffix)
	}
	raw := strings.Join(parts, ":")
	if len(raw) <= 128 {
		return raw
	}
	sum := sha256.Sum256([]byte(raw))
	return eventType + ":" + hex.EncodeToString(sum[:])
}

func fallback(value, backup string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return backup
}
