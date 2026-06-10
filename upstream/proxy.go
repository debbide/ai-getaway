package upstream

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ai-gateway/config"
	"ai-gateway/model"
	"ai-gateway/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const defaultMaxSSELineBytes = 1024 * 1024

var errBodyTooLarge = errors.New("proxy body too large")
var errNoUpstreamForAccess = errors.New("令牌额度耗尽")

func ProxyHandler(cfg config.Config, db *gorm.DB, hub *service.LogHub) gin.HandlerFunc {
	transport := newProxyTransport(cfg.UpstreamTimeout)
	return func(c *gin.Context) {
		service.AddActiveAPIConnection(1)
		defer service.AddActiveAPIConnection(-1)

		apiKey := c.MustGet("api_key").(model.APIKey)
		start := time.Now()
		requestInfo, err := parseRequestInfo(c.Request, cfg.MaxProxyBodyBytes)
		if errors.Is(err, errBodyTooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "请求体过大"})
			return
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求体解析失败"})
			return
		}
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
		accessSource := model.AccessSourcePlan
		if quotaReservation != nil {
			quotaReservationID = quotaReservation.ID
			accessSource = proxyAccessSource(quotaReservation.AccessSource)
		}
		protocol := model.ProtocolGPT
		if value, ok := c.Get("protocol"); ok {
			protocol = service.NormalizeProtocol(value.(string))
		}
		upstreamBaseURL, upstreamAPIKey, groupMultipliers, err := selectProxyUpstream(db, apiKey, accessSource, protocol, start)
		if err != nil {
			service.CancelQuotaReservation(db, quotaReservationID, time.Now())
			status := http.StatusForbidden
			if err == errNoUpstreamForAccess {
				status = http.StatusTooManyRequests
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		target, err := url.Parse(strings.TrimRight(upstreamBaseURL, "/"))
		if err != nil {
			service.CancelQuotaReservation(db, quotaReservationID, time.Now())
			c.JSON(500, gin.H{"error": "invalid upstream base url"})
			return
		}
		if quotaReservation != nil {
			if cappedInfo, ok, err := applyDynamicOutputLimit(db, c.Request, requestInfo, quotaReservation.ReservedUSDCents, groupMultipliers, cfg.MaxProxyBodyBytes); errors.Is(err, errBodyTooLarge) {
				service.CancelQuotaReservation(db, quotaReservationID, time.Now())
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "请求体过大"})
				return
			} else if err != nil {
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
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Transport = transport
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
				UserID:       apiKey.UserID,
				APIKeyID:     apiKey.ID,
				Method:       c.Request.Method,
				Path:         c.Request.URL.Path,
				ModelName:    requestInfo.Model,
				StatusCode:   resp.StatusCode,
				RequestType:  requestType(c.Request.URL.Path, requestInfo.Stream),
				AccessSource: accessSource,
			}
			if isEventStream(resp) {
				resp.Body = &usageStreamReadCloser{
					ReadCloser:     resp.Body,
					start:          start,
					log:            &log,
					db:             db,
					hub:            hub,
					multipliers:    groupMultipliers,
					reservationID:  quotaReservationID,
					maxBufferBytes: cfg.MaxSSEUsageBufferBytes,
					maxLineBytes:   defaultMaxSSELineBytes,
				}
			} else {
				body, err := readAllLimited(resp.Body, cfg.MaxProxyBodyBytes)
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

func selectProxyUpstream(db *gorm.DB, apiKey model.APIKey, accessSource string, protocol string, now time.Time) (string, string, string, error) {
	if accessSource == model.AccessSourcePlan {
		if service.HasDirectPublicChannelAccess(apiKey.User, now) && apiKey.User.PlanID == nil {
			if apiKey.User.PublicChannelID == nil {
				return "", "", "", errNoUpstreamForAccess
			}
			var publicChannel model.PublicChannel
			if db.Where("id = ? AND enabled = ? AND remaining_usd_cents > 0", *apiKey.User.PublicChannelID, true).First(&publicChannel).Error != nil {
				return "", "", "", errNoUpstreamForAccess
			}
			if !service.SupportsProtocol(publicChannel.SupportsGPT, publicChannel.SupportsClaude, protocol) {
				return "", "", "", errors.New("protocol not supported by plan")
			}
			db.Model(&publicChannel).Updates(map[string]interface{}{"last_used_at": &now})
			return publicChannel.BaseURL, publicChannel.APIKey, publicChannel.GroupMultipliers, nil
		}
		if service.HasActiveSubscription(apiKey.User, now) && apiKey.User.Plan != nil && apiKey.User.Plan.PlanType == model.PlanTypePublic {
			if !service.PlanChannelSupportsProtocol(apiKey.User.Plan, protocol) {
				return "", "", "", errors.New("protocol not supported by plan")
			}
			if apiKey.User.Plan.PublicChannelID != nil {
				var publicChannel model.PublicChannel
				if db.Where("id = ? AND enabled = ? AND remaining_usd_cents > 0", *apiKey.User.Plan.PublicChannelID, true).First(&publicChannel).Error != nil {
					return "", "", "", errNoUpstreamForAccess
				}
				if !service.SupportsProtocol(publicChannel.SupportsGPT, publicChannel.SupportsClaude, protocol) {
					return "", "", "", errors.New("protocol not supported by plan")
				}
				db.Model(&publicChannel).Updates(map[string]interface{}{"last_used_at": &now})
				return publicChannel.BaseURL, publicChannel.APIKey, publicChannel.GroupMultipliers, nil
			}
			if apiKey.User.Plan.PollingPoolID != nil {
				var poolAccount model.PollingPoolAccount
				if db.Joins("JOIN polling_pools ON polling_pools.id = polling_pool_accounts.polling_pool_id").
					Where("polling_pool_accounts.polling_pool_id = ? AND polling_pool_accounts.enabled = ? AND (polling_pool_accounts.auth_type = ? OR polling_pool_accounts.remaining_usd_cents > 0) AND polling_pools.enabled = ?", *apiKey.User.Plan.PollingPoolID, true, service.OpenAIAccountAuthOAuth, true).
					Order("polling_pool_accounts.sort_order asc, polling_pool_accounts.id asc").
					First(&poolAccount).Error != nil {
					return "", "", "", errNoUpstreamForAccess
				}
				if err := service.RefreshPollingPoolAccountOAuth(db, &poolAccount, now); err != nil {
					return "", "", "", errors.New("openai oauth token refresh failed")
				}
				db.Model(&poolAccount).Updates(map[string]interface{}{"last_used_at": &now})
				return poolAccount.BaseURL, poolAccount.APIKey, poolAccount.GroupMultipliers, nil
			}
			return "", "", "", errNoUpstreamForAccess
		}
	}

	upstream, err := loadUserUpstreamForAccess(db, apiKey.UserID, accessSource)
	if err != nil && accessSource == model.AccessSourcePlan {
		upstream, err = loadUserUpstreamForAccess(db, apiKey.UserID, model.AccessSourceBalance)
	}
	if err != nil {
		return "", "", "", errors.New("no active upstream account bound")
	}
	if !service.SupportsProtocol(upstream.SupportsGPT, upstream.SupportsClaude, protocol) {
		return "", "", "", errors.New("protocol not supported by upstream")
	}
	if upstream.GroupMultipliers == "" {
		upstream.GroupMultipliers = loadUpstreamGroupMultipliers(db, upstream)
	}
	db.Model(&upstream).Updates(map[string]interface{}{"last_used_at": &now})
	return upstream.BaseURL, upstream.APIKey, upstream.GroupMultipliers, nil
}

func loadUserUpstreamForAccess(db *gorm.DB, userID uint, accessType string) (model.UpstreamAccount, error) {
	var upstream model.UpstreamAccount
	err := db.Where("user_id = ? AND access_type = ? AND status = ?", userID, proxyAccessSource(accessType), model.UpstreamStatusActive).First(&upstream).Error
	if err == nil {
		return upstream, nil
	}
	if accessType == model.AccessSourcePlan {
		err = db.Where("user_id = ? AND (access_type = ? OR access_type IS NULL) AND status = ?", userID, "", model.UpstreamStatusActive).First(&upstream).Error
	}
	return upstream, err
}

func loadUpstreamGroupMultipliers(db *gorm.DB, upstream model.UpstreamAccount) string {
	if channelID := upstream.ChannelID; channelID != nil {
		var channel model.UpstreamChannel
		if db.Select("group_multipliers").Where("id = ?", *channelID).First(&channel).Error == nil {
			return channel.GroupMultipliers
		}
	}
	var channel model.UpstreamChannel
	if db.Select("group_multipliers").Where("enabled = ? AND name = ?", true, upstream.Channel).First(&channel).Error == nil {
		return channel.GroupMultipliers
	}
	return ""
}

func proxyAccessSource(source string) string {
	if strings.TrimSpace(source) == model.AccessSourceBalance {
		return model.AccessSourceBalance
	}
	return model.AccessSourcePlan
}

func newProxyTransport(upstreamTimeout time.Duration) *http.Transport {
	if upstreamTimeout <= 0 {
		upstreamTimeout = 120 * time.Second
	}
	responseHeaderTimeout := upstreamTimeout
	if responseHeaderTimeout > 30*time.Second {
		responseHeaderTimeout = 30 * time.Second
	}
	dialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}
}

type requestInfo struct {
	Model     string
	Stream    bool
	BodyBytes int64
}

func parseRequestInfo(req *http.Request, maxBodyBytes int64) (requestInfo, error) {
	if req.Body == nil || req.Method == http.MethodGet {
		return requestInfo{}, nil
	}
	body, err := readAllLimited(req.Body, maxBodyBytes)
	if err != nil {
		return requestInfo{}, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	var payload struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return requestInfo{BodyBytes: int64(len(body))}, nil
	}
	return requestInfo{Model: payload.Model, Stream: payload.Stream, BodyBytes: int64(len(body))}, nil
}

func readAllLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}
	if maxBytes <= 0 {
		return io.ReadAll(reader)
	}
	limited := io.LimitReader(reader, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return body, err
	}
	if int64(len(body)) > maxBytes {
		return nil, errBodyTooLarge
	}
	return body, nil
}

