package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"

	UserStatusPending  = "pending"
	UserStatusApproved = "approved"
	UserStatusDisabled = "disabled"

	OrderStatusPendingPayment = "pending_payment"
	OrderStatusPendingReview  = "pending_review"
	OrderStatusApproved       = "approved"
	OrderStatusRejected       = "rejected"

	APIKeyStatusActive   = "active"
	APIKeyStatusDisabled = "disabled"

	UpstreamStatusActive   = "active"
	UpstreamStatusDisabled = "disabled"

	ModelPricingStatusActive   = "active"
	ModelPricingStatusDisabled = "disabled"
)

type User struct {
	gorm.Model
	Username      string `gorm:"size:64;not null"`
	Email         string `gorm:"size:128;uniqueIndex;not null"`
	PasswordHash  string `gorm:"size:255;not null" json:"-"`
	Role          string `gorm:"size:20;default:user;index"`
	Status        string `gorm:"size:32;default:pending;index"`
	EmailVerified bool   `gorm:"default:false;index"`
	PlanID        *uint
	Plan          *Plan
	ExpiresAt     *time.Time
}

type Plan struct {
	gorm.Model
	Name               string `gorm:"size:64;uniqueIndex;not null"`
	Code               string `gorm:"size:64;uniqueIndex"`
	BadgeText          string `gorm:"size:32"`
	PlanType           string `gorm:"size:32;default:subscription;index"`
	QuotaPeriod        string `gorm:"size:16;default:weekly;index"`
	PriceCents         int64  `gorm:"not null"`
	SettlementUSDCents int64  `gorm:"default:0"`
	DurationDays       int    `gorm:"not null"`
	Description        string `gorm:"size:255"`
	Enabled            bool   `gorm:"default:true;index"`
}

type Order struct {
	gorm.Model
	UserID             uint `gorm:"index;not null"`
	User               User
	PlanID             uint `gorm:"index;not null"`
	Plan               Plan
	AmountCents        int64
	SettlementUSDCents int64  `gorm:"default:0"`
	Status             string `gorm:"size:32;default:pending_payment;index"`
	PaymentRef         string `gorm:"size:128"`
	AdminNote          string `gorm:"size:255"`
	ApprovedAt         *time.Time
	ApprovedByID       *uint
}

type UpstreamAccount struct {
	gorm.Model
	UserID     uint `gorm:"uniqueIndex;not null"`
	User       User
	Channel    string `gorm:"size:64;not null"`
	BaseURL    string `gorm:"size:255;not null"`
	Username   string `gorm:"size:128"`
	Password   string `gorm:"size:255" json:"-"`
	APIKey     string `gorm:"size:512;not null" json:"-"`
	Status     string `gorm:"size:32;default:active;index"`
	LastUsedAt *time.Time
}

type UpstreamChannel struct {
	gorm.Model
	Name    string `gorm:"size:64;uniqueIndex;not null"`
	BaseURL string `gorm:"size:255;not null"`
	Enabled bool   `gorm:"default:true;index"`
}

type DocPage struct {
	gorm.Model
	Title       string `gorm:"size:128;not null"`
	Slug        string `gorm:"size:128;uniqueIndex;not null"`
	GroupName   string `gorm:"size:64;index"`
	Description string `gorm:"size:255"`
	Content     string `gorm:"type:longtext"`
	SortOrder   int    `gorm:"default:0;index"`
	Enabled     bool   `gorm:"default:true;index"`
}

type APIKey struct {
	gorm.Model
	UserID       uint `gorm:"index;not null"`
	User         User
	Name         string `gorm:"size:64;not null"`
	KeyHash      string `gorm:"size:64;uniqueIndex;not null" json:"-"`
	KeyPrefix    string `gorm:"size:20;index;not null"`
	KeyEncrypted string `gorm:"size:768;default:''" json:"-"` // AES-GCM, owner-only decrypt
	Status       string `gorm:"size:32;default:active;index"`
	LastUsedAt   *time.Time
}

