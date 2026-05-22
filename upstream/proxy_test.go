package upstream

import (
	"testing"

	"ai-gateway/model"
)

func TestFillUsageResponsesShape(t *testing.T) {
	log := model.APILog{ModelName: "gpt-5.3-codex"}

	fillUsage(nil, &log, []byte(`{
		"model": "gpt-5.3-codex",
		"usage": {
			"input_tokens": 1200,
			"output_tokens": 345,
			"total_tokens": 1545
		}
	}`))

	if log.PromptTokens != 1200 {
		t.Fatalf("PromptTokens = %d, want 1200", log.PromptTokens)
	}
	if log.CompletionTokens != 345 {
		t.Fatalf("CompletionTokens = %d, want 345", log.CompletionTokens)
	}
	if log.TotalTokens != 1545 {
		t.Fatalf("TotalTokens = %d, want 1545", log.TotalTokens)
	}
	if log.EstimatedUSDCents <= 0 {
		t.Fatalf("EstimatedUSDCents = %d, want positive", log.EstimatedUSDCents)
	}
}

func TestFillStreamUsageResponseCompletedEvent(t *testing.T) {
	log := model.APILog{ModelName: "gpt-5.3-codex"}

	fillStreamUsage(nil, &log, []byte("event: response.completed\n"+
		"data: {\"response\":{\"model\":\"gpt-5.3-codex-spark\",\"usage\":{\"input_tokens\":44,\"output_tokens\":11,\"total_tokens\":55}}}\n\n"+
		"data: [DONE]\n\n"))

	if log.ModelName != "gpt-5.3-codex-spark" {
		t.Fatalf("ModelName = %q, want gpt-5.3-codex-spark", log.ModelName)
	}
	if log.PromptTokens != 44 || log.CompletionTokens != 11 || log.TotalTokens != 55 {
		t.Fatalf("usage = %d/%d/%d, want 44/11/55", log.PromptTokens, log.CompletionTokens, log.TotalTokens)
	}
}

func TestFillUsageChatCompletionsShape(t *testing.T) {
	log := model.APILog{}

	fillUsage(nil, &log, []byte(`{
		"model": "gpt-4o-mini",
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 7,
			"total_tokens": 17
		}
	}`))

	if log.ModelName != "gpt-4o-mini" {
		t.Fatalf("ModelName = %q, want gpt-4o-mini", log.ModelName)
	}
	if log.PromptTokens != 10 || log.CompletionTokens != 7 || log.TotalTokens != 17 {
		t.Fatalf("usage = %d/%d/%d, want 10/7/17", log.PromptTokens, log.CompletionTokens, log.TotalTokens)
	}
}

func TestFillUsageCachedInputTokens(t *testing.T) {
	log := model.APILog{}

	fillUsage(nil, &log, []byte(`{
		"model": "gpt-5.5",
		"usage": {
			"input_tokens": 5269,
			"output_tokens": 561,
			"total_tokens": 63046,
			"input_tokens_details": {
				"cached_tokens": 57216
			}
		}
	}`))

	if log.CachedInputTokens != 57216 {
		t.Fatalf("CachedInputTokens = %d, want 57216", log.CachedInputTokens)
	}
	if log.EstimatedUSDMicros <= 0 {
		t.Fatalf("EstimatedUSDMicros = %d, want positive", log.EstimatedUSDMicros)
	}
}

func TestFillUsageGPT54MatchesObservedUpstreamCost(t *testing.T) {
	log := model.APILog{}

	fillUsage(nil, &log, []byte(`{
		"model": "gpt-5.4",
		"usage": {
			"input_tokens": 13294,
			"output_tokens": 6,
			"total_tokens": 13300,
			"input_tokens_details": {
				"cached_tokens": 9600
			}
		}
	}`))

	if log.InputUSDMicros != 9235 {
		t.Fatalf("InputUSDMicros = %d, want 9235", log.InputUSDMicros)
	}
	if log.CachedInputUSDMicros != 8160 {
		t.Fatalf("CachedInputUSDMicros = %d, want 8160", log.CachedInputUSDMicros)
	}
	if log.OutputUSDMicros != 90 {
		t.Fatalf("OutputUSDMicros = %d, want 90", log.OutputUSDMicros)
	}
	if log.EstimatedUSDMicros != 17485 {
		t.Fatalf("EstimatedUSDMicros = %d, want 17485", log.EstimatedUSDMicros)
	}
}