func applyDynamicOutputLimit(db *gorm.DB, req *http.Request, info requestInfo, reservedUSDCents int64, groupMultipliers any, maxBodyBytes int64) (requestInfo, bool, error) {
	if req == nil || req.Body == nil || req.Method == http.MethodGet || reservedUSDCents <= 0 || info.BodyBytes <= 0 || info.Model == "" {
		return info, true, nil
	}
	body, err := readAllLimited(req.Body, maxBodyBytes)
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
	start            time.Time
	log              *model.APILog
	db               *gorm.DB
	hub              *service.LogHub
	multipliers      any
	reservationID    uint
	lineBuf          bytes.Buffer
	bufferedBytes    int64
	maxBufferBytes   int64
	maxLineBytes     int64
	bufferingStopped bool
	firstToken       bool
	closed           bool
}

func (r *usageStreamReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		if !r.firstToken {
			r.log.FirstTokenMs = time.Since(r.start).Milliseconds()
			r.firstToken = true
		}
		r.consume(p[:n])
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
	r.flushLine()
	finalizeUsageLog(r.db, r.hub, r.log, r.start, r.reservationID, r.multipliers)
}

func (r *usageStreamReadCloser) consume(chunk []byte) {
	if r.bufferingStopped {
		return
	}
	if r.maxBufferBytes > 0 && r.bufferedBytes+int64(len(chunk)) > r.maxBufferBytes {
		r.bufferingStopped = true
		r.lineBuf.Reset()
		return
	}
	r.bufferedBytes += int64(len(chunk))

	for _, b := range chunk {
		if b == '\n' {
			r.flushLine()
			continue
		}
		if r.maxLineBytes > 0 && int64(r.lineBuf.Len()) >= r.maxLineBytes {
			r.bufferingStopped = true
			r.lineBuf.Reset()
			return
		}
		_ = r.lineBuf.WriteByte(b)
	}
}

func (r *usageStreamReadCloser) flushLine() {
	if r.lineBuf.Len() == 0 {
		return
	}
	line := strings.TrimSpace(r.lineBuf.String())
	r.lineBuf.Reset()
	fillStreamUsageLine(r.db, r.log, line, r.multipliers)
}

func fillStreamUsage(db *gorm.DB, log *model.APILog, body []byte, groupMultipliers ...any) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 1024), 1024*1024*10)
	for scanner.Scan() {
		fillStreamUsageLine(db, log, strings.TrimSpace(scanner.Text()), firstGroupMultipliers(groupMultipliers))
	}
}

func fillStreamUsageLine(db *gorm.DB, log *model.APILog, line string, groupMultipliers any) {
	if !strings.HasPrefix(line, "data:") {
		return
	}
	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if data == "" || data == "[DONE]" {
		return
	}
	fillUsage(db, log, []byte(data), groupMultipliers)
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
	if hub != nil {
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
