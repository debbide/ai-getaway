package controller

import (
	"errors"
	"strings"
	"time"

	"ai-gateway/model"
	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RedeemCodeController struct {
	db *gorm.DB
}

func NewRedeemCodeController(db *gorm.DB) *RedeemCodeController {
	return &RedeemCodeController{db: db}
}

type redeemCodeUserRequest struct {
	Code string `json:"code" binding:"required"`
}

func (r *RedeemCodeController) Redeem(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var req redeemCodeUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	codeValue := normalizeRedeemCode(req.Code)
	if len(codeValue) != 12 {
		response.Error(c, 400, "invalid redeem code")
		return
	}

	now := time.Now()
	var order *model.Order
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var code model.RedeemCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Plan.PublicChannel").
			Preload("Plan.PollingPool.Accounts").
			Where("code = ?", codeValue).
			First(&code).Error; err != nil {
			return err
		}
		if code.Status != model.RedeemCodeStatusUnused {
			return errors.New("redeem code already used")
		}
		if !code.Plan.Enabled {
			return errors.New("plan not found")
		}
		if code.Plan.IsLottery {
			return errors.New("lottery plan cannot be redeemed")
		}

		createdOrder, err := applyRedeemCodeSubscription(tx, user.ID, code.Plan, now, code.Code)
		if err != nil {
			return err
		}
		order = createdOrder
		result := tx.Model(&model.RedeemCode{}).
			Where("id = ? AND status = ?", code.ID, model.RedeemCodeStatusUnused).
			Updates(map[string]interface{}{
				"status":      model.RedeemCodeStatusRedeemed,
				"redeemed_by": user.ID,
				"order_id":    createdOrder.ID,
				"redeemed_at": &now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("redeem code already used")
		}
		return nil
	})
	if err != nil {
		writeRedeemError(c, err)
		return
	}
	response.OK(c, gin.H{"order": order})
}

func writeRedeemError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Error(c, 404, "redeem code not found")
	case strings.Contains(err.Error(), "sold out"):
		response.Error(c, 409, err.Error())
	case err.Error() == "redeem code already used", err.Error() == "active subscription in effect":
		response.Error(c, 409, err.Error())
	case err.Error() == "plan not found", err.Error() == "lottery plan cannot be redeemed", err.Error() == "invalid redeem code":
		response.Error(c, 400, err.Error())
	default:
		response.Error(c, 500, "failed to redeem code")
	}
}
