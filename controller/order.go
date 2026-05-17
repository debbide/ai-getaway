package controller

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"ai-gateway/config"
	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderController struct {
	db  *gorm.DB
	cfg config.Config
}

const (
	onlinePaymentTTL = 5 * time.Minute
	manualPaymentTTL = 2 * time.Hour
)

func NewOrderController(cfg config.Config, db *gorm.DB) *OrderController {
	return &OrderController{db: db, cfg: cfg}
}

type epayPaymentResult struct {
	PID             string
	OutTradeNo      string
	TradeNo         string
	TradeStatus     string
	Money           string
	PaymentChannel  string
	PaidAmountCents int64
	PaidAt          *time.Time
	RawSummary      string
}

type createOrderRequest struct {
	PlanID        uint   `json:"plan_id" binding:"required"`
	PaymentMethod string `json:"payment_method"`
}

type submitManualPaymentRequest struct {
	UserPaymentNote string `json:"user_payment_note"`
}

func (o *OrderController) Create(c *gin.Context) {
	ctxUser := c.MustGet("user").(model.User)
	var user model.User
	if err := o.db.Preload("Plan").First(&user, ctxUser.ID).Error; err != nil {
		response.Error(c, 401, "user not found")
		return
	}
	expirePendingPaymentOrders(o.db)

	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	var plan model.Plan
	if err := o.db.Preload("PublicChannel").Where("id = ? AND enabled = ?", req.PlanID, true).First(&plan).Error; err != nil {
		response.Error(c, 404, "plan not found")
		return
	}
	if plan.IsLottery {
		response.Error(c, 400, "lottery plan cannot be purchased")
		return
	}
	if activeSubscriptionBlocksPlanOrder(o.db, user, plan) {
		response.Error(c, 409, "active subscription in effect")
		return
	}
	if plan.PlanType == model.PlanTypePublic && (plan.PublicChannel == nil || !plan.PublicChannel.Enabled || plan.PublicChannel.RemainingUSDCents < plan.SettlementUSDCents) {
		response.Error(c, 409, "public plan sold out")
		return
	}
	orderType := orderTypeForPlan(o.db, user, plan, time.Now(), service.UsedUSDCentsSince)
	amountCents := orderAmountCentsForPlan(user, plan, orderType)
	paymentMethod := normalizePaymentMethod(req.PaymentMethod)
	if plan.PriceCents > 0 {
		if err := ensureSystemSettingColumns(o.db); err != nil {
			response.Error(c, 500, "failed to load settings")
			return
		}
		setting := loadSettings(o.db)
		if paymentMethod == model.PaymentMethodOnline && !setting.OnlinePaymentEnabled {
			response.Error(c, 400, "online payment disabled")
			return
		}
		if paymentMethod == model.PaymentMethodManual && !setting.ManualPaymentEnabled {
			response.Error(c, 400, "manual payment disabled")
			return
		}
	} else {
		paymentMethod = "free"
	}

	var existing model.Order
	err := o.db.Preload("Plan").
		Where("user_id = ? AND plan_id = ? AND status IN ?", ctxUser.ID, plan.ID, []string{model.OrderStatusPendingPayment, model.OrderStatusPendingReview}).
		Order("id desc").
		First(&existing).Error
	if err == nil {
		if existing.Status == model.OrderStatusPendingReview {
			response.Error(c, 409, "order already waiting review")
			return
		}
		if plan.PriceCents == 0 {
			if err := completeFreeOrder(o.db, &existing); err != nil {
				writeFreeOrderError(c, err)
				return
			}
			response.OK(c, gin.H{"order": existing, "reused": true})
			return
		}
		if updates, changed := pendingOrderPaymentMethodUpdates(existing, paymentMethod, ctxUser.ID); changed {
			if err := o.db.Model(&existing).Updates(updates).Error; err != nil {
				response.Error(c, 500, "failed to create order")
				return
			}
			if err := o.db.Preload("Plan").First(&existing, existing.ID).Error; err != nil {
				response.Error(c, 500, "failed to create order")
				return
			}
			existing.PaymentMethod = paymentMethod
		}
		response.OK(c, gin.H{"order": existing, "reused": true})
		return
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		response.Error(c, 500, "failed to create order")
		return
	}

	order := model.Order{
		UserID:             ctxUser.ID,
		PlanID:             plan.ID,
		OrderType:          orderType,
		AmountCents:        amountCents,
		SettlementUSDCents: plan.SettlementUSDCents,
		Status:             model.OrderStatusPendingPayment,
		PaymentMethod:      paymentMethod,
		PaymentRef:         fmt.Sprintf("ORDER%d%d", ctxUser.ID, time.Now().UnixNano()),
	}
	if err := o.db.Create(&order).Error; err != nil {
		response.Error(c, 500, "failed to create order")
		return
	}
	order.Plan = plan
	if plan.PriceCents == 0 {
		if err := completeFreeOrder(o.db, &order); err != nil {
			o.db.Delete(&order)
			writeFreeOrderError(c, err)
			return
		}
	}
	response.Created(c, gin.H{"order": order, "reused": false})
}

