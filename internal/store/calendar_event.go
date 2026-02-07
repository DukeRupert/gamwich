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
	return s.CreateWithRecurrence(title, description, startTime, endTime, allDay, familyMemberID, location, "")
}

func (s *EventStore) CreateWithRecurrence(title, description string, startTime, endTime time.Time, allDay bool, familyMemberID *int64, location, recurrenceRule string) (*model.CalendarEvent, error) {
	var allDayInt int
	if allDay {
		allDayInt = 1
	}

	var memberID sql.NullInt64
	if familyMemberID != nil {
		memberID = sql.NullInt64{Int64: *familyMemberID, Valid: true}
	}

	result, err := s.db.Exec(
		`INSERT INTO calendar_events (title, description, start_time, end_time, all_day, family_member_id, location, recurrence_rule)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		title, description, startTime.UTC(), endTime.UTC(), allDayInt, memberID, location, recurrenceRule,
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

func (s *EventStore) CreateException(parentID int64, originalStart time.Time, title, description string, startTime, endTime time.Time, allDay bool, familyMemberID *int64, location string, cancelled bool) (*model.CalendarEvent, error) {
	var allDayInt, cancelledInt int
	if allDay {
		allDayInt = 1
	}
	if cancelled {
		cancelledInt = 1
	}

	var memberID sql.NullInt64
	if familyMemberID != nil {
		memberID = sql.NullInt64{Int64: *familyMemberID, Valid: true}
	}

	result, err := s.db.Exec(
		`INSERT INTO calendar_events (title, description, start_time, end_time, all_day, family_member_id, location, recurrence_parent_id, original_start_time, cancelled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		title, description, startTime.UTC(), endTime.UTC(), allDayInt, memberID, location, parentID, originalStart.UTC(), cancelledInt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert exception event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return s.GetByID(id)
}

func scanEvent(scanner interface{ Scan(...any) error }) (*model.CalendarEvent, error) {
	var e model.CalendarEvent
	var allDayInt, cancelledInt int
	var memberID sql.NullInt64
	var parentID sql.NullInt64
	var originalStart sql.NullTime
	var reminderMinutes sql.NullInt64

	err := scanner.Scan(
		&e.ID, &e.Title, &e.Description, &e.StartTime, &e.EndTime,
		&allDayInt, &memberID, &e.Location, &e.RecurrenceRule,
		&parentID, &originalStart, &cancelledInt, &reminderMinutes,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	e.AllDay = allDayInt != 0
	e.Cancelled = cancelledInt != 0
	if memberID.Valid {
		e.FamilyMemberID = &memberID.Int64
	}
	if parentID.Valid {
		e.RecurrenceParentID = &parentID.Int64
	}
	if originalStart.Valid {
		e.OriginalStartTime = &originalStart.Time
	}
	if reminderMinutes.Valid {
		v := int(reminderMinutes.Int64)
		e.ReminderMinutes = &v
	}

	return &e, nil
}

const selectCols = `id, title, description, start_time, end_time, all_day, family_member_id, location, recurrence_rule, recurrence_parent_id, original_start_time, cancelled, reminder_minutes, created_at, updated_at`

func (s *EventStore) GetByID(id int64) (*model.CalendarEvent, error) {
	row := s.db.QueryRow(
		`SELECT `+selectCols+` FROM calendar_events WHERE id = ?`, id,
	)
	e, err := scanEvent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query calendar event: %w", err)
	}
	return e, nil
}

func (s *EventStore) ListByDateRange(start, end time.Time) ([]model.CalendarEvent, error) {
	rows, err := s.db.Query(
		`SELECT `+selectCols+`
		 FROM calendar_events
		 WHERE start_time < ? AND end_time > ?
		   AND recurrence_rule = ''
		   AND recurrence_parent_id IS NULL
		 ORDER BY all_day DESC, start_time ASC`,
		end.UTC(), start.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("query calendar events: %w", err)
	}
	defer rows.Close()

	var events []model.CalendarEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan calendar event: %w", err)
		}
		events = append(events, *e)
	}
	return events, rows.Err()
}

