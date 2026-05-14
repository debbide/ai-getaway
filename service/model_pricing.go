package service

import (
	"errors"
	"math"
	"strings"
	"time"

	"ai-gateway/model"

	"gorm.io/gorm"
)

const OpenAIPricingSourceURL = "https://openai.com/api/pricing/"

type OfficialModelPrice struct {
	ModelName                string
	DisplayName              string
	InputUSDPerMillion       float64
	CachedInputUSDPerMillion float64
	OutputUSDPerMillion      float64
	Notes                    string
}

type BillingResult struct {
	InputTokens              int64
	CachedInputTokens        int64
	OutputTokens             int64
	InputUSDMicros           int64
	CachedInputUSDMicros     int64
	OutputUSDMicros          int64
	TotalUSDMicros           int64
	TotalUSDCents            int64
	InputUSDPerMillion       float64
	CachedInputUSDPerMillion float64
	OutputUSDPerMillion      float64
	BillingMultiplier        float64
	BillingSource            string
}

func OfficialOpenAIModelPrices() []OfficialModelPrice {
	return []OfficialModelPrice{
		{ModelName: "gpt-5.5", DisplayName: "GPT-5.5", InputUSDPerMillion: 5.00, CachedInputUSDPerMillion: 1.353, OutputUSDPerMillion: 30.00},
		{ModelName: "gpt-5.4", DisplayName: "GPT-5.4", InputUSDPerMillion: 2.50, CachedInputUSDPerMillion: 0.85, OutputUSDPerMillion: 15.00},
		{ModelName: "gpt-5.4-mini", DisplayName: "GPT-5.4 Mini", InputUSDPerMillion: 0.75, CachedInputUSDPerMillion: 0.075, OutputUSDPerMillion: 4.50},
		{ModelName: "gpt-5.4-nano", DisplayName: "GPT-5.4 Nano", InputUSDPerMillion: 0.20, CachedInputUSDPerMillion: 0.02, OutputUSDPerMillion: 1.25},
		{ModelName: "gpt-5.3-codex", DisplayName: "GPT-5.3 Codex", InputUSDPerMillion: 1.75, CachedInputUSDPerMillion: 0.175, OutputUSDPerMillion: 14.00},
		{ModelName: "gpt-5.2", DisplayName: "GPT-5.2", InputUSDPerMillion: 1.75, CachedInputUSDPerMillion: 0.175, OutputUSDPerMillion: 14.00},
		{ModelName: "gpt-5.2-chat-latest", DisplayName: "GPT-5.2 Chat", InputUSDPerMillion: 1.75, CachedInputUSDPerMillion: 0.175, OutputUSDPerMillion: 14.00},
		{ModelName: "gpt-5.2-pro", DisplayName: "GPT-5.2 Pro", InputUSDPerMillion: 21.00, CachedInputUSDPerMillion: 0, OutputUSDPerMillion: 168.00},
		{ModelName: "gpt-5", DisplayName: "GPT-5", InputUSDPerMillion: 1.25, CachedInputUSDPerMillion: 0.125, OutputUSDPerMillion: 10.00},
		{ModelName: "gpt-5-codex", DisplayName: "GPT-5 Codex", InputUSDPerMillion: 1.25, CachedInputUSDPerMillion: 0.125, OutputUSDPerMillion: 10.00},
		{ModelName: "gpt-4.1", DisplayName: "GPT-4.1", InputUSDPerMillion: 2.00, CachedInputUSDPerMillion: 0.50, OutputUSDPerMillion: 8.00},
		{ModelName: "gpt-4.1-mini", DisplayName: "GPT-4.1 Mini", InputUSDPerMillion: 0.40, CachedInputUSDPerMillion: 0.10, OutputUSDPerMillion: 1.60},
		{ModelName: "gpt-4.1-nano", DisplayName: "GPT-4.1 Nano", InputUSDPerMillion: 0.10, CachedInputUSDPerMillion: 0.025, OutputUSDPerMillion: 0.40},
		{ModelName: "gpt-4o", DisplayName: "GPT-4o", InputUSDPerMillion: 2.50, CachedInputUSDPerMillion: 1.25, OutputUSDPerMillion: 10.00},
		{ModelName: "gpt-4o-mini", DisplayName: "GPT-4o Mini", InputUSDPerMillion: 0.15, CachedInputUSDPerMillion: 0.075, OutputUSDPerMillion: 0.60},
		{ModelName: "o3", DisplayName: "o3", InputUSDPerMillion: 2.00, CachedInputUSDPerMillion: 0.50, OutputUSDPerMillion: 8.00},
		{ModelName: "o4-mini", DisplayName: "o4-mini", InputUSDPerMillion: 1.10, CachedInputUSDPerMillion: 0.275, OutputUSDPerMillion: 4.40},
	}
}

