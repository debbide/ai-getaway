package service

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-gateway/model"

	"gorm.io/gorm"
)

const (
	defaultChannelMonitorIntervalSeconds = 300
	minChannelMonitorIntervalSeconds     = 60
	maxChannelMonitorIntervalSeconds     = 86400
	channelMonitorTimeout                = 10 * time.Second
	channelMonitorDegradedLatencyMs      = 2000
)

func NormalizeChannelMonitorInterval(seconds int) int {
	if seconds <= 0 {
		return defaultChannelMonitorIntervalSeconds
	}
	if seconds < minChannelMonitorIntervalSeconds {
		return minChannelMonitorIntervalSeconds
	}
	if seconds > maxChannelMonitorIntervalSeconds {
		return maxChannelMonitorIntervalSeconds
	}
	return seconds
}

func StartChannelMonitorRunner(db *gorm.DB) {
	if db == nil {
		return
	}
	go func() {
		time.Sleep(3 * time.Second)
		runDueChannelMonitors(db)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			runDueChannelMonitors(db)
		}
	}()
}

func RunChannelMonitorNow(db *gorm.DB, monitorID uint) (*model.ChannelMonitorRecord, error) {
	var monitor model.ChannelMonitor
	if err := db.First(&monitor, monitorID).Error; err != nil {
		return nil, err
	}
	return pingChannelMonitor(db, monitor)
}

func runDueChannelMonitors(db *gorm.DB) {
	var monitors []model.ChannelMonitor
	if err := db.Where("enabled = ?", true).Order("id asc").Find(&monitors).Error; err != nil {
		log.Printf("channel monitor load failed: %v", err)
		return
	}
	for _, monitor := range monitors {
		interval := NormalizeChannelMonitorInterval(monitor.MonitorIntervalSeconds)
		var latest model.ChannelMonitorRecord
		err := db.Where("channel_monitor_id = ?", monitor.ID).Order("checked_at desc").First(&latest).Error
		if err == nil && latest.CheckedAt.After(time.Now().Add(-time.Duration(interval)*time.Second)) {
			continue
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("channel monitor latest record failed: %v", err)
			continue
		}
		if _, err := pingChannelMonitor(db, monitor); err != nil {
			log.Printf("channel monitor ping failed: %v", err)
		}
	}
	pruneChannelMonitorRecords(db)
}

func pingChannelMonitor(db *gorm.DB, monitor model.ChannelMonitor) (*model.ChannelMonitorRecord, error) {
	target := strings.TrimSpace(monitor.APIURL)
	record := model.ChannelMonitorRecord{
		ChannelMonitorID: monitor.ID,
		Status:           model.ChannelMonitorStatusUnavailable,
		CheckedAt:        time.Now(),
	}
	if target == "" {
		record.ErrorMessage = "api url required"
		if err := db.Create(&record).Error; err != nil {
			return nil, err
		}
		return &record, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), channelMonitorTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		record.ErrorMessage = err.Error()
		if createErr := db.Create(&record).Error; createErr != nil {
			return nil, createErr
		}
		return &record, nil
	}
	req.Header.Set("User-Agent", "ai-getaway-channel-monitor/1.0")
	req.Header.Set("Accept", "application/json,text/plain,*/*")

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	record.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		record.ErrorMessage = err.Error()
	} else {
		defer resp.Body.Close()
		record.StatusCode = resp.StatusCode
		record.Status = classifyChannelMonitorStatus(resp.StatusCode, record.LatencyMs)
	}
	if err := db.Create(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func classifyChannelMonitorStatus(statusCode int, latencyMs int64) string {
	if statusCode <= 0 || statusCode >= 500 {
		return model.ChannelMonitorStatusUnavailable
	}
	if latencyMs >= channelMonitorDegradedLatencyMs || statusCode == http.StatusTooManyRequests || statusCode == http.StatusRequestTimeout {
		return model.ChannelMonitorStatusDegraded
	}
	return model.ChannelMonitorStatusAvailable
}

func pruneChannelMonitorRecords(db *gorm.DB) {
	cutoff := time.Now().AddDate(0, 0, -35)
	if err := db.Unscoped().Where("checked_at < ?", cutoff).Delete(&model.ChannelMonitorRecord{}).Error; err != nil {
		log.Printf("channel monitor prune failed: %v", err)
	}
}
