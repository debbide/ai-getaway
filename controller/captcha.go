package controller

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"time"

	"ai-gateway/model"
	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CaptchaController struct {
	db *gorm.DB
}

func NewCaptchaController(db *gorm.DB) *CaptchaController {
	return &CaptchaController{db: db}
}

func (c *CaptchaController) CreateSlide(cxt *gin.Context) {
	challengeID, err := randomHex(16)
	if err != nil {
		response.Error(cxt, 500, "failed to create captcha")
		return
	}
	target, err := randomInt(60, 220)
	if err != nil {
		response.Error(cxt, 500, "failed to create captcha")
		return
	}

	captcha := model.SlideCaptcha{
		ChallengeID: challengeID,
		TargetX:     target,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
	if err := c.db.Create(&captcha).Error; err != nil {
		response.Error(cxt, 500, "failed to create captcha")
		return
	}

	response.OK(cxt, gin.H{
		"challenge_id": challengeID,
		"target_x":     target,
		"track_width":  280,
		"piece_width":  42,
		"expires_in":   300,
	})
}

func VerifySlideCaptcha(db *gorm.DB, challengeID string, x int) bool {
	var captcha model.SlideCaptcha
	if err := db.Where("challenge_id = ? AND used_at IS NULL", challengeID).First(&captcha).Error; err != nil {
		return false
	}
	if time.Now().After(captcha.ExpiresAt) {
		return false
	}
	delta := captcha.TargetX - x
	if delta < 0 {
		delta = -delta
	}
	if delta > 10 {
		return false
	}
	now := time.Now()
	db.Model(&captcha).Update("used_at", &now)
	return true
}

func randomHex(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func randomInt(min, max int) (int, error) {
	if max <= min {
		return min, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return 0, err
	}
	return min + int(n.Int64()), nil
}
