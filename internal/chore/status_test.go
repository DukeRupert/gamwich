package chore

import (
	"testing"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
)

func TestOneOffPending(t *testing.T) {
	c := model.Chore{ID: 1, Title: "Buy shelves", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	today := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)

	status, due := ComputeStatus(c, nil, today)
	if status != StatusPending {
		t.Errorf("status = %q, want %q", status, StatusPending)
	}
	if due != nil {
		t.Errorf("due = %v, want nil", due)
	}
}

func TestOneOffCompleted(t *testing.T) {
	c := model.Chore{ID: 1, Title: "Buy shelves", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	today := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC)

	status, _ := ComputeStatus(c, &completed, today)
	if status != StatusCompleted {
		t.Errorf("status = %q, want %q", status, StatusCompleted)
	}
}

func TestDailyPending(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "Wash dishes",
		RecurrenceRule: "FREQ=DAILY",
		CreatedAt:      time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC),
	}
	today := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)

	status, due := ComputeStatus(c, nil, today)
	if status != StatusPending {
		t.Errorf("status = %q, want %q", status, StatusPending)
	}
	if due == nil {
		t.Fatal("due should not be nil")
	}
	expected := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
	if !due.Equal(expected) {
		t.Errorf("due = %v, want %v", due, expected)
	}
}

func TestDailyCompleted(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "Wash dishes",
		RecurrenceRule: "FREQ=DAILY",
		CreatedAt:      time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC),
	}
	today := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 2, 5, 8, 0, 0, 0, time.UTC)

	status, _ := ComputeStatus(c, &completed, today)
	if status != StatusCompleted {
		t.Errorf("status = %q, want %q", status, StatusCompleted)
	}
}

func TestDailyOverdue(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "Wash dishes",
		RecurrenceRule: "FREQ=DAILY",
		CreatedAt:      time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC),
	}
	today := time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC)
	// Last completed 2 days ago
	completed := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC)

	status, due := ComputeStatus(c, &completed, today)
	// Due date is Feb 5 (today) but yesterday (Feb 4) was missed => current due is today, completed on Feb 3 < Feb 5
	if status != StatusPending {
		// The most recent due occurrence <= today is Feb 5; last completion was Feb 3 < Feb 5
		t.Errorf("status = %q, want %q", status, StatusPending)
	}
	if due == nil {
		t.Fatal("due should not be nil")
	}
}

func TestDailyOverduePreviousDay(t *testing.T) {
	// Test the true overdue case: weekly chore due on Monday, today is Tuesday, never completed.
	c2 := model.Chore{
		ID: 2, Title: "Weekly review",
		RecurrenceRule: "FREQ=WEEKLY",
		CreatedAt:      time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), // Monday Jan 5
	}
	// Today is Tuesday Feb 3 (day after Monday due date Feb 2)
	today := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC) // Tuesday

	status, due := ComputeStatus(c2, nil, today)
	if status != StatusOverdue {
		t.Errorf("status = %q, want %q", status, StatusOverdue)
	}
	if due == nil {
		t.Fatal("due should not be nil")
	}
	// Should be due on Monday Feb 2
	expected := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	if !due.Equal(expected) {
		t.Errorf("due = %v, want %v", due, expected)
	}
}

func TestWeeklyNotDue(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "Weekly clean",
		RecurrenceRule: "FREQ=WEEKLY",
		CreatedAt:      time.Date(2026, 2, 2, 9, 0, 0, 0, time.UTC), // Monday
	}
	// Today is Wednesday Feb 4 — next occurrence is Monday Feb 9
	today := time.Date(2026, 2, 4, 12, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 2, 2, 15, 0, 0, 0, time.UTC) // completed on due day

	status, _ := ComputeStatus(c, &completed, today)
	if status != StatusCompleted {
		t.Errorf("status = %q, want %q", status, StatusCompleted)
	}
}

func TestWeeklyDue(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "Weekly clean",
		RecurrenceRule: "FREQ=WEEKLY",
		CreatedAt:      time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), // Monday Jan 5
	}
	// Today is Monday Feb 2 — due today, no completion
	today := time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC)

	status, due := ComputeStatus(c, nil, today)
	if status != StatusPending {
		t.Errorf("status = %q, want %q", status, StatusPending)
	}
	if due == nil {
		t.Fatal("due should not be nil")
	}
}

func TestBiweekly(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "Biweekly",
		RecurrenceRule: "FREQ=WEEKLY;INTERVAL=2",
		CreatedAt:      time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), // Monday Jan 5
	}
	// Next occurrence after Jan 5: Jan 19, Feb 2, Feb 16...
	// Today is Jan 12 — no occurrence this week
	today := time.Date(2026, 1, 12, 12, 0, 0, 0, time.UTC)

	status, due := ComputeStatus(c, nil, today)
	// Most recent due date <= today is Jan 5
	if status != StatusOverdue {
		t.Errorf("status = %q, want %q", status, StatusOverdue)
	}
	if due == nil {
		t.Fatal("due should not be nil")
	}
	expected := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	if !due.Equal(expected) {
		t.Errorf("due = %v, want %v", due, expected)
	}
}

func TestMonthly(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "Monthly clean",
		RecurrenceRule: "FREQ=MONTHLY",
		CreatedAt:      time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
	}
	// Today is Feb 15 — due today, completed today
	today := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 2, 15, 8, 0, 0, 0, time.UTC)

	status, _ := ComputeStatus(c, &completed, today)
	if status != StatusCompleted {
		t.Errorf("status = %q, want %q", status, StatusCompleted)
	}
}

func TestInvalidRuleFallback(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "Bad rule",
		RecurrenceRule: "INVALID",
		CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	today := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)

	status, _ := ComputeStatus(c, nil, today)
	if status != StatusPending {
		t.Errorf("status = %q, want %q", status, StatusPending)
	}
}

func TestIsDueOnDate(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "Daily",
		RecurrenceRule: "FREQ=DAILY",
		CreatedAt:      time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC),
	}

	// Should be due on Feb 5
	if !IsDueOnDate(c, time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)) {
		t.Error("expected daily chore to be due on Feb 5")
	}

	// Should not be due before creation
	if IsDueOnDate(c, time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)) {
		t.Error("expected daily chore not to be due before creation")
	}
}

func TestIsDueOnDateOneOff(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "One-off",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	// One-off chores are always "due"
	if !IsDueOnDate(c, time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)) {
		t.Error("expected one-off chore to always be due")
	}
}

func TestIsDueOnDateWeekly(t *testing.T) {
	c := model.Chore{
		ID: 1, Title: "Weekly Monday",
		RecurrenceRule: "FREQ=WEEKLY",
		CreatedAt:      time.Date(2026, 2, 2, 9, 0, 0, 0, time.UTC), // Monday
	}

	// Due on Monday Feb 9
	if !IsDueOnDate(c, time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC)) {
		t.Error("expected weekly chore to be due on Monday")
	}

	// Not due on Tuesday Feb 10
	if IsDueOnDate(c, time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)) {
		t.Error("expected weekly chore not to be due on Tuesday")
	}
}
