package service

import (
	"testing"
	"time"

	"ai-gateway/model"

	"gorm.io/gorm"
)

func TestModelPricingSyncUpdatesDoesNotRestoreDeletedModel(t *testing.T) {
	now := time.Now()
	updates := modelPricingSyncUpdates(model.ModelPricing{
		Model: gorm.Model{
			DeletedAt: gorm.DeletedAt{Time: now, Valid: true},
		},
		BillingMultiplier: 2,
		GroupMultiplier:   3,
	}, model.ModelPricing{
		DisplayName:              "GPT",
		Provider:                 "openai",
		InputUSDPerMillion:       1,
		CachedInputUSDPerMillion: 0.5,
		OutputUSDPerMillion:      4,
		OfficialSource:           OpenAIPricingSourceURL,
	})

	if _, ok := updates["deleted_at"]; ok {
		t.Fatal("modelPricingSyncUpdates() contains deleted_at, want sync to preserve deleted models")
	}
}