func orderTypeForPlan(db *gorm.DB, user model.User, targetPlan model.Plan, now time.Time, usedSince func(*gorm.DB, uint, time.Time) int64) string {
	if !service.HasActiveSubscription(user, now) || user.Plan == nil || targetPlan.PriceCents == 0 {
		return model.OrderTypePurchase
	}
	if user.Plan.PriceCents == 0 {
		return model.OrderTypePurchase
	}
	if user.PlanID != nil && *user.PlanID == targetPlan.ID {
		return model.OrderTypeRenewal
	}
	if activePublicPlanQuotaUsedUp(db, user, now, usedSince) {
		return model.OrderTypePurchase
	}
	if targetPlan.PriceCents > user.Plan.PriceCents {
		return model.OrderTypeUpgrade
	}
	return model.OrderTypePurchase
}

func orderAmountCentsForPlan(user model.User, targetPlan model.Plan, orderType string) int64 {
	if orderType != model.OrderTypeUpgrade || user.Plan == nil {
		return targetPlan.PriceCents
	}
	diff := targetPlan.PriceCents - user.Plan.PriceCents
	if diff < 0 {
		return 0
	}
	return diff
}

func writeFreeOrderError(c *gin.Context, err error) {
	switch err.Error() {
	case "public plan sold out", "free plan sold out", "free plan user limit reached":
		response.Error(c, 409, err.Error())
	default:
		response.Error(c, 500, "failed to create order")
	}
}

func normalizePaymentMethod(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case model.PaymentMethodManual:
		return model.PaymentMethodManual
	case "free":
		return "free"
	default:
		return model.PaymentMethodOnline
	}
}

func pendingOrderPaymentMethodUpdates(order model.Order, paymentMethod string, userID uint) (map[string]interface{}, bool) {
	if order.PaymentMethod == paymentMethod {
		return nil, false
	}
	if order.PaymentURLGeneratedAt != nil && paymentMethod != model.PaymentMethodManual {
		return nil, false
	}
	updates := map[string]interface{}{
		"payment_method":           paymentMethod,
		"user_payment_note":        "",
		"payment_channel":          "",
		"paid_amount_cents":        0,
		"paid_at":                  nil,
		"provider_trade_no":        nil,
		"payment_raw":              "",
		"payment_url_generated_at": nil,
	}
	if order.PaymentURLGeneratedAt != nil {
		updates["payment_ref"] = fmt.Sprintf("ORDER%d%d", userID, time.Now().UnixNano())
	}
	return updates, true
}

func completeFreeOrder(db *gorm.DB, order *model.Order) error {
	if order.Plan.ID == 0 {
		if err := db.Preload("Plan").First(order, order.ID).Error; err != nil {
			return err
		}
	}
	now := time.Now()
	updates := paymentOrderUpdates(model.OrderStatusPendingReview, nil, now)
	updates["payment_channel"] = "free"
	updates["paid_amount_cents"] = 0
	if order.Plan.PlanType != model.PlanTypePublic {
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := claimFreePlan(tx, order.UserID, order.PlanID); err != nil {
				return err
			}
			result := tx.Model(&model.Order{}).
				Where("id = ? AND status = ?", order.ID, model.OrderStatusPendingPayment).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return handlePaidOrderAlreadyProcessed(tx, order, nil)
			}
			return nil
		}); err != nil {
			return err
		}
		_ = db.Preload("Plan").First(order, order.ID).Error
		go service.SendOrderPaymentAdminNotification(db, order.ID)
		return nil
	}
	return completeFreePublicOrder(db, order)
}

