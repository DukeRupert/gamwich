package model

import "time"

type CalendarEvent struct {
	ID                 int64      `json:"id"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	StartTime          time.Time  `json:"start_time"`
	EndTime            time.Time  `json:"end_time"`
	AllDay             bool       `json:"all_day"`
	FamilyMemberID     *int64     `json:"family_member_id"`
	Location           string     `json:"location"`
	RecurrenceRule     string     `json:"recurrence_rule"`
	RecurrenceParentID *int64     `json:"recurrence_parent_id"`
	OriginalStartTime  *time.Time `json:"original_start_time"`
	Cancelled          bool       `json:"cancelled"`
	ReminderMinutes    *int       `json:"reminder_minutes,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