func TestFillUsageGPT55MatchesObservedUpstreamCost(t *testing.T) {
	log := model.APILog{}

	fillUsage(nil, &log, []byte(`{
		"model": "gpt-5.5",
		"usage": {
			"input_tokens": 4055,
			"output_tokens": 22,
			"total_tokens": 14677,
			"input_tokens_details": {
				"cached_tokens": 10600
			}
		}
	}`))

	if log.InputUSDMicros != 20275 {
		t.Fatalf("InputUSDMicros = %d, want 20275", log.InputUSDMicros)
	}
	if log.CachedInputUSDMicros != 14342 {
		t.Fatalf("CachedInputUSDMicros = %d, want 14342", log.CachedInputUSDMicros)
	}
	if log.OutputUSDMicros != 660 {
		t.Fatalf("OutputUSDMicros = %d, want 660", log.OutputUSDMicros)
	}
	if log.EstimatedUSDMicros != 35277 {
		t.Fatalf("EstimatedUSDMicros = %d, want 35277", log.EstimatedUSDMicros)
	}
}

func TestFillUsageGPT55VariantUsesGPT55Pricing(t *testing.T) {
	log := model.APILog{}

	fillUsage(nil, &log, []byte(`{
		"model": "gpt-5.5-2026-05-01",
		"usage": {
			"input_tokens": 1000,
			"output_tokens": 100,
			"total_tokens": 2100,
			"input_tokens_details": {
				"cached_tokens": 1000
			}
		}
	}`))

	if log.InputUSDPerMillion != 5.00 || log.CachedInputUSDPerMillion != 1.353 || log.OutputUSDPerMillion != 30.00 {
		t.Fatalf("pricing = %.3f/%.3f/%.3f, want 5.000/1.353/30.000", log.InputUSDPerMillion, log.CachedInputUSDPerMillion, log.OutputUSDPerMillion)
	}
	if log.EstimatedUSDMicros != 9353 {
		t.Fatalf("EstimatedUSDMicros = %d, want 9353", log.EstimatedUSDMicros)
	}
}

func TestFillUsagePrefersUpstreamCost(t *testing.T) {
	log := model.APILog{}

	fillUsage(nil, &log, []byte(`{
		"model": "gpt-4o-mini",
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 7,
			"total_tokens": 17,
			"cost_usd": 0.017485
		}
	}`))

	if log.EstimatedUSDMicros != 17485 {
		t.Fatalf("EstimatedUSDMicros = %d, want 17485", log.EstimatedUSDMicros)
	}
	if log.BillingSource != "upstream_cost" {
		t.Fatalf("BillingSource = %q, want upstream_cost", log.BillingSource)
	}
}

func TestFillUsageAppliesGroupMultiplierToUpstreamCost(t *testing.T) {
	log := model.APILog{}

	fillUsage(nil, &log, []byte(`{
		"model": "gpt-4o-mini",
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 7,
			"total_tokens": 17,
			"cost_usd": 0.017485
		}
	}`), map[string]float64{"gpt-4o-mini": 2})

	if log.EstimatedUSDMicros != 34970 {
		t.Fatalf("EstimatedUSDMicros = %d, want 34970", log.EstimatedUSDMicros)
	}
	if log.GroupMultiplier != 2 {
		t.Fatalf("GroupMultiplier = %.2f, want 2", log.GroupMultiplier)
	}
}

func TestFillUsageAppliesChannelGroupMultiplier(t *testing.T) {
	log := model.APILog{}

	fillUsage(nil, &log, []byte(`{
		"model": "gpt-4o-mini",
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 7,
			"total_tokens": 17
		}
	}`), `{"gpt-4o-mini":2}`)

	if log.GroupMultiplier != 2 {
		t.Fatalf("GroupMultiplier = %.2f, want 2", log.GroupMultiplier)
	}
	if log.BillingMultiplier != 2 {
		t.Fatalf("BillingMultiplier = %.2f, want 2", log.BillingMultiplier)
	}
	if log.EstimatedUSDMicros != 11 {
		t.Fatalf("EstimatedUSDMicros = %d, want 11", log.EstimatedUSDMicros)
	}
}

func TestCapResponseToQuotaMarksExceeded(t *testing.T) {
	log := model.APILog{EstimatedUSDMicros: 9_000_000}

	if !capResponseToQuota(&log, 1_000_000) {
		t.Fatal("capResponseToQuota() = false, want true")
	}
	if log.ErrorMessage != "令牌额度耗尽" {
		t.Fatalf("ErrorMessage = %q", log.ErrorMessage)
	}
}

func TestStreamExceedsQuotaAtBudgetBoundary(t *testing.T) {
	log := model.APILog{}
	body := []byte("data: {\"model\":\"gpt-4o-mini\",\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":7,\"total_tokens\":17,\"cost_usd\":0.017485}}\n\n")

	if !streamExceedsQuota(nil, &log, body, nil, 17485) {
		t.Fatal("streamExceedsQuota() = false, want true at quota boundary")
	}
}

func TestEstimatedTokensFromBytesRoundsConservatively(t *testing.T) {
	if got := estimatedTokensFromBytes(1); got != 1 {
		t.Fatalf("estimatedTokensFromBytes(1) = %d, want 1", got)
	}
	if got := estimatedTokensFromBytes(3); got != 2 {
		t.Fatalf("estimatedTokensFromBytes(3) = %d, want 2", got)
	}
}
