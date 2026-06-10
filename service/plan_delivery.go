package service

import (
	"errors"
	"strings"

	"ai-gateway/model"

	"gorm.io/gorm"
)

var ErrPlanChannelSoldOut = errors.New("public plan sold out")

func NormalizeProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case model.ProtocolClaude, "anthropic":
		return model.ProtocolClaude
	default:
		return model.ProtocolGPT
	}
}

func SupportsProtocol(supportsGPT, supportsClaude bool, protocol string) bool {
	switch NormalizeProtocol(protocol) {
	case model.ProtocolClaude:
		return supportsClaude
	default:
		return supportsGPT
	}
}

func PlanChannelRemainingUSDCents(plan model.Plan) int64 {
	if plan.PublicChannel != nil {
		return plan.PublicChannel.RemainingUSDCents
	}
	if plan.PollingPool != nil {
		var total int64
		for _, account := range plan.PollingPool.Accounts {
			if account.Enabled {
				total += account.RemainingUSDCents
			}
		}
		return total
	}
	return 0
}

func PlanChannelSupportsProtocol(plan *model.Plan, protocol string) bool {
	if plan == nil {
		return false
	}
	if plan.PublicChannel != nil {
		return SupportsProtocol(plan.PublicChannel.SupportsGPT, plan.PublicChannel.SupportsClaude, protocol)
	}
	if plan.PollingPool != nil {
		return SupportsProtocol(plan.PollingPool.SupportsGPT, plan.PollingPool.SupportsClaude, protocol)
	}
	return true
}

func PlanChannelHasQuota(plan model.Plan) bool {
	if plan.PlanType != model.PlanTypePublic {
		return true
	}
	if plan.SettlementUSDCents <= 0 {
		return true
	}
	return PlanChannelRemainingUSDCents(plan) >= plan.SettlementUSDCents
}

func DeductPlanChannelQuota(tx *gorm.DB, plan model.Plan) error {
	if plan.PlanType != model.PlanTypePublic {
		return nil
	}
	if plan.SettlementUSDCents <= 0 {
		return nil
	}
	if plan.PublicChannelID != nil {
		if plan.PublicChannel == nil || !plan.PublicChannel.Enabled || plan.PublicChannel.RemainingUSDCents < plan.SettlementUSDCents {
			return ErrPlanChannelSoldOut
		}
		result := tx.Model(&model.PublicChannel{}).
			Where("id = ? AND enabled = ? AND remaining_usd_cents >= ?", *plan.PublicChannelID, true, plan.SettlementUSDCents).
			Update("remaining_usd_cents", gorm.Expr("remaining_usd_cents - ?", plan.SettlementUSDCents))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrPlanChannelSoldOut
		}
		return nil
	}
	if plan.PollingPoolID == nil || plan.PollingPool == nil || !plan.PollingPool.Enabled {
		return ErrPlanChannelSoldOut
	}

	var oauthCount int64
	if err := tx.Model(&model.PollingPoolAccount{}).
		Where("polling_pool_id = ? AND enabled = ? AND auth_type = ?", *plan.PollingPoolID, true, OpenAIAccountAuthOAuth).
		Count(&oauthCount).Error; err != nil {
		return err
	}
	if oauthCount > 0 {
		return nil
	}

	remaining := plan.SettlementUSDCents
	var accounts []model.PollingPoolAccount
	if err := tx.Where("polling_pool_id = ? AND enabled = ? AND remaining_usd_cents > 0", *plan.PollingPoolID, true).
		Order("sort_order asc, id asc").
		Find(&accounts).Error; err != nil {
		return err
	}
	for _, account := range accounts {
		if remaining <= 0 {
			break
		}
		deduct := account.RemainingUSDCents
		if deduct > remaining {
			deduct = remaining
		}
		result := tx.Model(&model.PollingPoolAccount{}).
			Where("id = ? AND enabled = ? AND remaining_usd_cents >= ?", account.ID, true, deduct).
			Update("remaining_usd_cents", gorm.Expr("remaining_usd_cents - ?", deduct))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrPlanChannelSoldOut
		}
		remaining -= deduct
	}
	if remaining > 0 {
		return ErrPlanChannelSoldOut
	}
	return nil
}
