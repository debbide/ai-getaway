package controller

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"ai-gateway/config"
	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/service"
	"ai-gateway/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AuthController struct {
	cfg         config.Config
	db          *gorm.DB
	redisClient *redis.Client
}

func NewAuthController(cfg config.Config, db *gorm.DB, redisClient *redis.Client) *AuthController {
	return &AuthController{cfg: cfg, db: db, redisClient: redisClient}
}

type registerRequest struct {
	Username    string `json:"username" binding:"required,min=2,max=64"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	EmailCode   string `json:"email_code" binding:"required,len=6"`
	ChallengeID string `json:"challenge_id" binding:"required"`
	CaptchaX    int    `json:"captcha_x"`
}

type loginRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required"`
	ChallengeID string `json:"challenge_id" binding:"required"`
	CaptchaX    int    `json:"captcha_x"`
}

type emailCodeRequest struct {
	Email       string `json:"email" binding:"required,email"`
	ChallengeID string `json:"challenge_id" binding:"required"`
	CaptchaX    int    `json:"captcha_x"`
}

type changePasswordRequest struct {
	OldPassword     string `json:"old_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=7"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

type updateBalanceBillingGroupRequest struct {
	BillingGroupID *uint `json:"billing_group_id"`
}

func (a *AuthController) SendEmailCode(c *gin.Context) {
	if err := ensureSystemSettingColumns(a.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(a.db)
	if !setting.AllowRegistration {
		response.Error(c, 403, "registration disabled")
		return
	}

	var req emailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if !VerifySlideCaptcha(a.redisClient, req.ChallengeID, req.CaptchaX) {
		response.Error(c, 400, "invalid slide captcha")
		return
	}
	if !emailAllowedByWhitelist(req.Email, setting.EmailWhitelist) {
		response.Error(c, 400, emailWhitelistErrorMessage(setting.EmailWhitelist))
		return
	}

	code, err := randomCode()
	if err != nil {
		response.Error(c, 500, "failed to create email code")
		return
	}
	verification := model.EmailVerification{
		Email:     req.Email,
		CodeHash:  utils.HashToken(code),
		Purpose:   "register",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := a.db.Create(&verification).Error; err != nil {
		response.Error(c, 500, "failed to save email code")
		return
	}

	if err := service.NewMailer(setting).SendVerification(req.Email, code); err != nil {
		response.Error(c, 500, "failed to send email: "+err.Error())
		return
	}

	response.OK(c, gin.H{"expires_in": 600})
}

func (a *AuthController) Register(c *gin.Context) {
	if err := ensureSystemSettingColumns(a.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(a.db)
	if !setting.AllowRegistration {
		response.Error(c, 403, "registration disabled")
		return
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if !VerifySlideCaptcha(a.redisClient, req.ChallengeID, req.CaptchaX) {
		response.Error(c, 400, "invalid slide captcha")
		return
	}
	if !emailAllowedByWhitelist(req.Email, setting.EmailWhitelist) {
		response.Error(c, 400, emailWhitelistErrorMessage(setting.EmailWhitelist))
		return
	}
	if !a.verifyEmailCode(req.Email, req.EmailCode, "register") {
		response.Error(c, 400, "invalid email verification code")
		return
	}

	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		response.Error(c, 500, "failed to hash password")
		return
	}

	user := model.User{
		Username:      req.Username,
		Email:         req.Email,
		PasswordHash:  passwordHash,
		Role:          model.RoleUser,
		Status:        model.UserStatusApproved,
		EmailVerified: true,
	}
	if err := a.db.Create(&user).Error; err != nil {
		response.Error(c, 409, "email already registered")
		return
	}

	response.Created(c, gin.H{"id": user.ID, "status": user.Status})
}

func emailWhitelistErrorMessage(whitelistJSON string) string {
	domains := parseEmailWhitelist(whitelistJSON)
	if len(domains) == 0 {
		return "email domain not allowed"
	}
	return "email domain not allowed: " + strings.Join(domains, ", ")
}

func (a *AuthController) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if !VerifySlideCaptcha(a.redisClient, req.ChallengeID, req.CaptchaX) {
		response.Error(c, 400, "invalid slide captcha")
		return
	}

	var user model.User
	if err := a.db.Preload("Plan").Preload("PublicChannel").Where("email = ?", req.Email).First(&user).Error; err != nil {
		response.Error(c, 401, "invalid credentials")
		return
	}
	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		response.Error(c, 401, "invalid credentials")
		return
	}
	if user.Status == model.UserStatusDisabled {
		response.Error(c, 403, "user disabled")
		return
	}
	if !user.EmailVerified {
		response.Error(c, 403, "email not verified")
		return
	}

	token, err := utils.GenerateJWT(user.ID, user.Role, a.cfg.JWTSecret)
	if err != nil {
		response.Error(c, 500, "failed to generate token")
		return
	}

	response.OK(c, gin.H{
		"token": token,
		"user":  publicUser(user),
	})
}

func (a *AuthController) Me(c *gin.Context) {
	base := c.MustGet("user").(model.User)
	var user model.User
	if err := a.db.Preload("Plan").Preload("PublicChannel").First(&user, base.ID).Error; err != nil {
		response.Error(c, 404, "user not found")
		return
	}
	body := publicUser(user)
	subscriptionStartedAt := service.SubscriptionStartAt(a.db, user, time.Now())
	if subscriptionStartedAt != nil {
		body["subscription_started_at"] = subscriptionStartedAt
	}
	if service.HasCallableAccess(user, time.Now()) {
		if usage, ok := service.UserAccessQuotaUsage(a.db, user, time.Now()); ok {
			body["quota_usage"] = usage
		}
		if subscriptionStartedAt != nil && user.ExpiresAt != nil {
			if user.Plan != nil {
				body["total_quota_usage"] = service.PlanTotalQuotaUsage(a.db, user.ID, user.Plan, *subscriptionStartedAt, *user.ExpiresAt)
			}
		}
	}
	body["claimed_free_plan_ids"] = a.claimedFreePlanIDs(user.ID)
	response.OK(c, body)
}

func (a *AuthController) claimedFreePlanIDs(userID uint) []uint {
	var orders []model.Order
	if err := a.db.Select("plan_id").
		Where("user_id = ? AND payment_method = ? AND status IN ?", userID, "free", []string{
			model.OrderStatusPendingReview,
			model.OrderStatusApproved,
			model.OrderStatusManualReview,
		}).
		Find(&orders).Error; err != nil {
		return []uint{}
	}
	seen := map[uint]bool{}
	ids := make([]uint, 0, len(orders))
	for _, order := range orders {
		if order.PlanID != nil && !seen[*order.PlanID] {
			seen[*order.PlanID] = true
			ids = append(ids, *order.PlanID)
		}
	}
	return ids
}

func (a *AuthController) ChangePassword(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		response.Error(c, 400, "password confirmation mismatch")
		return
	}

	var freshUser model.User
	if err := a.db.First(&freshUser, user.ID).Error; err != nil {
		response.Error(c, 404, "user not found")
		return
	}
	if !utils.CheckPassword(freshUser.PasswordHash, req.OldPassword) {
		response.Error(c, 400, "invalid old password")
		return
	}
	passwordHash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		response.Error(c, 500, "failed to hash password")
		return
	}
	if err := a.db.Model(&freshUser).Update("password_hash", passwordHash).Error; err != nil {
		response.Error(c, 500, "failed to update password")
		return
	}
	response.OK(c, nil)
}

func (a *AuthController) BalanceBillingGroups(c *gin.Context) {
	var groups []model.BillingGroup
	if err := a.db.Where("enabled = ? AND public_selectable = ? AND balance_channel_id IS NOT NULL AND balance_api_key <> ?", true, true, "").Order("name asc").Find(&groups).Error; err != nil {
		response.Error(c, 500, "failed to list billing groups")
		return
	}
	var defaultGroup *gin.H
	var group model.BillingGroup
	if err := a.db.Where("enabled = ? AND is_default = ? AND balance_channel_id IS NOT NULL AND balance_api_key <> ?", true, true, "").Order("id asc").First(&group).Error; err == nil {
		defaultGroup = &gin.H{
			"id":         group.ID,
			"name":       group.Name,
			"multiplier": group.Multiplier,
		}
	}
	items := make([]gin.H, 0, len(groups))
	for _, group := range groups {
		items = append(items, gin.H{
			"id":         group.ID,
			"name":       group.Name,
			"multiplier": group.Multiplier,
		})
	}
	response.OK(c, gin.H{"items": items, "default_group": defaultGroup})
}

func (a *AuthController) UpdateBalanceBillingGroup(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var req updateBalanceBillingGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	updates := map[string]interface{}{"balance_billing_group_id": nil}
	if req.BillingGroupID != nil && *req.BillingGroupID > 0 {
		var group model.BillingGroup
		if err := a.db.Where("id = ? AND enabled = ? AND public_selectable = ?", *req.BillingGroupID, true, true).First(&group).Error; err != nil {
			response.Error(c, 404, "billing group not found")
			return
		}
		updates["balance_billing_group_id"] = group.ID
	}
	if err := a.db.Model(&model.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update billing group")
		return
	}
	response.OK(c, nil)
}

func publicUser(user model.User) gin.H {
	body := gin.H{
		"id":                       user.ID,
		"username":                 user.Username,
		"email":                    user.Email,
		"role":                     user.Role,
		"status":                   user.Status,
		"balance_usd_cents":        user.BalanceUSDCents,
		"balance_billing_group_id": user.BalanceBillingGroupID,
		"expires_at":               user.ExpiresAt,
		"email_verified":           user.EmailVerified,
	}
	if service.HasActiveSubscription(user, time.Now()) && user.Plan != nil {
		body["plan"] = gin.H{
			"id":                   user.Plan.ID,
			"name":                 user.Plan.Name,
			"badge_text":           user.Plan.BadgeText,
			"plan_type":            user.Plan.PlanType,
			"settlement_usd_cents": user.Plan.SettlementUSDCents,
			"quota_period":         user.Plan.QuotaPeriod,
			"price_cents":          user.Plan.PriceCents,
			"duration_days":        user.Plan.DurationDays,
			"description":          user.Plan.Description,
		}
	} else if service.HasDirectPublicChannelAccess(user, time.Now()) && user.PublicChannel != nil {
		body["plan"] = gin.H{
			"id":                   nil,
			"name":                 "公共渠道授权",
			"badge_text":           "公共渠道",
			"plan_type":            model.PlanTypePublic,
			"settlement_usd_cents": user.PublicChannel.RemainingUSDCents,
			"quota_period":         service.DirectPublicChannelPeriod(user.PublicChannelPeriod),
			"price_cents":          0,
			"duration_days":        0,
			"description":          user.PublicChannel.Name,
		}
	}
	return body
}

func (a *AuthController) verifyEmailCode(email, code, purpose string) bool {
	var verification model.EmailVerification
	err := a.db.Where("email = ? AND purpose = ? AND code_hash = ? AND used_at IS NULL", email, purpose, utils.HashToken(code)).
		Order("id desc").
		First(&verification).Error
	if err != nil || time.Now().After(verification.ExpiresAt) {
		return false
	}
	now := time.Now()
	a.db.Model(&verification).Update("used_at", &now)
	return true
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
