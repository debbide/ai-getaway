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

func TestOrderPaymentAdminRecipientsIncludesApprovedAndLegacyAdmins(t *testing.T) {
	admins := []model.User{
		{Username: "approved", Email: " Admin@Example.com ", Role: model.RoleAdmin, Status: model.UserStatusApproved},
		{Username: "legacy", Email: "legacy@example.com", Role: model.RoleAdmin},
		{Username: "pending", Email: "pending@example.com", Role: model.RoleAdmin, Status: model.UserStatusPending},
		{Username: "disabled", Email: "disabled@example.com", Role: model.RoleAdmin, Status: model.UserStatusDisabled},
		{Username: "dupe", Email: "admin@example.com", Role: model.RoleAdmin, Status: model.UserStatusApproved},
	}

	got := orderPaymentAdminRecipients(model.SystemSetting{ContactEmail: "ops@example.com"}, admins)

	if len(got) != 2 {
		t.Fatalf("recipient count = %d, want 2: %#v", len(got), got)
	}
	if got[0].Email != "admin@example.com" {
		t.Fatalf("first recipient = %q, want normalized admin@example.com", got[0].Email)
	}
	if got[1].Email != "legacy@example.com" {
		t.Fatalf("second recipient = %q, want legacy@example.com", got[1].Email)
	}
}

func TestOrderPaymentAdminRecipientsFallsBackToContactEmail(t *testing.T) {
	got := orderPaymentAdminRecipients(model.SystemSetting{ContactEmail: " Ops@Example.com "}, nil)

	if len(got) != 1 {
		t.Fatalf("recipient count = %d, want 1", len(got))
	}
	if got[0].Email != "ops@example.com" || got[0].Username != "admin" {
		t.Fatalf("fallback recipient = %#v, want normalized contact admin", got[0])
	}
}

func TestOrderPaymentAdminRecipientsSkipsDefaultContactEmail(t *testing.T) {
	got := orderPaymentAdminRecipients(model.SystemSetting{ContactEmail: "support@example.com"}, nil)

	if len(got) != 0 {
		t.Fatalf("recipient count = %d, want 0", len(got))
	}
}
