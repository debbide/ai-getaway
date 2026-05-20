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

type ChannelStatusController struct {
	db *gorm.DB
}

func NewChannelStatusController(db *gorm.DB) *ChannelStatusController {
	return &ChannelStatusController{db: db}
}

type publicChannelStatusResponse struct {
	ID            uint                        `json:"id"`
	ModelName     string                      `json:"model_name"`
	Status        string                      `json:"status"`
	StatusLabel   string                      `json:"status_label"`
	Availability  float64                     `json:"availability"`
	LatencyMs     int64                       `json:"latency_ms"`
	LastCheckedAt *time.Time                  `json:"last_checked_at"`
	Summary       publicChannelStatusSummary  `json:"summary"`
	Records       []publicChannelStatusRecord `json:"records"`
}

type publicChannelStatusSummary struct {
	Available   int `json:"available"`
	Degraded    int `json:"degraded"`
	Unavailable int `json:"unavailable"`
	Total       int `json:"total"`
}

type publicChannelStatusRecord struct {
	Status    string    `json:"status"`
	LatencyMs int64     `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at"`
}

func (s *ChannelStatusController) PublicList(c *gin.Context) {
	rangeDays := parseStatusRangeDays(c.DefaultQuery("range_days", "7"))
	cutoff := time.Now().AddDate(0, 0, -rangeDays)

	var monitors []model.ChannelMonitor
	if err := s.db.Where("enabled = ?", true).Order("model_name asc, id asc").Find(&monitors).Error; err != nil {
		response.Error(c, 500, "failed to load channel status")
		return
	}

	items := make([]publicChannelStatusResponse, 0, len(monitors))
	for _, monitor := range monitors {
		items = append(items, s.mapPublicChannelStatus(monitor, cutoff))
	}
	response.OK(c, gin.H{
		"range_days": rangeDays,
		"items":      items,
	})
}

func (s *ChannelStatusController) mapPublicChannelStatus(monitor model.ChannelMonitor, cutoff time.Time) publicChannelStatusResponse {
	var recent []model.ChannelMonitorRecord
	s.db.Where("channel_monitor_id = ?", monitor.ID).Order("checked_at desc").Limit(60).Find(&recent)

	var rangeRecords []model.ChannelMonitorRecord
	s.db.Where("channel_monitor_id = ? AND checked_at >= ?", monitor.ID, cutoff).Find(&rangeRecords)

	summary := publicChannelStatusSummary{Total: len(rangeRecords)}
	for _, record := range rangeRecords {
		switch record.Status {
		case model.ChannelMonitorStatusAvailable:
			summary.Available++
		case model.ChannelMonitorStatusDegraded:
			summary.Degraded++
		default:
			summary.Unavailable++
		}
	}

	status := model.ChannelMonitorStatusUnavailable
	var latency int64
	var lastCheckedAt *time.Time
	if len(recent) > 0 {
		status = recent[0].Status
		latency = recent[0].LatencyMs
		checkedAt := recent[0].CheckedAt
		lastCheckedAt = &checkedAt
	}

	records := make([]publicChannelStatusRecord, 0, len(recent))
	for i := len(recent) - 1; i >= 0; i-- {
		records = append(records, publicChannelStatusRecord{
			Status:    recent[i].Status,
			LatencyMs: recent[i].LatencyMs,
			CheckedAt: recent[i].CheckedAt,
		})
	}

	return publicChannelStatusResponse{
		ID:            monitor.ID,
		ModelName:     monitor.ModelName,
		Status:        status,
		StatusLabel:   channelMonitorStatusLabel(status),
		Availability:  channelAvailability(summary),
		LatencyMs:     latency,
		LastCheckedAt: lastCheckedAt,
		Summary:       summary,
		Records:       records,
	}
}

func parseStatusRangeDays(value string) int {
	days, _ := strconv.Atoi(strings.TrimSpace(value))
	switch days {
	case 15, 30:
		return days
	default:
		return 7
	}
}

func channelAvailability(summary publicChannelStatusSummary) float64 {
	if summary.Total == 0 {
		return 0
	}
	availableScore := float64(summary.Available) + float64(summary.Degraded)*0.5
	return math.Round(availableScore/float64(summary.Total)*10000) / 100
}

func channelMonitorStatusLabel(status string) string {
	switch status {
	case model.ChannelMonitorStatusAvailable:
		return "正常"
	case model.ChannelMonitorStatusDegraded:
		return "波动"
	case model.ChannelMonitorStatusUnavailable:
		return "不可用"
	default:
		return "未知"
	}
}
