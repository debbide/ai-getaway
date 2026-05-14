package controller

import (
	"errors"
	"time"

	"ai-gateway/config"
	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/service"
	"ai-gateway/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type APIKeyController struct {
	db  *gorm.DB
	cfg config.Config
}

func NewAPIKeyController(cfg config.Config, db *gorm.DB) *APIKeyController {
	return &APIKeyController{db: db, cfg: cfg}
}

type createAPIKeyRequest struct {
	Name string `json:"name" binding:"required,min=2,max=64"`
}

func maskDisplayAPIKey(plaintext, prefix string) string {
	if plaintext == "" {
		if prefix != "" {
			return prefix + "···"
		}
		return "················"
	}
	n := len(plaintext)
	if n <= 14 {
		h := 8
		if n < h {
			h = n
		}
		return plaintext[:h] + "···"
	}
	if n <= 22 {
		return plaintext[:8] + "····" + plaintext[n-4:]
	}
	return plaintext[:10] + "··········" + plaintext[n-4:]
}

func (a *APIKeyController) dedupeUserAPIKeys(userID uint) {
	var keys []model.APIKey
	a.db.Where("user_id = ?", userID).Order("id desc").Find(&keys)
	for i := 1; i < len(keys); i++ {
		a.db.Unscoped().Delete(&model.APIKey{}, keys[i].ID)
	}
}

func (a *APIKeyController) loadUpstream(userID uint) (*model.UpstreamAccount, error) {
	var upstream model.UpstreamAccount
	if err := a.db.Where("user_id = ? AND status = ?", userID, model.UpstreamStatusActive).First(&upstream).Error; err != nil {
		return nil, err
	}
	return &upstream, nil
}

// Create 仅当用户尚无任何密钥时创建第一条（全站每用户仅保留一条记录）。
func (a *APIKeyController) Create(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if user.Status != model.UserStatusApproved {
		response.Error(c, 403, "account pending approval")
		return
	}
	if !service.HasActiveSubscription(user, time.Now()) {
		response.Error(c, 403, "subscription expired")
		return
	}
	if _, err := a.loadUpstream(user.ID); err != nil {
		response.Error(c, 403, "no active upstream account bound")
		return
	}

	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	var n int64
	a.db.Model(&model.APIKey{}).Where("user_id = ?", user.ID).Count(&n)
	if n > 0 {
		response.Error(c, 409, "api key already exists")
		return
	}

	key, hash, prefix, err := utils.GenerateAPIKey()
	if err != nil {
		response.Error(c, 500, "failed to generate api key")
		return
	}

	enc, err := utils.EncryptAPIKeySecret(a.cfg.JWTSecret, key)
	if err != nil {
		response.Error(c, 500, "failed to generate api key")
		return
	}

	apiKey := model.APIKey{
		UserID:       user.ID,
		Name:         req.Name,
		KeyHash:      hash,
		KeyPrefix:    prefix,
		KeyEncrypted: enc,
		Status:       model.APIKeyStatusActive,
	}
	if err := a.db.Create(&apiKey).Error; err != nil {
		response.Error(c, 500, "failed to create api key")
		return
	}

	response.Created(c, gin.H{
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key":        key,
		"key_masked": maskDisplayAPIKey(key, prefix),
		"key_prefix": apiKey.KeyPrefix,
		"status":     apiKey.Status,
	})
}

// Rotate 删除当前用户全部密钥记录后新建一条（更新密钥 = 替换，旧哈希立即失效）。
func (a *APIKeyController) Rotate(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if user.Status != model.UserStatusApproved {
		response.Error(c, 403, "account pending approval")
		return
	}
	if !service.HasActiveSubscription(user, time.Now()) {
		response.Error(c, 403, "subscription expired")
		return
	}
	if _, err := a.loadUpstream(user.ID); err != nil {
		response.Error(c, 403, "no active upstream account bound")
		return
	}

	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	var n int64
	a.db.Model(&model.APIKey{}).Where("user_id = ?", user.ID).Count(&n)
	if n == 0 {
		response.Error(c, 409, "no api key to rotate")
		return
	}

	key, hash, prefix, err := utils.GenerateAPIKey()
	if err != nil {
		response.Error(c, 500, "failed to generate api key")
		return
	}

	enc, err := utils.EncryptAPIKeySecret(a.cfg.JWTSecret, key)
	if err != nil {
		response.Error(c, 500, "failed to generate api key")
		return
	}

	apiKey := model.APIKey{
		UserID:       user.ID,
		Name:         req.Name,
		KeyHash:      hash,
		KeyPrefix:    prefix,
		KeyEncrypted: enc,
		Status:       model.APIKeyStatusActive,
	}

	err = a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("user_id = ?", user.ID).Delete(&model.APIKey{}).Error; err != nil {
			return err
		}
		return tx.Create(&apiKey).Error
	})
	if err != nil {
		response.Error(c, 500, "failed to rotate api key")
		return
	}

	response.OK(c, gin.H{
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key":        key,
		"key_masked": maskDisplayAPIKey(key, prefix),
		"key_prefix": apiKey.KeyPrefix,
		"status":     apiKey.Status,
	})
}

// List 最多返回一条；列表不含明文，仅掩码与是否可复制。
func (a *APIKeyController) List(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	a.dedupeUserAPIKeys(user.ID)

	var k model.APIKey
	err := a.db.Where("user_id = ?", user.ID).Order("id desc").First(&k).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.OK(c, []gin.H{})
		return
	}
	if err != nil {
		response.Error(c, 500, "failed to list api keys")
		return
	}

	item := gin.H{
		"id":         k.ID,
		"name":       k.Name,
		"key_prefix": k.KeyPrefix,
		"status":     k.Status,
		"created_at": k.CreatedAt,
		"can_copy":   false,
		"key_masked": k.KeyPrefix + "···",
	}
	if k.KeyEncrypted != "" {
		plain, err := utils.DecryptAPIKeySecret(a.cfg.JWTSecret, k.KeyEncrypted)
		if err == nil {
			item["can_copy"] = true
			item["key_masked"] = maskDisplayAPIKey(plain, k.KeyPrefix)
		}
	}
	response.OK(c, []gin.H{item})
}

// Secret 返回当前用户唯一密钥的明文（用于「复制」），不在列表接口中返回明文。
func (a *APIKeyController) Secret(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var k model.APIKey
	if err := a.db.Where("user_id = ?", user.ID).Order("id desc").First(&k).Error; err != nil {
		response.Error(c, 404, "api key not found")
		return
	}
	if k.KeyEncrypted == "" {
		response.Error(c, 404, "api key secret unavailable")
		return
	}
	plain, err := utils.DecryptAPIKeySecret(a.cfg.JWTSecret, k.KeyEncrypted)
	if err != nil {
		response.Error(c, 500, "failed to decrypt api key")
		return
	}
	response.OK(c, gin.H{"key": plain})
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

func (a *APIKeyController) Enable(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if err := a.db.Model(&model.APIKey{}).
		Where("id = ? AND user_id = ?", c.Param("id"), user.ID).
		Update("status", model.APIKeyStatusActive).Error; err != nil {
		response.Error(c, 500, "failed to enable api key")
		return
	}
	response.OK(c, nil)
}
