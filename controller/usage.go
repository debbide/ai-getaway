package controller

import (
	"math"
	"strconv"
	"strings"
	"time"

	"ai-gateway/model"
	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UsageController struct {
	db *gorm.DB
}

func NewUsageController(db *gorm.DB) *UsageController {
	return &UsageController{db: db}
}

type usageSummary struct {
	TotalRequests      int64 `json:"total_requests"`
	TotalTokens        int64 `json:"total_tokens"`
	PromptTokens       int64 `json:"prompt_tokens"`
	CompletionTokens   int64 `json:"completion_tokens"`
	TotalUSDCents      int64 `json:"total_usd_cents"`
	TotalUSDMicros     int64 `json:"total_usd_micros"`
	AverageLatencyMs   int64 `json:"average_latency_ms"`
}

type usageLogItem struct {
	ID                       uint      `json:"id"`
	APIKeyID                 uint      `json:"api_key_id"`
	APIKeyName               string    `json:"api_key_name"`
	APIKeyPrefix             string    `json:"api_key_prefix"`
	Method                   string    `json:"method"`
	Path                     string    `json:"path"`
	Endpoint                 string    `json:"endpoint"`
	Model                    string    `json:"model"`
	RequestType              string    `json:"request_type"`
	BillingMode              string    `json:"billing_mode"`
	StatusCode               int       `json:"status_code"`
	PromptTokens             int64     `json:"prompt_tokens"`
	CachedInputTokens        int64     `json:"cached_input_tokens"`
	CompletionTokens         int64     `json:"completion_tokens"`
	TotalTokens              int64     `json:"total_tokens"`
	EstimatedUSDCents        int64     `json:"estimated_usd_cents"`
	EstimatedUSDMicros       int64     `json:"estimated_usd_micros"`
	InputUSDMicros           int64     `json:"input_usd_micros"`
	CachedInputUSDMicros     int64     `json:"cached_input_usd_micros"`
	OutputUSDMicros          int64     `json:"output_usd_micros"`
	InputUSDPerMillion       float64   `json:"input_usd_per_million"`
	CachedInputUSDPerMillion float64   `json:"cached_input_usd_per_million"`
	OutputUSDPerMillion      float64   `json:"output_usd_per_million"`
	BillingMultiplier        float64   `json:"billing_multiplier"`
	BillingSource            string    `json:"billing_source"`
	FirstTokenMs             int64     `json:"first_token_ms"`
	LatencyMs                int64     `json:"latency_ms"`
	ErrorMessage             string    `json:"error_message"`
	CreatedAt                time.Time `json:"created_at"`
}

func (u *UsageController) List(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	page := clampInt(queryInt(c, "page", 1), 1, 100000)
	pageSize := clampInt(queryInt(c, "page_size", 20), 1, 100)
	apiKeyID := uint(queryInt(c, "api_key_id", 0))
	rangeValue := c.DefaultQuery("range", "7d")

	buildQuery := func() *gorm.DB {
		query := u.db.Model(&model.APILog{}).Where("api_logs.user_id = ?", user.ID)
		if apiKeyID > 0 {
			query = query.Where("api_logs.api_key_id = ?", apiKeyID)
		}
		if since, ok := usageRangeStart(rangeValue); ok {
			query = query.Where("api_logs.created_at >= ?", since)
		}
		return query
	}

	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		response.Error(c, 500, "failed to list usage logs")
		return
	}

	var summary usageSummary
	if err := buildQuery().Select(`
		COUNT(*) AS total_requests,
		COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
		COALESCE(SUM(total_tokens), 0) AS total_tokens,
		COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
		COALESCE(SUM(CASE WHEN estimated_usd_micros > 0 THEN CEILING(estimated_usd_micros / 10000) ELSE estimated_usd_cents END), 0) AS total_usd_cents,
		COALESCE(SUM(estimated_usd_micros), 0) AS total_usd_micros,
		COALESCE(ROUND(AVG(latency_ms)), 0) AS average_latency_ms
	`).Scan(&summary).Error; err != nil {
		response.Error(c, 500, "failed to list usage logs")
		return
	}

	if total == 0 {
		response.OK(c, gin.H{
			"items":     []usageLogItem{},
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"pages":     1,
			"summary":   summary,
		})
		return
	}

	var logs []model.APILog
	if err := buildQuery().
		Preload("APIKey").
		Order("api_logs.created_at desc, api_logs.id desc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&logs).Error; err != nil {
		response.Error(c, 500, "failed to list usage logs")
		return
	}

	items := make([]usageLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, mapUsageLog(log))
	}

	response.OK(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     int(math.Max(1, math.Ceil(float64(total)/float64(pageSize)))),
		"summary":   summary,
	})
}

func mapUsageLog(log model.APILog) usageLogItem {
	modelName := log.ModelName
	if modelName == "" {
		modelName = "-"
	}
	requestType := log.RequestType
	if requestType == "" {
		requestType = "chat"
	}
	return usageLogItem{
		ID:                       log.ID,
		APIKeyID:                 log.APIKeyID,
		APIKeyName:               log.APIKey.Name,
		APIKeyPrefix:             log.APIKey.KeyPrefix,
		Method:                   log.Method,
		Path:                     log.Path,
		Endpoint:                 strings.TrimPrefix(log.Path, "/v1"),
		Model:                    modelName,
		RequestType:              requestType,
		BillingMode:              "usage",
		StatusCode:               log.StatusCode,
		PromptTokens:             log.PromptTokens,
		CachedInputTokens:        log.CachedInputTokens,
		CompletionTokens:         log.CompletionTokens,
		TotalTokens:              log.TotalTokens,
		EstimatedUSDCents:        log.EstimatedUSDCents,
		EstimatedUSDMicros:       log.EstimatedUSDMicros,
		InputUSDMicros:           log.InputUSDMicros,
		CachedInputUSDMicros:     log.CachedInputUSDMicros,
		OutputUSDMicros:          log.OutputUSDMicros,
		InputUSDPerMillion:       log.InputUSDPerMillion,
		CachedInputUSDPerMillion: log.CachedInputUSDPerMillion,
		OutputUSDPerMillion:      log.OutputUSDPerMillion,
		BillingMultiplier:        log.BillingMultiplier,
		BillingSource:            log.BillingSource,
		FirstTokenMs:             log.FirstTokenMs,
		LatencyMs:                log.LatencyMs,
		ErrorMessage:             log.ErrorMessage,
		CreatedAt:                log.CreatedAt,
	}
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return fallback
	}
	return value
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func usageRangeStart(value string) (time.Time, bool) {
	now := time.Now()
	switch value {
	case "24h":
		return now.Add(-24 * time.Hour), true
	case "7d":
		return now.AddDate(0, 0, -7), true
	case "30d":
		return now.AddDate(0, 0, -30), true
	case "all":
		return time.Time{}, false
	default:
		return now.AddDate(0, 0, -7), true
	}
}