func completeFreePublicOrder(db *gorm.DB, order *model.Order) error {
	now := time.Now()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := claimFreePlan(tx, order.UserID, order.PlanID); err != nil {
			return err
		}
		var plan model.Plan
		if err := tx.Preload("PublicChannel").First(&plan, order.PlanID).Error; err != nil {
			return err
		}
		orderUpdates := paymentOrderUpdates(model.OrderStatusApproved, nil, now)
		orderUpdates["payment_channel"] = "free"
		orderUpdates["paid_amount_cents"] = 0
		result := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", order.ID, model.OrderStatusPendingPayment).
			Updates(orderUpdates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return handlePaidOrderAlreadyProcessed(tx, order, nil)
		}
		if plan.PublicChannelID == nil || plan.PublicChannel == nil || !plan.PublicChannel.Enabled || plan.PublicChannel.RemainingUSDCents < plan.SettlementUSDCents {
			return fmt.Errorf("public plan sold out")
		}
		result = tx.Model(&model.PublicChannel{}).
			Where("id = ? AND remaining_usd_cents >= ?", *plan.PublicChannelID, plan.SettlementUSDCents).
			Update("remaining_usd_cents", gorm.Expr("remaining_usd_cents - ?", plan.SettlementUSDCents))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("public plan sold out")
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

func claimFreePlan(db *gorm.DB, userID, planID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var plan model.Plan
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&plan, planID).Error; err != nil {
			return err
		}
		if plan.PriceCents != 0 || plan.IsLottery {
			return nil
		}
		var totalClaimed int64
		if err := tx.Model(&model.Order{}).
			Where("plan_id = ? AND payment_method = ? AND status IN ?", planID, "free", freeClaimedOrderStatuses()).
			Count(&totalClaimed).Error; err != nil {
			return err
		}
		if plan.FreeTotalLimit > 0 && totalClaimed >= int64(plan.FreeTotalLimit) {
			return fmt.Errorf("free plan sold out")
		}
		perUserLimit := plan.FreePerUserLimit
		if perUserLimit <= 0 {
			perUserLimit = 1
		}
		var claimedByUser int64
		if err := tx.Model(&model.Order{}).
			Where("user_id = ? AND plan_id = ? AND payment_method = ? AND status IN ?", userID, planID, "free", freeClaimedOrderStatuses()).
			Count(&claimedByUser).Error; err != nil {
			return err
		}
		if claimedByUser >= int64(perUserLimit) {
			return fmt.Errorf("free plan user limit reached")
		}
		result := tx.Model(&model.Plan{}).Where("id = ?", planID).Update("free_claimed_count", int(totalClaimed)+1)
		return result.Error
	})
}

func refreshFreePlanClaimedCount(db *gorm.DB, planID uint) error {
	var totalClaimed int64
	if err := db.Model(&model.Order{}).
		Where("plan_id = ? AND payment_method = ? AND status IN ?", planID, "free", freeClaimedOrderStatuses()).
		Count(&totalClaimed).Error; err != nil {
		return err
	}
	return db.Model(&model.Plan{}).Where("id = ?", planID).Update("free_claimed_count", int(totalClaimed)).Error
}

func freeClaimedOrderStatuses() []string {
	return []string{
		model.OrderStatusPendingReview,
		model.OrderStatusApproved,
		model.OrderStatusManualReview,
	}
}

func activeSubscriptionBlocksPlanOrder(db *gorm.DB, user model.User, targetPlan model.Plan) bool {
	return activeSubscriptionBlocksPlanOrderAt(db, user, targetPlan, time.Now(), service.UsedUSDCentsSince)
}

func activeSubscriptionBlocksPlanOrderAt(db *gorm.DB, user model.User, targetPlan model.Plan, now time.Time, usedSince func(*gorm.DB, uint, time.Time) int64) bool {
	if !service.HasActiveSubscription(user, now) {
		return false
	}
	if targetPlan.PriceCents == 0 && !targetPlan.IsLottery {
		return true
	}
	if user.Plan != nil && user.Plan.PriceCents == 0 {
		return false
	}
	if user.PlanID != nil && *user.PlanID == targetPlan.ID {
		return false
	}
	if activePublicPlanQuotaUsedUp(db, user, now, usedSince) {
		return false
	}
	if user.Plan != nil && targetPlan.PriceCents > user.Plan.PriceCents {
		return false
	}
	if user.Plan == nil || user.Plan.PlanType != model.PlanTypePublic {
		return true
	}
	return true
}

