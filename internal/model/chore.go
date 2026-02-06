package model

import "time"

type ChoreArea struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Chore struct {
	ID             int64     `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	AreaID         *int64    `json:"area_id"`
	Points         int       `json:"points"`
	RecurrenceRule string    `json:"recurrence_rule"`
	AssignedTo     *int64    `json:"assigned_to"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ChoreCompletion struct {
	ID          int64     `json:"id"`
	ChoreID     int64     `json:"chore_id"`
	CompletedBy *int64    `json:"completed_by"`
	CompletedAt time.Time `json:"completed_at"`
}
