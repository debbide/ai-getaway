package service

import (
	"time"

	"ai-gateway/model"
)

func HasActiveSubscription(user model.User, now time.Time) bool {
	if user.Status != model.UserStatusApproved {
		return false
	}
	if user.PlanID == nil {
		return false
	}
	if user.ExpiresAt == nil {
		return false
	}
	return now.Before(*user.ExpiresAt)
}
