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

	OrderStatusPendingReview = "pending_review"
	OrderStatusApproved      = "approved"
	OrderStatusRejected      = "rejected"

	APIKeyStatusActive   = "active"
	APIKeyStatusDisabled = "disabled"

	UpstreamStatusActive   = "active"
	UpstreamStatusDisabled = "disabled"
)

type User struct {
	gorm.Model
	Username     string `gorm:"size:64;not null"`
	Email        string `gorm:"size:128;uniqueIndex;not null"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	Role         string `gorm:"size:20;default:user;index"`
	Status       string `gorm:"size:32;default:pending;index"`
	PlanID       *uint
	Plan         *Plan
	QuotaTokens  int64
	UsedTokens   int64
	ExpiresAt    *time.Time
}

type Plan struct {
	gorm.Model
	Name         string `gorm:"size:64;uniqueIndex;not null"`
	PriceCents   int64  `gorm:"not null"`
	QuotaTokens  int64  `gorm:"not null"`
	DurationDays int    `gorm:"not null"`
	Description  string `gorm:"size:255"`
	Enabled      bool   `gorm:"default:true;index"`
}

type Order struct {
	gorm.Model
	UserID       uint `gorm:"index;not null"`
	User         User
	PlanID       uint `gorm:"index;not null"`
	Plan         Plan
	AmountCents  int64
	Status       string `gorm:"size:32;default:pending_review;index"`
	PaymentRef   string `gorm:"size:128"`
	AdminNote    string `gorm:"size:255"`
	ApprovedAt   *time.Time
	ApprovedByID *uint
}

type UpstreamAccount struct {
	gorm.Model
	UserID     uint `gorm:"uniqueIndex;not null"`
	User       User
	Channel    string `gorm:"size:64;not null"`
	BaseURL    string `gorm:"size:255;not null"`
	APIKey     string `gorm:"size:512;not null" json:"-"`
	Status     string `gorm:"size:32;default:active;index"`
	LastUsedAt *time.Time
}

type APIKey struct {
	gorm.Model
	UserID     uint `gorm:"index;not null"`
	User       User
	Name       string `gorm:"size:64;not null"`
	KeyHash    string `gorm:"size:64;uniqueIndex;not null" json:"-"`
	KeyPrefix  string `gorm:"size:20;index;not null"`
	Status     string `gorm:"size:32;default:active;index"`
	LastUsedAt *time.Time
}

type APILog struct {
	gorm.Model
	UserID       uint `gorm:"index;not null"`
	APIKeyID     uint `gorm:"index;not null"`
	Method       string
	Path         string
	StatusCode   int
	PromptTokens int64
	TotalTokens  int64
	LatencyMs    int64
	ErrorMessage string `gorm:"size:512"`
}