func activePublicPlanQuotaUsedUp(db *gorm.DB, user model.User, now time.Time, usedSince func(*gorm.DB, uint, time.Time) int64) bool {
	if user.Plan == nil || user.Plan.PlanType != model.PlanTypePublic {
		return false
	}
	limit := service.PlanTotalLimitUSDCents(user.Plan)
	if limit <= 0 {
		return false
	}
	start := time.Time{}
	if startedAt := service.SubscriptionStartAt(db, user, now); startedAt != nil {
		start = *startedAt
	}
	return usedSince(db, user.ID, start) >= limit
}

func (o *OrderController) ListMine(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	expirePendingPaymentOrders(o.db)
	var orders []model.Order
	o.db.Preload("Plan").Where("user_id = ?", user.ID).Order("id desc").Find(&orders)
	response.OK(c, orders)
}

func (o *OrderController) Pay(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var order model.Order
	if err := o.db.Preload("Plan").Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&order).Error; err != nil {
		response.Error(c, 404, "order not found")
		return
	}
	if expirePendingPaymentOrder(o.db, &order) {
		response.Error(c, 409, "order payment timeout")
		return
	}
	if order.Status != model.OrderStatusPendingPayment {
		response.Error(c, 409, "order not pending payment")
		return
	}
	if order.PaymentMethod == model.PaymentMethodManual {
		response.Error(c, 409, "manual payment selected")
		return
	}

	if err := ensureSystemSettingColumns(o.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(o.db)
	if !setting.OnlinePaymentEnabled {
		response.Error(c, 400, "online payment disabled")
		return
	}
	payURL, err := buildEpayURL(c, o.cfg, setting, order)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	now := time.Now()
	o.db.Model(&order).Update("payment_url_generated_at", &now)
	response.OK(c, gin.H{"payment_url": payURL, "order": order})
}

func (o *OrderController) ManualPaymentInfo(c *gin.Context) {
	if err := ensureSystemSettingColumns(o.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(o.db)
	if !setting.ManualPaymentEnabled {
		response.Error(c, 400, "manual payment disabled")
		return
	}
	response.OK(c, gin.H{
		"manual_payment_qr_code": setting.ManualPaymentQRCode,
	})
}

func (o *OrderController) SubmitManualPayment(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var req submitManualPaymentRequest
	_ = c.ShouldBindJSON(&req)

	var order model.Order
	if err := o.db.Preload("Plan").Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&order).Error; err != nil {
		response.Error(c, 404, "order not found")
		return
	}
	if expirePendingPaymentOrder(o.db, &order) {
		response.Error(c, 409, "order payment timeout")
		return
	}
	if order.Status != model.OrderStatusPendingPayment {
		response.Error(c, 409, "order not pending payment")
		return
	}
	if order.PaymentMethod != model.PaymentMethodManual {
		response.Error(c, 409, "manual payment not selected")
		return
	}
	if err := ensureSystemSettingColumns(o.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(o.db)
	if !setting.ManualPaymentEnabled {
		response.Error(c, 400, "manual payment disabled")
		return
	}
	if strings.TrimSpace(setting.ManualPaymentQRCode) == "" {
		response.Error(c, 400, "manual payment qr code missing")
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":            model.OrderStatusPendingReview,
		"payment_channel":   model.PaymentMethodManual,
		"paid_amount_cents": order.AmountCents,
		"paid_at":           &now,
		"user_payment_note": strings.TrimSpace(req.UserPaymentNote),
	}
	if err := o.db.Model(&order).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update order")
		return
	}
	if err := o.db.Preload("Plan").First(&order, order.ID).Error; err != nil {
		response.Error(c, 500, "failed to update order")
		return
	}
	go service.SendOrderPaymentAdminNotification(o.db, order.ID)
	response.OK(c, gin.H{"order": order})
}

func (o *OrderController) MarkPaid(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var order model.Order
	if err := o.db.Preload("Plan").Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&order).Error; err != nil {
		response.Error(c, 404, "order not found")
		return
	}
	if order.Status == model.OrderStatusPendingReview || order.Status == model.OrderStatusApproved {
		response.OK(c, gin.H{"order": order})
		return
	}
	expirePendingPaymentOrder(o.db, &order)
	if order.Status == model.OrderStatusPaymentTimeout {
		response.Error(c, 409, "order payment timeout")
		return
	}
	if order.Status != model.OrderStatusPendingPayment {
		response.Error(c, 409, "order not pending payment")
		return
	}

	if err := ensureSystemSettingColumns(o.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(o.db)
	if !setting.OnlinePaymentEnabled {
		response.Error(c, 400, "online payment disabled")
		return
	}
	payment, paid, err := queryEpayPaid(c, o.db, setting, order)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if !paid {
		response.Error(c, 409, "payment not completed")
		return
	}

	if err := completePaidOrder(o.db, &order, payment, nil); err != nil {
		response.Error(c, 500, "failed to update order")
		return
	}
	response.OK(c, gin.H{"order": order})
}

