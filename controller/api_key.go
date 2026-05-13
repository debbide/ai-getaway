package controller

import (
	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type APIKeyController struct {
	db *gorm.DB
}

func NewAPIKeyController(db *gorm.DB) *APIKeyController {
	return &APIKeyController{db: db}
}

type createAPIKeyRequest struct {
	Name string `json:"name" binding:"required,min=2,max=64"`
}

func (a *APIKeyController) Create(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if user.Status != model.UserStatusApproved {
		response.Error(c, 403, "account pending approval")
		return
	}

	var upstream model.UpstreamAccount
	if err := a.db.Where("user_id = ? AND status = ?", user.ID, model.UpstreamStatusActive).First(&upstream).Error; err != nil {
		response.Error(c, 403, "no active upstream account bound")
		return
	}

	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	key, hash, prefix, err := utils.GenerateAPIKey()
	if err != nil {
		response.Error(c, 500, "failed to generate api key")
		return
	}

	apiKey := model.APIKey{
		UserID:    user.ID,
		Name:      req.Name,
		KeyHash:   hash,
		KeyPrefix: prefix,
		Status:    model.APIKeyStatusActive,
	}
	if err := a.db.Create(&apiKey).Error; err != nil {
		response.Error(c, 500, "failed to create api key")
		return
	}

	response.Created(c, gin.H{
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key":        key,
		"key_prefix": apiKey.KeyPrefix,
		"status":     apiKey.Status,
	})
}

func (a *APIKeyController) List(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var keys []model.APIKey
	a.db.Where("user_id = ?", user.ID).Order("id desc").Find(&keys)
	response.OK(c, keys)
}

func (a *APIKeyController) Disable(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if err := a.db.Model(&model.APIKey{}).
		Where("id = ? AND user_id = ?", c.Param("id"), user.ID).
		Update("status", model.APIKeyStatusDisabled).Error; err != nil {
		response.Error(c, 500, "failed to disable api key")
		return
	}
	response.OK(c, nil)
}
