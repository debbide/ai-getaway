package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const endpointSpeedTimeout = 10 * time.Second

type EndpointSpeedController struct {
	db *gorm.DB
}

func NewEndpointSpeedController(db *gorm.DB) *EndpointSpeedController {
	return &EndpointSpeedController{db: db}
}

type endpointSpeedRequest struct {
	URL string `json:"url" binding:"required"`
}

func (s *EndpointSpeedController) Test(c *gin.Context) {
	var req endpointSpeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	target := strings.TrimSpace(req.URL)
	if !isHTTPURL(target) {
		response.Error(c, 400, "invalid api endpoint")
		return
	}
	configured, err := s.isConfiguredEndpoint(target)
	if err != nil {
		response.Error(c, 500, "failed to load settings")
		return
	}
	if !configured {
		response.Error(c, 400, "invalid api endpoint")
		return
	}

	latencyMs, statusCode, err := testEndpointSpeed(c.Request.Context(), target)
	if err != nil {
		response.Error(c, 502, "speed test failed")
		return
	}
	response.OK(c, gin.H{
		"latency_ms":  latencyMs,
		"status_code": statusCode,
		"result":      formatLatency(latencyMs),
	})
}

func (s *EndpointSpeedController) isConfiguredEndpoint(target string) (bool, error) {
	if err := ensureSystemSettingColumns(s.db); err != nil {
		return false, err
	}
	setting := loadSettings(s.db)
	var endpoints []apiEndpointSetting
	if err := json.Unmarshal([]byte(setting.APIEndpoints), &endpoints); err != nil {
		return false, err
	}
	for _, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.URL) == target {
			return true, nil
		}
	}
	return false, nil
}

func testEndpointSpeed(parent context.Context, target string) (int64, int, error) {
	ctx, cancel := context.WithTimeout(parent, endpointSpeedTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "ai-getaway-local-speed-test/1.0")
	req.Header.Set("Accept", "application/json,text/plain,*/*")

	client := &http.Client{Timeout: endpointSpeedTimeout}
	start := time.Now()
	resp, err := client.Do(req)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		return latencyMs, 0, err
	}
	defer resp.Body.Close()
	return latencyMs, resp.StatusCode, nil
}

func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func formatLatency(latencyMs int64) string {
	return strconv.FormatInt(latencyMs, 10) + "ms"
}
