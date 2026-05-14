package middleware

import (
	"testing"
	"time"

	"ai-gateway/model"
)

func TestAllowPlanQuotaRejectsUserWithoutActiveSubscription(t *testing.T) {
	if allowPlanQuota(nil, model.User{}) {
		t.Fatal("allowPlanQuota() = true, want false for user without active subscription")
	}
}

func TestAllowPlanQuotaRejectsUserWithoutPlan(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	user := model.User{
		Status:    model.UserStatusApproved,
		PlanID:    ptrUint(1),
		ExpiresAt: &expiresAt,
	}
	if allowPlanQuota(nil, user) {
		t.Fatal("allowPlanQuota() = true, want false for user without loaded plan")
	}
}

func ptrUint(value uint) *uint {
	return &value
}
