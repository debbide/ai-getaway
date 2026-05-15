package service

import (
	"testing"
	"time"

	"ai-gateway/model"
)

func TestQuotaUsageWindowStartsAtSubscriptionStartInsideNaturalWindow(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-1 * time.Hour)

	start, _ := QuotaUsageWindow(model.QuotaPeriodWeekly, &startedAt, now)

	if !start.Equal(startedAt) {
		t.Fatalf("start = %s, want %s", start, startedAt)
	}
}

func TestQuotaUsageWindowKeepsNaturalWindowAfterSubscriptionStart(t *testing.T) {
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	startedAt := now.AddDate(0, 0, -30)
	weekStart, _ := QuotaWindow(model.QuotaPeriodWeekly, now)

	start, _ := QuotaUsageWindow(model.QuotaPeriodWeekly, &startedAt, now)

	if !start.Equal(weekStart) {
		t.Fatalf("start = %s, want %s", start, weekStart)
	}
}
