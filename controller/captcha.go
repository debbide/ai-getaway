package controller

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	slideCaptchaTTL        = 5 * time.Minute
	slideCaptchaTrackWidth = 280
	slideCaptchaPieceWidth = 42
	slideCaptchaTop        = 54
	slideCaptchaHeight     = 158
)

type CaptchaController struct {
	redisClient *redis.Client
}

func NewCaptchaController(redisClient *redis.Client) *CaptchaController {
	return &CaptchaController{redisClient: redisClient}
}

func (c *CaptchaController) CreateSlide(cxt *gin.Context) {
	if c.redisClient == nil {
		response.Error(cxt, 500, "failed to create captcha")
		return
	}
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

	ctx, cancel := context.WithTimeout(cxt.Request.Context(), time.Second)
	defer cancel()
	if err := c.redisClient.Set(ctx, slideCaptchaKey(challengeID), target, slideCaptchaTTL).Err(); err != nil {
		response.Error(cxt, 500, "failed to create captcha")
		return
	}

	response.OK(cxt, gin.H{
		"challenge_id": challengeID,
		"image":        slideCaptchaImage(target),
		"track_width":  slideCaptchaTrackWidth,
		"piece_width":  slideCaptchaPieceWidth,
		"expires_in":   int(slideCaptchaTTL.Seconds()),
	})
}

func VerifySlideCaptcha(redisClient *redis.Client, challengeID string, x int) bool {
	if redisClient == nil || challengeID == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	raw, err := redisClient.GetDel(ctx, slideCaptchaKey(challengeID)).Result()
	if err != nil {
		return false
	}
	target, err := strconv.Atoi(raw)
	if err != nil {
		return false
	}
	delta := target - x
	if delta < 0 {
		delta = -delta
	}
	return delta <= 10
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

func slideCaptchaKey(challengeID string) string {
	return "captcha:slide:" + challengeID
}

func slideCaptchaImage(targetX int) string {
	holeX := targetX - slideCaptchaPieceWidth/2
	if holeX < 0 {
		holeX = 0
	}
	if holeX+slideCaptchaPieceWidth > slideCaptchaTrackWidth {
		holeX = slideCaptchaTrackWidth - slideCaptchaPieceWidth
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
<defs>
<linearGradient id="sky" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#d6c6dc"/><stop offset=".48" stop-color="#b9a4d3"/><stop offset="1" stop-color="#9a81c5"/></linearGradient>
<pattern id="dots" width="34" height="34" patternUnits="userSpaceOnUse"><circle cx="4" cy="4" r="1.4" fill="#6959a0" opacity=".22"/></pattern>
</defs>
<rect width="100%%" height="100%%" fill="url(#sky)"/>
<rect width="100%%" height="100%%" fill="url(#dots)"/>
<circle cx="22" cy="76" r="22" fill="#d6e5d0" opacity=".7"/>
<circle cx="88" cy="40" r="18" fill="#ececd8" opacity=".34"/>
<circle cx="199" cy="36" r="34" fill="#e9eacb" opacity=".58"/>
<circle cx="252" cy="136" r="34" fill="#dfb3bb" opacity=".64"/>
<rect x="146" y="22" width="108" height="2" rx="1" fill="#5281f4" opacity=".48" transform="rotate(-28 146 22)"/>
<rect x="22" y="109" width="146" height="2" rx="1" fill="#4c9488" opacity=".28" transform="rotate(-10 22 109)"/>
<rect x="%d" y="%d" width="%d" height="%d" rx="7" fill="#ffffff" opacity=".18" stroke="#ffffff" stroke-width="2"/>
</svg>`, slideCaptchaTrackWidth, slideCaptchaHeight, slideCaptchaTrackWidth, slideCaptchaHeight, holeX, slideCaptchaTop, slideCaptchaPieceWidth, slideCaptchaPieceWidth)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}
