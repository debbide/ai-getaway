package controller

import (
	"ai-gateway/config"
	"ai-gateway/model"
	"ai-gateway/response"
	"ai-gateway/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthController struct {
	cfg config.Config
	db  *gorm.DB
}

func NewAuthController(cfg config.Config, db *gorm.DB) *AuthController {
	return &AuthController{cfg: cfg, db: db}
}

type registerRequest struct {
	Username string `json:"username" binding:"required,min=2,max=64"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (a *AuthController) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		response.Error(c, 500, "failed to hash password")
		return
	}

	user := model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Role:         model.RoleUser,
		Status:       model.UserStatusPending,
	}
	if err := a.db.Create(&user).Error; err != nil {
		response.Error(c, 409, "email already registered")
		return
	}

	response.Created(c, gin.H{"id": user.ID, "status": user.Status})
}

func (a *AuthController) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	var user model.User
	if err := a.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		response.Error(c, 401, "invalid credentials")
		return
	}
	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		response.Error(c, 401, "invalid credentials")
		return
	}
	if user.Status == model.UserStatusDisabled {
		response.Error(c, 403, "user disabled")
		return
	}

	token, err := utils.GenerateJWT(user.ID, user.Role, a.cfg.JWTSecret)
	if err != nil {
		response.Error(c, 500, "failed to generate token")
		return
	}

	response.OK(c, gin.H{
		"token": token,
		"user":  publicUser(user),
	})
}

func (a *AuthController) Me(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	response.OK(c, publicUser(user))
}

func publicUser(user model.User) gin.H {
	return gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"email":        user.Email,
		"role":         user.Role,
		"status":       user.Status,
		"quota_tokens": user.QuotaTokens,
		"used_tokens":  user.UsedTokens,
		"expires_at":   user.ExpiresAt,
	}
}