func (o *OrderController) EpayNotify(c *gin.Context) {
	params := map[string]string{}
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	if err := c.Request.ParseForm(); err == nil {
		for key, values := range c.Request.PostForm {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}
	}

	if err := ensureSystemSettingColumns(o.db); err != nil {
		c.String(500, "fail")
		return
	}
	setting := loadSettings(o.db)
	if setting.EpayKey == "" || epaySign(params, setting.EpayKey) != params["sign"] {
		c.String(400, "fail")
		return
	}
	if params["trade_status"] != "TRADE_SUCCESS" {
		c.String(200, "success")
		return
	}

	var order model.Order
	if err := o.db.Preload("Plan").Where("payment_ref = ?", params["out_trade_no"]).First(&order).Error; err != nil {
		c.String(404, "fail")
		return
	}
	if expirePendingPaymentOrder(o.db, &order) || order.Status == model.OrderStatusPaymentTimeout {
		c.String(200, "success")
		return
	}
	payment, err := validateEpayPayment(setting, order, epayPaymentResultFromParams(params))
	if err != nil {
		markPaymentManualReview(o.db, &order, epayPaymentResultFromParams(params), err.Error())
		c.String(200, "success")
		return
	}
	if err := completePaidOrder(o.db, &order, payment, nil); err != nil {
		c.String(500, "fail")
		return
	}
	c.String(200, "success")
}

func pendingPaymentExpiresAt(order model.Order) time.Time {
	if order.PaymentMethod == model.PaymentMethodManual {
		return order.CreatedAt.Add(manualPaymentTTL)
	}
	return order.CreatedAt.Add(onlinePaymentTTL)
}

func expirePendingPaymentOrders(db *gorm.DB) {
	db.Model(&model.Order{}).
		Where(
			"status = ? AND ((payment_method = ? AND created_at <= ?) OR ((payment_method IS NULL OR payment_method <> ?) AND created_at <= ?))",
			model.OrderStatusPendingPayment,
			model.PaymentMethodManual,
			time.Now().Add(-manualPaymentTTL),
			model.PaymentMethodManual,
			time.Now().Add(-onlinePaymentTTL),
		).
		Update("status", model.OrderStatusPaymentTimeout)
}

func expirePendingPaymentOrder(db *gorm.DB, order *model.Order) bool {
	if order.Status != model.OrderStatusPendingPayment {
		return false
	}
	if time.Now().Before(pendingPaymentExpiresAt(*order)) {
		return false
	}
	db.Model(order).Update("status", model.OrderStatusPaymentTimeout)
	order.Status = model.OrderStatusPaymentTimeout
	return true
}

