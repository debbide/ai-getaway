package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ai-gateway/config"
	"ai-gateway/response"
	"ai-gateway/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type LoadBalancerController struct {
	cfg         config.Config
	redisClient *redis.Client
}

type internalLogsResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items []service.RuntimeLogEntry `json:"items"`
	} `json:"data"`
}

func NewLoadBalancerController(cfg config.Config, redisClient *redis.Client) *LoadBalancerController {
	return &LoadBalancerController{cfg: cfg, redisClient: redisClient}
}

func (l *LoadBalancerController) Nodes(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	response.OK(c, service.ClusterSnapshotForAdmin(ctx, l.cfg, l.redisClient))
}

func (l *LoadBalancerController) PingNode(c *gin.Context) {
	instanceID := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	for _, node := range service.ClusterSnapshotForAdmin(ctx, l.cfg, l.redisClient).Nodes {
		if node.InstanceID == instanceID {
			response.OK(c, node)
			return
		}
	}
	response.Error(c, http.StatusNotFound, "cluster node not found")
}

func (l *LoadBalancerController) NodeLogs(c *gin.Context) {
	instanceID := c.Param("id")
	limit := parseLogLimit(c.Query("limit"))
	if instanceID == l.cfg.InstanceID {
		response.OK(c, gin.H{"items": service.RuntimeLogs(limit)})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	node, ok := service.FindClusterNode(ctx, l.cfg, l.redisClient, instanceID)
	if !ok {
		response.Error(c, http.StatusNotFound, "cluster node not found")
		return
	}
	if node.AdvertiseURL == "" {
		response.Error(c, http.StatusBadRequest, "cluster node advertise url missing")
		return
	}
	logURL, err := joinClusterURL(node.AdvertiseURL, fmt.Sprintf("/internal/cluster/logs?limit=%d", limit))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, logURL, nil)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.Header.Set("X-Cluster-Token", l.cfg.ClusterToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "failed to fetch cluster node logs: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		response.Error(c, http.StatusBadGateway, fmt.Sprintf("cluster node logs returned %d", resp.StatusCode))
		return
	}
	var payload internalLogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		response.Error(c, http.StatusBadGateway, "failed to decode cluster node logs")
		return
	}
	response.OK(c, gin.H{"items": payload.Data.Items})
}

func parseLogLimit(value string) int {
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 200
	}
	if limit > 800 {
		return 800
	}
	return limit
}

func joinClusterURL(base string, pathWithQuery string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid instance url: %s", base)
	}
	path, rawQuery, _ := strings.Cut(pathWithQuery, "?")
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = rawQuery
	return parsed.String(), nil
}
