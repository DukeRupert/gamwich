package model

import "time"

type FamilyMember struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	AvatarEmoji string    `json:"avatar_emoji"`
	HasPIN      bool      `json:"has_pin"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
