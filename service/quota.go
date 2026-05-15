package service

import (
	"math"
	"time"

	"ai-gateway/model"

	"gorm.io/gorm"
)

type QuotaUsage struct {
	Period         string    `json:"period"`
	LimitUSDCents  int64     `json:"limit_usd_cents"`
	UsedUSDCents   int64     `json:"used_usd_cents"`
	RemainingCents int64     `json:"remaining_usd_cents"`
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
	Percent        float64   `json:"percent"`
}

func PlanQuotaUsage(db *gorm.DB, userID uint, plan *model.Plan, now time.Time) QuotaUsage {
	period := "weekly"
	if plan != nil && plan.QuotaPeriod == model.QuotaPeriodPublic {
		period = model.QuotaPeriodPublic
	} else if plan != nil && plan.QuotaPeriod == "daily" {
		period = "daily"
	}

	start, end := QuotaWindow(period, now)
	limit := int64(0)
	if plan != nil {
		limit = plan.SettlementUSDCents
	}
	used := UsedUSDCentsSince(db, userID, start)
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	percent := float64(0)
	if limit > 0 {
		percent = float64(used) / float64(limit) * 100
		if percent > 100 {
			percent = 100
		}
	}

	return QuotaUsage{
		Period:         period,
		LimitUSDCents:  limit,
		UsedUSDCents:   used,
		RemainingCents: remaining,
		WindowStart:    start,
		WindowEnd:      end,
		Percent:        percent,
	}
}

func PlanTotalQuotaUsage(db *gorm.DB, userID uint, plan *model.Plan, start time.Time, end time.Time) QuotaUsage {
	period := "weekly"
	if plan != nil && plan.QuotaPeriod == model.QuotaPeriodPublic {
		period = model.QuotaPeriodPublic
	} else if plan != nil && plan.QuotaPeriod == "daily" {
		period = "daily"
	}

	limit := PlanTotalLimitUSDCents(plan)
	used := UsedUSDCentsSince(db, userID, start)
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	percent := float64(0)
	if limit > 0 {
		percent = float64(used) / float64(limit) * 100
		if percent > 100 {
			percent = 100
		}
	}

	return QuotaUsage{
		Period:         period,
		LimitUSDCents:  limit,
		UsedUSDCents:   used,
		RemainingCents: remaining,
		WindowStart:    start,
		WindowEnd:      end,
		Percent:        percent,
	}
}

func PlanTotalLimitUSDCents(plan *model.Plan) int64 {
	if plan == nil {
		return 0
	}
	if plan.QuotaPeriod == model.QuotaPeriodPublic {
		return plan.SettlementUSDCents
	}
	units := plan.DurationDays
	if units < 1 {
		units = 1
	}
	if plan.QuotaPeriod != "daily" {
		units = int(math.Round(float64(units) / 7))
		if units < 1 {
			units = 1
		}
	}
	return plan.SettlementUSDCents * int64(units)
}

func QuotaWindow(period string, now time.Time) (time.Time, time.Time) {
	if period == model.QuotaPeriodPublic {
		return time.Time{}, now.AddDate(100, 0, 0)
	}
	location := now.Location()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	if period == "daily" {
		return dayStart, dayStart.AddDate(0, 0, 1)
	}

	daysSinceMonday := (int(now.Weekday()) + 6) % 7
	weekStart := dayStart.AddDate(0, 0, -daysSinceMonday)
	return weekStart, weekStart.AddDate(0, 0, 7)
}

func UsedUSDCentsSince(db *gorm.DB, userID uint, since time.Time) int64 {
	var total int64
	db.Model(&model.APILog{}).
		Where("user_id = ? AND created_at >= ?", userID, since).
		Select("COALESCE(SUM(CASE WHEN estimated_usd_micros > 0 THEN CEILING(estimated_usd_micros / 10000) ELSE estimated_usd_cents END), 0)").
		Scan(&total)
	return total
}
