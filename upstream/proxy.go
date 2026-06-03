package upstream

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ai-gateway/model"
	"ai-gateway/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ProxyHandler(db *gorm.DB, hub *service.LogHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		service.AddActiveAPIConnection(1)
		defer service.AddActiveAPIConnection(-1)

		apiKey := c.MustGet("api_key").(model.APIKey)
		upstreamBaseURL := ""
		upstreamAPIKey := ""
		groupMultipliers := ""
		if publicChannelValue, ok := c.Get("public_channel"); ok {
			publicChannel := publicChannelValue.(model.PublicChannel)
			upstreamBaseURL = publicChannel.BaseURL
			upstreamAPIKey = publicChannel.APIKey
			groupMultipliers = publicChannel.GroupMultipliers
		} else if poolAccountValue, ok := c.Get("pool_account"); ok {
			poolAccount := poolAccountValue.(model.PollingPoolAccount)
			upstreamBaseURL = poolAccount.BaseURL
			upstreamAPIKey = poolAccount.APIKey
			groupMultipliers = poolAccount.GroupMultipliers
		} else {
			upstreamAccount := c.MustGet("upstream").(model.UpstreamAccount)
			upstreamBaseURL = upstreamAccount.BaseURL
			upstreamAPIKey = upstreamAccount.APIKey
			groupMultipliers = upstreamAccount.GroupMultipliers
		}

		target, err := url.Parse(strings.TrimRight(upstreamBaseURL, "/"))
		if err != nil {
			c.JSON(500, gin.H{"error": "invalid upstream base url"})
			return
		}

		start := time.Now()
		requestInfo := parseRequestInfo(c.Request)
		quotaReservation, quotaAllowed, err := service.BeginQuotaReservation(db, apiKey.User, apiKey.ID, start)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "额度预占失败"})
			return
		}
		if !quotaAllowed {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "令牌额度耗尽"})
			return
		}
		quotaReservationID := uint(0)
		if quotaReservation != nil {
			quotaReservationID = quotaReservation.ID
			if cappedInfo, ok, err := applyDynamicOutputLimit(db, c.Request, requestInfo, quotaReservation.ReservedUSDCents, groupMultipliers); err != nil {
				service.CancelQuotaReservation(db, quotaReservationID, time.Now())
				c.JSON(http.StatusBadRequest, gin.H{"error": "请求体解析失败"})
				return
			} else if !ok {
				service.CancelQuotaReservation(db, quotaReservationID, time.Now())
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "令牌额度耗尽"})
				return
			} else {
				requestInfo = cappedInfo
			}
		}
		protocol := model.ProtocolGPT
		if value, ok := c.Get("protocol"); ok {
			protocol = service.NormalizeProtocol(value.(string))
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Path = c.Request.URL.Path
			req.URL.RawQuery = c.Request.URL.RawQuery
			req.Host = target.Host
			if protocol == model.ProtocolClaude {
				req.Header.Set("X-API-Key", upstreamAPIKey)
				req.Header.Set("Authorization", "Bearer "+upstreamAPIKey)
			} else {
				req.Header.Set("Authorization", "Bearer "+upstreamAPIKey)
			}
			req.Header.Set("X-Forwarded-User-ID", intToString(apiKey.UserID))
			req.Header.Del("Accept-Encoding")
		}
		proxy.FlushInterval = 100 * time.Millisecond
		proxy.ModifyResponse = func(resp *http.Response) error {
			if !shouldRecordUsageResponse(resp.StatusCode) {
				service.CancelQuotaReservation(db, quotaReservationID, time.Now())
				return nil
			}
			log := model.APILog{
				UserID:      apiKey.UserID,
				APIKeyID:    apiKey.ID,
				Method:      c.Request.Method,
				Path:        c.Request.URL.Path,
				ModelName:   requestInfo.Model,
				StatusCode:  resp.StatusCode,
				RequestType: requestType(c.Request.URL.Path, requestInfo.Stream),
			}
			if isEventStream(resp) {
				resp.Body = &usageStreamReadCloser{
					ReadCloser:    resp.Body,
					start:         start,
					log:           &log,
					db:            db,
					hub:           hub,
					multipliers:   groupMultipliers,
					reservationID: quotaReservationID,
				}
			} else {
				body, err := io.ReadAll(resp.Body)
				if err == nil {
					fillUsage(db, &log, body, groupMultipliers)
					finalizeUsageLog(db, hub, &log, start, quotaReservationID, groupMultipliers)
					resp.Body = io.NopCloser(bytes.NewReader(body))
					return nil
				}
				finalizeUsageLog(db, hub, &log, start, quotaReservationID, groupMultipliers)
			}
			return nil
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			service.CancelQuotaReservation(db, quotaReservationID, time.Now())
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"上游请求失败"}`))
		}

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

type requestInfo struct {
	Model     string
	Stream    bool
	BodyBytes int64
}

func parseRequestInfo(req *http.Request) requestInfo {
	if req.Body == nil || req.Method == http.MethodGet {
		return requestInfo{}
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return requestInfo{}
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return requestInfo{BodyBytes: int64(len(body))}
	}
	return requestInfo{Model: payload.Model, Stream: payload.Stream, BodyBytes: int64(len(body))}
}

func applyDynamicOutputLimit(db *gorm.DB, req *http.Request, info requestInfo, reservedUSDCents int64, groupMultipliers any) (requestInfo, bool, error) {
	if req == nil || req.Body == nil || req.Method == http.MethodGet || reservedUSDCents <= 0 || info.BodyBytes <= 0 || info.Model == "" {
		return info, true, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return info, true, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 {
		return info, true, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return info, true, err
	}

	maxOutputTokens := dynamicMaxOutputTokens(db, info.Model, int64(len(body)), reservedUSDCents, groupMultipliers)
	if maxOutputTokens <= 0 {
		return info, false, nil
	}
	if maxOutputTokens >= math.MaxInt32 {
		return info, true, nil
	}
	field := outputLimitField(req.URL.Path, payload)
	if field == "" {
		return info, true, nil
	}
	setTokenLimit(payload, field, maxOutputTokens)

	nextBody, err := json.Marshal(payload)
	if err != nil {
		return info, true, err
	}
	req.Body = io.NopCloser(bytes.NewReader(nextBody))
	req.ContentLength = int64(len(nextBody))
	req.Header.Set("Content-Length", intToString(uint(len(nextBody))))
	info.BodyBytes = int64(len(nextBody))
	return info, true, nil
}

func dynamicMaxOutputTokens(db *gorm.DB, modelName string, bodyBytes int64, reservedUSDCents int64, groupMultipliers any) int64 {
	pricing, _ := service.FindModelPricing(db, modelName)
	multiplier := pricing.BillingMultiplier
	if multiplier <= 0 {
		multiplier = 1
	}
	effectiveMultiplier := multiplier * service.ResolveGroupMultiplier(pricing, groupMultipliers)
	if pricing.BillingMode == model.ModelBillingModeRequest {
		requestMicros := int64(math.Round(pricing.RequestUSD * effectiveMultiplier * 1_000_000))
		if requestMicros > reservedUSDCents*10_000 {
			return 0
		}
		return math.MaxInt32
	}
	outputMicrosPerToken := pricing.OutputUSDPerMillion * effectiveMultiplier
	if outputMicrosPerToken <= 0 {
		return math.MaxInt32
	}
	inputTokens := estimatedTokensFromBytes(bodyBytes)
	inputMicros := int64(math.Ceil(float64(inputTokens) * pricing.InputUSDPerMillion * effectiveMultiplier))
	availableMicros := reservedUSDCents*10_000 - inputMicros
	if availableMicros <= 0 {
		return 0
	}
	maxTokens := int64(math.Floor(float64(availableMicros) / outputMicrosPerToken))
	if maxTokens < 1 {
		return 0
	}
	if maxTokens > math.MaxInt32 {
		return math.MaxInt32
	}
	return maxTokens
}

func outputLimitField(path string, payload map[string]any) string {
	endpoint := strings.TrimPrefix(strings.ToLower(path), "/v1/")
	switch {
	case strings.Contains(endpoint, "responses"):
		return "max_output_tokens"
	case strings.Contains(endpoint, "messages"):
		return "max_tokens"
	case strings.Contains(endpoint, "chat/completions"):
		if _, ok := payload["max_tokens"]; ok {
			return "max_tokens"
		}
		return "max_completion_tokens"
	default:
		return ""
	}
}

func setTokenLimit(payload map[string]any, field string, maxTokens int64) {
	if current, ok := numericJSONInt(payload[field]); ok && current > 0 && current < maxTokens {
		return
	}
	payload[field] = maxTokens
}

func numericJSONInt(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		if typed <= 0 {
			return 0, false
		}
		return int64(math.Floor(typed)), true
	case int64:
		return typed, typed > 0
	case int:
		return int64(typed), typed > 0
	default:
		return 0, false
	}
}

func estimatedTokensFromBytes(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return (value + 1) / 2
}

func fillUsage(db *gorm.DB, log *model.APILog, body []byte, groupMultipliers ...any) {
	var payload usagePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return
	}
	if payload.Response.Model != "" || payload.Response.Usage.TotalTokens > 0 || payload.Response.Usage.InputTokens > 0 {
		payload.Model = firstNonEmpty(payload.Response.Model, payload.Model)
		payload.Usage = payload.Response.Usage
	}
	applyUsage(db, log, payload.Model, payload.Usage, firstGroupMultipliers(groupMultipliers))
}

type usagePayload struct {
	Model    string        `json:"model"`
	Usage    responseUsage `json:"usage"`
	Response struct {
		Model string        `json:"model"`
		Usage responseUsage `json:"usage"`
	} `json:"response"`
}

type responseUsage struct {
	PromptTokens       int64        `json:"prompt_tokens"`
	CompletionTokens   int64        `json:"completion_tokens"`
	TotalTokens        int64        `json:"total_tokens"`
	InputTokens        int64        `json:"input_tokens"`
	OutputTokens       int64        `json:"output_tokens"`
	InputTokenDetails  tokenDetails `json:"input_tokens_details"`
	PromptTokenDetails tokenDetails `json:"prompt_tokens_details"`
	Cost               float64      `json:"cost"`
	TotalCost          float64      `json:"total_cost"`
	CostUSD            float64      `json:"cost_usd"`
}

type tokenDetails struct {
	CachedTokens int64 `json:"cached_tokens"`
}

func applyUsage(db *gorm.DB, log *model.APILog, modelName string, usage responseUsage, groupMultipliers any) {
	if modelName != "" {
		log.ModelName = modelName
	}
	promptTokens := usage.PromptTokens
	if promptTokens <= 0 {
		promptTokens = usage.InputTokens
	}
	completionTokens := usage.CompletionTokens
	if completionTokens <= 0 {
		completionTokens = usage.OutputTokens
	}
	totalTokens := usage.TotalTokens
	if totalTokens <= 0 {
		totalTokens = promptTokens + completionTokens
	}
	cachedInputTokens := usage.InputTokenDetails.CachedTokens
	if cachedInputTokens <= 0 {
		cachedInputTokens = usage.PromptTokenDetails.CachedTokens
	}
	if promptTokens > 0 {
		log.PromptTokens = promptTokens
	}
	if cachedInputTokens > 0 {
		log.CachedInputTokens = cachedInputTokens
	}
	if completionTokens > 0 {
		log.CompletionTokens = completionTokens
	}
	if totalTokens > 0 {
		log.TotalTokens = totalTokens
	}
	if !usesRequestBilling(db, log.ModelName) && applyUpstreamCost(log, usage, groupMultipliers) {
		return
	}
	applyBillingResult(log, service.BillUsageWithGroupMultipliers(db, log.ModelName, promptTokens, cachedInputTokens, completionTokens, totalTokens, groupMultipliers))
}

func usesRequestBilling(db *gorm.DB, modelName string) bool {
	pricing, _ := service.FindModelPricing(db, modelName)
	return pricing.BillingMode == model.ModelBillingModeRequest
}

type usageStreamReadCloser struct {
	io.ReadCloser
	start         time.Time
	log           *model.APILog
	db            *gorm.DB
	hub           *service.LogHub
	multipliers   any
	reservationID uint
	buf           bytes.Buffer
	firstToken    bool
	closed        bool
}

func (r *usageStreamReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		if !r.firstToken {
			r.log.FirstTokenMs = time.Since(r.start).Milliseconds()
			r.firstToken = true
		}
		r.buf.Write(p[:n])
	}
	if err == io.EOF {
		r.finish()
	}
	return n, err
}

func (r *usageStreamReadCloser) Close() error {
	r.finish()
	return r.ReadCloser.Close()
}

func (r *usageStreamReadCloser) finish() {
	if r.closed {
		return
	}
	r.closed = true
	fillStreamUsage(r.db, r.log, r.buf.Bytes(), r.multipliers)
	finalizeUsageLog(r.db, r.hub, r.log, r.start, r.reservationID, r.multipliers)
}

func fillStreamUsage(db *gorm.DB, log *model.APILog, body []byte, groupMultipliers ...any) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 1024), 1024*1024*10)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		fillUsage(db, log, []byte(data), firstGroupMultipliers(groupMultipliers))
	}
}

func finalizeUsageLog(db *gorm.DB, hub *service.LogHub, log *model.APILog, start time.Time, reservationID uint, groupMultipliers ...any) {
	now := time.Now()
	if log.RequestType == "" {
		log.RequestType = requestType(log.Path, false)
	}
	if log.CompletionTokens <= 0 && log.TotalTokens > log.PromptTokens {
		log.CompletionTokens = log.TotalTokens - log.PromptTokens
	}
	if log.EstimatedUSDMicros <= 0 {
		applyBillingResult(log, service.BillUsageWithGroupMultipliers(db, log.ModelName, log.PromptTokens, log.CachedInputTokens, log.CompletionTokens, log.TotalTokens, firstGroupMultipliers(groupMultipliers)))
	}
	log.LatencyMs = time.Since(start).Milliseconds()
	service.CompleteQuotaReservationWithAPILog(db, reservationID, log, now)
	hub.Broadcast(service.LogEvent{
		UserID:     log.UserID,
		APIKeyID:   log.APIKeyID,
		Method:     log.Method,
		Path:       log.Path,
		StatusCode: log.StatusCode,
		LatencyMs:  log.LatencyMs,
		CreatedAt:  now,
	})
}

func isEventStream(resp *http.Response) bool {
	return strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream")
}

func shouldRecordUsageResponse(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

func requestType(path string, stream bool) string {
	endpoint := strings.TrimPrefix(path, "/v1/")
	if stream {
		return "stream"
	}
	switch {
	case strings.Contains(endpoint, "messages"):
		return "claude"
	case strings.Contains(endpoint, "responses"):
		return "responses"
	case strings.Contains(endpoint, "chat/completions"):
		return "chat"
	default:
		return "api"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func applyBillingResult(log *model.APILog, result service.BillingResult) {
	if result.InputTokens > 0 {
		log.PromptTokens = result.InputTokens
	}
	if result.CachedInputTokens > 0 {
		log.CachedInputTokens = result.CachedInputTokens
	}
	if result.OutputTokens > 0 {
		log.CompletionTokens = result.OutputTokens
	}
	log.InputUSDMicros = result.InputUSDMicros
	log.CachedInputUSDMicros = result.CachedInputUSDMicros
	log.OutputUSDMicros = result.OutputUSDMicros
	log.RequestUSDMicros = result.RequestUSDMicros
	log.EstimatedUSDMicros = result.TotalUSDMicros
	log.EstimatedUSDCents = result.TotalUSDCents
	log.InputUSDPerMillion = result.InputUSDPerMillion
	log.CachedInputUSDPerMillion = result.CachedInputUSDPerMillion
	log.OutputUSDPerMillion = result.OutputUSDPerMillion
	log.RequestUSD = result.RequestUSD
	log.BillingMultiplier = result.BillingMultiplier
	log.GroupMultiplier = result.GroupMultiplier
	log.BillingSource = result.BillingSource
}

func applyUpstreamCost(log *model.APILog, usage responseUsage, groupMultipliers any) bool {
	costUSD := upstreamCostUSD(usage)
	if costUSD <= 0 {
		return false
	}

	multiplier := service.ResolveGroupMultiplier(model.ModelPricing{ModelName: log.ModelName, GroupMultiplier: 1}, groupMultipliers)
	micros := int64(costUSD*multiplier*1_000_000 + 0.5)
	log.InputUSDMicros = 0
	log.CachedInputUSDMicros = 0
	log.OutputUSDMicros = 0
	log.RequestUSDMicros = 0
	log.EstimatedUSDMicros = micros
	log.EstimatedUSDCents = service.USDmicrosToCents(micros)
	log.RequestUSD = 0
	log.BillingMultiplier = multiplier
	log.GroupMultiplier = multiplier
	log.BillingSource = "upstream_cost"
	return true
}

func firstGroupMultipliers(values []any) any {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func upstreamCostUSD(usage responseUsage) float64 {
	switch {
	case usage.CostUSD > 0:
		return usage.CostUSD
	case usage.TotalCost > 0:
		return usage.TotalCost
	case usage.Cost > 0:
		return usage.Cost
	default:
		return 0
	}
}

func intToString(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
