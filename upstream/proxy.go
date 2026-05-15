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
		if publicChannelValue, ok := c.Get("public_channel"); ok {
			publicChannel := publicChannelValue.(model.PublicChannel)
			upstreamBaseURL = publicChannel.BaseURL
			upstreamAPIKey = publicChannel.APIKey
		} else {
			upstreamAccount := c.MustGet("upstream").(model.UpstreamAccount)
			upstreamBaseURL = upstreamAccount.BaseURL
			upstreamAPIKey = upstreamAccount.APIKey
		}

		target, err := url.Parse(strings.TrimRight(upstreamBaseURL, "/"))
		if err != nil {
			c.JSON(500, gin.H{"error": "invalid upstream base url"})
			return
		}

		start := time.Now()
		requestInfo := parseRequestInfo(c.Request)
		proxy := httputil.NewSingleHostReverseProxy(target)
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Path = c.Request.URL.Path
			req.URL.RawQuery = c.Request.URL.RawQuery
			req.Host = target.Host
			req.Header.Set("Authorization", "Bearer "+upstreamAPIKey)
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
					ReadCloser: resp.Body,
					start:      start,
					log:        &log,
					db:         db,
					hub:        hub,
				}
			} else {
				body, err := io.ReadAll(resp.Body)
				if err == nil {
					fillUsage(db, &log, body)
					finalizeUsageLog(db, hub, &log, start)
					resp.Body = io.NopCloser(bytes.NewReader(body))
					return nil
				}
				finalizeUsageLog(db, hub, &log, start)
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
			db.Create(&log)
			http.Error(w, `{"error":"upstream request failed"}`, http.StatusBadGateway)
		}

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

type requestInfo struct {
	Model  string
	Stream bool
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
		return requestInfo{}
	}
	return requestInfo{Model: payload.Model, Stream: payload.Stream}
}

func fillUsage(db *gorm.DB, log *model.APILog, body []byte) {
	var payload usagePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return
	}
	if payload.Response.Model != "" || payload.Response.Usage.TotalTokens > 0 || payload.Response.Usage.InputTokens > 0 {
		payload.Model = firstNonEmpty(payload.Response.Model, payload.Model)
		payload.Usage = payload.Response.Usage
	}
	applyUsage(db, log, payload.Model, payload.Usage)
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

func applyUsage(db *gorm.DB, log *model.APILog, modelName string, usage responseUsage) {
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
	if applyUpstreamCost(log, usage) {
		return
	}
	applyBillingResult(log, service.BillUsage(db, log.ModelName, promptTokens, cachedInputTokens, completionTokens, totalTokens))
}

type usageStreamReadCloser struct {
	io.ReadCloser
	start      time.Time
	log        *model.APILog
	db         *gorm.DB
	hub        *service.LogHub
	buf        bytes.Buffer
	firstToken bool
	closed     bool
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
	fillStreamUsage(r.db, r.log, r.buf.Bytes())
	finalizeUsageLog(r.db, r.hub, r.log, r.start)
}

func fillStreamUsage(db *gorm.DB, log *model.APILog, body []byte) {
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
		fillUsage(db, log, []byte(data))
	}
}

func finalizeUsageLog(db *gorm.DB, hub *service.LogHub, log *model.APILog, start time.Time) {
	if log.RequestType == "" {
		log.RequestType = requestType(log.Path, false)
	}
	if log.CompletionTokens <= 0 && log.TotalTokens > log.PromptTokens {
		log.CompletionTokens = log.TotalTokens - log.PromptTokens
	}
	if log.EstimatedUSDMicros <= 0 {
		applyBillingResult(log, service.BillUsage(db, log.ModelName, log.PromptTokens, log.CachedInputTokens, log.CompletionTokens, log.TotalTokens))
	}
	log.LatencyMs = time.Since(start).Milliseconds()
	db.Create(log)
	hub.Broadcast(service.LogEvent{
		UserID:     log.UserID,
		APIKeyID:   log.APIKeyID,
		Method:     log.Method,
		Path:       log.Path,
		StatusCode: log.StatusCode,
		LatencyMs:  log.LatencyMs,
		CreatedAt:  time.Now(),
	})
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
	log.BillingSource = result.BillingSource
}

func applyUpstreamCost(log *model.APILog, usage responseUsage) bool {
	costUSD := upstreamCostUSD(usage)
	if costUSD <= 0 {
		return false
	}

	micros := int64(costUSD*1_000_000 + 0.5)
	log.InputUSDMicros = 0
	log.CachedInputUSDMicros = 0
	log.OutputUSDMicros = 0
	log.EstimatedUSDMicros = micros
	log.EstimatedUSDCents = service.USDmicrosToCents(micros)
	log.BillingMultiplier = 1
	log.BillingSource = "upstream_cost"
	return true
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
