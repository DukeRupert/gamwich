package model

import "time"

type Session struct {
	ID          int64     `json:"id"`
	Token       string    `json:"token"`
	UserID      int64     `json:"user_id"`
	HouseholdID int64     `json:"household_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type MagicLink struct {
	ID          int64      `json:"id"`
	Token       string     `json:"token"`
	Email       string     `json:"email"`
	Purpose     string     `json:"purpose"`
	HouseholdID *int64     `json:"household_id"`
	ExpiresAt   time.Time  `json:"expires_at"`
	UsedAt      *time.Time `json:"used_at"`
	Attempts    int        `json:"attempts"`
	CreatedAt   time.Time  `json:"created_at"`
}
