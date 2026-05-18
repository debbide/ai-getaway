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
	OrderStatusPaymentTimeout = "payment_timeout"
	OrderStatusPaidLate       = "paid_late"
	OrderStatusManualReview   = "pending_manual_review"

	PaymentMethodOnline = "online"
	PaymentMethodManual = "manual"

	OrderTypePurchase = "purchase"
	OrderTypeRenewal  = "renewal"
	OrderTypeUpgrade  = "upgrade"

	APIKeyStatusActive   = "active"
	APIKeyStatusDisabled = "disabled"

	EmailTemplateOrderPaymentAdmin    = "order_payment_admin"
	EmailTemplateOrderApprovedUser    = "order_approved_user"
	EmailTemplateSubscriptionExpiring = "subscription_expiring"

	UpstreamStatusActive   = "active"
	UpstreamStatusDisabled = "disabled"

	ModelPricingStatusActive   = "active"
	ModelPricingStatusDisabled = "disabled"

	PlanTypeSubscription = "subscription"
	PlanTypePublic       = "public"

	QuotaPeriodDaily  = "daily"
	QuotaPeriodWeekly = "weekly"
	QuotaPeriodPublic = "public"

	ProtocolGPT    = "gpt"
	ProtocolClaude = "claude"

	RedeemCodeStatusUnused   = "unused"
	RedeemCodeStatusRedeemed = "redeemed"
	RedeemCodeStatusDisabled = "disabled"
)

type User struct {
	gorm.Model
	Username              string `gorm:"size:64;not null"`
	Email                 string `gorm:"size:128;uniqueIndex;not null"`
	PasswordHash          string `gorm:"size:255;not null" json:"-"`
	Role                  string `gorm:"size:20;default:user;index"`
	Status                string `gorm:"size:32;default:pending;index"`
	EmailVerified         bool   `gorm:"default:false;index"`
	PlanID                *uint
	Plan                  *Plan
	ExpiresAt             *time.Time
	SubscriptionStartedAt *time.Time
}

type Plan struct {
	gorm.Model
	Name               string `gorm:"size:64;uniqueIndex;not null"`
	Code               string `gorm:"size:64;uniqueIndex"`
	BadgeText          string `gorm:"size:32"`
	PlanType           string `gorm:"size:32;default:subscription;index"`
	QuotaPeriod        string `gorm:"size:16;default:weekly;index"`
	PublicChannelID    *uint  `gorm:"index"`
	PublicChannel      *PublicChannel
	PollingPoolID      *uint `gorm:"index"`
	PollingPool        *PollingPool
	PriceCents         int64  `gorm:"not null"`
	SettlementUSDCents int64  `gorm:"default:0"`
	DurationDays       int    `gorm:"not null"`
	Description        string `gorm:"size:255"`
	IsLottery          bool   `gorm:"default:false;index"`
	LotteryURL         string `gorm:"size:512"`
	FreePerUserLimit   int    `gorm:"default:1"`
	FreeTotalLimit     int    `gorm:"default:0"`
	FreeClaimedCount   int    `gorm:"default:0"`
	Enabled            bool   `gorm:"default:true;index"`
}

type Order struct {
	gorm.Model
	UserID                uint `gorm:"index;not null"`
	User                  User
	PlanID                uint `gorm:"index;not null"`
	Plan                  Plan
	OrderType             string `gorm:"size:32;default:purchase;index"`
	AmountCents           int64
	SettlementUSDCents    int64   `gorm:"default:0"`
	Status                string  `gorm:"size:32;default:pending_payment;index"`
	PaymentMethod         string  `gorm:"size:32;default:online;index"`
	PaymentRef            string  `gorm:"size:128;uniqueIndex"`
	ProviderTradeNo       *string `gorm:"size:128;uniqueIndex"`
	PaymentChannel        string  `gorm:"size:32"`
	PaidAmountCents       int64   `gorm:"default:0"`
	PaidAt                *time.Time
	PaymentURLGeneratedAt *time.Time
	PaymentRaw            string `gorm:"type:text"`
	UserPaymentNote       string `gorm:"size:255"`
	AdminNote             string `gorm:"size:255"`
	ApprovedAt            *time.Time
	ApprovedByID          *uint
}

type RedeemCode struct {
	gorm.Model
	Code       string `gorm:"size:32;uniqueIndex;not null"`
	PlanID     uint   `gorm:"index;not null"`
	Plan       Plan
	Status     string `gorm:"size:32;default:unused;index"`
	RedeemedBy *uint  `gorm:"index"`
	User       *User  `gorm:"foreignKey:RedeemedBy"`
	OrderID    *uint  `gorm:"index"`
	Order      *Order
	RedeemedAt *time.Time
	CreatedBy  *uint  `gorm:"index"`
	Creator    *User  `gorm:"foreignKey:CreatedBy"`
	Note       string `gorm:"size:255"`
}

type UpstreamAccount struct {
	gorm.Model
	UserID         uint `gorm:"uniqueIndex;not null"`
	User           User
	Channel        string `gorm:"size:64;not null"`
	BaseURL        string `gorm:"size:255;not null"`
	Username       string `gorm:"size:128"`
	Password       string `gorm:"size:255" json:"-"`
	APIKey         string `gorm:"size:512;not null" json:"-"`
	SupportsGPT    bool   `gorm:"default:true"`
	SupportsClaude bool   `gorm:"default:false"`
	Status         string `gorm:"size:32;default:active;index"`
	LastUsedAt     *time.Time
}

