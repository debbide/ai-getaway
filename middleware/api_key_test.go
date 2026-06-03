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

func TestAllowAPIKeyDisabledLimitAllowsWithoutRedis(t *testing.T) {
	if !allowAPIKey(nil, 1, 0) {
		t.Fatal("allowAPIKey() = false, want true when limit is disabled")
	}
}

func TestAllowAPIKeyNoRedisAllows(t *testing.T) {
	if !allowAPIKey(nil, 1, 120) {
		t.Fatal("allowAPIKey() = false, want true when redis is unavailable")
	}
}

func ptrUint(value uint) *uint {
	return &value
}
