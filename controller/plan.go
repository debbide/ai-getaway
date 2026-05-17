package controller

import (
	"ai-gateway/model"
	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PlanController struct {
	db *gorm.DB
}

func NewPlanController(db *gorm.DB) *PlanController {
	return &PlanController{db: db}
}

func (p *PlanController) List(c *gin.Context) {
	var plans []model.Plan
	p.db.Preload("PublicChannel").Preload("PollingPool.Accounts").Where("enabled = ?", true).Order("price_cents asc").Find(&plans)
	response.OK(c, plans)
}
