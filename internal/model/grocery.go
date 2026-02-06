package model

import "time"

type GroceryCategory struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type GroceryList struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type GroceryItem struct {
	ID        int64      `json:"id"`
	ListID    int64      `json:"list_id"`
	Name      string     `json:"name"`
	Quantity  string     `json:"quantity"`
	Unit      string     `json:"unit"`
	Notes     string     `json:"notes"`
	Category  string     `json:"category"`
	Checked   bool       `json:"checked"`
	CheckedBy *int64     `json:"checked_by"`
	CheckedAt *time.Time `json:"checked_at"`
	AddedBy   *int64     `json:"added_by"`
	SortOrder int        `json:"sort_order"`
	CreatedAt time.Time  `json:"created_at"`
}
