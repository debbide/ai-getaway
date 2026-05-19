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

func TestActiveSubscriptionAllowsHigherPricedUpgrade(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	currentPlanID := uint(1)
	expiresAt := now.Add(24 * time.Hour)
	user := model.User{
		Status:    model.UserStatusApproved,
		PlanID:    &currentPlanID,
		ExpiresAt: &expiresAt,
		Plan: &model.Plan{
			PriceCents: 1000,
			PlanType:   model.PlanTypeSubscription,
		},
	}
	targetPlan := model.Plan{
		PriceCents: 1500,
		PlanType:   model.PlanTypeSubscription,
	}

	blocked := activeSubscriptionBlocksPlanOrderAt(nil, user, targetPlan, now, func(*gorm.DB, uint, time.Time) int64 {
		return 0
	})

	if blocked {
		t.Fatal("expected higher priced plan to be available as upgrade")
	}
	if got := orderTypeForPlan(nil, user, targetPlan, now, func(*gorm.DB, uint, time.Time) int64 { return 0 }); got != model.OrderTypeUpgrade {
		t.Fatalf("order type = %q, want %q", got, model.OrderTypeUpgrade)
	}
	if got := orderAmountCentsForPlan(user, targetPlan, model.OrderTypeUpgrade); got != 500 {
		t.Fatalf("upgrade amount = %d, want 500", got)
	}
}

func TestActiveSubscriptionBlocksLowerPricedPlan(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	currentPlanID := uint(1)
	expiresAt := now.Add(24 * time.Hour)
	user := model.User{
		Status:    model.UserStatusApproved,
		PlanID:    &currentPlanID,
		ExpiresAt: &expiresAt,
		Plan: &model.Plan{
			PriceCents: 1500,
			PlanType:   model.PlanTypeSubscription,
		},
	}
	targetPlan := model.Plan{
		PriceCents: 1000,
		PlanType:   model.PlanTypeSubscription,
	}

	blocked := activeSubscriptionBlocksPlanOrderAt(nil, user, targetPlan, now, func(*gorm.DB, uint, time.Time) int64 {
		return 0
	})

	if !blocked {
		t.Fatal("expected lower priced plan to be blocked while subscription is active")
	}
}

func TestActiveSubscriptionAllowsSamePlanRenewal(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	currentPlanID := uint(1)
	expiresAt := now.Add(24 * time.Hour)
	user := model.User{
		Status:    model.UserStatusApproved,
		PlanID:    &currentPlanID,
		ExpiresAt: &expiresAt,
		Plan: &model.Plan{
			PriceCents: 1000,
			PlanType:   model.PlanTypeSubscription,
		},
	}
	targetPlan := model.Plan{
		PlanType:   model.PlanTypeSubscription,
		PriceCents: 1000,
	}
	targetPlan.ID = currentPlanID

	blocked := activeSubscriptionBlocksPlanOrderAt(nil, user, targetPlan, now, func(*gorm.DB, uint, time.Time) int64 {
		return 0
	})

	if blocked {
		t.Fatal("expected same active plan to be available as renewal")
	}
	if got := orderTypeForPlan(nil, user, targetPlan, now, func(*gorm.DB, uint, time.Time) int64 { return 0 }); got != model.OrderTypeRenewal {
		t.Fatalf("order type = %q, want %q", got, model.OrderTypeRenewal)
	}
}

func TestUsedUpPublicSubscriptionCreatesPurchaseNotUpgrade(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	publicPlanID := uint(1)
	expiresAt := now.Add(24 * time.Hour)
	user := model.User{
		Status:    model.UserStatusApproved,
		PlanID:    &publicPlanID,
		ExpiresAt: &expiresAt,
		Plan: &model.Plan{
			PriceCents:         1000,
			PlanType:           model.PlanTypePublic,
			QuotaPeriod:        model.QuotaPeriodPublic,
			SettlementUSDCents: 1000,
		},
	}
	targetPlan := model.Plan{
		PlanType:   model.PlanTypeSubscription,
		PriceCents: 1500,
	}

	got := orderTypeForPlan(nil, user, targetPlan, now, func(*gorm.DB, uint, time.Time) int64 { return 1000 })

	if got != model.OrderTypePurchase {
		t.Fatalf("order type = %q, want %q", got, model.OrderTypePurchase)
	}
}

func TestFreeSubscriptionCreatesPurchaseForPaidPlan(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	freePlanID := uint(1)
	expiresAt := now.Add(24 * time.Hour)
	user := model.User{
		Status:    model.UserStatusApproved,
		PlanID:    &freePlanID,
		ExpiresAt: &expiresAt,
		Plan: &model.Plan{
			PriceCents: 0,
			PlanType:   model.PlanTypeSubscription,
		},
	}
	targetPlan := model.Plan{
		PlanType:   model.PlanTypeSubscription,
		PriceCents: 1500,
	}

	blocked := activeSubscriptionBlocksPlanOrderAt(nil, user, targetPlan, now, func(*gorm.DB, uint, time.Time) int64 {
		return 0
	})
	if blocked {
		t.Fatal("expected paid plan to be available when current plan is free")
	}

	got := orderTypeForPlan(nil, user, targetPlan, now, func(*gorm.DB, uint, time.Time) int64 { return 0 })
	if got != model.OrderTypePurchase {
		t.Fatalf("order type = %q, want %q", got, model.OrderTypePurchase)
	}
	if amount := orderAmountCentsForPlan(user, targetPlan, got); amount != targetPlan.PriceCents {
		t.Fatalf("amount = %d, want %d", amount, targetPlan.PriceCents)
	}
}

func TestRenewalExtendsFromExistingExpiry(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	expiresAt := now.AddDate(0, 0, 5)
	user := model.User{ExpiresAt: &expiresAt}
	plan := model.Plan{DurationDays: 30, PlanType: model.PlanTypeSubscription}

	got := subscriptionExpiresAtForOrder(user, plan, model.OrderTypeRenewal, now)
	want := expiresAt.AddDate(0, 0, 30)

	if got == nil || !got.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", got, want)
	}
}

func TestUpgradeKeepsExistingExpiry(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	expiresAt := now.AddDate(0, 0, 5)
	user := model.User{ExpiresAt: &expiresAt}
	plan := model.Plan{DurationDays: 30, PlanType: model.PlanTypeSubscription}

	got := subscriptionExpiresAtForOrder(user, plan, model.OrderTypeUpgrade, now)

	if got == nil || !got.Equal(expiresAt) {
		t.Fatalf("expiresAt = %v, want %v", got, expiresAt)
	}
}

func TestPublicPlanWithoutDurationHasNoExpiry(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	plan := model.Plan{DurationDays: 0, PlanType: model.PlanTypePublic}

	got := subscriptionExpiresAtForOrder(model.User{}, plan, model.OrderTypePurchase, now)

	if got != nil {
		t.Fatalf("expiresAt = %v, want nil", got)
	}
}

func TestPublicPlanWithDurationExpiresByDuration(t *testing.T) {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	plan := model.Plan{DurationDays: 7, PlanType: model.PlanTypePublic}

	got := subscriptionExpiresAtForOrder(model.User{}, plan, model.OrderTypePurchase, now)
	want := now.AddDate(0, 0, 7)

	if got == nil || !got.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", got, want)
	}
}
