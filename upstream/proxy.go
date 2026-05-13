package upstream

import (
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
		apiKey := c.MustGet("api_key").(model.APIKey)
		upstreamAccount := c.MustGet("upstream").(model.UpstreamAccount)

		target, err := url.Parse(strings.TrimRight(upstreamAccount.BaseURL, "/"))
		if err != nil {
			c.JSON(500, gin.H{"error": "invalid upstream base url"})
			return
		}

		start := time.Now()
		var responseBody bytes.Buffer
		proxy := httputil.NewSingleHostReverseProxy(target)
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Path = c.Request.URL.Path
			req.URL.RawQuery = c.Request.URL.RawQuery
			req.Host = target.Host
			req.Header.Set("Authorization", "Bearer "+upstreamAccount.APIKey)
			req.Header.Set("X-Forwarded-User-ID", intToString(apiKey.UserID))
		}
		proxy.FlushInterval = 100 * time.Millisecond
		proxy.ModifyResponse = func(resp *http.Response) error {
			log := model.APILog{
				UserID:     apiKey.UserID,
				APIKeyID:   apiKey.ID,
				Method:     c.Request.Method,
				Path:       c.Request.URL.Path,
				StatusCode: resp.StatusCode,
				LatencyMs:  time.Since(start).Milliseconds(),
			}
			if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
				body, err := io.ReadAll(resp.Body)
				if err == nil {
					responseBody.Write(body)
					fillUsage(&log, body)
					resp.Body = io.NopCloser(bytes.NewReader(body))
				}
			}
			db.Create(&log)
			hub.Broadcast(service.LogEvent{
				UserID:     log.UserID,
				APIKeyID:   log.APIKeyID,
				Method:     log.Method,
				Path:       log.Path,
				StatusCode: log.StatusCode,
				LatencyMs:  log.LatencyMs,
				CreatedAt:  time.Now(),
			})
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

func fillUsage(log *model.APILog, body []byte) {
	var payload struct {
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int64   `json:"prompt_tokens"`
			CompletionTokens int64   `json:"completion_tokens"`
			TotalTokens      int64   `json:"total_tokens"`
			Cost             float64 `json:"cost"`
			TotalCost        float64 `json:"total_cost"`
			CostUSD          float64 `json:"cost_usd"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		log.PromptTokens = payload.Usage.PromptTokens
		log.TotalTokens = payload.Usage.TotalTokens
		log.EstimatedUSDCents = usageCostCents(payload.Model, payload.Usage.PromptTokens, payload.Usage.CompletionTokens, payload.Usage.TotalTokens, payload.Usage.Cost, payload.Usage.TotalCost, payload.Usage.CostUSD)
	}
}

func usageCostCents(modelName string, promptTokens, completionTokens, totalTokens int64, costValues ...float64) int64 {
	for _, cost := range costValues {
		if cost > 0 {
			return int64(math.Ceil(cost * 100))
		}
	}
	if totalTokens <= 0 {
		return 0
	}

	inputPerMillion, outputPerMillion := modelRatesUSD(modelName)
	outputTokens := completionTokens
	if outputTokens <= 0 && totalTokens > promptTokens {
		outputTokens = totalTokens - promptTokens
	}
	inputTokens := promptTokens
	if inputTokens <= 0 {
		inputTokens = totalTokens - outputTokens
	}
	usd := (float64(inputTokens)/1_000_000)*inputPerMillion + (float64(outputTokens)/1_000_000)*outputPerMillion
	if usd <= 0 {
		return 0
	}
	return int64(math.Ceil(usd * 100))
}

func modelRatesUSD(modelName string) (float64, float64) {
	name := strings.ToLower(modelName)
	switch {
	case strings.Contains(name, "gpt-4o-mini"), strings.Contains(name, "gpt-4.1-mini"), strings.Contains(name, "mini"):
		return 0.15, 0.60
	case strings.Contains(name, "gpt-4o"), strings.Contains(name, "gpt-4.1"):
		return 2.50, 10.00
	case strings.Contains(name, "o1"), strings.Contains(name, "o3"):
		return 15.00, 60.00
	default:
		return 1.00, 4.00
	}
}

func intToString(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
