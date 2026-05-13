package upstream

import (
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
		Usage struct {
			PromptTokens int64 `json:"prompt_tokens"`
			TotalTokens  int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		log.PromptTokens = payload.Usage.PromptTokens
		log.TotalTokens = payload.Usage.TotalTokens
	}
}

func intToString(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
