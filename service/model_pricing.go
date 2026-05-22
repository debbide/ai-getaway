package service

import (
	"encoding/json"
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
	GroupMultiplier          float64
}

type BillingResult struct {
	InputTokens              int64
	CachedInputTokens        int64
	OutputTokens             int64
	InputUSDMicros           int64
	CachedInputUSDMicros     int64
	OutputUSDMicros          int64
	RequestUSDMicros         int64
	TotalUSDMicros           int64
	TotalUSDCents            int64
	InputUSDPerMillion       float64
	CachedInputUSDPerMillion float64
	OutputUSDPerMillion      float64
	BillingMode              string
	RequestUSD               float64
	BillingMultiplier        float64
	GroupMultiplier          float64
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
			BillingMode:              model.ModelBillingModeToken,
			RequestUSD:               0,
			BillingMultiplier:        1,
			GroupMultiplier:          1,
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
			groupMultiplier := existing.GroupMultiplier
			if groupMultiplier <= 0 {
				groupMultiplier = 1
			}
			billingMode := fallbackBillingMode(existing.BillingMode)
			updates := map[string]interface{}{
				"display_name":                 pricing.DisplayName,
				"provider":                     pricing.Provider,
				"input_usd_per_million":        pricing.InputUSDPerMillion,
				"cached_input_usd_per_million": pricing.CachedInputUSDPerMillion,
				"output_usd_per_million":       pricing.OutputUSDPerMillion,
				"billing_mode":                 billingMode,
				"request_usd":                  existing.RequestUSD,
				"billing_multiplier":           multiplier,
				"group_multiplier":             groupMultiplier,
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
	return BillUsageWithGroupMultipliers(db, modelName, inputTokens, cachedInputTokens, outputTokens, totalTokens, nil)
}

func BillUsageWithGroupMultipliers(db *gorm.DB, modelName string, inputTokens, cachedInputTokens, outputTokens, totalTokens int64, groupMultipliers any) BillingResult {
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
	groupMultiplier := ResolveGroupMultiplier(pricing, groupMultipliers)
	effectiveMultiplier := multiplier * groupMultiplier
	billingMode := fallbackBillingMode(pricing.BillingMode)

	if billingMode == model.ModelBillingModeRequest {
		requestMicros := priceRequestMicros(pricing.RequestUSD, effectiveMultiplier)
		return BillingResult{
			InputTokens:              inputTokens,
			CachedInputTokens:        cachedInputTokens,
			OutputTokens:             outputTokens,
			RequestUSDMicros:         requestMicros,
			TotalUSDMicros:           requestMicros,
			TotalUSDCents:            USDmicrosToCents(requestMicros),
			InputUSDPerMillion:       pricing.InputUSDPerMillion,
			CachedInputUSDPerMillion: pricing.CachedInputUSDPerMillion,
			OutputUSDPerMillion:      pricing.OutputUSDPerMillion,
			BillingMode:              billingMode,
			RequestUSD:               pricing.RequestUSD,
			BillingMultiplier:        effectiveMultiplier,
			GroupMultiplier:          groupMultiplier,
			BillingSource:            source,
		}
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
	inputMicros := priceMicros(uncachedInputTokens, pricing.InputUSDPerMillion, effectiveMultiplier)
	cachedMicros := priceMicros(cachedInputTokens, pricing.CachedInputUSDPerMillion, effectiveMultiplier)
	outputMicros := priceMicros(outputTokens, pricing.OutputUSDPerMillion, effectiveMultiplier)
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
		BillingMode:              billingMode,
		RequestUSD:               pricing.RequestUSD,
		BillingMultiplier:        effectiveMultiplier,
		GroupMultiplier:          groupMultiplier,
		BillingSource:            source,
	}
}

func normalizeGroupMultipliers(values any) map[string]float64 {
	switch typed := values.(type) {
	case nil:
		return nil
	case map[string]float64:
		return typed
	case string:
		return ParseGroupMultipliers(typed)
	default:
		return nil
	}
}

func ResolveGroupMultiplier(pricing model.ModelPricing, groupMultipliers any) float64 {
	values := normalizeGroupMultipliers(groupMultipliers)
	if values != nil {
		if multiplier := matchedGroupMultiplier(pricing.ModelName, values); multiplier > 0 {
			return multiplier
		}
	}
	if pricing.GroupMultiplier > 0 {
		return pricing.GroupMultiplier
	}
	return 1
}

func ParseGroupMultipliers(raw string) map[string]float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values map[string]float64
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	normalized := make(map[string]float64, len(values))
	for modelName, multiplier := range values {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" || multiplier <= 0 {
			continue
		}
		normalized[modelName] = multiplier
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func EncodeGroupMultipliers(values map[string]float64) string {
	normalized := make(map[string]float64, len(values))
	for modelName, multiplier := range values {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" || multiplier <= 0 {
			continue
		}
		normalized[modelName] = multiplier
	}
	if len(normalized) == 0 {
		return ""
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	return string(data)
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
			BillingMode:              model.ModelBillingModeToken,
			RequestUSD:               0,
			BillingMultiplier:        1,
			GroupMultiplier:          1,
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
		BillingMode:              model.ModelBillingModeToken,
		RequestUSD:               0,
		BillingMultiplier:        1,
		GroupMultiplier:          1,
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

func matchedGroupMultiplier(modelName string, groupMultipliers map[string]float64) float64 {
	name := strings.ToLower(strings.TrimSpace(modelName))
	if name == "" {
		return 0
	}
	var matched float64
	matchedLen := 0
	for key, multiplier := range groupMultipliers {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" || multiplier <= 0 {
			continue
		}
		if name != key && !strings.HasPrefix(name, key+"-") {
			continue
		}
		if len(key) > matchedLen {
			matched = multiplier
			matchedLen = len(key)
		}
	}
	return matched
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

func priceRequestMicros(usd float64, multiplier float64) int64 {
	if usd <= 0 {
		return 0
	}
	if multiplier <= 0 {
		multiplier = 1
	}
	return int64(math.Round(usd * multiplier * 1_000_000))
}

func fallbackBillingMode(value string) string {
	if value == model.ModelBillingModeRequest {
		return model.ModelBillingModeRequest
	}
	return model.ModelBillingModeToken
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
