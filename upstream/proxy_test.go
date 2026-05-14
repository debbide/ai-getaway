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
