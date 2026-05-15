package controller

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OrderController struct {
	db *gorm.DB
}

const pendingPaymentTTL = 5 * time.Minute

func NewOrderController(db *gorm.DB) *OrderController {
	return &OrderController{db: db}
}

type createOrderRequest struct {
	PlanID uint `json:"plan_id" binding:"required"`
}

func (o *OrderController) Create(c *gin.Context) {
	ctxUser := c.MustGet("user").(model.User)
	var user model.User
	if err := o.db.Preload("Plan").First(&user, ctxUser.ID).Error; err != nil {
		response.Error(c, 401, "user not found")
		return
	}
	if activeSubscriptionBlocksNewOrder(o.db, user) {
		response.Error(c, 409, "active subscription in effect")
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
	if plan.PlanType == model.PlanTypePublic && (plan.PublicChannel == nil || !plan.PublicChannel.Enabled || plan.PublicChannel.RemainingUSDCents < plan.SettlementUSDCents) {
		response.Error(c, 409, "public plan sold out")
		return
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
		AmountCents:        plan.PriceCents,
		SettlementUSDCents: plan.SettlementUSDCents,
		Status:             model.OrderStatusPendingPayment,
		PaymentRef:         fmt.Sprintf("ORDER%d%d", ctxUser.ID, time.Now().UnixNano()),
	}
	if err := o.db.Create(&order).Error; err != nil {
		response.Error(c, 500, "failed to create order")
		return
	}
	order.Plan = plan
	response.Created(c, gin.H{"order": order, "reused": false})
}

func activeSubscriptionBlocksNewOrder(db *gorm.DB, user model.User) bool {
	if !service.HasActiveSubscription(user, time.Now()) {
		return false
	}
	if user.Plan == nil || user.Plan.PlanType != model.PlanTypePublic {
		return true
	}
	limit := service.PlanTotalLimitUSDCents(user.Plan)
	if limit <= 0 {
		return true
	}
	start := time.Time{}
	if startedAt := subscriptionStartAt(db, user); startedAt != nil {
		start = *startedAt
	}
	return service.UsedUSDCentsSince(db, user.ID, start) < limit
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

	if err := ensureSystemSettingColumns(o.db); err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	setting := loadSettings(o.db)
	payURL, err := buildEpayURL(c, setting, order)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.OK(c, gin.H{"payment_url": payURL, "order": order})
}

func (o *OrderController) MarkPaid(c *gin.Context) {
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
	if order.Status == model.OrderStatusPendingReview || order.Status == model.OrderStatusApproved {
		response.OK(c, gin.H{"order": order})
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
	paid, err := queryEpayPaid(c, setting, order)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if !paid {
		response.Error(c, 409, "payment not completed")
		return
	}

	if err := completePaidOrder(o.db, &order, nil); err != nil {
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
	if expirePendingPaymentOrder(o.db, &order) {
		c.String(200, "success")
		return
	}
	if order.Status == model.OrderStatusPendingPayment {
		if err := completePaidOrder(o.db, &order, nil); err != nil {
			c.String(500, "fail")
			return
		}
	}
	c.String(200, "success")
}

func pendingPaymentExpiresAt(order model.Order) time.Time {
	return order.CreatedAt.Add(pendingPaymentTTL)
}

func expirePendingPaymentOrders(db *gorm.DB) {
	db.Model(&model.Order{}).
		Where("status = ? AND created_at <= ?", model.OrderStatusPendingPayment, time.Now().Add(-pendingPaymentTTL)).
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

func completePaidOrder(db *gorm.DB, order *model.Order, approvedByID *uint) error {
	if order.Plan.ID == 0 {
		if err := db.Preload("Plan").First(order, order.ID).Error; err != nil {
			return err
		}
	}
	if order.Plan.PlanType != model.PlanTypePublic {
		if err := db.Model(order).Update("status", model.OrderStatusPendingReview).Error; err != nil {
			return err
		}
		order.Status = model.OrderStatusPendingReview
		go service.SendOrderPaymentAdminNotification(db, order.ID)
		return nil
	}

	now := time.Now()
	expiresAt := now.AddDate(100, 0, 0)
	err := db.Transaction(func(tx *gorm.DB) error {
		var plan model.Plan
		if err := tx.Preload("PublicChannel").First(&plan, order.PlanID).Error; err != nil {
			return err
		}
		if plan.PublicChannelID == nil || plan.PublicChannel == nil || !plan.PublicChannel.Enabled || plan.PublicChannel.RemainingUSDCents < plan.SettlementUSDCents {
			return fmt.Errorf("public plan sold out")
		}
		result := tx.Model(&model.PublicChannel{}).
			Where("id = ? AND remaining_usd_cents >= ?", *plan.PublicChannelID, plan.SettlementUSDCents).
			Update("remaining_usd_cents", gorm.Expr("remaining_usd_cents - ?", plan.SettlementUSDCents))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("public plan sold out")
		}
		orderUpdates := map[string]interface{}{
			"status":      model.OrderStatusApproved,
			"approved_at": &now,
		}
		if approvedByID != nil {
			orderUpdates["approved_by_id"] = *approvedByID
		}
		if err := tx.Model(order).Updates(orderUpdates).Error; err != nil {
			return err
		}
		return tx.Model(&model.User{}).Where("id = ?", order.UserID).Updates(map[string]interface{}{
			"status":     model.UserStatusApproved,
			"plan_id":    plan.ID,
			"expires_at": &expiresAt,
		}).Error
	})
	if err != nil {
		return err
	}
	order.Status = model.OrderStatusApproved
	order.ApprovedAt = &now
	go service.SendOrderApprovedUserNotification(db, order.ID, order.AdminNote)
	return nil
}

func buildEpayURL(c *gin.Context, setting model.SystemSetting, order model.Order) (string, error) {
	if setting.EpaySubmitURL == "" || setting.EpayPID == "" || setting.EpayKey == "" {
		return "", fmt.Errorf("payment config missing")
	}
	submitURL := normalizeEpaySubmitURL(setting.EpaySubmitURL)
	baseURL := requestBaseURL(c)
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

func queryEpayPaid(c *gin.Context, setting model.SystemSetting, order model.Order) (bool, error) {
	if setting.EpaySubmitURL == "" || setting.EpayPID == "" || setting.EpayKey == "" {
		return false, fmt.Errorf("payment config missing")
	}

	values := url.Values{}
	values.Set("act", "order")
	values.Set("pid", setting.EpayPID)
	values.Set("key", setting.EpayKey)
	values.Set("out_trade_no", order.PaymentRef)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, epayQueryURL(setting.EpaySubmitURL)+"?"+values.Encode(), nil)
	if err != nil {
		return false, fmt.Errorf("failed to verify payment")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to verify payment")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("failed to verify payment")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, fmt.Errorf("failed to verify payment")
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("failed to verify payment")
	}

	return epayResultPaid(result), nil
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

func requestBaseURL(c *gin.Context) string {
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
