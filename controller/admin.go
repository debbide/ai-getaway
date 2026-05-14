package controller

import (
	"encoding/json"
	"errors"
	"time"

	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/service"
	"ai-gateway/utils"

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
	ChannelID uint   `json:"channel_id"`
	Channel   string `json:"channel"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key" binding:"required"`
	AdminNote string `json:"admin_note"`
}

type planRequest struct {
	Name               string `json:"name" binding:"required,min=2,max=64"`
	Code               string `json:"code"`
	BadgeText          string `json:"badge_text"`
	PlanType           string `json:"plan_type"`
	QuotaPeriod        string `json:"quota_period"`
	PriceCents         int64  `json:"price_cents" binding:"required,min=1"`
	SettlementUSDCents int64  `json:"settlement_usd_cents"`
	DurationDays       int    `json:"duration_days" binding:"required,min=1"`
	Description        string `json:"description"`
	Enabled            bool   `json:"enabled"`
}

type updateUserRequest struct {
	Username      string `json:"username"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	EmailVerified *bool  `json:"email_verified"`
	PlanID        *uint  `json:"plan_id"`
	PlanIDPresent bool   `json:"-"`
	ChannelID     uint   `json:"channel_id"`
	APIKey        string `json:"api_key"`
}

func (r *updateUserRequest) UnmarshalJSON(data []byte) error {
	type request updateUserRequest
	if err := json.Unmarshal(data, (*request)(r)); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	value, ok := raw["plan_id"]
	if !ok {
		return nil
	}

	r.PlanIDPresent = true
	if string(value) == "null" {
		r.PlanID = nil
		return nil
	}

	var planID uint
	if err := json.Unmarshal(value, &planID); err != nil {
		return err
	}
	r.PlanID = &planID
	return nil
}

type createUserRequest struct {
	Username      string `json:"username" binding:"required,min=2,max=64"`
	Email         string `json:"email" binding:"required,email"`
	Password      string `json:"password" binding:"required,min=8"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	EmailVerified bool   `json:"email_verified"`
	PlanID        *uint  `json:"plan_id"`
}

type rejectOrderRequest struct {
	AdminNote string `json:"admin_note"`
}

type updateOrderRequest struct {
	ChannelID   uint   `json:"channel_id"`
	Channel     string `json:"channel"`
	BaseURL     string `json:"base_url"`
	APIKey      string `json:"api_key"`
	AdminNote   string `json:"admin_note"`
	PlanID      *uint  `json:"plan_id"`
	AmountCents *int64 `json:"amount_cents"`
}

type upstreamChannelRequest struct {
	Name    string `json:"name" binding:"required,min=2,max=64"`
	BaseURL string `json:"base_url" binding:"required,url"`
	Enabled bool   `json:"enabled"`
}

type modelPricingRequest struct {
	ModelName                string  `json:"model" binding:"required,min=1,max=128"`
	DisplayName              string  `json:"display_name"`
	Provider                 string  `json:"provider"`
	InputUSDPerMillion       float64 `json:"input_usd_per_million" binding:"min=0"`
	CachedInputUSDPerMillion float64 `json:"cached_input_usd_per_million" binding:"min=0"`
	OutputUSDPerMillion      float64 `json:"output_usd_per_million" binding:"min=0"`
	BillingMultiplier        float64 `json:"billing_multiplier" binding:"min=0"`
	Status                   string  `json:"status"`
	Notes                    string  `json:"notes"`
}

type orderResponse struct {
	model.Order
	Upstream *model.UpstreamAccount `json:"Upstream,omitempty"`
}

func (a *AdminController) Users(c *gin.Context) {
	var users []model.User
	a.db.Preload("Plan").Order("id desc").Find(&users)
	response.OK(c, users)
}

