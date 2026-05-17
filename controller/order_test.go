package controller

import (
	"testing"
	"time"

	"ai-gateway/model"

	"gorm.io/gorm"
)

func TestPendingOrderPaymentMethodUpdatesSwitchesStartedOnlineOrderToManual(t *testing.T) {
	generatedAt := time.Now()
	order := model.Order{
		PaymentMethod:         model.PaymentMethodOnline,
		PaymentURLGeneratedAt: &generatedAt,
		PaymentRef:            "ORDER123",
		UserPaymentNote:       "old note",
		PaymentChannel:        "alipay",
		PaidAmountCents:       100,
		PaidAt:                &generatedAt,
		PaymentRaw:            `{"old":true}`,
	}

	updates, changed := pendingOrderPaymentMethodUpdates(order, model.PaymentMethodManual, 7)
	if !changed {
		t.Fatal("expected payment method change")
	}
	if got := updates["payment_method"]; got != model.PaymentMethodManual {
		t.Fatalf("payment_method = %v, want %q", got, model.PaymentMethodManual)
	}
	if got, ok := updates["payment_ref"].(string); !ok || got == "" || got == order.PaymentRef {
		t.Fatalf("payment_ref = %v, want a regenerated non-empty ref", updates["payment_ref"])
	}
	if got := updates["payment_url_generated_at"]; got != nil {
		t.Fatalf("payment_url_generated_at = %v, want nil", got)
	}
	if got := updates["user_payment_note"]; got != "" {
		t.Fatalf("user_payment_note = %v, want empty", got)
	}
	if got := updates["payment_channel"]; got != "" {
		t.Fatalf("payment_channel = %v, want empty", got)
	}
	if got := updates["paid_amount_cents"]; got != 0 {
		t.Fatalf("paid_amount_cents = %v, want 0", got)
	}
	if got := updates["paid_at"]; got != nil {
		t.Fatalf("paid_at = %v, want nil", got)
	}
	if got := updates["provider_trade_no"]; got != nil {
		t.Fatalf("provider_trade_no = %v, want nil", got)
	}
	if got := updates["payment_raw"]; got != "" {
		t.Fatalf("payment_raw = %v, want empty", got)
	}
}

func TestPendingOrderPaymentMethodUpdatesDoesNotRestartOnlinePayment(t *testing.T) {
	generatedAt := time.Now()
	order := model.Order{
		PaymentMethod:         model.PaymentMethodManual,
		PaymentURLGeneratedAt: &generatedAt,
	}

	updates, changed := pendingOrderPaymentMethodUpdates(order, model.PaymentMethodOnline, 7)
	if changed {
		t.Fatalf("changed = true, want false with updates %v", updates)
	}
}

func TestActivePublicSubscriptionWithUsedQuotaCanBuyPaidPlan(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	publicPlanID := uint(1)
	expiresAt := now.Add(24 * time.Hour)
	user := model.User{
		Status:    model.UserStatusApproved,
		PlanID:    &publicPlanID,
		ExpiresAt: &expiresAt,
		Plan: &model.Plan{
			PlanType:           model.PlanTypePublic,
			QuotaPeriod:        model.QuotaPeriodPublic,
			SettlementUSDCents: 1000,
		},
	}
	user.ID = 7
	targetPlan := model.Plan{
		PlanType:    model.PlanTypeSubscription,
		QuotaPeriod: model.QuotaPeriodWeekly,
		PriceCents:  990,
	}

	blocked := activeSubscriptionBlocksPlanOrderAt(nil, user, targetPlan, now, func(*gorm.DB, uint, time.Time) int64 {
		return 1000
	})

	if blocked {
		t.Fatal("expected used-up public subscription to allow paid plan purchase")
	}
}

func TestActivePublicSubscriptionWithUsedQuotaStillBlocksFreePlan(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	publicPlanID := uint(1)
	expiresAt := now.Add(24 * time.Hour)
	user := model.User{
		Status:    model.UserStatusApproved,
		PlanID:    &publicPlanID,
		ExpiresAt: &expiresAt,
		Plan: &model.Plan{
			PlanType:           model.PlanTypePublic,
			QuotaPeriod:        model.QuotaPeriodPublic,
			SettlementUSDCents: 1000,
		},
	}
	user.ID = 7
	targetPlan := model.Plan{
		PlanType:    model.PlanTypeSubscription,
		QuotaPeriod: model.QuotaPeriodWeekly,
		PriceCents:  0,
	}

	blocked := activeSubscriptionBlocksPlanOrderAt(nil, user, targetPlan, now, func(*gorm.DB, uint, time.Time) int64 {
		return 1000
	})

	if !blocked {
		t.Fatal("expected active subscription to block free plan claim")
	}
}