func completePaidOrder(db *gorm.DB, order *model.Order, payment *epayPaymentResult, approvedByID *uint) error {
	if order.Plan.ID == 0 {
		if err := db.Preload("Plan").First(order, order.ID).Error; err != nil {
			return err
		}
	}
	if order.Plan.PlanType != model.PlanTypePublic {
		now := time.Now()
		updates := paymentOrderUpdates(model.OrderStatusPendingReview, payment, now)
		result := db.Model(&model.Order{}).
			Where("id = ? AND status = ?", order.ID, model.OrderStatusPendingPayment).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return handlePaidOrderAlreadyProcessed(db, order, payment)
		}
		if err := db.Preload("Plan").First(order, order.ID).Error; err != nil {
			return err
		}
		go service.SendOrderPaymentAdminNotification(db, order.ID)
		return nil
	}

	now := time.Now()
	err := db.Transaction(func(tx *gorm.DB) error {
		var plan model.Plan
		if err := tx.Preload("PublicChannel").First(&plan, order.PlanID).Error; err != nil {
			return err
		}
		orderUpdates := paymentOrderUpdates(model.OrderStatusApproved, payment, now)
		if approvedByID != nil {
			orderUpdates["approved_by_id"] = *approvedByID
		}
		result := tx.Model(&model.Order{}).
			Where("id = ? AND status = ?", order.ID, model.OrderStatusPendingPayment).
			Updates(orderUpdates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return handlePaidOrderAlreadyProcessed(tx, order, payment)
		}
		if plan.PublicChannelID == nil || plan.PublicChannel == nil || !plan.PublicChannel.Enabled || plan.PublicChannel.RemainingUSDCents < plan.SettlementUSDCents {
			return fmt.Errorf("public plan sold out")
		}
		result = tx.Model(&model.PublicChannel{}).
			Where("id = ? AND remaining_usd_cents >= ?", *plan.PublicChannelID, plan.SettlementUSDCents).
			Update("remaining_usd_cents", gorm.Expr("remaining_usd_cents - ?", plan.SettlementUSDCents))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("public plan sold out")
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

func paymentOrderUpdates(status string, payment *epayPaymentResult, now time.Time) map[string]interface{} {
	updates := map[string]interface{}{
		"status":  status,
		"paid_at": &now,
	}
	if status == model.OrderStatusApproved {
		updates["approved_at"] = &now
	}
	if payment != nil {
		updates["paid_amount_cents"] = payment.PaidAmountCents
		updates["payment_channel"] = payment.PaymentChannel
		updates["payment_raw"] = payment.RawSummary
		if payment.PaidAt != nil {
			updates["paid_at"] = payment.PaidAt
		}
		if payment.TradeNo != "" {
			tradeNo := payment.TradeNo
			updates["provider_trade_no"] = &tradeNo
		}
	}
	return updates
}

func applyApprovedSubscription(db *gorm.DB, order *model.Order, plan model.Plan, now time.Time) error {
	var user model.User
	if err := db.Preload("Plan").First(&user, order.UserID).Error; err != nil {
		return err
	}
	startedAt := subscriptionStartedAtBeforeOrder(db, user, order, now)
	expiresAt := subscriptionExpiresAtForOrder(user, plan, order.OrderType, now)
	if startedAt == nil || order.OrderType == model.OrderTypePurchase {
		value := now
		startedAt = &value
	}
	return db.Model(&model.User{}).Where("id = ?", order.UserID).Updates(map[string]interface{}{
		"status":                  model.UserStatusApproved,
		"plan_id":                 plan.ID,
		"expires_at":              expiresAt,
		"subscription_started_at": startedAt,
	}).Error
}

func subscriptionStartedAtBeforeOrder(db *gorm.DB, user model.User, order *model.Order, now time.Time) *time.Time {
	if user.SubscriptionStartedAt != nil {
		return user.SubscriptionStartedAt
	}
	if user.PlanID != nil {
		var lastOrder model.Order
		result := db.Where("user_id = ? AND plan_id = ? AND status = ? AND id <> ?", user.ID, *user.PlanID, model.OrderStatusApproved, order.ID).
			Order("approved_at DESC, id DESC").
			Limit(1).
			Find(&lastOrder)
		if result.Error == nil && result.RowsAffected > 0 && lastOrder.ApprovedAt != nil {
			return lastOrder.ApprovedAt
		}
	}
	if service.HasActiveSubscription(user, now) && user.Plan != nil && user.ExpiresAt != nil && user.Plan.DurationDays > 0 {
		fallbackStartedAt := user.ExpiresAt.AddDate(0, 0, -user.Plan.DurationDays)
		return &fallbackStartedAt
	}
	return nil
}

func subscriptionExpiresAtForOrder(user model.User, plan model.Plan, orderType string, now time.Time) *time.Time {
	durationDays := plan.DurationDays
	if plan.PlanType == model.PlanTypePublic {
		durationDays = 36500
	}
	if durationDays < 1 {
		durationDays = 1
	}
	base := now
	if orderType == model.OrderTypeRenewal && user.ExpiresAt != nil && user.ExpiresAt.After(now) {
		base = *user.ExpiresAt
	}
	if orderType == model.OrderTypeUpgrade && user.ExpiresAt != nil && user.ExpiresAt.After(now) {
		expiresAt := *user.ExpiresAt
		return &expiresAt
	}
	expiresAt := base.AddDate(0, 0, durationDays)
	return &expiresAt
}

func handlePaidOrderAlreadyProcessed(db *gorm.DB, order *model.Order, payment *epayPaymentResult) error {
	var fresh model.Order
	if err := db.First(&fresh, order.ID).Error; err != nil {
		return err
	}
	switch fresh.Status {
	case model.OrderStatusPendingReview, model.OrderStatusApproved, model.OrderStatusManualReview:
		*order = fresh
		return nil
	case model.OrderStatusPaymentTimeout:
		*order = fresh
		return fmt.Errorf("order payment timeout")
	default:
		return fmt.Errorf("order not pending payment")
	}
}

func markPaymentManualReview(db *gorm.DB, order *model.Order, payment *epayPaymentResult, reason string) error {
	if order == nil || order.ID == 0 {
		return nil
	}
	now := time.Now()
	status := model.OrderStatusManualReview
	updates := paymentOrderUpdates(status, payment, now)
	updates["admin_note"] = strings.TrimSpace(reason)
	result := db.Model(&model.Order{}).
		Where("id = ? AND status = ?", order.ID, model.OrderStatusPendingPayment).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		order.Status = status
	}
	return nil
}

func buildEpayURL(c *gin.Context, cfg config.Config, setting model.SystemSetting, order model.Order) (string, error) {
	if setting.EpaySubmitURL == "" || setting.EpayPID == "" || setting.EpayKey == "" {
		return "", fmt.Errorf("payment config missing")
	}
	submitURL := normalizeEpaySubmitURL(setting.EpaySubmitURL)
	baseURL := requestBaseURL(c, cfg)
	notifyURL := setting.EpayNotifyURL
	if notifyURL == "" {
		notifyURL = baseURL + "/api/payment/epay/notify"
	}
	returnURL := setting.EpayReturnURL
	if returnURL == "" {
		returnURL = baseURL + "/console"
	}
	params := map[string]string{
		"pid":          setting.EpayPID,
		"out_trade_no": order.PaymentRef,
		"notify_url":   notifyURL,
		"return_url":   returnURL,
		"name":         order.Plan.Name,
		"money":        fmt.Sprintf("%.2f", float64(order.AmountCents)/100),
	}
	params["sign"] = epaySign(params, setting.EpayKey)
	params["sign_type"] = "MD5"

	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	separator := "?"
	if strings.Contains(submitURL, "?") {
		separator = "&"
	}
	return submitURL + separator + values.Encode(), nil
}

func normalizeEpaySubmitURL(rawURL string) string {
	cleanURL := strings.TrimSpace(rawURL)
	cleanURL = strings.TrimRight(cleanURL, "/")
	lowerURL := strings.ToLower(cleanURL)
	if strings.HasSuffix(lowerURL, "/submit.php") {
		return cleanURL
	}
	if strings.HasSuffix(lowerURL, "/mapi.php") {
		return cleanURL[:len(cleanURL)-len("/mapi.php")] + "/submit.php"
	}
	if strings.HasSuffix(lowerURL, ".php") {
		return cleanURL
	}
	return cleanURL + "/submit.php"
}

func epayQueryURL(rawURL string) string {
	cleanURL := strings.TrimSpace(rawURL)
	cleanURL = strings.TrimRight(cleanURL, "/")
	lowerURL := strings.ToLower(cleanURL)
	if strings.HasSuffix(lowerURL, "/api.php") {
		return cleanURL
	}
	if strings.HasSuffix(lowerURL, "/submit.php") {
		return cleanURL[:len(cleanURL)-len("/submit.php")] + "/api.php"
	}
	if strings.HasSuffix(lowerURL, "/mapi.php") {
		return cleanURL[:len(cleanURL)-len("/mapi.php")] + "/api.php"
	}
	if strings.HasSuffix(lowerURL, ".php") {
		return cleanURL
	}
	return cleanURL + "/api.php"
}

func queryEpayPaid(c *gin.Context, db *gorm.DB, setting model.SystemSetting, order model.Order) (*epayPaymentResult, bool, error) {
	if setting.EpaySubmitURL == "" || setting.EpayPID == "" || setting.EpayKey == "" {
		return nil, false, fmt.Errorf("payment config missing")
	}

	values := url.Values{}
	values.Set("act", "order")
	values.Set("pid", setting.EpayPID)
	values.Set("key", setting.EpayKey)
	values.Set("out_trade_no", order.PaymentRef)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, epayQueryURL(setting.EpaySubmitURL)+"?"+values.Encode(), nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to verify payment")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to verify payment")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("failed to verify payment")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, false, fmt.Errorf("failed to verify payment")
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, false, fmt.Errorf("failed to verify payment")
	}

	if !epayResultPaid(result) {
		return nil, false, nil
	}
	payment, err := validateEpayPayment(setting, order, epayPaymentResultFromQuery(result))
	if err != nil {
		markPaymentManualReview(db, &order, payment, err.Error())
		return nil, false, err
	}
	return payment, true, nil
}

