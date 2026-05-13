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

func (a *AdminController) Users(c *gin.Context) {
	var users []model.User
	a.db.Preload("Plan").Order("id desc").Find(&users)
	response.OK(c, users)
}

func (a *AdminController) Orders(c *gin.Context) {
	var orders []model.Order
	a.db.Preload("User").Preload("Plan").Order("id desc").Find(&orders)
	response.OK(c, orders)
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
	if err := a.db.Model(&model.Order{}).Where("id = ?", c.Param("id")).Updates(map[string]interface{}{
		"status":     model.OrderStatusRejected,
		"admin_note": c.Query("note"),
	}).Error; err != nil {
		response.Error(c, 500, "failed to reject order")
		return
	}
	response.OK(c, nil)
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