func (a *AdminController) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		response.Error(c, 500, "failed to hash password")
		return
	}

	role := req.Role
	if role != model.RoleAdmin {
		role = model.RoleUser
	}
	status := req.Status
	if status != model.UserStatusApproved && status != model.UserStatusDisabled {
		status = model.UserStatusPending
	}

	user := model.User{
		Username:      req.Username,
		Email:         req.Email,
		PasswordHash:  passwordHash,
		Role:          role,
		Status:        status,
		EmailVerified: req.EmailVerified,
		PlanID:        req.PlanID,
	}
	if req.PlanID != nil {
		var plan model.Plan
		if err := a.db.First(&plan, *req.PlanID).Error; err != nil {
			response.Error(c, 404, "plan not found")
			return
		}
		expiresAt := time.Now().AddDate(0, 0, plan.DurationDays)
		user.ExpiresAt = &expiresAt
	}
	if err := a.db.Create(&user).Error; err != nil {
		response.Error(c, 409, "email already exists")
		return
	}
	response.Created(c, user)
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
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Password != "" {
		if len(req.Password) < 8 {
			response.Error(c, 400, "password must be at least 8 characters")
			return
		}
		passwordHash, err := utils.HashPassword(req.Password)
		if err != nil {
			response.Error(c, 500, "failed to hash password")
			return
		}
		updates["password_hash"] = passwordHash
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
	if req.PlanIDPresent {
		if req.PlanID == nil {
			updates["plan_id"] = nil
			updates["expires_at"] = nil
		} else {
			var plan model.Plan
			if err := a.db.First(&plan, *req.PlanID).Error; err != nil {
				response.Error(c, 404, "plan not found")
				return
			}
			expiresAt := time.Now().AddDate(0, 0, plan.DurationDays)
			updates["plan_id"] = plan.ID
			updates["expires_at"] = &expiresAt
		}
	}
	if len(updates) == 0 {
		response.OK(c, nil)
		return
	}
	var user model.User
	if err := a.db.First(&user, c.Param("id")).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, 404, "user not found")
			return
		}
		response.Error(c, 500, "failed to update user")
		return
	}
	planChanged := req.PlanIDPresent && !sameUintPointer(user.PlanID, req.PlanID)
	var selectedChannel *model.UpstreamChannel
	if planChanged && req.PlanID != nil {
		if req.ChannelID == 0 || req.APIKey == "" {
			response.Error(c, 400, "upstream rebinding required after plan change")
			return
		}
		channel, err := a.loadUpstreamChannel(req.ChannelID)
		if err != nil {
			response.Error(c, 404, "upstream channel not found")
			return
		}
		selectedChannel = channel
	}
	if err := a.db.Model(&user).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update user")
		return
	}
	if selectedChannel != nil {
		if err := a.db.Model(&model.UpstreamAccount{}).
			Where("user_id = ?", user.ID).
			Updates(map[string]interface{}{
				"channel": selectedChannel.Name,
				"base_url": selectedChannel.BaseURL,
				"api_key": req.APIKey,
				"status": model.UpstreamStatusActive,
			}).Error; err != nil {
			response.Error(c, 500, "failed to update user")
			return
		}
	}
	if err := a.syncUserAccessState(&user, updates); err != nil {
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

	userIDs := make([]uint, 0, len(orders))
	for _, order := range orders {
		userIDs = append(userIDs, order.UserID)
	}

	var upstreams []model.UpstreamAccount
	if len(userIDs) > 0 {
		a.db.Where("user_id IN ?", userIDs).Find(&upstreams)
	}
	upstreamByUserID := map[uint]model.UpstreamAccount{}
	for _, upstream := range upstreams {
		upstreamByUserID[upstream.UserID] = upstream
	}

	items := make([]orderResponse, 0, len(orders))
	for _, order := range orders {
		item := orderResponse{Order: order}
		if upstream, ok := upstreamByUserID[order.UserID]; ok {
			item.Upstream = &upstream
		}
		items = append(items, item)
	}
	response.OK(c, items)
}

