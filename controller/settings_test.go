package controller

import "testing"

func TestNormalizeEmailWhitelistJSON(t *testing.T) {
	got := normalizeEmailWhitelistJSON(`["QQ.com","@gmail.com","invalid","gmail.com","outlook.com"]`)
	want := `["qq.com","gmail.com","outlook.com"]`
	if got != want {
		t.Fatalf("normalizeEmailWhitelistJSON() = %s, want %s", got, want)
	}
}

func TestEmailAllowedByWhitelist(t *testing.T) {
	whitelist := `["qq.com","gmail.com"]`
	if !emailAllowedByWhitelist("User@GMAIL.com", whitelist) {
		t.Fatal("expected gmail.com email to be allowed")
	}
	if emailAllowedByWhitelist("user@example.com", whitelist) {
		t.Fatal("expected example.com email to be blocked")
	}
	if !emailAllowedByWhitelist("user@example.com", "") {
		t.Fatal("expected empty whitelist to allow registration")
	}
}