type UpstreamChannel struct {
	gorm.Model
	Name           string `gorm:"size:64;uniqueIndex;not null"`
	BaseURL        string `gorm:"size:255;not null"`
	SupportsGPT    bool   `gorm:"default:true"`
	SupportsClaude bool   `gorm:"default:false"`
	Enabled        bool   `gorm:"default:true;index"`
}

type PublicChannel struct {
	gorm.Model
	Name              string `gorm:"size:64;uniqueIndex;not null"`
	BaseURL           string `gorm:"size:255;not null"`
	APIKey            string `gorm:"size:512;not null" json:"-"`
	SupportsGPT       bool   `gorm:"default:true"`
	SupportsClaude    bool   `gorm:"default:false"`
	TotalUSDCents     int64  `gorm:"default:0"`
	RemainingUSDCents int64  `gorm:"default:0;index"`
	Enabled           bool   `gorm:"default:true;index"`
	LastUsedAt        *time.Time
}

type PollingPool struct {
	gorm.Model
	Name           string `gorm:"size:64;uniqueIndex;not null"`
	SupportsGPT    bool   `gorm:"default:true"`
	SupportsClaude bool   `gorm:"default:false"`
	Enabled        bool   `gorm:"default:true;index"`
	Accounts       []PollingPoolAccount
}

type PollingPoolAccount struct {
	gorm.Model
	PollingPoolID     uint `gorm:"index;not null"`
	PollingPool       PollingPool
	Name              string `gorm:"size:64;not null"`
	BaseURL           string `gorm:"size:255;not null"`
	APIKey            string `gorm:"size:512;not null" json:"-"`
	TotalUSDCents     int64  `gorm:"default:0"`
	RemainingUSDCents int64  `gorm:"default:0;index"`
	Enabled           bool   `gorm:"default:true;index"`
	SortOrder         int    `gorm:"default:0;index"`
	LastUsedAt        *time.Time
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

type Announcement struct {
	gorm.Model
	Title       string `gorm:"size:160;not null"`
	Summary     string `gorm:"size:512"`
	Content     string `gorm:"type:longtext"`
	LinkText    string `gorm:"size:64"`
	LinkURL     string `gorm:"size:512"`
	SortOrder   int    `gorm:"default:0;index"`
	Pinned      bool   `gorm:"default:false;index"`
	Enabled     bool   `gorm:"default:true;index"`
	PublishedAt *time.Time
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
	Featured                 bool    `gorm:"default:false;index"`
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
	SiteTitle                      string `gorm:"size:128;default:星空AI"`
	ContactEmail                   string `gorm:"size:128;default:support@example.com"`
	APIEndpoints                   string `gorm:"type:text"`
	NavigationItems                string `gorm:"type:text"`
	PricingTitle                   string `gorm:"size:128;default:简单透明的定价"`
	PricingSubtitle                string `gorm:"size:255;default:保质保量无降智不掺假"`
	PricingNotice                  string `gorm:"size:512;default:本站仅支持 GPT 模型使用，具体型号请查看 /models 页面；如需使用 Claude 模型，请前往顶部菜单更多中转 → Claude Code 中转"`
	AllowRegistration              bool   `gorm:"default:true"`
	SMTPHost                       string `gorm:"size:128"`
	SMTPPort                       int    `gorm:"default:587"`
	SMTPUsername                   string `gorm:"size:128"`
	SMTPPassword                   string `gorm:"size:255" json:"-"`
	SMTPFromEmail                  string `gorm:"size:128"`
	SMTPFromName                   string `gorm:"size:128"`
	SMTPUseTLS                     bool   `gorm:"default:true"`
	OrderPaymentAdminEmailEnabled  bool   `gorm:"default:false"`
	OrderApprovedUserEmailEnabled  bool   `gorm:"default:false"`
	SubscriptionExpireEmailEnabled bool   `gorm:"default:false"`
	SubscriptionExpireRemindDays   int    `gorm:"default:3"`
	EpayPID                        string `gorm:"column:epay_pid;size:128"`
	EpayKey                        string `gorm:"size:255" json:"-"`
	EpayNotifyURL                  string `gorm:"size:512"`
	EpayReturnURL                  string `gorm:"size:512"`
	EpaySubmitURL                  string `gorm:"size:512"`
	OnlinePaymentEnabled           bool   `gorm:"default:true"`
	ManualPaymentEnabled           bool   `gorm:"default:true"`
	ManualPaymentQRCode            string `gorm:"type:longtext"`
	MockAPIOnlineEnabled           bool   `gorm:"default:false"`
	MockAPIOnlineBase              int    `gorm:"default:0"`
}

type EmailTemplate struct {
	gorm.Model
	Type        string `gorm:"size:64;uniqueIndex;not null"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"size:255"`
	Subject     string `gorm:"size:255;not null"`
	Body        string `gorm:"type:longtext"`
	Enabled     bool   `gorm:"default:true;index"`
}

type EmailNotificationLog struct {
	gorm.Model
	EventType   string `gorm:"size:64;index;not null"`
	UserID      *uint  `gorm:"index"`
	OrderID     *uint  `gorm:"index"`
	SentTo      string `gorm:"size:191;index;not null"`
	Fingerprint string `gorm:"size:128;uniqueIndex;not null"`
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