func validateEpayPayment(setting model.SystemSetting, order model.Order, payment *epayPaymentResult) (*epayPaymentResult, error) {
	if payment == nil {
		return nil, fmt.Errorf("payment result missing")
	}
	if strings.TrimSpace(payment.PID) != "" && strings.TrimSpace(payment.PID) != strings.TrimSpace(setting.EpayPID) {
		return payment, fmt.Errorf("payment pid mismatch")
	}
	if payment.OutTradeNo != order.PaymentRef {
		return payment, fmt.Errorf("payment order mismatch")
	}
	if strings.ToUpper(payment.TradeStatus) != "TRADE_SUCCESS" {
		return payment, fmt.Errorf("payment not completed")
	}
	if payment.PaidAmountCents != order.AmountCents {
		return payment, fmt.Errorf("payment amount mismatch")
	}
	return payment, nil
}

func epayPaymentResultFromParams(params map[string]string) *epayPaymentResult {
	paidAt := time.Now()
	amount := parseMoneyCents(params["money"])
	raw, _ := json.Marshal(map[string]string{
		"pid":          params["pid"],
		"out_trade_no": params["out_trade_no"],
		"trade_no":     params["trade_no"],
		"trade_status": params["trade_status"],
		"type":         params["type"],
		"money":        params["money"],
	})
	return &epayPaymentResult{
		PID:             strings.TrimSpace(params["pid"]),
		OutTradeNo:      strings.TrimSpace(params["out_trade_no"]),
		TradeNo:         strings.TrimSpace(params["trade_no"]),
		TradeStatus:     strings.TrimSpace(params["trade_status"]),
		Money:           strings.TrimSpace(params["money"]),
		PaymentChannel:  strings.TrimSpace(params["type"]),
		PaidAmountCents: amount,
		PaidAt:          &paidAt,
		RawSummary:      string(raw),
	}
}

