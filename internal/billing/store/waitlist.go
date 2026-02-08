package store

import (
	"database/sql"
	"fmt"

	"github.com/dukerupert/gamwich/internal/billing/model"
)

type WaitlistStore struct {
	db *sql.DB
}

func NewWaitlistStore(db *sql.DB) *WaitlistStore {
	return &WaitlistStore{db: db}
}

// Create adds an email to the waitlist. Duplicate email+plan pairs are silently ignored.
func (s *WaitlistStore) Create(email, plan string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO waitlist (email, plan) VALUES (?, ?)`,
		email, plan,
	)
	if err != nil {
		return fmt.Errorf("insert waitlist: %w", err)
	}
	return nil
}

// List returns all waitlist entries ordered by creation time.
func (s *WaitlistStore) List() ([]model.WaitlistEntry, error) {
	rows, err := s.db.Query(`SELECT id, email, plan, created_at FROM waitlist ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list waitlist: %w", err)
	}
	defer rows.Close()

	var entries []model.WaitlistEntry
	for rows.Next() {
		var e model.WaitlistEntry
		if err := rows.Scan(&e.ID, &e.Email, &e.Plan, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan waitlist: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Count returns the number of waitlist entries.
func (s *WaitlistStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM waitlist`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count waitlist: %w", err)
	}
	return count, nil
}
