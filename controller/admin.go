package controller

import (
	"time"

	"ai-gateway/model"
	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminController struct {
	db *gorm.DB
}

func NewAdminController(db *gorm.DB) *AdminController {
	return &AdminController{db: db}
}

type approveOrderRequest struct {
	Channel   string `json:"channel" binding:"required"`
	BaseURL   string `json:"base_url" binding:"required,url"`
	APIKey    string `json:"api_key" binding:"required"`
	AdminNote string `json:"admin_note"`
}

type planRequest struct {
	Name               string `json:"name" binding:"required,min=2,max=64"`
	Code               string `json:"code"`
	PlanType           string `json:"plan_type"`
	PriceCents         int64  `json:"price_cents" binding:"required,min=1"`
	SettlementUSDCents int64  `json:"settlement_usd_cents"`
	QuotaTokens        int64  `json:"quota_tokens"`
	DailyQuotaTokens   int64  `json:"daily_quota_tokens"`
	WeeklyQuotaTokens  int64  `json:"weekly_quota_tokens"`
	DurationDays       int    `json:"duration_days" binding:"required,min=1"`
	Description        string `json:"description"`
	Enabled            bool   `json:"enabled"`
}

type updateUserRequest struct {
	Username      string `json:"username"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	EmailVerified *bool  `json:"email_verified"`
	PlanID        *uint  `json:"plan_id"`
	QuotaTokens   *int64 `json:"quota_tokens"`
	UsedTokens    *int64 `json:"used_tokens"`
}

type rejectOrderRequest struct {
	AdminNote string `json:"admin_note"`
}

func (a *AdminController) Users(c *gin.Context) {
	var users []model.User
	a.db.Preload("Plan").Order("id desc").Find(&users)
	response.OK(c, users)
}

func (a *AdminController) UpdateUser(c *gin.Context) {
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Username != "" {
		updates["username"] = req.Username
	}
	if req.Role == model.RoleUser || req.Role == model.RoleAdmin {
		updates["role"] = req.Role
	}
	if req.Status == model.UserStatusPending || req.Status == model.UserStatusApproved || req.Status == model.UserStatusDisabled {
		updates["status"] = req.Status
	}
	if req.EmailVerified != nil {
		updates["email_verified"] = *req.EmailVerified
	}
	if req.PlanID != nil {
		updates["plan_id"] = *req.PlanID
	}
	if req.QuotaTokens != nil {
		updates["quota_tokens"] = *req.QuotaTokens
	}
	if req.UsedTokens != nil {
		updates["used_tokens"] = *req.UsedTokens
	}
	if len(updates) == 0 {
		response.OK(c, nil)
		return
	}
	if err := a.db.Model(&model.User{}).Where("id = ?", c.Param("id")).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update user")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) DeleteUser(c *gin.Context) {
	if err := a.db.Delete(&model.User{}, c.Param("id")).Error; err != nil {
		response.Error(c, 500, "failed to delete user")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) Orders(c *gin.Context) {
	var orders []model.Order
	a.db.Preload("User").Preload("Plan").Order("id desc").Find(&orders)
	response.OK(c, orders)
}

func (a *AdminController) Plans(c *gin.Context) {
	var plans []model.Plan
	a.db.Unscoped().Order("price_cents asc").Find(&plans)
	response.OK(c, plans)
}

func (a *AdminController) CreatePlan(c *gin.Context) {
	var req planRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	plan := model.Plan{
		Name:               req.Name,
		Code:               req.Code,
		PlanType:           fallbackPlanType(req.PlanType),
		PriceCents:         req.PriceCents,
		SettlementUSDCents: req.SettlementUSDCents,
		QuotaTokens:        req.QuotaTokens,
		DailyQuotaTokens:   req.DailyQuotaTokens,
		WeeklyQuotaTokens:  req.WeeklyQuotaTokens,
		DurationDays:       req.DurationDays,
		Description:        req.Description,
		Enabled:            req.Enabled,
	}
	if err := a.db.Create(&plan).Error; err != nil {
		response.Error(c, 500, "failed to create plan")
		return
	}
	response.Created(c, plan)
}

func (a *AdminController) UpdatePlan(c *gin.Context) {
	var req planRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	updates := map[string]interface{}{
		"name":                 req.Name,
		"code":                 req.Code,
		"plan_type":            fallbackPlanType(req.PlanType),
		"price_cents":          req.PriceCents,
		"settlement_usd_cents": req.SettlementUSDCents,
		"quota_tokens":         req.QuotaTokens,
		"daily_quota_tokens":   req.DailyQuotaTokens,
		"weekly_quota_tokens":  req.WeeklyQuotaTokens,
		"duration_days":        req.DurationDays,
		"description":          req.Description,
		"enabled":              req.Enabled,
	}
	if err := a.db.Model(&model.Plan{}).Where("id = ?", c.Param("id")).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update plan")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) DeletePlan(c *gin.Context) {
	if err := a.db.Delete(&model.Plan{}, c.Param("id")).Error; err != nil {
		response.Error(c, 500, "failed to delete plan")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) ApproveOrder(c *gin.Context) {
	admin := c.MustGet("user").(model.User)
	var req approveOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	var order model.Order
	if err := a.db.Preload("Plan").First(&order, c.Param("id")).Error; err != nil {
		response.Error(c, 404, "order not found")
		return
	}
	if order.Status != model.OrderStatusPendingReview {
		response.Error(c, 409, "order already reviewed")
		return
	}

	now := time.Now()
	expiresAt := now.AddDate(0, 0, order.Plan.DurationDays)
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":         model.OrderStatusApproved,
			"admin_note":     req.AdminNote,
			"approved_at":    &now,
			"approved_by_id": admin.ID,
		}).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.User{}).Where("id = ?", order.UserID).Updates(map[string]interface{}{
			"status":       model.UserStatusApproved,
			"plan_id":      order.PlanID,
			"quota_tokens": order.Plan.QuotaTokens,
			"used_tokens":  0,
			"expires_at":   &expiresAt,
		}).Error; err != nil {
			return err
		}

		upstream := model.UpstreamAccount{
			UserID:  order.UserID,
			Channel: req.Channel,
			BaseURL: req.BaseURL,
			APIKey:  req.APIKey,
			Status:  model.UpstreamStatusActive,
		}
		return tx.Where(model.UpstreamAccount{UserID: order.UserID}).Assign(upstream).FirstOrCreate(&upstream).Error
	})
	if err != nil {
		response.Error(c, 500, "failed to approve order")
		return
	}

	response.OK(c, gin.H{"status": model.OrderStatusApproved})
}

func (a *AdminController) RejectOrder(c *gin.Context) {
	var req rejectOrderRequest
	_ = c.ShouldBindJSON(&req)
	if req.AdminNote == "" {
		req.AdminNote = c.Query("note")
	}
	if err := a.db.Model(&model.Order{}).Where("id = ?", c.Param("id")).Updates(map[string]interface{}{
		"status":     model.OrderStatusRejected,
		"admin_note": req.AdminNote,
	}).Error; err != nil {
		response.Error(c, 500, "failed to reject order")
		return
	}
	response.OK(c, nil)
}

func fallbackPlanType(value string) string {
	if value == "" {
		return "subscription"
	}
	return value
}

func (a *AdminController) Upstreams(c *gin.Context) {
	var upstreams []model.UpstreamAccount
	a.db.Preload("User").Order("id desc").Find(&upstreams)
	response.OK(c, upstreams)
}

func (a *AdminController) APIKeys(c *gin.Context) {
	var keys []model.APIKey
	a.db.Preload("User").Order("id desc").Find(&keys)
	response.OK(c, keys)
}

func (a *AdminController) Stats(c *gin.Context) {
	var users, orders, apiKeys, calls int64
	a.db.Model(&model.User{}).Count(&users)
	a.db.Model(&model.Order{}).Count(&orders)
	a.db.Model(&model.APIKey{}).Count(&apiKeys)
	a.db.Model(&model.APILog{}).Count(&calls)
	response.OK(c, gin.H{
		"users":    users,
		"orders":   orders,
		"api_keys": apiKeys,
		"calls":    calls,
	})
}
