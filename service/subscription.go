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
