package service

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"strings"

	"ai-gateway/model"
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
		"SiteTitle": fallback(m.settings.SiteTitle, "AI Gateway"),
		"Code":      code,
	}); err != nil {
		return err
	}

	fromName := fallback(m.settings.SMTPFromName, m.settings.SiteTitle)
	headers := map[string]string{
		"From":         fmt.Sprintf("%s <%s>", fromName, m.settings.SMTPFromEmail),
		"To":           email,
		"Subject":      "邮箱验证码",
		"MIME-Version": "1.0",
		"Content-Type": `text/html; charset="UTF-8"`,
	}

	var message strings.Builder
	for key, value := range headers {
		message.WriteString(key + ": " + value + "\r\n")
	}
	message.WriteString("\r\n")
	message.Write(body.Bytes())

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

func fallback(value, backup string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return backup
}
