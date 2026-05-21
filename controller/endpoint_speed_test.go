package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsHTTPURL(t *testing.T) {
	valid := []string{
		"https://example.com",
		"http://127.0.0.1:8080/v1",
	}
	for _, value := range valid {
		if !isHTTPURL(value) {
			t.Fatalf("expected %q to be a valid http url", value)
		}
	}

	invalid := []string{
		"",
		"example.com",
		"ftp://example.com",
		"/v1",
	}
	for _, value := range invalid {
		if isHTTPURL(value) {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

func TestEndpointSpeedUsesLocalHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != "ai-getaway-local-speed-test/1.0" {
			t.Fatalf("unexpected user agent %q", r.UserAgent())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	latencyMs, statusCode, err := testEndpointSpeed(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("testEndpointSpeed returned error: %v", err)
	}
	if latencyMs < 0 {
		t.Fatalf("expected non-negative latency, got %d", latencyMs)
	}
	if statusCode != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, statusCode)
	}
}
