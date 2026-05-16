package controller

import (
	"testing"
	"time"

	"ai-gateway/model"
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
