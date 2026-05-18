package service

import (
	"strings"
	"testing"

	"ai-gateway/model"
)

func TestNotificationFingerprintKeepsShortLegacyFormat(t *testing.T) {
	userID := uint(7)
	orderID := uint(9)

	got := notificationFingerprint(model.EmailTemplateOrderApprovedUser, &userID, &orderID, "USER@example.com ", "")
	want := model.EmailTemplateOrderApprovedUser + ":user@example.com:u7:o9"

	if got != want {
		t.Fatalf("fingerprint = %q, want %q", got, want)
	}
}

func TestNotificationFingerprintHashesLongValues(t *testing.T) {
	userID := uint(7)
	got := notificationFingerprint(model.EmailTemplateSubscriptionExpiring, &userID, nil, strings.Repeat("a", 180)+"@example.com", "20260518145413")

	if len(got) > 128 {
		t.Fatalf("fingerprint length = %d, want <= 128", len(got))
	}
	if !strings.HasPrefix(got, model.EmailTemplateSubscriptionExpiring+":") {
		t.Fatalf("fingerprint = %q, want event type prefix", got)
	}
}
