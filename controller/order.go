package controller

import (
	"ai-gateway/model"
	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OrderController struct {
	db *gorm.DB
}

func NewOrderController(db *gorm.DB) *OrderController {
	return &OrderController{db: db}
}

type createOrderRequest struct {
	PlanID     uint   `json:"plan_id" binding:"required"`
	PaymentRef string `json:"payment_ref"`
}

func (o *OrderController) Create(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	var plan model.Plan
	if err := o.db.Where("id = ? AND enabled = ?", req.PlanID, true).First(&plan).Error; err != nil {
		response.Error(c, 404, "plan not found")
		return
	}

	order := model.Order{
		UserID:      user.ID,
		PlanID:      plan.ID,
		AmountCents: plan.PriceCents,
		Status:      model.OrderStatusPendingReview,
		PaymentRef:  req.PaymentRef,
	}
	if err := o.db.Create(&order).Error; err != nil {
		response.Error(c, 500, "failed to create order")
		return
	}
	response.Created(c, order)
}

func (o *OrderController) ListMine(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var orders []model.Order
	o.db.Preload("Plan").Where("user_id = ?", user.ID).Order("id desc").Find(&orders)
	response.OK(c, orders)
}
