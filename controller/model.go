package controller

import (
	"ai-gateway/model"
	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ModelController struct {
	db *gorm.DB
}

func NewModelController(db *gorm.DB) *ModelController {
	return &ModelController{db: db}
}

func (m *ModelController) List(c *gin.Context) {
	var models []model.ModelPricing
	m.db.Where("status = ?", model.ModelPricingStatusActive).
		Order("provider asc, model asc").
		Find(&models)
	response.OK(c, models)
}
