package model

import "time"

type Note struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	AuthorID  *int64     `json:"author_id"`
	Pinned    bool       `json:"pinned"`
	Priority  string     `json:"priority"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