func (a *AdminController) Plans(c *gin.Context) {
	var plans []model.Plan
	a.db.Order("price_cents asc").Find(&plans)
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
		BadgeText:          req.BadgeText,
		PlanType:           fallbackPlanType(req.PlanType),
		QuotaPeriod:        fallbackQuotaPeriod(req.QuotaPeriod),
		PriceCents:         req.PriceCents,
		SettlementUSDCents: req.SettlementUSDCents,
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
		"badge_text":           req.BadgeText,
		"plan_type":            fallbackPlanType(req.PlanType),
		"quota_period":         fallbackQuotaPeriod(req.QuotaPeriod),
		"price_cents":          req.PriceCents,
		"settlement_usd_cents": req.SettlementUSDCents,
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

	if req.ChannelID > 0 {
		channel, err := a.loadUpstreamChannel(req.ChannelID)
		if err != nil {
			response.Error(c, 404, "upstream channel not found")
			return
		}
		req.Channel = channel.Name
		req.BaseURL = channel.BaseURL
	}
	if req.Channel == "" || req.BaseURL == "" {
		response.Error(c, 400, "upstream channel is required")
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

func (a *AdminController) UpdateOrder(c *gin.Context) {
	var req updateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	var order model.Order
	if err := a.db.First(&order, c.Param("id")).Error; err != nil {
		response.Error(c, 404, "order not found")
		return
	}

	updates := map[string]interface{}{}
	if req.AdminNote != "" {
		updates["admin_note"] = req.AdminNote
	}
	if req.PlanID != nil {
		if order.Status == model.OrderStatusApproved {
			response.Error(c, 409, "approved order plan cannot be changed")
			return
		}
		updates["plan_id"] = *req.PlanID
	}
	if req.AmountCents != nil && *req.AmountCents > 0 {
		updates["amount_cents"] = *req.AmountCents
	}
	if req.ChannelID > 0 {
		channel, err := a.loadUpstreamChannel(req.ChannelID)
		if err != nil {
			response.Error(c, 404, "upstream channel not found")
			return
		}
		req.Channel = channel.Name
		req.BaseURL = channel.BaseURL
	}

	upstreamUpdates := map[string]interface{}{}
	if req.Channel != "" {
		upstreamUpdates["channel"] = req.Channel
	}
	if req.BaseURL != "" {
		upstreamUpdates["base_url"] = req.BaseURL
	}
	if req.APIKey != "" {
		upstreamUpdates["api_key"] = req.APIKey
	}

	if len(updates) == 0 && len(upstreamUpdates) == 0 {
		response.OK(c, nil)
		return
	}

	err := a.db.Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&order).Updates(updates).Error; err != nil {
				return err
			}
		}
		if len(upstreamUpdates) > 0 {
			if order.Status != model.OrderStatusApproved {
				return nil
			}
			upstream := model.UpstreamAccount{
				UserID: order.UserID,
				Status: model.UpstreamStatusActive,
			}
			if req.Channel != "" {
				upstream.Channel = req.Channel
			}
			if req.BaseURL != "" {
				upstream.BaseURL = req.BaseURL
			}
			if req.APIKey != "" {
				upstream.APIKey = req.APIKey
			}
			return tx.Where(model.UpstreamAccount{UserID: order.UserID}).Assign(upstreamUpdates).FirstOrCreate(&upstream).Error
		}
		return nil
	})
	if err != nil {
		response.Error(c, 500, "failed to update order")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) loadUpstreamChannel(id uint) (*model.UpstreamChannel, error) {
	var channel model.UpstreamChannel
	if err := a.db.Where("id = ? AND enabled = ?", id, true).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (a *AdminController) syncUserAccessState(user *model.User, updates map[string]interface{}) error {
	status := user.Status
	if value, ok := updates["status"].(string); ok && value != "" {
		status = value
	}

	var planID *uint
	planID = user.PlanID
	if value, ok := updates["plan_id"]; ok {
		switch typed := value.(type) {
		case nil:
			planID = nil
		case uint:
			planID = &typed
		}
	}

	var expiresAt *time.Time
	expiresAt = user.ExpiresAt
	if value, ok := updates["expires_at"]; ok {
		switch typed := value.(type) {
		case nil:
			expiresAt = nil
		case *time.Time:
			expiresAt = typed
		}
	}

	planChanged := !sameUintPointer(user.PlanID, planID)
	active := status == model.UserStatusApproved && planID != nil && expiresAt != nil && time.Now().Before(*expiresAt)
	if active && !planChanged {
		return nil
	}

	return a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.APIKey{}).
			Where("user_id = ?", user.ID).
			Update("status", model.APIKeyStatusDisabled).Error; err != nil {
			return err
		}
		return tx.Model(&model.UpstreamAccount{}).
			Where("user_id = ?", user.ID).
			Update("status", model.UpstreamStatusDisabled).Error
	})
}

