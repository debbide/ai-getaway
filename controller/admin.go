package controller

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
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
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	APIKey    string `json:"api_key" binding:"required"`
	AdminNote string `json:"admin_note"`
}

type planRequest struct {
	Name               string `json:"name" binding:"required,min=2,max=64"`
	Code               string `json:"code"`
	BadgeText          string `json:"badge_text"`
	PlanType           string `json:"plan_type"`
	QuotaPeriod        string `json:"quota_period"`
	PublicChannelID    *uint  `json:"public_channel_id"`
	PollingPoolID      *uint  `json:"polling_pool_id"`
	PriceCents         int64  `json:"price_cents" binding:"min=0"`
	SettlementUSDCents int64  `json:"settlement_usd_cents"`
	DurationDays       int    `json:"duration_days"`
	Description        string `json:"description"`
	IsLottery          bool   `json:"is_lottery"`
	LotteryURL         string `json:"lottery_url"`
	FreePerUserLimit   int    `json:"free_per_user_limit"`
	FreeTotalLimit     int    `json:"free_total_limit"`
	Enabled            bool   `json:"enabled"`
}

type updateUserRequest struct {
	Username          string `json:"username"`
	Email             string `json:"email"`
	Password          string `json:"password"`
	Role              string `json:"role"`
	Status            string `json:"status"`
	EmailVerified     *bool  `json:"email_verified"`
	PlanID            *uint  `json:"plan_id"`
	PlanIDPresent     bool   `json:"-"`
	ResetSubscription bool   `json:"reset_subscription"`
	ChannelID         uint   `json:"channel_id"`
	UpstreamUsername  string `json:"upstream_username"`
	UpstreamPassword  string `json:"upstream_password"`
	APIKey            string `json:"api_key"`
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

type closeOrderRequest struct {
	AdminNote string `json:"admin_note"`
}

type updateOrderRequest struct {
	ChannelID   uint   `json:"channel_id"`
	Channel     string `json:"channel"`
	BaseURL     string `json:"base_url"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	APIKey      string `json:"api_key"`
	AdminNote   string `json:"admin_note"`
	PlanID      *uint  `json:"plan_id"`
	AmountCents *int64 `json:"amount_cents"`
}

type upstreamChannelRequest struct {
	Name           string `json:"name" binding:"required,min=2,max=64"`
	BaseURL        string `json:"base_url" binding:"required,url"`
	SupportsGPT    bool   `json:"supports_gpt"`
	SupportsClaude bool   `json:"supports_claude"`
	Enabled        bool   `json:"enabled"`
}

type publicChannelRequest struct {
	Name              string `json:"name" binding:"required,min=2,max=64"`
	BaseURL           string `json:"base_url" binding:"required,url"`
	APIKey            string `json:"api_key"`
	SupportsGPT       bool   `json:"supports_gpt"`
	SupportsClaude    bool   `json:"supports_claude"`
	TotalUSDCents     int64  `json:"total_usd_cents" binding:"min=0"`
	RemainingUSDCents int64  `json:"remaining_usd_cents" binding:"min=0"`
	Enabled           bool   `json:"enabled"`
}

type pollingPoolRequest struct {
	Name           string                      `json:"name" binding:"required,min=2,max=64"`
	SupportsGPT    bool                        `json:"supports_gpt"`
	SupportsClaude bool                        `json:"supports_claude"`
	Enabled        bool                        `json:"enabled"`
	Accounts       []pollingPoolAccountRequest `json:"accounts"`
}

type pollingPoolAccountRequest struct {
	ID                uint   `json:"id"`
	Name              string `json:"name"`
	BaseURL           string `json:"base_url"`
	APIKey            string `json:"api_key"`
	TotalUSDCents     int64  `json:"total_usd_cents"`
	RemainingUSDCents int64  `json:"remaining_usd_cents"`
	Enabled           bool   `json:"enabled"`
	SortOrder         int    `json:"sort_order"`
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
	Featured                 bool    `json:"featured"`
	Notes                    string  `json:"notes"`
}

type orderResponse struct {
	model.Order
	Upstream *model.UpstreamAccount `json:"Upstream,omitempty"`
}

type adminUserResponse struct {
	model.User
	Upstream *adminUpstreamResponse `json:"Upstream,omitempty"`
}

type adminUpstreamResponse struct {
	ID             uint       `json:"ID"`
	UserID         uint       `json:"UserID"`
	Channel        string     `json:"Channel"`
	BaseURL        string     `json:"BaseURL"`
	Username       string     `json:"Username"`
	Password       string     `json:"Password"`
	APIKey         string     `json:"APIKey"`
	SupportsGPT    bool       `json:"SupportsGPT"`
	SupportsClaude bool       `json:"SupportsClaude"`
	Status         string     `json:"Status"`
	LastUsedAt     *time.Time `json:"LastUsedAt"`
	CreatedAt      time.Time  `json:"CreatedAt"`
	UpdatedAt      time.Time  `json:"UpdatedAt"`
}

type adminPublicChannelResponse struct {
	ID                uint       `json:"ID"`
	Name              string     `json:"Name"`
	BaseURL           string     `json:"BaseURL"`
	APIKey            string     `json:"APIKey"`
	SupportsGPT       bool       `json:"SupportsGPT"`
	SupportsClaude    bool       `json:"SupportsClaude"`
	TotalUSDCents     int64      `json:"TotalUSDCents"`
	RemainingUSDCents int64      `json:"RemainingUSDCents"`
	Enabled           bool       `json:"Enabled"`
	LastUsedAt        *time.Time `json:"LastUsedAt"`
	CreatedAt         time.Time  `json:"CreatedAt"`
	UpdatedAt         time.Time  `json:"UpdatedAt"`
}

type adminPollingPoolResponse struct {
	ID                uint                              `json:"ID"`
	Name              string                            `json:"Name"`
	SupportsGPT       bool                              `json:"SupportsGPT"`
	SupportsClaude    bool                              `json:"SupportsClaude"`
	Enabled           bool                              `json:"Enabled"`
	TotalUSDCents     int64                             `json:"TotalUSDCents"`
	RemainingUSDCents int64                             `json:"RemainingUSDCents"`
	Accounts          []adminPollingPoolAccountResponse `json:"Accounts"`
	CreatedAt         time.Time                         `json:"CreatedAt"`
	UpdatedAt         time.Time                         `json:"UpdatedAt"`
}

type adminPollingPoolAccountResponse struct {
	ID                uint       `json:"ID"`
	Name              string     `json:"Name"`
	BaseURL           string     `json:"BaseURL"`
	APIKey            string     `json:"APIKey"`
	TotalUSDCents     int64      `json:"TotalUSDCents"`
	RemainingUSDCents int64      `json:"RemainingUSDCents"`
	Enabled           bool       `json:"Enabled"`
	SortOrder         int        `json:"SortOrder"`
	LastUsedAt        *time.Time `json:"LastUsedAt"`
	CreatedAt         time.Time  `json:"CreatedAt"`
	UpdatedAt         time.Time  `json:"UpdatedAt"`
}

type paginatedResponse struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func parsePageParams(c *gin.Context, defaultPageSize int) (int, int) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("page_size")))
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func applyPagination(query *gorm.DB, page, pageSize int) *gorm.DB {
	return query.Offset((page - 1) * pageSize).Limit(pageSize)
}

func (a *AdminController) Users(c *gin.Context) {
	query := a.db.Preload("Plan")
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR email LIKE ? OR CAST(id AS CHAR) LIKE ?", like, like, like)
	}
	if role := strings.TrimSpace(c.Query("role")); role == model.RoleUser || role == model.RoleAdmin {
		query = query.Where("role = ?", role)
	}
	if status := strings.TrimSpace(c.Query("status")); status == model.UserStatusPending || status == model.UserStatusApproved || status == model.UserStatusDisabled {
		query = query.Where("status = ?", status)
	}
	if plan := strings.TrimSpace(c.Query("plan")); plan != "" {
		if strings.ContainsAny(plan, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			query = query.Joins("LEFT JOIN plans ON plans.id = users.plan_id").Where("plans.name LIKE ?", "%"+plan+"%")
		} else {
			query = query.Where("plan_id = ?", plan)
		}
	}

	page, pageSize := parsePageParams(c, 10)
	var total int64
	query.Model(&model.User{}).Count(&total)

	var users []model.User
	applyPagination(query, page, pageSize).Order("id desc").Find(&users)

	userIDs := make([]uint, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	var upstreams []model.UpstreamAccount
	if len(userIDs) > 0 {
		a.db.Where("user_id IN ?", userIDs).Find(&upstreams)
	}
	upstreamByUserID := map[uint]model.UpstreamAccount{}
	for _, upstream := range upstreams {
		upstreamByUserID[upstream.UserID] = upstream
	}

	items := make([]adminUserResponse, 0, len(users))
	for _, user := range users {
		item := adminUserResponse{User: user}
		if upstream, ok := upstreamByUserID[user.ID]; ok {
			item.Upstream = mapAdminUpstream(upstream)
		}
		items = append(items, item)
	}
	response.OK(c, paginatedResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func (a *AdminController) UserUpstream(c *gin.Context) {
	var upstream model.UpstreamAccount
	if err := a.db.Where("user_id = ?", c.Param("id")).First(&upstream).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, 404, "upstream account not found")
			return
		}
		response.Error(c, 500, "failed to load upstream account")
		return
	}
	response.OK(c, mapAdminUpstream(upstream))
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

	var user model.User
	if err := a.db.First(&user, c.Param("id")).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, 404, "user not found")
			return
		}
		response.Error(c, 500, "failed to update user")
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
	now := time.Now()
	planChanged := req.PlanIDPresent && !sameUintPointer(user.PlanID, req.PlanID)
	shouldResetSubscription := req.ResetSubscription || planChanged
	if req.PlanIDPresent {
		if req.PlanID == nil {
			updates["plan_id"] = nil
			updates["expires_at"] = nil
			updates["subscription_started_at"] = nil
		} else {
			var plan model.Plan
			if err := a.db.First(&plan, *req.PlanID).Error; err != nil {
				response.Error(c, 404, "plan not found")
				return
			}
			updates["plan_id"] = plan.ID
			if shouldResetSubscription {
				startedAt := now
				expiresAt := now.AddDate(0, 0, plan.DurationDays)
				updates["subscription_started_at"] = &startedAt
				updates["expires_at"] = &expiresAt
			}
		}
	}
	if len(updates) == 0 {
		response.OK(c, nil)
		return
	}
	upstreamUpdateRequested := req.ChannelID != 0 || req.UpstreamUsername != "" || req.UpstreamPassword != "" || req.APIKey != ""
	var selectedChannel *model.UpstreamChannel
	if upstreamUpdateRequested || (planChanged && req.PlanID != nil) {
		if req.ChannelID == 0 || strings.TrimSpace(req.UpstreamUsername) == "" || req.UpstreamPassword == "" || req.APIKey == "" {
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
	if err := a.syncUserAccessState(&user, updates); err != nil {
		response.Error(c, 500, "failed to update user")
		return
	}
	if selectedChannel != nil {
		upstream := model.UpstreamAccount{
			UserID: user.ID,
			Status: model.UpstreamStatusActive,
		}
		if err := a.db.Where(model.UpstreamAccount{UserID: user.ID}).
			Assign(map[string]interface{}{
				"channel":         selectedChannel.Name,
				"base_url":        selectedChannel.BaseURL,
				"username":        strings.TrimSpace(req.UpstreamUsername),
				"password":        req.UpstreamPassword,
				"api_key":         req.APIKey,
				"supports_gpt":    selectedChannel.SupportsGPT,
				"supports_claude": selectedChannel.SupportsClaude,
				"status":          model.UpstreamStatusActive,
			}).
			FirstOrCreate(&upstream).Error; err != nil {
			response.Error(c, 500, "failed to update user")
			return
		}
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
	expirePendingPaymentOrders(a.db)
	query := a.db.Model(&model.Order{}).Preload("User").Preload("Plan")
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Joins("LEFT JOIN users ON users.id = orders.user_id").
			Joins("LEFT JOIN plans ON plans.id = orders.plan_id").
			Where("CAST(orders.id AS CHAR) LIKE ? OR CAST(orders.user_id AS CHAR) LIKE ? OR users.username LIKE ? OR users.email LIKE ? OR plans.name LIKE ? OR orders.payment_ref LIKE ?", like, like, like, like, like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("orders.status = ?", status)
	}
	if planID := strings.TrimSpace(c.Query("plan_id")); planID != "" {
		query = query.Where("orders.plan_id = ?", planID)
	}
	if paymentMethod := strings.TrimSpace(c.Query("payment_method")); paymentMethod != "" {
		query = query.Where("orders.payment_method = ?", paymentMethod)
	}

	page, pageSize := parsePageParams(c, 10)
	var total int64
	query.Count(&total)

	var orders []model.Order
	applyPagination(query, page, pageSize).Order("orders.id desc").Find(&orders)

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
	response.OK(c, paginatedResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func (a *AdminController) Plans(c *gin.Context) {
	query := a.db.Model(&model.Plan{}).Preload("PublicChannel").Preload("PollingPool.Accounts")
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ? OR description LIKE ?", like, like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("enabled = ?", status == "enabled")
	}
	if planType := strings.TrimSpace(c.Query("plan_type")); planType != "" {
		query = query.Where("plan_type = ?", planType)
	}
	switch strings.TrimSpace(c.Query("category")) {
	case "daily":
		query = query.Where("is_lottery = ? AND price_cents > ? AND quota_period = ? AND plan_type <> ?", false, 0, model.QuotaPeriodDaily, model.PlanTypePublic)
	case "weekly":
		query = query.Where("is_lottery = ? AND price_cents > ? AND quota_period <> ? AND quota_period <> ? AND plan_type <> ?", false, 0, model.QuotaPeriodDaily, model.QuotaPeriodPublic, model.PlanTypePublic)
	case "public":
		query = query.Where("is_lottery = ? AND price_cents > ? AND (quota_period = ? OR plan_type = ?)", false, 0, model.QuotaPeriodPublic, model.PlanTypePublic)
	case "lottery":
		query = query.Where("is_lottery = ?", true)
	case "free":
		query = query.Where("is_lottery = ? AND price_cents = ?", false, 0)
	}
	page, pageSize := parsePageParams(c, 10)
	var total int64
	query.Count(&total)
	var plans []model.Plan
	applyPagination(query, page, pageSize).Order("price_cents asc").Find(&plans)
	response.OK(c, paginatedResponse{
		Items:    plans,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func (a *AdminController) CreatePlan(c *gin.Context) {
	var req planRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if err := a.validatePlanRequest(req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	plan := model.Plan{
		Name:               req.Name,
		Code:               req.Code,
		BadgeText:          req.BadgeText,
		PlanType:           fallbackPlanType(req.PlanType),
		QuotaPeriod:        fallbackQuotaPeriod(req.QuotaPeriod),
		PublicChannelID:    normalizedPublicChannelID(req),
		PollingPoolID:      normalizedPollingPoolID(req),
		PriceCents:         req.PriceCents,
		SettlementUSDCents: req.SettlementUSDCents,
		DurationDays:       fallbackDurationDays(req),
		Description:        req.Description,
		IsLottery:          req.IsLottery,
		LotteryURL:         strings.TrimSpace(req.LotteryURL),
		FreePerUserLimit:   normalizedFreePerUserLimit(req),
		FreeTotalLimit:     normalizedFreeTotalLimit(req),
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
	if err := a.validatePlanRequest(req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	updates := map[string]interface{}{
		"name":                 req.Name,
		"code":                 req.Code,
		"badge_text":           req.BadgeText,
		"plan_type":            fallbackPlanType(req.PlanType),
		"quota_period":         fallbackQuotaPeriod(req.QuotaPeriod),
		"public_channel_id":    normalizedPublicChannelID(req),
		"polling_pool_id":      normalizedPollingPoolID(req),
		"price_cents":          req.PriceCents,
		"settlement_usd_cents": req.SettlementUSDCents,
		"duration_days":        fallbackDurationDays(req),
		"description":          req.Description,
		"is_lottery":           req.IsLottery,
		"lottery_url":          strings.TrimSpace(req.LotteryURL),
		"free_per_user_limit":  normalizedFreePerUserLimit(req),
		"free_total_limit":     normalizedFreeTotalLimit(req),
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
	if order.Status != model.OrderStatusPendingReview && order.Status != model.OrderStatusManualReview && order.Status != model.OrderStatusPaidLate {
		response.Error(c, 409, "order already reviewed")
		return
	}

	now := time.Now()
	err := a.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Order{}).
			Where("id = ? AND status IN ?", order.ID, []string{model.OrderStatusPendingReview, model.OrderStatusManualReview, model.OrderStatusPaidLate}).
			Updates(map[string]interface{}{
				"status":         model.OrderStatusApproved,
				"admin_note":     req.AdminNote,
				"approved_at":    &now,
				"approved_by_id": admin.ID,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		if err := applyApprovedSubscription(tx, &order, order.Plan, now); err != nil {
			return err
		}

		upstream := model.UpstreamAccount{
			UserID:         order.UserID,
			Channel:        req.Channel,
			BaseURL:        req.BaseURL,
			Username:       req.Username,
			Password:       req.Password,
			APIKey:         req.APIKey,
			SupportsGPT:    true,
			SupportsClaude: false,
			Status:         model.UpstreamStatusActive,
		}
		if req.ChannelID > 0 {
			if channel, err := a.loadUpstreamChannel(req.ChannelID); err == nil {
				upstream.SupportsGPT = channel.SupportsGPT
				upstream.SupportsClaude = channel.SupportsClaude
			}
		}
		return tx.Where(model.UpstreamAccount{UserID: order.UserID}).Assign(upstream).FirstOrCreate(&upstream).Error
	})
	if err != nil {
		response.Error(c, 500, "failed to approve order")
		return
	}

	go service.SendOrderApprovedUserNotification(a.db, order.ID, req.AdminNote)
	response.OK(c, gin.H{"status": model.OrderStatusApproved})
}

func (a *AdminController) CompleteOrderPayment(c *gin.Context) {
	admin := c.MustGet("user").(model.User)
	var order model.Order
	if err := a.db.Preload("Plan").First(&order, c.Param("id")).Error; err != nil {
		response.Error(c, 404, "order not found")
		return
	}
	if order.Status == model.OrderStatusPendingPayment && expirePendingPaymentOrder(a.db, &order) {
		response.Error(c, 409, "order payment timeout")
		return
	}
	if order.Plan.PlanType == model.PlanTypePublic && (order.Status == model.OrderStatusPendingReview || order.Status == model.OrderStatusManualReview || order.Status == model.OrderStatusPaidLate) {
		if err := approvePublicPaidOrder(a.db, &order, &admin.ID); err != nil {
			if err.Error() == "public plan sold out" {
				response.Error(c, 409, err.Error())
				return
			}
			response.Error(c, 500, "failed to complete payment")
			return
		}
		response.OK(c, gin.H{"order": order})
		return
	}
	if order.Status != model.OrderStatusPendingPayment {
		response.Error(c, 409, "order not pending payment")
		return
	}
	if err := completePaidOrder(a.db, &order, nil, &admin.ID); err != nil {
		if err.Error() == "public plan sold out" {
			response.Error(c, 409, err.Error())
			return
		}
		response.Error(c, 500, "failed to complete payment")
		return
	}
	response.OK(c, gin.H{"order": order})
}

func approvePublicPaidOrder(db *gorm.DB, order *model.Order, approvedByID *uint) error {
	if order == nil {
		return gorm.ErrRecordNotFound
	}
	if order.Plan.ID == 0 {
		if err := db.Preload("Plan").First(order, order.ID).Error; err != nil {
			return err
		}
	}
	if order.Plan.PlanType != model.PlanTypePublic {
		return errors.New("order not pending payment")
	}

	now := time.Now()
	err := db.Transaction(func(tx *gorm.DB) error {
		var plan model.Plan
		if err := tx.Preload("PublicChannel").Preload("PollingPool.Accounts").First(&plan, order.PlanID).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{
			"status":      model.OrderStatusApproved,
			"approved_at": &now,
		}
		if approvedByID != nil {
			updates["approved_by_id"] = *approvedByID
		}
		result := tx.Model(&model.Order{}).
			Where("id = ? AND status IN ?", order.ID, []string{model.OrderStatusPendingReview, model.OrderStatusManualReview, model.OrderStatusPaidLate}).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := service.DeductPlanChannelQuota(tx, plan); err != nil {
			return err
		}
		return applyApprovedSubscription(tx, order, plan, now)
	})
	if err != nil {
		return err
	}
	order.Status = model.OrderStatusApproved
	order.ApprovedAt = &now
	go service.SendOrderApprovedUserNotification(db, order.ID, order.AdminNote)
	return nil
}

func (a *AdminController) RejectOrder(c *gin.Context) {
	var req rejectOrderRequest
	_ = c.ShouldBindJSON(&req)
	if req.AdminNote == "" {
		req.AdminNote = c.Query("note")
	}

	var order model.Order
	if err := a.db.First(&order, c.Param("id")).Error; err != nil {
		response.Error(c, 404, "order not found")
		return
	}
	err := a.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Order{}).
			Where("id = ? AND status IN ?", order.ID, []string{model.OrderStatusPendingReview, model.OrderStatusManualReview, model.OrderStatusPaidLate}).
			Updates(map[string]interface{}{
				"status":     model.OrderStatusRejected,
				"admin_note": req.AdminNote,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if order.PaymentMethod == "free" {
			return refreshFreePlanClaimedCount(tx, order.PlanID)
		}
		return nil
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, 409, "order not rejectable")
			return
		}
		response.Error(c, 500, "failed to reject order")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) CloseOrder(c *gin.Context) {
	var req closeOrderRequest
	_ = c.ShouldBindJSON(&req)
	note := strings.TrimSpace(req.AdminNote)
	if note == "" {
		note = "管理员关闭订单"
	}

	var order model.Order
	if err := a.db.First(&order, c.Param("id")).Error; err != nil {
		response.Error(c, 404, "order not found")
		return
	}

	updates := map[string]interface{}{
		"admin_note": note,
	}
	switch order.Status {
	case model.OrderStatusPendingPayment:
		updates["status"] = model.OrderStatusPaymentTimeout
	case model.OrderStatusPendingReview, model.OrderStatusManualReview, model.OrderStatusPaidLate:
		updates["status"] = model.OrderStatusRejected
	default:
		response.Error(c, 409, "order not closable")
		return
	}

	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Order{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
			return err
		}
		if order.PaymentMethod == "free" {
			return refreshFreePlanClaimedCount(tx, order.PlanID)
		}
		return nil
	})
	if err != nil {
		response.Error(c, 500, "failed to close order")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) DeleteOrder(c *gin.Context) {
	var order model.Order
	if err := a.db.First(&order, c.Param("id")).Error; err != nil {
		response.Error(c, 404, "order not found")
		return
	}
	if order.Status == model.OrderStatusApproved {
		response.Error(c, 409, "approved order cannot be deleted")
		return
	}
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.Order{}, order.ID).Error; err != nil {
			return err
		}
		if order.PaymentMethod == "free" {
			return refreshFreePlanClaimedCount(tx, order.PlanID)
		}
		return nil
	})
	if err != nil {
		response.Error(c, 500, "failed to delete order")
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
		if order.Status != model.OrderStatusPendingPayment || order.PaymentURLGeneratedAt != nil {
			response.Error(c, 409, "paid or payment-started order plan cannot be changed")
			return
		}
		updates["plan_id"] = *req.PlanID
	}
	if req.AmountCents != nil && *req.AmountCents > 0 {
		if order.Status != model.OrderStatusPendingPayment || order.PaymentURLGeneratedAt != nil {
			response.Error(c, 409, "paid or payment-started order amount cannot be changed")
			return
		}
		updates["amount_cents"] = *req.AmountCents
	}
	upstreamUpdates := map[string]interface{}{}
	if req.ChannelID > 0 {
		channel, err := a.loadUpstreamChannel(req.ChannelID)
		if err != nil {
			response.Error(c, 404, "upstream channel not found")
			return
		}
		req.Channel = channel.Name
		req.BaseURL = channel.BaseURL
		upstreamUpdates["supports_gpt"] = channel.SupportsGPT
		upstreamUpdates["supports_claude"] = channel.SupportsClaude
	}

	if req.Channel != "" {
		upstreamUpdates["channel"] = req.Channel
	}
	if req.BaseURL != "" {
		upstreamUpdates["base_url"] = req.BaseURL
	}
	if req.Username != "" {
		upstreamUpdates["username"] = req.Username
	}
	if req.Password != "" {
		upstreamUpdates["password"] = req.Password
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
			if req.Username != "" {
				upstream.Username = req.Username
			}
			if req.Password != "" {
				upstream.Password = req.Password
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
	if value == model.PlanTypePublic {
		return model.PlanTypePublic
	}
	return model.PlanTypeSubscription
}

func fallbackQuotaPeriod(value string) string {
	switch value {
	case model.QuotaPeriodDaily:
		return model.QuotaPeriodDaily
	case model.QuotaPeriodPublic:
		return model.QuotaPeriodPublic
	default:
		return model.QuotaPeriodWeekly
	}
}

func fallbackDurationDays(req planRequest) int {
	if fallbackPlanType(req.PlanType) == model.PlanTypePublic {
		return 1
	}
	if req.DurationDays < 1 {
		return 1
	}
	return req.DurationDays
}

func normalizedPublicChannelID(req planRequest) *uint {
	if fallbackPlanType(req.PlanType) != model.PlanTypePublic || req.PollingPoolID != nil && *req.PollingPoolID > 0 || req.PublicChannelID == nil || *req.PublicChannelID == 0 {
		return nil
	}
	return req.PublicChannelID
}

func normalizedPollingPoolID(req planRequest) *uint {
	if fallbackPlanType(req.PlanType) != model.PlanTypePublic || req.PollingPoolID == nil || *req.PollingPoolID == 0 {
		return nil
	}
	return req.PollingPoolID
}

func normalizedFreePerUserLimit(req planRequest) int {
	if req.IsLottery || req.PriceCents > 0 {
		return 0
	}
	if req.FreePerUserLimit <= 0 {
		return 1
	}
	return req.FreePerUserLimit
}

func normalizedFreeTotalLimit(req planRequest) int {
	if req.IsLottery || req.PriceCents > 0 || req.FreeTotalLimit < 0 {
		return 0
	}
	return req.FreeTotalLimit
}

func (a *AdminController) validatePlanRequest(req planRequest) error {
	planType := fallbackPlanType(req.PlanType)
	if req.SettlementUSDCents <= 0 {
		return errors.New("settlement usd quota required")
	}
	if !req.IsLottery && req.PriceCents < 0 {
		return errors.New("plan price required")
	}
	if req.IsLottery && strings.TrimSpace(req.LotteryURL) == "" {
		return errors.New("lottery url required")
	}
	if planType == model.PlanTypePublic {
		if fallbackQuotaPeriod(req.QuotaPeriod) != model.QuotaPeriodPublic {
			return errors.New("public plan quota period required")
		}
		hasPublicChannel := req.PublicChannelID != nil && *req.PublicChannelID > 0
		hasPollingPool := req.PollingPoolID != nil && *req.PollingPoolID > 0
		if hasPublicChannel == hasPollingPool {
			return errors.New("public channel or polling pool required")
		}
		if hasPublicChannel {
			var channel model.PublicChannel
			if err := a.db.Where("id = ? AND enabled = ?", *req.PublicChannelID, true).First(&channel).Error; err != nil {
				return errors.New("public channel not found")
			}
		} else {
			var pool model.PollingPool
			if err := a.db.Where("id = ? AND enabled = ?", *req.PollingPoolID, true).First(&pool).Error; err != nil {
				return errors.New("polling pool not found")
			}
		}
		return nil
	}
	if fallbackQuotaPeriod(req.QuotaPeriod) == model.QuotaPeriodPublic {
		return errors.New("public quota period only supports public plan")
	}
	if req.DurationDays < 1 {
		return errors.New("duration days required")
	}
	return nil
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
	query := a.db.Model(&model.UpstreamAccount{}).Preload("User")
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Joins("LEFT JOIN users ON users.id = upstream_accounts.user_id").
			Where("upstream_accounts.channel LIKE ? OR upstream_accounts.base_url LIKE ? OR upstream_accounts.username LIKE ? OR users.username LIKE ? OR users.email LIKE ? OR CAST(upstream_accounts.id AS CHAR) LIKE ?", like, like, like, like, like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("upstream_accounts.status = ?", status)
	}
	page, pageSize := parsePageParams(c, 10)
	var total int64
	query.Count(&total)
	var upstreams []model.UpstreamAccount
	applyPagination(query, page, pageSize).Order("upstream_accounts.id desc").Find(&upstreams)
	response.OK(c, paginatedResponse{
		Items:    upstreams,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func mapAdminUpstream(upstream model.UpstreamAccount) *adminUpstreamResponse {
	return &adminUpstreamResponse{
		ID:             upstream.ID,
		UserID:         upstream.UserID,
		Channel:        upstream.Channel,
		BaseURL:        upstream.BaseURL,
		Username:       upstream.Username,
		Password:       upstream.Password,
		APIKey:         upstream.APIKey,
		SupportsGPT:    upstream.SupportsGPT,
		SupportsClaude: upstream.SupportsClaude,
		Status:         upstream.Status,
		LastUsedAt:     upstream.LastUsedAt,
		CreatedAt:      upstream.CreatedAt,
		UpdatedAt:      upstream.UpdatedAt,
	}
}

func mapAdminPublicChannel(channel model.PublicChannel) adminPublicChannelResponse {
	return adminPublicChannelResponse{
		ID:                channel.ID,
		Name:              channel.Name,
		BaseURL:           channel.BaseURL,
		APIKey:            channel.APIKey,
		SupportsGPT:       channel.SupportsGPT,
		SupportsClaude:    channel.SupportsClaude,
		TotalUSDCents:     channel.TotalUSDCents,
		RemainingUSDCents: channel.RemainingUSDCents,
		Enabled:           channel.Enabled,
		LastUsedAt:        channel.LastUsedAt,
		CreatedAt:         channel.CreatedAt,
		UpdatedAt:         channel.UpdatedAt,
	}
}

func mapAdminPollingPool(pool model.PollingPool) adminPollingPoolResponse {
	items := make([]adminPollingPoolAccountResponse, 0, len(pool.Accounts))
	var total int64
	var remaining int64
	for _, account := range pool.Accounts {
		total += account.TotalUSDCents
		if account.Enabled {
			remaining += account.RemainingUSDCents
		}
		items = append(items, adminPollingPoolAccountResponse{
			ID:                account.ID,
			Name:              account.Name,
			BaseURL:           account.BaseURL,
			APIKey:            account.APIKey,
			TotalUSDCents:     account.TotalUSDCents,
			RemainingUSDCents: account.RemainingUSDCents,
			Enabled:           account.Enabled,
			SortOrder:         account.SortOrder,
			LastUsedAt:        account.LastUsedAt,
			CreatedAt:         account.CreatedAt,
			UpdatedAt:         account.UpdatedAt,
		})
	}
	return adminPollingPoolResponse{
		ID:                pool.ID,
		Name:              pool.Name,
		SupportsGPT:       pool.SupportsGPT,
		SupportsClaude:    pool.SupportsClaude,
		Enabled:           pool.Enabled,
		TotalUSDCents:     total,
		RemainingUSDCents: remaining,
		Accounts:          items,
		CreatedAt:         pool.CreatedAt,
		UpdatedAt:         pool.UpdatedAt,
	}
}

func (a *AdminController) UpstreamChannels(c *gin.Context) {
	query := a.db.Model(&model.UpstreamChannel{})
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR base_url LIKE ? OR CAST(id AS CHAR) LIKE ?", like, like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("enabled = ?", status == "enabled")
	}
	page, pageSize := parsePageParams(c, 10)
	var total int64
	query.Count(&total)
	var channels []model.UpstreamChannel
	applyPagination(query, page, pageSize).Order("id desc").Find(&channels)
	response.OK(c, paginatedResponse{
		Items:    channels,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func (a *AdminController) PublicChannels(c *gin.Context) {
	query := a.db.Model(&model.PublicChannel{})
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR base_url LIKE ? OR CAST(id AS CHAR) LIKE ?", like, like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("enabled = ?", status == "enabled")
	}
	page, pageSize := parsePageParams(c, 10)
	var total int64
	query.Count(&total)
	var channels []model.PublicChannel
	applyPagination(query, page, pageSize).Order("id desc").Find(&channels)
	items := make([]adminPublicChannelResponse, 0, len(channels))
	for _, channel := range channels {
		items = append(items, mapAdminPublicChannel(channel))
	}
	response.OK(c, paginatedResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func (a *AdminController) PollingPools(c *gin.Context) {
	query := a.db.Model(&model.PollingPool{}).Preload("Accounts", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order asc, id asc")
	})
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR CAST(id AS CHAR) LIKE ?", like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("enabled = ?", status == "enabled")
	}
	page, pageSize := parsePageParams(c, 10)
	var total int64
	query.Count(&total)
	var pools []model.PollingPool
	applyPagination(query, page, pageSize).Order("id desc").Find(&pools)
	items := make([]adminPollingPoolResponse, 0, len(pools))
	for _, pool := range pools {
		items = append(items, mapAdminPollingPool(pool))
	}
	response.OK(c, paginatedResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
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
		Featured:                 req.Featured,
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
		"featured":                     req.Featured,
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
		Name:           req.Name,
		BaseURL:        req.BaseURL,
		SupportsGPT:    req.SupportsGPT,
		SupportsClaude: req.SupportsClaude,
		Enabled:        req.Enabled,
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
		"name":            req.Name,
		"base_url":        req.BaseURL,
		"supports_gpt":    req.SupportsGPT,
		"supports_claude": req.SupportsClaude,
		"enabled":         req.Enabled,
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

func (a *AdminController) CreatePublicChannel(c *gin.Context) {
	var req publicChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if req.APIKey == "" {
		response.Error(c, 400, "api key required")
		return
	}
	if req.RemainingUSDCents <= 0 {
		req.RemainingUSDCents = req.TotalUSDCents
	}
	if req.RemainingUSDCents > req.TotalUSDCents {
		response.Error(c, 400, "remaining quota cannot exceed total quota")
		return
	}

	channel := model.PublicChannel{
		Name:              req.Name,
		BaseURL:           req.BaseURL,
		APIKey:            req.APIKey,
		SupportsGPT:       req.SupportsGPT,
		SupportsClaude:    req.SupportsClaude,
		TotalUSDCents:     req.TotalUSDCents,
		RemainingUSDCents: req.RemainingUSDCents,
		Enabled:           req.Enabled,
	}
	if err := a.db.Create(&channel).Error; err != nil {
		response.Error(c, 500, "failed to create public channel")
		return
	}
	response.Created(c, mapAdminPublicChannel(channel))
}

func (a *AdminController) UpdatePublicChannel(c *gin.Context) {
	var req publicChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if req.RemainingUSDCents > req.TotalUSDCents {
		response.Error(c, 400, "remaining quota cannot exceed total quota")
		return
	}

	updates := map[string]interface{}{
		"name":                req.Name,
		"base_url":            req.BaseURL,
		"supports_gpt":        req.SupportsGPT,
		"supports_claude":     req.SupportsClaude,
		"total_usd_cents":     req.TotalUSDCents,
		"remaining_usd_cents": req.RemainingUSDCents,
		"enabled":             req.Enabled,
	}
	if req.APIKey != "" {
		updates["api_key"] = req.APIKey
	}
	if err := a.db.Model(&model.PublicChannel{}).Where("id = ?", c.Param("id")).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update public channel")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) DeletePublicChannel(c *gin.Context) {
	if err := a.db.Delete(&model.PublicChannel{}, c.Param("id")).Error; err != nil {
		response.Error(c, 500, "failed to delete public channel")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) CreatePollingPool(c *gin.Context) {
	var req pollingPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	accounts, err := normalizePollingPoolAccounts(req.Accounts, true)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	pool := model.PollingPool{
		Name:           strings.TrimSpace(req.Name),
		SupportsGPT:    req.SupportsGPT,
		SupportsClaude: req.SupportsClaude,
		Enabled:        req.Enabled,
		Accounts:       accounts,
	}
	if err := a.db.Create(&pool).Error; err != nil {
		response.Error(c, 500, "failed to create polling pool")
		return
	}
	a.db.Preload("Accounts", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order asc, id asc")
	}).First(&pool, pool.ID)
	response.Created(c, mapAdminPollingPool(pool))
}

func (a *AdminController) UpdatePollingPool(c *gin.Context) {
	var req pollingPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	poolID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || poolID == 0 {
		response.Error(c, 400, "invalid polling pool")
		return
	}
	accounts, err := normalizePollingPoolAccounts(req.Accounts, false)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	err = a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.PollingPool{}).Where("id = ?", poolID).Updates(map[string]interface{}{
			"name":            strings.TrimSpace(req.Name),
			"supports_gpt":    req.SupportsGPT,
			"supports_claude": req.SupportsClaude,
			"enabled":         req.Enabled,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("polling_pool_id = ?", poolID).Delete(&model.PollingPoolAccount{}).Error; err != nil {
			return err
		}
		for i := range accounts {
			accounts[i].PollingPoolID = uint(poolID)
		}
		if len(accounts) > 0 {
			return tx.Create(&accounts).Error
		}
		return nil
	})
	if err != nil {
		response.Error(c, 500, "failed to update polling pool")
		return
	}
	response.OK(c, nil)
}

func (a *AdminController) DeletePollingPool(c *gin.Context) {
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("polling_pool_id = ?", c.Param("id")).Delete(&model.PollingPoolAccount{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PollingPool{}, c.Param("id")).Error
	})
	if err != nil {
		response.Error(c, 500, "failed to delete polling pool")
		return
	}
	response.OK(c, nil)
}

func normalizePollingPoolAccounts(input []pollingPoolAccountRequest, _ bool) ([]model.PollingPoolAccount, error) {
	if len(input) == 0 {
		return nil, errors.New("polling pool account required")
	}
	accounts := make([]model.PollingPoolAccount, 0, len(input))
	for i, item := range input {
		name := strings.TrimSpace(item.Name)
		baseURL := strings.TrimSpace(item.BaseURL)
		apiKey := strings.TrimSpace(item.APIKey)
		if name == "" {
			name = "账号" + strconv.Itoa(i+1)
		}
		if baseURL == "" {
			return nil, errors.New("account base url required")
		}
		if apiKey == "" {
			return nil, errors.New("account api key required")
		}
		if item.TotalUSDCents < 0 || item.RemainingUSDCents < 0 {
			return nil, errors.New("account quota invalid")
		}
		if item.RemainingUSDCents <= 0 {
			item.RemainingUSDCents = item.TotalUSDCents
		}
		if item.RemainingUSDCents > item.TotalUSDCents {
			return nil, errors.New("account remaining quota cannot exceed total quota")
		}
		accounts = append(accounts, model.PollingPoolAccount{
			Name:              name,
			BaseURL:           baseURL,
			APIKey:            apiKey,
			TotalUSDCents:     item.TotalUSDCents,
			RemainingUSDCents: item.RemainingUSDCents,
			Enabled:           item.Enabled,
			SortOrder:         item.SortOrder,
		})
	}
	return accounts, nil
}

func (a *AdminController) APIKeys(c *gin.Context) {
	var keys []model.APIKey
	a.db.Preload("User").Order("id desc").Find(&keys)
	response.OK(c, keys)
}

func (a *AdminController) Stats(c *gin.Context) {
	var users, admins, approvedUsers, orders, pendingOrders, apiKeys, calls, plans, enabledPlans int64
	a.db.Model(&model.User{}).Where("role = ?", model.RoleUser).Count(&users)
	a.db.Model(&model.User{}).Where("role = ?", model.RoleAdmin).Count(&admins)
	a.db.Model(&model.User{}).Where("role = ? AND status = ?", model.RoleUser, model.UserStatusApproved).Count(&approvedUsers)
	a.db.Model(&model.Order{}).Count(&orders)
	a.db.Model(&model.Order{}).Where("status IN ?", []string{
		model.OrderStatusPendingReview,
		model.OrderStatusManualReview,
		model.OrderStatusPaidLate,
	}).Count(&pendingOrders)
	a.db.Model(&model.APIKey{}).Count(&apiKeys)
	a.db.Model(&model.APILog{}).Count(&calls)
	a.db.Model(&model.Plan{}).Count(&plans)
	a.db.Model(&model.Plan{}).Where("enabled = ?", true).Count(&enabledPlans)
	response.OK(c, gin.H{
		"users":                  users,
		"admins":                 admins,
		"approved_users":         approvedUsers,
		"orders":                 orders,
		"pending_orders":         pendingOrders,
		"api_keys":               apiKeys,
		"calls":                  calls,
		"plans":                  plans,
		"enabled_plans":          enabledPlans,
		"active_api_connections": service.ActiveAPIConnections(),
		"system_load":            service.CurrentSystemLoad(),
	})
}
