package service

import (
	"testing"
	"time"

	"ai-gateway/model"
)

func TestHasBalanceAccessAllowsActiveSubscriptionFallback(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	planID := uint(1)
	user := model.User{
		Status:          model.UserStatusApproved,
		PlanID:          &planID,
		Plan:            &model.Plan{PlanType: model.PlanTypeSubscription, SettlementUSDCents: 100},
		ExpiresAt:       &expiresAt,
		BalanceUSDCents: MinQuotaRemainingUSDCents + 1,
	}

	if !HasActiveSubscription(user, time.Now()) {
		t.Fatal("expected active subscription")
	}
	if !HasBalanceAccess(user, time.Now()) {
		t.Fatal("expected balance access to remain available as quota fallback")
	}
}

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

func TestQuotaUsagePercentCapsAtPlanLimit(t *testing.T) {
	if got := capUsedUSDCents(2800, 2000); got != 2000 {
		t.Fatalf("capUsedUSDCents() = %d, want 2000", got)
	}
	if got := capUsedUSDCents(1800, 2000); got != 1800 {
		t.Fatalf("capUsedUSDCents() = %d, want 1800", got)
	}
}

func TestQuotaAllowsRequestRequiresMoreThanMinimumRemaining(t *testing.T) {
	if QuotaAllowsRequest(QuotaUsage{LimitUSDCents: 2000, RemainingCents: MinQuotaRemainingUSDCents}) {
		t.Fatal("QuotaAllowsRequest() = true, want false at minimum remaining threshold")
	}
	if !QuotaAllowsRequest(QuotaUsage{LimitUSDCents: 2000, RemainingCents: MinQuotaRemainingUSDCents + 1}) {
		t.Fatal("QuotaAllowsRequest() = false, want true above minimum remaining threshold")
	}
	if !QuotaAllowsRequest(QuotaUsage{}) {
		t.Fatal("QuotaAllowsRequest() = false, want true for unlimited quota")
	}
}

func TestCapAPILogCostCapsSingleRequestOverflow(t *testing.T) {
	log := model.APILog{
		APIKeyID:           1,
		Method:             "POST",
		Path:               "/v1/chat/completions",
		StatusCode:         200,
		EstimatedUSDCents:  900,
		EstimatedUSDMicros: 9_000_000,
		InputUSDMicros:     3_000_000,
		OutputUSDMicros:    6_000_000,
		BillingSource:      "upstream_cost",
	}

	capAPILogCost(&log, 100)

	if log.EstimatedUSDCents != 100 {
		t.Fatalf("EstimatedUSDCents = %d, want 100", log.EstimatedUSDCents)
	}
	if log.EstimatedUSDMicros != 1_000_000 {
		t.Fatalf("EstimatedUSDMicros = %d, want 1000000", log.EstimatedUSDMicros)
	}
	if log.BillingSource != "upstream_cost+quota_capped" {
		t.Fatalf("BillingSource = %q, want upstream_cost+quota_capped", log.BillingSource)
	}
	if log.InputUSDMicros+log.CachedInputUSDMicros+log.OutputUSDMicros != log.EstimatedUSDMicros {
		t.Fatalf("cost parts = %d, want %d", log.InputUSDMicros+log.CachedInputUSDMicros+log.OutputUSDMicros, log.EstimatedUSDMicros)
	}
}

func TestCapAPILogCostZeroesAfterQuotaExhausted(t *testing.T) {
	log := model.APILog{
		APIKeyID:           1,
		Method:             "POST",
		Path:               "/v1/chat/completions",
		StatusCode:         200,
		EstimatedUSDCents:  900,
		EstimatedUSDMicros: 9_000_000,
		InputUSDMicros:     3_000_000,
		OutputUSDMicros:    6_000_000,
	}

	capAPILogCost(&log, 0)

	if log.EstimatedUSDCents != 0 || log.EstimatedUSDMicros != 0 {
		t.Fatalf("log cost = %d cents/%d micros, want zero", log.EstimatedUSDCents, log.EstimatedUSDMicros)
	}
	if log.InputUSDMicros != 0 || log.CachedInputUSDMicros != 0 || log.OutputUSDMicros != 0 {
		t.Fatalf("cost parts = %d/%d/%d, want zero", log.InputUSDMicros, log.CachedInputUSDMicros, log.OutputUSDMicros)
	}
}

func TestCapAPILogCostScalesRequestCharge(t *testing.T) {
	log := model.APILog{
		APIKeyID:           1,
		Method:             "POST",
		Path:               "/v1/images/generations",
		StatusCode:         200,
		EstimatedUSDCents:  25,
		EstimatedUSDMicros: 250_000,
		RequestUSDMicros:   250_000,
		BillingSource:      "model_management",
	}

	capAPILogCost(&log, 10)

	if log.EstimatedUSDMicros != 100_000 || log.EstimatedUSDCents != 10 {
		t.Fatalf("log cost = %d micros/%d cents, want 100000/10", log.EstimatedUSDMicros, log.EstimatedUSDCents)
	}
	if log.RequestUSDMicros != 100_000 {
		t.Fatalf("RequestUSDMicros = %d, want 100000", log.RequestUSDMicros)
	}
	if log.BillingSource != "model_management+quota_capped" {
		t.Fatalf("BillingSource = %q, want model_management+quota_capped", log.BillingSource)
	}
}

func TestPriceRequestMicrosAppliesMultiplier(t *testing.T) {
	if got := priceRequestMicros(0.025, 6); got != 150_000 {
		t.Fatalf("priceRequestMicros() = %d, want 150000", got)
	}
}
