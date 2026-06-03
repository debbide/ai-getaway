package service

import (
	"math"
	"strings"
	"time"

	"ai-gateway/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

const MinQuotaRemainingUSDCents int64 = 10

func PlanQuotaUsage(db *gorm.DB, userID uint, plan *model.Plan, now time.Time) QuotaUsage {
	return PlanQuotaUsageFrom(db, userID, plan, nil, now)
}

func UserPlanQuotaUsage(db *gorm.DB, user model.User, now time.Time) (QuotaUsage, bool) {
	if !HasActiveSubscription(user, now) || user.Plan == nil {
		return QuotaUsage{}, false
	}
	startedAt := SubscriptionStartAt(db, user, now)
	return PlanQuotaUsageFrom(db, user.ID, user.Plan, startedAt, now), true
}

func UserAccessQuotaUsage(db *gorm.DB, user model.User, now time.Time) (QuotaUsage, bool) {
	if HasActiveSubscription(user, now) && user.Plan != nil {
		return UserPlanQuotaUsage(db, user, now)
	}
	if !HasDirectPublicChannelAccess(user, now) || user.PublicChannel == nil {
		return QuotaUsage{}, false
	}
	startedAt := SubscriptionStartAt(db, user, now)
	return DirectPublicChannelQuotaUsageFrom(db, user.ID, user.PublicChannel, user.PublicChannelPeriod, startedAt, now), true
}

func QuotaAllowsRequest(usage QuotaUsage) bool {
	if usage.LimitUSDCents <= 0 {
		return true
	}
	return usage.RemainingCents > MinQuotaRemainingUSDCents
}

func PlanQuotaUsageFrom(db *gorm.DB, userID uint, plan *model.Plan, activeFrom *time.Time, now time.Time) QuotaUsage {
	period := "weekly"
	if plan != nil && plan.QuotaPeriod == model.QuotaPeriodPublic {
		period = model.QuotaPeriodPublic
	} else if plan != nil && plan.QuotaPeriod == "daily" {
		period = "daily"
	}

	start, end := QuotaUsageWindow(period, activeFrom, now)
	limit := int64(0)
	if plan != nil {
		limit = plan.SettlementUSDCents
	}
	used := capUsedUSDCents(UsedUSDCentsSince(db, userID, start), limit)
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
	used := capUsedUSDCents(UsedUSDCentsSince(db, userID, start), limit)
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

func DirectPublicChannelQuotaUsageFrom(db *gorm.DB, userID uint, channel *model.PublicChannel, period string, activeFrom *time.Time, now time.Time) QuotaUsage {
	normalizedPeriod := DirectPublicChannelPeriod(period)
	start, end := QuotaUsageWindow(normalizedPeriod, activeFrom, now)
	used := UsedUSDCentsSince(db, userID, start)
	remaining := int64(0)
	if channel != nil && channel.RemainingUSDCents > 0 {
		remaining = channel.RemainingUSDCents
	}
	limit := used + remaining
	percent := float64(0)
	if limit > 0 {
		percent = float64(used) / float64(limit) * 100
		if percent > 100 {
			percent = 100
		}
	}
	return QuotaUsage{
		Period:         normalizedPeriod,
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

func QuotaUsageWindow(period string, activeFrom *time.Time, now time.Time) (time.Time, time.Time) {
	start, end := QuotaWindow(period, now)
	if activeFrom != nil && activeFrom.After(start) {
		start = *activeFrom
	}
	return start, end
}

func UsedUSDCentsSince(db *gorm.DB, userID uint, since time.Time) int64 {
	return usedAPILogUSDCentsSince(db, userID, since) + ActiveReservedUSDCentsSince(db, userID, since, 0)
}

func usedAPILogUSDCentsSince(db *gorm.DB, userID uint, since time.Time) int64 {
	if db == nil {
		return 0
	}
	var total int64
	db.Model(&model.APILog{}).
		Where("user_id = ? AND created_at >= ?", userID, since).
		Select("COALESCE(SUM(CASE WHEN estimated_usd_micros > 0 THEN CEILING(estimated_usd_micros / 10000) ELSE estimated_usd_cents END), 0)").
		Scan(&total)
	return total
}

func ActiveReservedUSDCentsSince(db *gorm.DB, userID uint, since time.Time, excludeReservationID uint) int64 {
	if db == nil {
		return 0
	}
	query := db.Model(&model.QuotaReservation{}).
		Where("user_id = ? AND status = ? AND created_at >= ?", userID, model.QuotaReservationStatusActive, since)
	if excludeReservationID > 0 {
		query = query.Where("id <> ?", excludeReservationID)
	}
	var total int64
	query.Select("COALESCE(SUM(reserved_usd_cents), 0)").Scan(&total)
	return total
}

func capUsedUSDCents(used int64, limit int64) int64 {
	if limit > 0 && used > limit {
		return limit
	}
	return used
}

func CreateAPILogWithinPlanQuota(db *gorm.DB, log *model.APILog, now time.Time) error {
	if db == nil || log == nil {
		return nil
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = now
	}
	if log.UserID == 0 {
		return db.Create(log).Error
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Plan").Preload("PublicChannel").First(&user, log.UserID).Error; err != nil {
			return err
		}
		if HasActiveSubscription(user, now) && user.Plan != nil {
			startedAt := SubscriptionStartAt(tx, user, now)
			usage := PlanQuotaUsageFrom(tx, user.ID, user.Plan, startedAt, now)
			if usage.LimitUSDCents > 0 {
				capAPILogCost(log, usage.RemainingCents)
			}
		}
		if user.PlanID == nil && HasDirectPublicChannelAccess(user, now) && user.PublicChannelID != nil {
			var channel model.PublicChannel
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&channel, *user.PublicChannelID).Error; err != nil {
				return err
			}
			capAPILogCost(log, channel.RemainingUSDCents)
			cost := APILogUSDCents(log)
			if err := tx.Create(log).Error; err != nil {
				return err
			}
			if cost <= 0 {
				return nil
			}
			return tx.Model(&model.PublicChannel{}).
				Where("id = ? AND remaining_usd_cents >= ?", channel.ID, cost).
				Update("remaining_usd_cents", gorm.Expr("remaining_usd_cents - ?", cost)).Error
		}
		return tx.Create(log).Error
	})
}

func BeginQuotaReservation(db *gorm.DB, user model.User, apiKeyID uint, now time.Time) (*model.QuotaReservation, bool, error) {
	if db == nil {
		return nil, true, nil
	}
	if !HasActiveSubscription(user, now) || user.Plan == nil {
		return nil, true, nil
	}

	var reservation *model.QuotaReservation
	err := db.Transaction(func(tx *gorm.DB) error {
		var lockedUser model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Plan").First(&lockedUser, user.ID).Error; err != nil {
			return err
		}
		if !HasActiveSubscription(lockedUser, now) || lockedUser.Plan == nil {
			return nil
		}
		startedAt := SubscriptionStartAt(tx, lockedUser, now)
		usage := PlanQuotaUsageFrom(tx, lockedUser.ID, lockedUser.Plan, startedAt, now)
		if !QuotaAllowsRequest(usage) {
			reservation = nil
			return nil
		}
		reserved := usage.RemainingCents - MinQuotaRemainingUSDCents
		if reserved <= 0 {
			reservation = nil
			return nil
		}
		item := model.QuotaReservation{
			UserID:           lockedUser.ID,
			APIKeyID:         apiKeyID,
			ReservedUSDCents: reserved,
			Status:           model.QuotaReservationStatusActive,
		}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		reservation = &item
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if reservation == nil {
		return nil, false, nil
	}
	return reservation, true, nil
}

func CompleteQuotaReservationWithAPILog(db *gorm.DB, reservationID uint, log *model.APILog, now time.Time) error {
	if reservationID == 0 {
		return CreateAPILogWithinPlanQuota(db, log, now)
	}
	if db == nil || log == nil {
		return nil
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = now
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var reservation model.QuotaReservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&reservation, reservationID).Error; err != nil {
			return err
		}
		if reservation.Status != model.QuotaReservationStatusActive {
			return tx.Create(log).Error
		}

		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Plan").Preload("PublicChannel").First(&user, reservation.UserID).Error; err != nil {
			return err
		}
		if HasActiveSubscription(user, now) && user.Plan != nil {
			startedAt := SubscriptionStartAt(tx, user, now)
			start, _ := QuotaUsageWindow(user.Plan.QuotaPeriod, startedAt, now)
			limit := user.Plan.SettlementUSDCents
			usedLogs := capUsedUSDCents(usedAPILogUSDCentsSince(tx, user.ID, start), limit)
			otherReserved := ActiveReservedUSDCentsSince(tx, user.ID, start, reservation.ID)
			available := limit - usedLogs - otherReserved
			if available < 0 {
				available = 0
			}
			capAPILogCost(log, available)
		}
		if err := tx.Create(log).Error; err != nil {
			return err
		}
		completedAt := now
		return tx.Model(&reservation).Updates(map[string]interface{}{
			"reserved_usd_cents": 0,
			"status":             model.QuotaReservationStatusCompleted,
			"completed_at":       &completedAt,
		}).Error
	})
}

