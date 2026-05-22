package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"ai-gateway/config"

	"github.com/redis/go-redis/v9"
)

const (
	clusterNodeKeyPrefix = "ai_getaway:cluster_node:"
	clusterNodeTTL       = 45 * time.Second
)

var instanceStartedAt = time.Now()

type ClusterNode struct {
	InstanceID        string            `json:"instance_id"`
	AdvertiseURL      string            `json:"advertise_url"`
	Hostname          string            `json:"hostname"`
	AppEnv            string            `json:"app_env"`
	ClusterMode       bool              `json:"cluster_mode"`
	RunBackgroundJobs bool              `json:"run_background_jobs"`
	StartedAt         time.Time         `json:"started_at"`
	LastSeenAt        time.Time         `json:"last_seen_at"`
	ExpiresAt         time.Time         `json:"expires_at"`
	IsSelf            bool              `json:"is_self"`
	Status            string            `json:"status"`
	LatencyMs         int64             `json:"latency_ms"`
	ReadyStatus       string            `json:"ready_status"`
	Checks            map[string]string `json:"checks,omitempty"`
	Error             string            `json:"error,omitempty"`
}

type ClusterSummary struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Warning int `json:"warning"`
	Offline int `json:"offline"`
}

type ClusterSnapshot struct {
	Summary ClusterSummary `json:"summary"`
	Nodes   []ClusterNode  `json:"nodes"`
}

type readyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func StartClusterRegistry(cfg config.Config, redisClient *redis.Client) {
	if redisClient == nil {
		return
	}
	if !cfg.ClusterMode {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := redisClient.Ping(ctx).Err()
		cancel()
		if err != nil {
			return
		}
	}
	registerClusterNode(cfg, redisClient)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			registerClusterNode(cfg, redisClient)
		}
	}()
}

func ClusterSnapshotForAdmin(ctx context.Context, cfg config.Config, redisClient *redis.Client) ClusterSnapshot {
	nodes := loadClusterNodes(ctx, cfg, redisClient)
	for i := range nodes {
		probeClusterNode(ctx, &nodes[i])
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsSelf != nodes[j].IsSelf {
			return nodes[i].IsSelf
		}
		return nodes[i].InstanceID < nodes[j].InstanceID
	})
	return ClusterSnapshot{Summary: summarizeClusterNodes(nodes), Nodes: nodes}
}

func FindClusterNode(ctx context.Context, cfg config.Config, redisClient *redis.Client, instanceID string) (ClusterNode, bool) {
	for _, node := range loadClusterNodes(ctx, cfg, redisClient) {
		if node.InstanceID == instanceID {
			return node, true
		}
	}
	return ClusterNode{}, false
}

func CurrentClusterNode(cfg config.Config) ClusterNode {
	hostname, _ := os.Hostname()
	now := time.Now()
	return ClusterNode{
		InstanceID:        cfg.InstanceID,
		AdvertiseURL:      cfg.InstanceURL,
		Hostname:          hostname,
		AppEnv:            cfg.AppEnv,
		ClusterMode:       cfg.ClusterMode,
		RunBackgroundJobs: cfg.RunBackgroundJobs,
		StartedAt:         instanceStartedAt,
		LastSeenAt:        now,
		ExpiresAt:         now.Add(clusterNodeTTL),
		IsSelf:            true,
		Status:            "online",
	}
}

func registerClusterNode(cfg config.Config, redisClient *redis.Client) {
	if redisClient == nil {
		return
	}
	node := CurrentClusterNode(cfg)
	payload, err := json.Marshal(node)
	if err != nil {
		log.Printf("cluster registry marshal failed: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := redisClient.Set(ctx, clusterNodeKey(cfg.InstanceID), payload, clusterNodeTTL).Err(); err != nil {
		log.Printf("cluster registry heartbeat failed: %v", err)
	}
}

func loadClusterNodes(ctx context.Context, cfg config.Config, redisClient *redis.Client) []ClusterNode {
	nodesByID := map[string]ClusterNode{}
	if redisClient != nil {
		iter := redisClient.Scan(ctx, 0, clusterNodeKeyPrefix+"*", 100).Iterator()
		for iter.Next(ctx) {
			value, err := redisClient.Get(ctx, iter.Val()).Result()
			if err != nil {
				continue
			}
			var node ClusterNode
			if err := json.Unmarshal([]byte(value), &node); err != nil || node.InstanceID == "" {
				continue
			}
			node.IsSelf = node.InstanceID == cfg.InstanceID
			nodesByID[node.InstanceID] = node
		}
		if err := iter.Err(); err != nil {
			log.Printf("cluster registry scan failed: %v", err)
		}
	}
	current := CurrentClusterNode(cfg)
	if _, ok := nodesByID[current.InstanceID]; !ok {
		nodesByID[current.InstanceID] = current
	} else {
		node := nodesByID[current.InstanceID]
		node.IsSelf = true
		if node.AdvertiseURL == "" {
			node.AdvertiseURL = current.AdvertiseURL
		}
		nodesByID[current.InstanceID] = node
	}

	nodes := make([]ClusterNode, 0, len(nodesByID))
	for _, node := range nodesByID {
		nodes = append(nodes, node)
	}
	return nodes
}

func probeClusterNode(ctx context.Context, node *ClusterNode) {
	if node.AdvertiseURL == "" {
		node.Status = "warning"
		node.Error = "INSTANCE_ADVERTISE_URL 未配置"
		return
	}
	readyURL, err := joinNodeURL(node.AdvertiseURL, "/ready")
	if err != nil {
		node.Status = "warning"
		node.Error = err.Error()
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, readyURL, nil)
	if err != nil {
		node.Status = "warning"
		node.Error = err.Error()
		return
	}
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	node.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		node.Status = "offline"
		node.Error = err.Error()
		return
	}
	defer resp.Body.Close()

	var ready readyResponse
	if err := json.NewDecoder(resp.Body).Decode(&ready); err != nil {
		node.Status = "warning"
		node.ReadyStatus = fmt.Sprintf("http_%d", resp.StatusCode)
		node.Error = err.Error()
		return
	}
	node.ReadyStatus = ready.Status
	node.Checks = ready.Checks
	if resp.StatusCode >= 500 || ready.Status != "ready" {
		node.Status = "warning"
		return
	}
	node.Status = "online"
}

func summarizeClusterNodes(nodes []ClusterNode) ClusterSummary {
	summary := ClusterSummary{Total: len(nodes)}
	for _, node := range nodes {
		switch node.Status {
		case "online":
			summary.Online++
		case "offline":
			summary.Offline++
		default:
			summary.Warning++
		}
	}
	return summary
}

func clusterNodeKey(instanceID string) string {
	return clusterNodeKeyPrefix + instanceID
}

func joinNodeURL(base string, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid instance url: %s", base)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = ""
	return parsed.String(), nil
}