type ModelPricing struct {
	gorm.Model
	ModelName                string `gorm:"column:model;size:128;uniqueIndex;not null"`
	DisplayName              string `gorm:"size:128"`
	Provider                 string `gorm:"size:64;default:openai;index"`
	InputUSDPerMillion       float64
	CachedInputUSDPerMillion float64
	OutputUSDPerMillion      float64
	BillingMultiplier        float64 `gorm:"default:1"`
	Status                   string  `gorm:"size:32;default:active;index"`
	Official                 bool    `gorm:"default:false;index"`
	OfficialSource           string  `gorm:"size:255"`
	OfficialSyncedAt         *time.Time
	Notes                    string `gorm:"size:255"`
}

type APILog struct {
	gorm.Model
	UserID                   uint `gorm:"index;not null"`
	APIKeyID                 uint `gorm:"index;not null"`
	APIKey                   APIKey
	Method                   string
	Path                     string
	ModelName                string `gorm:"column:model;size:128;index"`
	RequestType              string `gorm:"size:32;default:chat;index"`
	StatusCode               int
	PromptTokens             int64
	CachedInputTokens        int64
	CompletionTokens         int64
	TotalTokens              int64
	EstimatedUSDCents        int64   `gorm:"default:0;index"`
	EstimatedUSDMicros       int64   `gorm:"default:0;index"`
	InputUSDMicros           int64   `gorm:"default:0"`
	CachedInputUSDMicros     int64   `gorm:"default:0"`
	OutputUSDMicros          int64   `gorm:"default:0"`
	InputUSDPerMillion       float64 `gorm:"default:0"`
	CachedInputUSDPerMillion float64 `gorm:"default:0"`
	OutputUSDPerMillion      float64 `gorm:"default:0"`
	BillingMultiplier        float64 `gorm:"default:1"`
	BillingSource            string  `gorm:"size:64"`
	FirstTokenMs             int64
	LatencyMs                int64
	ErrorMessage             string `gorm:"size:512"`
}

type SystemSetting struct {
	gorm.Model
	SiteTitle        string `gorm:"size:128;default:星空AI"`
	APIEndpoints     string `gorm:"type:text"`
	TutorialVideoURL string `gorm:"size:512"`
	NavigationItems  string `gorm:"type:text"`
	PricingTitle     string `gorm:"size:128;default:简单透明的定价"`
	PricingSubtitle  string `gorm:"size:255;default:保质保量无降智不掺假"`
	PricingNotice    string `gorm:"size:512;default:本站仅支持 GPT 模型使用，具体型号请查看 /models 页面；如需使用 Claude 模型，请前往顶部菜单更多中转 → Claude Code 中转"`
	SMTPHost         string `gorm:"size:128"`
	SMTPPort         int    `gorm:"default:587"`
	SMTPUsername     string `gorm:"size:128"`
	SMTPPassword     string `gorm:"size:255" json:"-"`
	SMTPFromEmail    string `gorm:"size:128"`
	SMTPFromName     string `gorm:"size:128"`
	SMTPUseTLS       bool   `gorm:"default:true"`
	EpayPID          string `gorm:"column:epay_pid;size:128"`
	EpayKey          string `gorm:"size:255" json:"-"`
	EpayNotifyURL    string `gorm:"size:512"`
	EpayReturnURL    string `gorm:"size:512"`
	EpaySubmitURL    string `gorm:"size:512"`
}

type EmailVerification struct {
	gorm.Model
	Email     string    `gorm:"size:128;index;not null"`
	CodeHash  string    `gorm:"size:64;not null"`
	Purpose   string    `gorm:"size:32;index;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	UsedAt    *time.Time
}

type SlideCaptcha struct {
	gorm.Model
	ChallengeID string    `gorm:"size:64;uniqueIndex;not null"`
	TargetX     int       `gorm:"not null"`
	ExpiresAt   time.Time `gorm:"index;not null"`
	UsedAt      *time.Time
}