func sameUintPointer(left, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func fallbackPlanType(value string) string {
	if value == "" {
		return "subscription"
	}
	return value
}

func fallbackQuotaPeriod(value string) string {
	if value == "daily" {
		return "daily"
	}
	return "weekly"
}

func fallbackProvider(value string) string {
	if value == "" {
		return "openai"
	}
	return value
}

func fallbackMultiplier(value float64) float64 {
	if value <= 0 {
		return 1
	}
	return value
}

func fallbackModelPricingStatus(value string) string {
	if value == model.ModelPricingStatusDisabled {
		return model.ModelPricingStatusDisabled
	}
	return model.ModelPricingStatusActive
}

func (a *AdminController) Upstreams(c *gin.Context) {
	var upstreams []model.UpstreamAccount
	a.db.Preload("User").Order("id desc").Find(&upstreams)
	response.OK(c, upstreams)
}

func (a *AdminController) UpstreamChannels(c *gin.Context) {
	var channels []model.UpstreamChannel
	a.db.Order("id desc").Find(&channels)
	response.OK(c, channels)
}

func (a *AdminController) ModelPricings(c *gin.Context) {
	var models []model.ModelPricing
	a.db.Order("provider asc, model asc").Find(&models)
	response.OK(c, gin.H{
		"items":                  models,
		"official_source":        service.OpenAIPricingSourceURL,
		"official_snapshot_size": len(service.OfficialOpenAIModelPrices()),
	})
}

func (a *AdminController) CreateModelPricing(c *gin.Context) {
	var req modelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	pricing := model.ModelPricing{
		ModelName:                req.ModelName,
		DisplayName:              req.DisplayName,
		Provider:                 fallbackProvider(req.Provider),
		InputUSDPerMillion:       req.InputUSDPerMillion,
		CachedInputUSDPerMillion: req.CachedInputUSDPerMillion,
		OutputUSDPerMillion:      req.OutputUSDPerMillion,
		BillingMultiplier:        fallbackMultiplier(req.BillingMultiplier),
		Status:                   fallbackModelPricingStatus(req.Status),
		Notes:                    req.Notes,
	}
	if err := a.db.Create(&pricing).Error; err != nil {
		response.Error(c, 500, "failed to create model pricing")
		return
	}
	response.Created(c, pricing)
}

func (a *AdminController) UpdateModelPricing(c *gin.Context) {
	var req modelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	updates := map[string]interface{}{
		"model":                        req.ModelName,
		"display_name":                 req.DisplayName,
		"provider":                     fallbackProvider(req.Provider),
		"input_usd_per_million":        req.InputUSDPerMillion,
		"cached_input_usd_per_million": req.CachedInputUSDPerMillion,
		"output_usd_per_million":       req.OutputUSDPerMillion,
		"billing_multiplier":           fallbackMultiplier(req.BillingMultiplier),
		"status":                       fallbackModelPricingStatus(req.Status),
		"notes":                        req.Notes,
	}
	if err := a.db.Model(&model.ModelPricing{}).Where("id = ?", c.Param("id")).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update model pricing")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) DeleteModelPricing(c *gin.Context) {
	if err := a.db.Delete(&model.ModelPricing{}, c.Param("id")).Error; err != nil {
		response.Error(c, 500, "failed to delete model pricing")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) SyncOfficialModelPricings(c *gin.Context) {
	count, err := service.SyncOfficialOpenAIModelPrices(a.db)
	if err != nil {
		response.Error(c, 500, "failed to sync official model pricing")
		return
	}
	response.OK(c, gin.H{
		"synced":          count,
		"official_source": service.OpenAIPricingSourceURL,
	})
}

func (a *AdminController) CreateUpstreamChannel(c *gin.Context) {
	var req upstreamChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	channel := model.UpstreamChannel{
		Name:    req.Name,
		BaseURL: req.BaseURL,
		Enabled: req.Enabled,
	}
	if err := a.db.Create(&channel).Error; err != nil {
		response.Error(c, 500, "failed to create upstream channel")
		return
	}
	response.Created(c, channel)
}

func (a *AdminController) UpdateUpstreamChannel(c *gin.Context) {
	var req upstreamChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	updates := map[string]interface{}{
		"name":     req.Name,
		"base_url": req.BaseURL,
		"enabled":  req.Enabled,
	}
	if err := a.db.Model(&model.UpstreamChannel{}).Where("id = ?", c.Param("id")).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update upstream channel")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) DeleteUpstreamChannel(c *gin.Context) {
	if err := a.db.Delete(&model.UpstreamChannel{}, c.Param("id")).Error; err != nil {
		response.Error(c, 500, "failed to delete upstream channel")
		return
	}
	response.OK(c, nil)
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
