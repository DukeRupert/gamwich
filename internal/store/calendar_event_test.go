package store

import (
	"testing"
	"time"

	"github.com/dukerupert/gamwich/internal/database"
)

func setupTestDB(t *testing.T) *EventStore {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	// Ensure foreign keys are enforced (modernc/sqlite may not honor DSN param for :memory:)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewEventStore(db)
}

func TestCreateAndGetByID(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 5, 11, 0, 0, 0, time.UTC)

	event, err := s.Create("Team Meeting", "Weekly sync", start, end, false, nil, "Conference Room")
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if event.Title != "Team Meeting" {
		t.Errorf("title = %q, want %q", event.Title, "Team Meeting")
	}
	if event.Description != "Weekly sync" {
		t.Errorf("description = %q, want %q", event.Description, "Weekly sync")
	}
	if event.Location != "Conference Room" {
		t.Errorf("location = %q, want %q", event.Location, "Conference Room")
	}
	if event.AllDay {
		t.Error("all_day should be false")
	}
	if event.FamilyMemberID != nil {
		t.Errorf("family_member_id should be nil, got %v", *event.FamilyMemberID)
	}

	// GetByID
	got, err := s.GetByID(event.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Title != "Team Meeting" {
		t.Errorf("got title = %q, want %q", got.Title, "Team Meeting")
	}
}

func TestGetByIDNotFound(t *testing.T) {
	s := setupTestDB(t)

	got, err := s.GetByID(999)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent event")
	}
}

func TestCreateWithFamilyMember(t *testing.T) {
	s := setupTestDB(t)

	// Create a family member first
	_, err := s.db.Exec("INSERT INTO family_members (name, color, avatar_emoji, sort_order) VALUES (?, ?, ?, ?)", "Alice", "#FF0000", "A", 0)
	if err != nil {
		t.Fatalf("insert family member: %v", err)
	}

	memberID := int64(1)
	start := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 5, 15, 0, 0, 0, time.UTC)

	event, err := s.Create("Alice's Meeting", "", start, end, false, &memberID, "")
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if event.FamilyMemberID == nil || *event.FamilyMemberID != 1 {
		t.Errorf("family_member_id = %v, want 1", event.FamilyMemberID)
	}
}

func TestCreateAllDayEvent(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 6, 0, 0, 0, 0, time.UTC)

	event, err := s.Create("Holiday", "", start, end, true, nil, "")
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if !event.AllDay {
		t.Error("all_day should be true")
	}
}

func TestListByDateRange(t *testing.T) {
	s := setupTestDB(t)

	// Create events across several days
	day1Start := time.Date(2026, 2, 5, 9, 0, 0, 0, time.UTC)
	day1End := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)
	s.Create("Day 1 Event", "", day1Start, day1End, false, nil, "")

	day2Start := time.Date(2026, 2, 6, 9, 0, 0, 0, time.UTC)
	day2End := time.Date(2026, 2, 6, 10, 0, 0, 0, time.UTC)
	s.Create("Day 2 Event", "", day2Start, day2End, false, nil, "")

	day3Start := time.Date(2026, 2, 7, 9, 0, 0, 0, time.UTC)
	day3End := time.Date(2026, 2, 7, 10, 0, 0, 0, time.UTC)
	s.Create("Day 3 Event", "", day3Start, day3End, false, nil, "")

	// Query only day 1-2 range
	rangeStart := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, 2, 7, 0, 0, 0, 0, time.UTC)

	events, err := s.ListByDateRange(rangeStart, rangeEnd)
	if err != nil {
		t.Fatalf("list by range: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Title != "Day 1 Event" {
		t.Errorf("first event = %q, want %q", events[0].Title, "Day 1 Event")
	}
	if events[1].Title != "Day 2 Event" {
		t.Errorf("second event = %q, want %q", events[1].Title, "Day 2 Event")
	}
}

func TestListByDateRangeAllDayFirst(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 6, 0, 0, 0, 0, time.UTC)

	// Create timed event first
	s.Create("Morning Meeting", "", time.Date(2026, 2, 5, 9, 0, 0, 0, time.UTC), time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC), false, nil, "")
	// Create all-day event second
	s.Create("Holiday", "", start, end, true, nil, "")

	events, err := s.ListByDateRange(start, end)
	if err != nil {
		t.Fatalf("list by range: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	// All-day should come first
	if events[0].Title != "Holiday" {
		t.Errorf("first event = %q, want all-day event %q", events[0].Title, "Holiday")
	}
}

func TestListByDateRangeSpanningEvent(t *testing.T) {
	s := setupTestDB(t)

	// Event spans across query range
	eventStart := time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC)
	eventEnd := time.Date(2026, 2, 8, 0, 0, 0, 0, time.UTC)
	s.Create("Multi-day Event", "", eventStart, eventEnd, false, nil, "")

	// Query for a single day within the span
	rangeStart := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, 2, 6, 0, 0, 0, 0, time.UTC)

	events, err := s.ListByDateRange(rangeStart, rangeEnd)
	if err != nil {
		t.Fatalf("list by range: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (spanning event)", len(events))
	}
}

func TestUpdate(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 5, 11, 0, 0, 0, time.UTC)

	event, err := s.Create("Original Title", "", start, end, false, nil, "")
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	newStart := time.Date(2026, 2, 5, 14, 0, 0, 0, time.UTC)
	newEnd := time.Date(2026, 2, 5, 15, 30, 0, 0, time.UTC)
	updated, err := s.Update(event.ID, "Updated Title", "Added desc", newStart, newEnd, true, nil, "New Location")
	if err != nil {
		t.Fatalf("update event: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("title = %q, want %q", updated.Title, "Updated Title")
	}
	if updated.Description != "Added desc" {
		t.Errorf("description = %q, want %q", updated.Description, "Added desc")
	}
	if updated.Location != "New Location" {
		t.Errorf("location = %q, want %q", updated.Location, "New Location")
	}
	if !updated.AllDay {
		t.Error("all_day should be true after update")
	}
}

func TestDelete(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 5, 11, 0, 0, 0, time.UTC)

	event, err := s.Create("To Delete", "", start, end, false, nil, "")
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	if err := s.Delete(event.ID); err != nil {
		t.Fatalf("delete event: %v", err)
	}

	got, err := s.GetByID(event.ID)
	if err != nil {
		t.Fatalf("get by id after delete: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestFamilyMemberDeleteSetsNull(t *testing.T) {
	s := setupTestDB(t)

	// Create a family member
	_, err := s.db.Exec("INSERT INTO family_members (name, color, avatar_emoji, sort_order) VALUES (?, ?, ?, ?)", "Bob", "#00FF00", "B", 0)
	if err != nil {
		t.Fatalf("insert family member: %v", err)
	}

	memberID := int64(1)
	start := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 5, 11, 0, 0, 0, time.UTC)

	event, err := s.Create("Bob's Event", "", start, end, false, &memberID, "")
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	// Delete the family member
	_, err = s.db.Exec("DELETE FROM family_members WHERE id = ?", memberID)
	if err != nil {
		t.Fatalf("delete family member: %v", err)
	}

	// Event should still exist with null family_member_id
	got, err := s.GetByID(event.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got == nil {
		t.Fatal("event should still exist after member deletion")
	}
	if got.FamilyMemberID != nil {
		t.Errorf("family_member_id should be nil after member deletion, got %v", *got.FamilyMemberID)
	}
}
