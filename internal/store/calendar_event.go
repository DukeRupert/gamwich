package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
)

type EventStore struct {
	db *sql.DB
}

func NewEventStore(db *sql.DB) *EventStore {
	return &EventStore{db: db}
}

func (s *EventStore) Create(title, description string, startTime, endTime time.Time, allDay bool, familyMemberID *int64, location string) (*model.CalendarEvent, error) {
	var allDayInt int
	if allDay {
		allDayInt = 1
	}

	var memberID sql.NullInt64
	if familyMemberID != nil {
		memberID = sql.NullInt64{Int64: *familyMemberID, Valid: true}
	}

	result, err := s.db.Exec(
		`INSERT INTO calendar_events (title, description, start_time, end_time, all_day, family_member_id, location)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		title, description, startTime.UTC(), endTime.UTC(), allDayInt, memberID, location,
	)
	if err != nil {
		return nil, fmt.Errorf("insert calendar event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return s.GetByID(id)
}

func (s *EventStore) GetByID(id int64) (*model.CalendarEvent, error) {
	var e model.CalendarEvent
	var allDayInt int
	var memberID sql.NullInt64

	err := s.db.QueryRow(
		`SELECT id, title, description, start_time, end_time, all_day, family_member_id, location, created_at, updated_at
		 FROM calendar_events WHERE id = ?`,
		id,
	).Scan(&e.ID, &e.Title, &e.Description, &e.StartTime, &e.EndTime, &allDayInt, &memberID, &e.Location, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query calendar event: %w", err)
	}

	e.AllDay = allDayInt != 0
	if memberID.Valid {
		e.FamilyMemberID = &memberID.Int64
	}

	return &e, nil
}

func (s *EventStore) ListByDateRange(start, end time.Time) ([]model.CalendarEvent, error) {
	rows, err := s.db.Query(
		`SELECT id, title, description, start_time, end_time, all_day, family_member_id, location, created_at, updated_at
		 FROM calendar_events
		 WHERE start_time < ? AND end_time > ?
		 ORDER BY all_day DESC, start_time ASC`,
		end.UTC(), start.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("query calendar events: %w", err)
	}
	defer rows.Close()

	var events []model.CalendarEvent
	for rows.Next() {
		var e model.CalendarEvent
		var allDayInt int
		var memberID sql.NullInt64

		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.StartTime, &e.EndTime, &allDayInt, &memberID, &e.Location, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan calendar event: %w", err)
		}

		e.AllDay = allDayInt != 0
		if memberID.Valid {
			e.FamilyMemberID = &memberID.Int64
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *EventStore) Update(id int64, title, description string, startTime, endTime time.Time, allDay bool, familyMemberID *int64, location string) (*model.CalendarEvent, error) {
	var allDayInt int
	if allDay {
		allDayInt = 1
	}

	var memberID sql.NullInt64
	if familyMemberID != nil {
		memberID = sql.NullInt64{Int64: *familyMemberID, Valid: true}
	}

	_, err := s.db.Exec(
		`UPDATE calendar_events
		 SET title = ?, description = ?, start_time = ?, end_time = ?, all_day = ?, family_member_id = ?, location = ?
		 WHERE id = ?`,
		title, description, startTime.UTC(), endTime.UTC(), allDayInt, memberID, location, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update calendar event: %w", err)
	}

	return s.GetByID(id)
}

func (s *EventStore) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM calendar_events WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete calendar event: %w", err)
	}
	return nil
}
