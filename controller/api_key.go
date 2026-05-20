package controller

import (
	"errors"
	"strconv"
	"strings"
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

type adminUpdateAPIKeyRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
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
		a.db.Model(&model.APIKey{}).
			Where("id = ?", keys[i].ID).
			Update("status", model.APIKeyStatusDisabled)
	}
}

func (a *APIKeyController) loadUpstream(userID uint) (*model.UpstreamAccount, error) {
	var upstream model.UpstreamAccount
	if err := a.db.Where("user_id = ? AND status = ?", userID, model.UpstreamStatusActive).First(&upstream).Error; err != nil {
		return nil, err
	}
	return &upstream, nil
}

func (a *APIKeyController) ensureCallablePlan(user model.User) error {
	var fresh model.User
	if err := a.db.Preload("PublicChannel").Preload("Plan.PublicChannel").Preload("Plan.PollingPool.Accounts").First(&fresh, user.ID).Error; err != nil {
		return err
	}
	if !service.HasCallableAccess(fresh, time.Now()) {
		return errors.New("subscription expired")
	}
	if service.HasDirectPublicChannelAccess(fresh, time.Now()) && fresh.PlanID == nil {
		if fresh.PublicChannel == nil || !fresh.PublicChannel.Enabled || fresh.PublicChannel.RemainingUSDCents <= 0 {
			return errors.New("public channel sold out")
		}
		return nil
	}
	if fresh.Plan == nil {
		return errors.New("subscription expired")
	}
	if fresh.Plan.PlanType == model.PlanTypePublic {
		if !service.PlanChannelHasQuota(*fresh.Plan) {
			return errors.New("public channel sold out")
		}
		return nil
	}
	if _, err := a.loadUpstream(user.ID); err != nil {
		return errors.New("no active upstream account bound")
	}
	return nil
}

// Create 仅当用户尚无任何密钥时创建第一条（全站每用户仅保留一条记录）。
func (a *APIKeyController) Create(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if user.Status != model.UserStatusApproved {
		response.Error(c, 403, "account pending approval")
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
		if err := tx.Model(&model.APIKey{}).
			Where("user_id = ?", user.ID).
			Update("status", model.APIKeyStatusDisabled).Error; err != nil {
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

func (a *APIKeyController) AdminList(c *gin.Context) {
	page, pageSize := 1, 10
	if value, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1"))); err == nil && value > 0 {
		page = value
	}
	if value, err := strconv.Atoi(strings.TrimSpace(c.Query("page_size"))); err == nil && value > 0 {
		pageSize = value
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := a.db.Model(&model.APIKey{}).Preload("User")
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Joins("LEFT JOIN users ON users.id = api_keys.user_id").
			Where("api_keys.name LIKE ? OR api_keys.key_prefix LIKE ? OR users.username LIKE ? OR users.email LIKE ? OR CAST(api_keys.id AS CHAR) LIKE ?", like, like, like, like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("api_keys.status = ?", status)
	}

	var total int64
	query.Count(&total)
	var keys []model.APIKey
	query.Order("api_keys.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&keys)
	response.OK(c, gin.H{
		"items":     keys,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (a *APIKeyController) AdminUpdate(c *gin.Context) {
	var req adminUpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if name := strings.TrimSpace(req.Name); name != "" {
		updates["name"] = name
	}
	if req.Status == model.APIKeyStatusActive || req.Status == model.APIKeyStatusDisabled {
		updates["status"] = req.Status
	}
	if len(updates) == 0 {
		response.OK(c, nil)
		return
	}

	result := a.db.Model(&model.APIKey{}).Where("id = ?", c.Param("id")).Updates(updates)
	if result.Error != nil {
		response.Error(c, 500, "failed to update api key")
		return
	}
	if result.RowsAffected == 0 {
		response.Error(c, 404, "api key not found")
		return
	}
	response.OK(c, nil)
}

func (a *APIKeyController) AdminDelete(c *gin.Context) {
	result := a.db.Model(&model.APIKey{}).
		Where("id = ?", c.Param("id")).
		Update("status", model.APIKeyStatusDisabled)
	if result.Error != nil {
		response.Error(c, 500, "failed to delete api key")
		return
	}
	if result.RowsAffected == 0 {
		response.Error(c, 404, "api key not found")
		return
	}
	response.OK(c, nil)
}