func SyncOfficialOpenAIModelPrices(db *gorm.DB) (int, error) {
	now := time.Now()
	synced := 0
	for _, item := range OfficialOpenAIModelPrices() {
		var existing model.ModelPricing
		err := db.Where("model = ?", item.ModelName).First(&existing).Error
		pricing := model.ModelPricing{
			ModelName:                item.ModelName,
			DisplayName:              item.DisplayName,
			Provider:                 "openai",
			InputUSDPerMillion:       item.InputUSDPerMillion,
			CachedInputUSDPerMillion: item.CachedInputUSDPerMillion,
			OutputUSDPerMillion:      item.OutputUSDPerMillion,
			BillingMultiplier:        1,
			Status:                   model.ModelPricingStatusActive,
			Official:                 true,
			OfficialSource:           OpenAIPricingSourceURL,
			OfficialSyncedAt:         &now,
			Notes:                    item.Notes,
		}
		if err == nil {
			multiplier := existing.BillingMultiplier
			if multiplier <= 0 {
				multiplier = 1
			}
			updates := map[string]interface{}{
				"display_name":                 pricing.DisplayName,
				"provider":                     pricing.Provider,
				"input_usd_per_million":        pricing.InputUSDPerMillion,
				"cached_input_usd_per_million": pricing.CachedInputUSDPerMillion,
				"output_usd_per_million":       pricing.OutputUSDPerMillion,
				"billing_multiplier":           multiplier,
				"official":                     true,
				"official_source":              pricing.OfficialSource,
				"official_synced_at":           pricing.OfficialSyncedAt,
				"notes":                        pricing.Notes,
			}
			if existing.Status == "" {
				updates["status"] = model.ModelPricingStatusActive
			}
			if err := db.Model(&existing).Updates(updates).Error; err != nil {
				return synced, err
			}
			synced++
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return synced, err
		}
		if err := db.Create(&pricing).Error; err != nil {
			return synced, err
		}
		synced++
	}
	return synced, nil
}

func BillUsage(db *gorm.DB, modelName string, inputTokens, cachedInputTokens, outputTokens, totalTokens int64) BillingResult {
	if outputTokens <= 0 && totalTokens > inputTokens {
		outputTokens = totalTokens - inputTokens
	}
	if inputTokens <= 0 {
		inputTokens = totalTokens - outputTokens
	}
	if cachedInputTokens < 0 {
		cachedInputTokens = 0
	}

	pricing, source := FindModelPricing(db, modelName)
	multiplier := pricing.BillingMultiplier
	if multiplier <= 0 {
		multiplier = 1
	}

	uncachedInputTokens := inputTokens
	if cachedInputTokens > 0 && totalTokens < inputTokens+cachedInputTokens+outputTokens {
		uncachedInputTokens = inputTokens - cachedInputTokens
		if uncachedInputTokens < 0 {
			uncachedInputTokens = 0
		}
	}
	if cachedInputTokens > 0 && pricing.CachedInputUSDPerMillion <= 0 {
		uncachedInputTokens = inputTokens
		cachedInputTokens = 0
	}
	inputMicros := priceMicros(uncachedInputTokens, pricing.InputUSDPerMillion, multiplier)
	cachedMicros := priceMicros(cachedInputTokens, pricing.CachedInputUSDPerMillion, multiplier)
	outputMicros := priceMicros(outputTokens, pricing.OutputUSDPerMillion, multiplier)
	totalMicros := inputMicros + cachedMicros + outputMicros

	return BillingResult{
		InputTokens:              inputTokens,
		CachedInputTokens:        cachedInputTokens,
		OutputTokens:             outputTokens,
		InputUSDMicros:           inputMicros,
		CachedInputUSDMicros:     cachedMicros,
		OutputUSDMicros:          outputMicros,
		TotalUSDMicros:           totalMicros,
		TotalUSDCents:            USDmicrosToCents(totalMicros),
		InputUSDPerMillion:       pricing.InputUSDPerMillion,
		CachedInputUSDPerMillion: pricing.CachedInputUSDPerMillion,
		OutputUSDPerMillion:      pricing.OutputUSDPerMillion,
		BillingMultiplier:        multiplier,
		BillingSource:            source,
	}
}

