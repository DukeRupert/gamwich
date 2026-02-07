package chore

import (
	"log/slog"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/recurrence"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusOverdue   Status = "overdue"
	StatusNotDue    Status = "not_due"
)

type ChoreWithStatus struct {
	model.Chore
	Status         Status
	DueDate        *time.Time
	LastCompletion *time.Time
	AreaName       string
	MemberName     string
	MemberColor    string
	MemberEmoji    string
}

// ComputeStatus determines the status and due date for a chore given its last completion.
func ComputeStatus(chore model.Chore, lastCompletion *time.Time, today time.Time) (Status, *time.Time) {
	today = startOfDay(today)

	// One-off chore (no recurrence rule)
	if chore.RecurrenceRule == "" {
		if lastCompletion != nil {
			return StatusCompleted, nil
		}
		return StatusPending, nil
	}

	// Recurring chore — expand to find the current due date
	rule, err := recurrence.Parse(chore.RecurrenceRule)
	if err != nil {
		slog.Error("invalid recurrence rule", "chore_id", chore.ID, "rule", chore.RecurrenceRule, "error", err)
		// Fall back: treat as one-off
		if lastCompletion != nil {
			return StatusCompleted, nil
		}
		return StatusPending, nil
	}

	// Expand from creation through end of tomorrow to find all relevant due dates
	tomorrow := today.Add(48 * time.Hour)
	// Use a 1-hour event duration for expansion purposes
	eventEnd := chore.CreatedAt.Add(time.Hour)
	occurrences := recurrence.Expand(rule, chore.CreatedAt, eventEnd, chore.CreatedAt, tomorrow)

	if len(occurrences) == 0 {
		return StatusNotDue, nil
	}

	// Find the most recent due date that is <= today (end of today)
	endOfToday := today.Add(24 * time.Hour)
	var currentDue *time.Time
	for i := len(occurrences) - 1; i >= 0; i-- {
		occDate := startOfDay(occurrences[i].Start)
		if occDate.Before(endOfToday) {
			currentDue = &occDate
			break
		}
	}

	if currentDue == nil {
		// All occurrences are in the future
		return StatusNotDue, nil
	}

	// Check if completed since the current due date
	if lastCompletion != nil && !startOfDay(*lastCompletion).Before(*currentDue) {
		return StatusCompleted, currentDue
	}

	// Check if overdue: due date is before today
	if currentDue.Before(today) {
		return StatusOverdue, currentDue
	}

	return StatusPending, currentDue
}

// IsDueOnDate checks if a chore has a due occurrence on the given date.
func IsDueOnDate(chore model.Chore, date time.Time) bool {
	if chore.RecurrenceRule == "" {
		// One-off chores are always "due" until completed
		return true
	}

	rule, err := recurrence.Parse(chore.RecurrenceRule)
	if err != nil {
		return false
	}

	dayStart := startOfDay(date)
	dayEnd := dayStart.Add(24 * time.Hour)
	eventEnd := chore.CreatedAt.Add(time.Hour)
	occurrences := recurrence.Expand(rule, chore.CreatedAt, eventEnd, dayStart, dayEnd)
	return len(occurrences) > 0
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
