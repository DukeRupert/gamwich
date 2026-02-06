package model

import "time"

type Account struct {
	ID               int64     `json:"id"`
	Email            string    `json:"email"`
	StripeCustomerID *string   `json:"stripe_customer_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Subscription struct {
	ID                   int64      `json:"id"`
	AccountID            int64      `json:"account_id"`
	StripeSubscriptionID *string    `json:"stripe_subscription_id"`
	Plan                 string     `json:"plan"`
	Status               string     `json:"status"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end"`
	CancelAtPeriodEnd    bool       `json:"cancel_at_period_end"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type LicenseKey struct {
	ID             int64      `json:"id"`
	AccountID      int64      `json:"account_id"`
	SubscriptionID int64      `json:"subscription_id"`
	Key            string     `json:"key"`
	Plan           string     `json:"plan"`
	Features       string     `json:"features"`
	ActivatedAt    *time.Time `json:"activated_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Session struct {
	ID        int64     `json:"id"`
	Token     string    `json:"token"`
	AccountID int64     `json:"account_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