func FindModelPricing(db *gorm.DB, modelName string) (model.ModelPricing, string) {
	if db != nil && modelName != "" {
		var pricing model.ModelPricing
		if err := db.Where("model = ? AND status = ?", modelName, model.ModelPricingStatusActive).First(&pricing).Error; err == nil {
			return pricing, "model_management"
		}
	}

	if item, ok := findOfficialModelPrice(modelName); ok {
		return model.ModelPricing{
			ModelName:                item.ModelName,
			DisplayName:              item.DisplayName,
			Provider:                 "openai",
			InputUSDPerMillion:       item.InputUSDPerMillion,
			CachedInputUSDPerMillion: item.CachedInputUSDPerMillion,
			OutputUSDPerMillion:      item.OutputUSDPerMillion,
			BillingMultiplier:        1,
			Status:                   model.ModelPricingStatusActive,
			Official:                 true,
			OfficialSource:           OpenAIPricingSourceURL,
		}, "official_fallback"
	}

	input, cached, output := fallbackRatesUSD(modelName)
	return model.ModelPricing{
		ModelName:                modelName,
		Provider:                 "fallback",
		InputUSDPerMillion:       input,
		CachedInputUSDPerMillion: cached,
		OutputUSDPerMillion:      output,
		BillingMultiplier:        1,
		Status:                   model.ModelPricingStatusActive,
	}, "fallback"
}

func findOfficialModelPrice(modelName string) (OfficialModelPrice, bool) {
	name := strings.ToLower(strings.TrimSpace(modelName))
	if name == "" {
		return OfficialModelPrice{}, false
	}

	var matched OfficialModelPrice
	matchedLen := 0
	for _, item := range OfficialOpenAIModelPrices() {
		itemName := strings.ToLower(item.ModelName)
		if name != itemName && !strings.HasPrefix(name, itemName+"-") {
			continue
		}
		if len(itemName) > matchedLen {
			matched = item
			matchedLen = len(itemName)
		}
	}
	return matched, matchedLen > 0
}

func USDmicrosToCents(micros int64) int64 {
	if micros <= 0 {
		return 0
	}
	return int64(math.Ceil(float64(micros) / 10_000))
}

func priceMicros(tokens int64, usdPerMillion float64, multiplier float64) int64 {
	if tokens <= 0 || usdPerMillion <= 0 {
		return 0
	}
	return int64(math.Round(float64(tokens) * usdPerMillion * multiplier))
}

func fallbackRatesUSD(modelName string) (float64, float64, float64) {
	name := strings.ToLower(modelName)
	switch {
	case strings.Contains(name, "gpt-4o-mini"), strings.Contains(name, "gpt-4.1-mini"), strings.Contains(name, "mini"):
		return 0.15, 0.075, 0.60
	case strings.Contains(name, "gpt-4o"), strings.Contains(name, "gpt-4.1"):
		return 2.50, 1.25, 10.00
	case strings.Contains(name, "o1"), strings.Contains(name, "o3"):
		return 15.00, 7.50, 60.00
	default:
		return 1.00, 0.50, 4.00
	}
}