func APILogUSDCents(log *model.APILog) int64 {
	if log == nil {
		return 0
	}
	if log.EstimatedUSDMicros > 0 {
		return USDmicrosToCents(log.EstimatedUSDMicros)
	}
	if log.EstimatedUSDCents > 0 {
		return log.EstimatedUSDCents
	}
	return 0
}

func capAPILogCost(log *model.APILog, remainingUSDCents int64) {
	if remainingUSDCents < 0 {
		remainingUSDCents = 0
	}
	remainingMicros := remainingUSDCents * 10_000
	currentMicros := log.EstimatedUSDMicros
	if currentMicros <= 0 && log.EstimatedUSDCents > 0 {
		currentMicros = log.EstimatedUSDCents * 10_000
	}
	if currentMicros <= remainingMicros {
		return
	}

	scaleUSDMicros(log, remainingMicros, currentMicros)
	log.EstimatedUSDMicros = remainingMicros
	log.EstimatedUSDCents = USDmicrosToCents(remainingMicros)
	if remainingMicros == 0 {
		log.EstimatedUSDCents = 0
	}
	markQuotaCapped(log)
}

func scaleUSDMicros(log *model.APILog, targetMicros int64, currentMicros int64) {
	if currentMicros <= 0 {
		log.InputUSDMicros = 0
		log.CachedInputUSDMicros = 0
		log.OutputUSDMicros = 0
		log.RequestUSDMicros = 0
		return
	}
	if targetMicros <= 0 {
		log.InputUSDMicros = 0
		log.CachedInputUSDMicros = 0
		log.OutputUSDMicros = 0
		log.RequestUSDMicros = 0
		return
	}

	if log.RequestUSDMicros > 0 && log.InputUSDMicros == 0 && log.CachedInputUSDMicros == 0 && log.OutputUSDMicros == 0 {
		log.RequestUSDMicros = targetMicros
		return
	}
	log.InputUSDMicros = scalePart(log.InputUSDMicros, targetMicros, currentMicros)
	log.CachedInputUSDMicros = scalePart(log.CachedInputUSDMicros, targetMicros, currentMicros)
	used := log.InputUSDMicros + log.CachedInputUSDMicros
	if used >= targetMicros {
		log.OutputUSDMicros = 0
		log.RequestUSDMicros = 0
		return
	}
	log.OutputUSDMicros = targetMicros - used
	log.RequestUSDMicros = 0
}

func scalePart(value int64, targetMicros int64, currentMicros int64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(math.Floor(float64(value) * float64(targetMicros) / float64(currentMicros)))
}

func markQuotaCapped(log *model.APILog) {
	if log.BillingSource == "" {
		log.BillingSource = "quota_capped"
		return
	}
	if strings.Contains(log.BillingSource, "quota_capped") {
		return
	}
	log.BillingSource += "+quota_capped"
}
