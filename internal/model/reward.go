package model

import "time"

type Reward struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	PointCost   int       `json:"point_cost"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

type RewardRedemption struct {
	ID          int64     `json:"id"`
	RewardID    int64     `json:"reward_id"`
	RedeemedBy  *int64    `json:"redeemed_by"`
	PointsSpent int       `json:"points_spent"`
	RedeemedAt  time.Time `json:"redeemed_at"`
}

type PointBalance struct {
	MemberID   int64  `json:"member_id"`
	MemberName string `json:"member_name"`
	TotalEarned int   `json:"total_earned"`
	TotalSpent  int   `json:"total_spent"`
	Balance     int   `json:"balance"`
}
