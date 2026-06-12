package service

import (
	"time"

	"ai-gateway/model"

	"gorm.io/gorm"
)

func HasActiveSubscription(user model.User, now time.Time) bool {
	if user.Status != model.UserStatusApproved {
		return false
	}
	if user.PlanID == nil {
		return false
	}
	if user.Plan != nil && user.Plan.PlanType == model.PlanTypePublic && user.Plan.DurationDays <= 0 {
		return true
	}
	if user.ExpiresAt == nil {
		return false
	}
	return now.Before(*user.ExpiresAt)
}

func HasDirectPublicChannelAccess(user model.User, now time.Time) bool {
	if user.Status != model.UserStatusApproved || user.PublicChannelID == nil {
		return false
	}
	if normalizedDirectPublicPeriod(user.PublicChannelPeriod) == model.QuotaPeriodPublic {
		return true
	}
	if user.ExpiresAt == nil {
		return false
	}
	return now.Before(*user.ExpiresAt)
}

func HasCallableAccess(user model.User, now time.Time) bool {
	return HasActiveSubscription(user, now) || HasDirectPublicChannelAccess(user, now) || HasBalanceAccess(user, now)
}

func HasBalanceAccess(user model.User, now time.Time) bool {
	return user.Status == model.UserStatusApproved &&
		user.BalanceUSDCents > MinQuotaRemainingUSDCents
}

func SubscriptionStartAt(db *gorm.DB, user model.User, now time.Time) *time.Time {
	if user.SubscriptionStartedAt != nil {
		return user.SubscriptionStartedAt
	}
	if db != nil && user.PlanID != nil {
		var lastOrder model.Order
		result := db.Where("user_id = ? AND plan_id = ? AND status = ?", user.ID, *user.PlanID, model.OrderStatusApproved).
			Order("approved_at DESC, id DESC").
			Limit(1).
			Find(&lastOrder)
		if result.Error == nil && result.RowsAffected > 0 && lastOrder.ApprovedAt != nil {
			return lastOrder.ApprovedAt
		}
	}
	if HasActiveSubscription(user, now) && user.Plan != nil && user.ExpiresAt != nil && user.Plan.DurationDays > 0 {
		fallbackStartedAt := user.ExpiresAt.AddDate(0, 0, -user.Plan.DurationDays)
		return &fallbackStartedAt
	}
	return nil
}

func PlanQuotaStartAt(db *gorm.DB, user model.User, now time.Time) *time.Time {
	startedAt := SubscriptionStartAt(db, user, now)
	if user.QuotaResetAt != nil && (startedAt == nil || user.QuotaResetAt.After(*startedAt)) {
		return user.QuotaResetAt
	}
	return startedAt
}

func DirectPublicChannelPeriod(value string) string {
	return normalizedDirectPublicPeriod(value)
}

func normalizedDirectPublicPeriod(value string) string {
	switch value {
	case model.QuotaPeriodDaily, model.QuotaPeriodWeekly:
		return value
	default:
		return model.QuotaPeriodPublic
	}
}
