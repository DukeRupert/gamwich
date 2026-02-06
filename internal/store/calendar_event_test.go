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
	if event.RecurrenceRule != "" {
		t.Errorf("recurrence_rule should be empty, got %q", event.RecurrenceRule)
	}
	if event.RecurrenceParentID != nil {
		t.Errorf("recurrence_parent_id should be nil")
	}
	if event.Cancelled {
		t.Error("cancelled should be false")
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

func TestListByDateRangeExcludesRecurring(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 5, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 5, 10, 0, 0, 0, time.UTC)

	// Create a normal event
	s.Create("Normal Event", "", start, end, false, nil, "")

	// Create a recurring event
	s.CreateWithRecurrence("Weekly Meeting", "", start, end, false, nil, "", "FREQ=WEEKLY")

	// ListByDateRange should only return the normal event
	rangeStart := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, 2, 6, 0, 0, 0, 0, time.UTC)
	events, err := s.ListByDateRange(rangeStart, rangeEnd)
	if err != nil {
		t.Fatalf("list by range: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (should exclude recurring)", len(events))
	}
	if events[0].Title != "Normal Event" {
		t.Errorf("event = %q, want %q", events[0].Title, "Normal Event")
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

// --- Recurrence-specific tests ---

func TestCreateWithRecurrence(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC) // Tuesday
	end := time.Date(2026, 2, 3, 11, 0, 0, 0, time.UTC)

	event, err := s.CreateWithRecurrence("Soccer Practice", "", start, end, false, nil, "Field", "FREQ=WEEKLY;BYDAY=TU,TH")
	if err != nil {
		t.Fatalf("create recurring event: %v", err)
	}
	if event.RecurrenceRule != "FREQ=WEEKLY;BYDAY=TU,TH" {
		t.Errorf("recurrence_rule = %q, want %q", event.RecurrenceRule, "FREQ=WEEKLY;BYDAY=TU,TH")
	}
	if event.RecurrenceParentID != nil {
		t.Error("parent event should have nil recurrence_parent_id")
	}
}

func TestListRecurring(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 3, 11, 0, 0, 0, time.UTC)

	// Create a recurring event
	s.CreateWithRecurrence("Weekly Meeting", "", start, end, false, nil, "", "FREQ=WEEKLY")

	// Create a normal event
	s.Create("Normal Event", "", start, end, false, nil, "")

	// ListRecurring should only return the recurring one
	events, err := s.ListRecurring(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("list recurring: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Title != "Weekly Meeting" {
		t.Errorf("event = %q, want %q", events[0].Title, "Weekly Meeting")
	}
}

func TestListRecurringBeforeFilter(t *testing.T) {
	s := setupTestDB(t)

	// Create a recurring event starting Feb 10
	start := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 10, 11, 0, 0, 0, time.UTC)
	s.CreateWithRecurrence("Future Meeting", "", start, end, false, nil, "", "FREQ=WEEKLY")

	// ListRecurring with before=Feb 5 should return nothing
	events, err := s.ListRecurring(time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("list recurring: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("got %d events, want 0 (event starts after query date)", len(events))
	}
}

func TestCreateAndListExceptions(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC) // Tuesday
	end := time.Date(2026, 2, 3, 11, 0, 0, 0, time.UTC)

	parent, err := s.CreateWithRecurrence("Weekly Meeting", "", start, end, false, nil, "", "FREQ=WEEKLY")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Create a modified exception for Feb 10
	origStart := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	newStart := time.Date(2026, 2, 10, 14, 0, 0, 0, time.UTC)
	newEnd := time.Date(2026, 2, 10, 15, 0, 0, 0, time.UTC)

	exc, err := s.CreateException(parent.ID, origStart, "Weekly Meeting (moved)", "", newStart, newEnd, false, nil, "", false)
	if err != nil {
		t.Fatalf("create exception: %v", err)
	}
	if exc.RecurrenceParentID == nil || *exc.RecurrenceParentID != parent.ID {
		t.Error("exception should reference parent")
	}
	if exc.OriginalStartTime == nil {
		t.Fatal("original_start_time should be set")
	}
	if !exc.OriginalStartTime.Equal(origStart) {
		t.Errorf("original_start_time = %v, want %v", exc.OriginalStartTime, origStart)
	}
	if exc.Cancelled {
		t.Error("exception should not be cancelled")
	}

	// Create a cancelled exception for Feb 17
	origStart2 := time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC)
	_, err = s.CreateException(parent.ID, origStart2, "Weekly Meeting", "", origStart2, origStart2.Add(time.Hour), false, nil, "", true)
	if err != nil {
		t.Fatalf("create cancelled exception: %v", err)
	}

	// List exceptions
	exceptions, err := s.ListExceptions(parent.ID)
	if err != nil {
		t.Fatalf("list exceptions: %v", err)
	}
	if len(exceptions) != 2 {
		t.Fatalf("got %d exceptions, want 2", len(exceptions))
	}
	if !exceptions[1].Cancelled {
		t.Error("second exception should be cancelled")
	}
}

func TestDeleteExceptions(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 3, 11, 0, 0, 0, time.UTC)

	parent, _ := s.CreateWithRecurrence("Weekly", "", start, end, false, nil, "", "FREQ=WEEKLY")

	origStart := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	s.CreateException(parent.ID, origStart, "Modified", "", origStart, origStart.Add(time.Hour), false, nil, "", false)

	err := s.DeleteExceptions(parent.ID)
	if err != nil {
		t.Fatalf("delete exceptions: %v", err)
	}

	exceptions, _ := s.ListExceptions(parent.ID)
	if len(exceptions) != 0 {
		t.Errorf("got %d exceptions after delete, want 0", len(exceptions))
	}
}

func TestDeleteParentCascadesExceptions(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 3, 11, 0, 0, 0, time.UTC)

	parent, _ := s.CreateWithRecurrence("Weekly", "", start, end, false, nil, "", "FREQ=WEEKLY")

	origStart := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	exc, _ := s.CreateException(parent.ID, origStart, "Modified", "", origStart, origStart.Add(time.Hour), false, nil, "", false)

	// Delete parent — exceptions should cascade
	if err := s.Delete(parent.ID); err != nil {
		t.Fatalf("delete parent: %v", err)
	}

	got, _ := s.GetByID(exc.ID)
	if got != nil {
		t.Error("exception should be deleted via CASCADE")
	}
}

func TestUpdateWithRecurrence(t *testing.T) {
	s := setupTestDB(t)

	start := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 3, 11, 0, 0, 0, time.UTC)

	event, _ := s.CreateWithRecurrence("Weekly Meeting", "", start, end, false, nil, "", "FREQ=WEEKLY")

	updated, err := s.UpdateWithRecurrence(event.ID, "Biweekly Meeting", "", start, end, false, nil, "", "FREQ=WEEKLY;INTERVAL=2")
	if err != nil {
		t.Fatalf("update with recurrence: %v", err)
	}
	if updated.RecurrenceRule != "FREQ=WEEKLY;INTERVAL=2" {
		t.Errorf("recurrence_rule = %q, want %q", updated.RecurrenceRule, "FREQ=WEEKLY;INTERVAL=2")
	}
}
