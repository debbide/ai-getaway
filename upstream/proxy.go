package upstream

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
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
		quotaBudgetMicros, quotaLimited := requestQuotaBudgetMicros(db, apiKey.User, start)
		if quotaLimited && estimateUsageMicros(db, requestInfo.Model, requestInfo.BodyBytes, 0, groupMultipliers) >= quotaBudgetMicros {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "subscription quota exceeded"})
			return
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
					ReadCloser:        resp.Body,
					start:             start,
					log:               &log,
					db:                db,
					hub:               hub,
					multipliers:       groupMultipliers,
					quotaLimited:      quotaLimited,
					quotaBudgetMicros: quotaBudgetMicros,
					requestBodyBytes:  requestInfo.BodyBytes,
				}
			} else {
				body, err := io.ReadAll(resp.Body)
				if err == nil {
					fillUsage(db, &log, body, groupMultipliers)
					quotaExceeded := quotaLimited && capResponseToQuota(&log, quotaBudgetMicros)
					if quotaExceeded {
						log.StatusCode = http.StatusTooManyRequests
					}
					finalizeUsageLog(db, hub, &log, start, groupMultipliers)
					if quotaExceeded {
						body = quotaExceededBody()
						resp.StatusCode = http.StatusTooManyRequests
						resp.Status = "429 Too Many Requests"
						resp.Header.Set("Content-Type", "application/json; charset=utf-8")
						resp.Header.Set("X-Quota-Exceeded", "true")
						resp.ContentLength = int64(len(body))
						resp.Header.Set("Content-Length", intToString(uint(len(body))))
					}
					resp.Body = io.NopCloser(bytes.NewReader(body))
					return nil
				}
				finalizeUsageLog(db, hub, &log, start, groupMultipliers)
			}
			return nil
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log := model.APILog{
				UserID:       apiKey.UserID,
				APIKeyID:     apiKey.ID,
				Method:       r.Method,
				Path:         r.URL.Path,
				StatusCode:   502,
				LatencyMs:    time.Since(start).Milliseconds(),
				ErrorMessage: err.Error(),
			}
			service.CreateAPILogWithinPlanQuota(db, &log, time.Now())
			http.Error(w, `{"error":"upstream request failed"}`, http.StatusBadGateway)
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
	if applyUpstreamCost(log, usage, groupMultipliers) {
		return
	}
	applyBillingResult(log, service.BillUsageWithGroupMultipliers(db, log.ModelName, promptTokens, cachedInputTokens, completionTokens, totalTokens, groupMultipliers))
}

type usageStreamReadCloser struct {
	io.ReadCloser
	start             time.Time
	log               *model.APILog
	db                *gorm.DB
	hub               *service.LogHub
	multipliers       any
	quotaLimited      bool
	quotaBudgetMicros int64
	requestBodyBytes  int64
	buf               bytes.Buffer
	firstToken        bool
	closed            bool
	cutoff            bool
}

func (r *usageStreamReadCloser) Read(p []byte) (int, error) {
	if r.cutoff {
		r.finish()
		return 0, io.EOF
	}
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		if !r.firstToken {
			r.log.FirstTokenMs = time.Since(r.start).Milliseconds()
			r.firstToken = true
		}
		r.buf.Write(p[:n])
		if r.streamBudgetExceeded() {
			r.cutoff = true
			_ = r.ReadCloser.Close()
			r.finish()
			return 0, io.EOF
		}
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
	if r.cutoff || capResponseToQuota(r.log, r.quotaBudgetMicros) {
		r.log.StatusCode = http.StatusTooManyRequests
	}
	finalizeUsageLog(r.db, r.hub, r.log, r.start, r.multipliers)
}

func (r *usageStreamReadCloser) streamBudgetExceeded() bool {
	if !r.quotaLimited {
		return false
	}
	if streamExceedsQuota(r.db, r.log, r.buf.Bytes(), r.multipliers, r.quotaBudgetMicros) {
		return true
	}
	return estimateUsageMicros(r.db, r.log.ModelName, r.requestBodyBytes, int64(r.buf.Len()), r.multipliers) >= r.quotaBudgetMicros
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

func streamExceedsQuota(db *gorm.DB, log *model.APILog, body []byte, groupMultipliers any, quotaBudgetMicros int64) bool {
	current := *log
	fillStreamUsage(db, &current, body, groupMultipliers)
	return current.EstimatedUSDMicros >= quotaBudgetMicros
}

func capResponseToQuota(log *model.APILog, quotaBudgetMicros int64) bool {
	if quotaBudgetMicros <= 0 || log == nil || log.EstimatedUSDMicros <= quotaBudgetMicros {
		return false
	}
	log.ErrorMessage = appendErrorMessage(log.ErrorMessage, "subscription quota reached during request")
	return true
}

func finalizeUsageLog(db *gorm.DB, hub *service.LogHub, log *model.APILog, start time.Time, groupMultipliers ...any) {
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
	service.CreateAPILogWithinPlanQuota(db, log, now)
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

func requestQuotaBudgetMicros(db *gorm.DB, user model.User, now time.Time) (int64, bool) {
	usage, ok := service.UserPlanQuotaUsage(db, user, now)
	if !ok || usage.LimitUSDCents <= 0 {
		return 0, false
	}
	if usage.RemainingCents <= 0 {
		return 0, true
	}
	return usage.RemainingCents * 10_000, true
}

func estimateUsageMicros(db *gorm.DB, modelName string, inputBytes int64, outputBytes int64, groupMultipliers any) int64 {
	inputTokens := estimatedTokensFromBytes(inputBytes)
	outputTokens := estimatedTokensFromBytes(outputBytes)
	if inputTokens <= 0 && outputTokens <= 0 {
		return 0
	}
	return service.BillUsageWithGroupMultipliers(db, modelName, inputTokens, 0, outputTokens, inputTokens+outputTokens, groupMultipliers).TotalUSDMicros
}

func estimatedTokensFromBytes(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return (value + 1) / 2
}

func appendErrorMessage(current string, next string) string {
	current = strings.TrimSpace(current)
	if current == "" {
		return next
	}
	if strings.Contains(current, next) {
		return current
	}
	return current + "; " + next
}

func quotaExceededBody() []byte {
	return []byte(`{"error":"subscription quota exceeded"}`)
}

func isEventStream(resp *http.Response) bool {
	return strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream")
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
	log.EstimatedUSDMicros = result.TotalUSDMicros
	log.EstimatedUSDCents = result.TotalUSDCents
	log.InputUSDPerMillion = result.InputUSDPerMillion
	log.CachedInputUSDPerMillion = result.CachedInputUSDPerMillion
	log.OutputUSDPerMillion = result.OutputUSDPerMillion
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
	log.EstimatedUSDMicros = micros
	log.EstimatedUSDCents = service.USDmicrosToCents(micros)
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
