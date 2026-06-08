package middleware

import (
	"testing"
)

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