// ListRecurring returns all recurring parent events whose start_time is before the given date.
func (s *EventStore) ListRecurring(before time.Time) ([]model.CalendarEvent, error) {
	rows, err := s.db.Query(
		`SELECT `+selectCols+`
		 FROM calendar_events
		 WHERE recurrence_rule != ''
		   AND recurrence_parent_id IS NULL
		   AND start_time < ?
		 ORDER BY start_time ASC`,
		before.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("query recurring events: %w", err)
	}
	defer rows.Close()

	var events []model.CalendarEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan recurring event: %w", err)
		}
		events = append(events, *e)
	}
	return events, rows.Err()
}

// ListExceptions returns all exception events for a given parent recurring event.
func (s *EventStore) ListExceptions(parentID int64) ([]model.CalendarEvent, error) {
	rows, err := s.db.Query(
		`SELECT `+selectCols+`
		 FROM calendar_events
		 WHERE recurrence_parent_id = ?
		 ORDER BY original_start_time ASC`,
		parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query exceptions: %w", err)
	}
	defer rows.Close()

	var events []model.CalendarEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan exception: %w", err)
		}
		events = append(events, *e)
	}
	return events, rows.Err()
}

// DeleteExceptions removes all exception events for a given parent.
func (s *EventStore) DeleteExceptions(parentID int64) error {
	_, err := s.db.Exec("DELETE FROM calendar_events WHERE recurrence_parent_id = ?", parentID)
	if err != nil {
		return fmt.Errorf("delete exceptions: %w", err)
	}
	return nil
}

func (s *EventStore) Update(id int64, title, description string, startTime, endTime time.Time, allDay bool, familyMemberID *int64, location string) (*model.CalendarEvent, error) {
	return s.UpdateWithRecurrence(id, title, description, startTime, endTime, allDay, familyMemberID, location, "")
}

func (s *EventStore) UpdateWithRecurrence(id int64, title, description string, startTime, endTime time.Time, allDay bool, familyMemberID *int64, location, recurrenceRule string) (*model.CalendarEvent, error) {
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
		 SET title = ?, description = ?, start_time = ?, end_time = ?, all_day = ?, family_member_id = ?, location = ?, recurrence_rule = ?
		 WHERE id = ?`,
		title, description, startTime.UTC(), endTime.UTC(), allDayInt, memberID, location, recurrenceRule, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update calendar event: %w", err)
	}

	return s.GetByID(id)
}

func (s *EventStore) SetReminderMinutes(id int64, minutes *int) error {
	var val any
	if minutes != nil {
		val = *minutes
	}
	_, err := s.db.Exec(`UPDATE calendar_events SET reminder_minutes = ? WHERE id = ?`, val, id)
	if err != nil {
		return fmt.Errorf("set reminder minutes: %w", err)
	}
	return nil
}

// ListUpcomingWithReminders returns non-cancelled events that have a reminder set
// and whose (start_time - reminder_minutes) falls within the given window.
func (s *EventStore) ListUpcomingWithReminders(windowStart, windowEnd time.Time) ([]model.CalendarEvent, error) {
	rows, err := s.db.Query(
		`SELECT `+selectCols+`
		 FROM calendar_events
		 WHERE reminder_minutes IS NOT NULL
		   AND cancelled = 0
		   AND datetime(start_time, '-' || reminder_minutes || ' minutes') >= ?
		   AND datetime(start_time, '-' || reminder_minutes || ' minutes') < ?
		 ORDER BY start_time ASC`,
		windowStart.UTC().Format("2006-01-02 15:04:05"),
		windowEnd.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, fmt.Errorf("query upcoming reminders: %w", err)
	}
	defer rows.Close()

	var events []model.CalendarEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan reminder event: %w", err)
		}
		events = append(events, *e)
	}
	return events, rows.Err()
}

func (s *EventStore) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM calendar_events WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete calendar event: %w", err)
	}
	return nil
}
