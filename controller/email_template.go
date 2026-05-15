package controller

import (
	"strings"

	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type EmailTemplateController struct {
	db *gorm.DB
}

func NewEmailTemplateController(db *gorm.DB) *EmailTemplateController {
	return &EmailTemplateController{db: db}
}

type updateEmailTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Subject     string `json:"subject" binding:"required"`
	Body        string `json:"body" binding:"required"`
	Enabled     bool   `json:"enabled"`
}

func (e *EmailTemplateController) List(c *gin.Context) {
	service.SeedEmailTemplates(e.db)
	var templates []model.EmailTemplate
	e.db.Order("id asc").Find(&templates)
	response.OK(c, gin.H{
		"items":     templates,
		"variables": emailTemplateVariablesHelp(),
		"types":     emailTemplateTypeLabels(),
	})
}

func (e *EmailTemplateController) Update(c *gin.Context) {
	var req updateEmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	templateType := strings.TrimSpace(c.Param("type"))
	if _, ok := emailTemplateTypeLabels()[templateType]; !ok {
		response.Error(c, 404, "email template not found")
		return
	}
	service.SeedEmailTemplates(e.db)

	updates := map[string]interface{}{
		"subject": req.Subject,
		"body":    req.Body,
		"enabled": req.Enabled,
	}
	if strings.TrimSpace(req.Name) != "" {
		updates["name"] = strings.TrimSpace(req.Name)
	}
	updates["description"] = strings.TrimSpace(req.Description)

	if err := e.db.Model(&model.EmailTemplate{}).Where("type = ?", templateType).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update email template")
		return
	}
	response.OK(c, nil)
}

func emailTemplateTypeLabels() map[string]string {
	return map[string]string{
		model.EmailTemplateOrderPaymentAdmin:    "订单支付待审核通知",
		model.EmailTemplateOrderApprovedUser:    "订单审核通过通知",
		model.EmailTemplateSubscriptionExpiring: "套餐到期提醒",
	}
}

func emailTemplateVariablesHelp() []string {
	return []string{
		"{site_title}",
		"{contact_email}",
		"{username}",
		"{email}",
		"{order_id}",
		"{payment_ref}",
		"{amount}",
		"{plan_name}",
		"{duration_days}",
		"{expires_at}",
		"{days_left}",
		"{admin_note}",
		"{admin_name}",
	}
}
