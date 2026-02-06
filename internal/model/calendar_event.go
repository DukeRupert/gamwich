package model

import "time"

type CalendarEvent struct {
	ID             int64     `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	AllDay         bool      `json:"all_day"`
	FamilyMemberID *int64    `json:"family_member_id"`
	Location       string    `json:"location"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