func epayPaymentResultFromQuery(result map[string]interface{}) *epayPaymentResult {
	status := strings.TrimSpace(fmt.Sprint(result["trade_status"]))
	if status == "" && epayResultPaid(result) {
		status = "TRADE_SUCCESS"
	}
	paidAt := time.Now()
	money := firstResultString(result, "money", "amount")
	raw, _ := json.Marshal(result)
	return &epayPaymentResult{
		PID:             firstResultString(result, "pid"),
		OutTradeNo:      firstResultString(result, "out_trade_no"),
		TradeNo:         firstResultString(result, "trade_no"),
		TradeStatus:     status,
		Money:           money,
		PaymentChannel:  firstResultString(result, "type", "payment_type"),
		PaidAmountCents: parseMoneyCents(money),
		PaidAt:          &paidAt,
		RawSummary:      string(raw),
	}
}

func firstResultString(result map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(result[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func parseMoneyCents(value string) int64 {
	amount, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(amount * 100))
}

func epayResultPaid(result map[string]interface{}) bool {
	tradeStatus := strings.ToUpper(fmt.Sprint(result["trade_status"]))
	if tradeStatus == "TRADE_SUCCESS" {
		return true
	}

	status := strings.ToLower(fmt.Sprint(result["status"]))
	if status == "1" || status == "success" || status == "paid" {
		return true
	}

	code := strings.ToLower(fmt.Sprint(result["code"]))
	return code == "1" && strings.Contains(strings.ToLower(fmt.Sprint(result["msg"])), "success")
}

func requestBaseURL(c *gin.Context, cfg config.Config) string {
	if cfg.PublicBaseURL != "" {
		return cfg.PublicBaseURL
	}
	proto := c.GetHeader("X-Forwarded-Proto")
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return proto + "://" + host
}

func epaySign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for paramKey, value := range params {
		if paramKey == "sign" || paramKey == "sign_type" || value == "" {
			continue
		}
		keys = append(keys, paramKey)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, paramKey := range keys {
		parts = append(parts, paramKey+"="+params[paramKey])
	}
	hash := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return hex.EncodeToString(hash[:])
}
