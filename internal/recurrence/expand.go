package recurrence

import "time"

// Occurrence represents a single generated occurrence of a recurring event.
type Occurrence struct {
	Start time.Time
	End   time.Time
}

// Expand generates all occurrences of a recurring event within [rangeStart, rangeEnd).
// eventStart and eventEnd define the first occurrence's time span (used for duration).
func Expand(rule Rule, eventStart, eventEnd time.Time, rangeStart, rangeEnd time.Time) []Occurrence {
	duration := eventEnd.Sub(eventStart)
	var results []Occurrence
	count := 0

	iter := newIterator(rule, eventStart)
	for {
		occStart := iter.next()
		if occStart.IsZero() {
			break
		}

		// Stop conditions
		if rule.Until != nil && occStart.After(*rule.Until) {
			break
		}
		if occStart.After(rangeEnd) || occStart.Equal(rangeEnd) {
			break
		}

		count++
		if rule.Count > 0 && count > rule.Count {
			break
		}

		occEnd := occStart.Add(duration)

		// Include if occurrence overlaps with range: occStart < rangeEnd && occEnd > rangeStart
		if occStart.Before(rangeEnd) && occEnd.After(rangeStart) {
			results = append(results, Occurrence{Start: occStart, End: occEnd})
		}
	}

	return results
}

type iterator struct {
	rule       Rule
	baseStart  time.Time
	current    time.Time
	weekDayIdx int
	started    bool
	count      int
}

func newIterator(rule Rule, start time.Time) *iterator {
	return &iterator{
		rule:      rule,
		baseStart: start,
		current:   start,
	}
}

func (it *iterator) next() time.Time {
	// Safety limit to prevent infinite loops
	const maxIterations = 10000

	for it.count < maxIterations {
		it.count++
		t := it.advance()
		if t.IsZero() {
			return time.Time{}
		}
		return t
	}
	return time.Time{}
}

func (it *iterator) advance() time.Time {
	switch it.rule.Freq {
	case Daily:
		return it.advanceDaily()
	case Weekly:
		if len(it.rule.ByDay) > 0 {
			return it.advanceWeeklyByDay()
		}
		return it.advanceWeeklySimple()
	case Monthly:
		return it.advanceMonthly()
	case Yearly:
		return it.advanceYearly()
	}
	return time.Time{}
}

func (it *iterator) advanceDaily() time.Time {
	if !it.started {
		it.started = true
		return it.current
	}
	it.current = it.current.AddDate(0, 0, it.rule.Interval)
	return it.current
}

func (it *iterator) advanceWeeklySimple() time.Time {
	if !it.started {
		it.started = true
		return it.current
	}
	it.current = it.current.AddDate(0, 0, 7*it.rule.Interval)
	return it.current
}

func (it *iterator) advanceWeeklyByDay() time.Time {
	if !it.started {
		it.started = true
		// Find the week start (Monday at midnight, same time as baseStart)
		it.current = weekStart(it.baseStart)
		it.weekDayIdx = 0
		return it.findNextByDay()
	}

	it.weekDayIdx++
	if it.weekDayIdx >= len(it.rule.ByDay) {
		// Move to next week period
		it.current = it.current.AddDate(0, 0, 7*it.rule.Interval)
		it.current = weekStart(it.current)
		it.weekDayIdx = 0
	}
	return it.findNextByDay()
}

func (it *iterator) findNextByDay() time.Time {
	for it.weekDayIdx < len(it.rule.ByDay) {
		day := it.rule.ByDay[it.weekDayIdx]
		// Calculate the date for this weekday in the current week
		mondayOfWeek := it.current
		offset := int(day) - int(time.Monday)
		if offset < 0 {
			offset += 7 // Sunday
		}
		candidate := time.Date(
			mondayOfWeek.Year(), mondayOfWeek.Month(), mondayOfWeek.Day()+offset,
			it.baseStart.Hour(), it.baseStart.Minute(), it.baseStart.Second(), 0,
			it.baseStart.Location(),
		)

		// Skip dates before the event start
		if !candidate.Before(it.baseStart) {
			return candidate
		}
		it.weekDayIdx++
	}

	// All days in this week are before start; advance to next week
	it.current = it.current.AddDate(0, 0, 7*it.rule.Interval)
	it.current = weekStart(it.current)
	it.weekDayIdx = 0
	return it.findNextByDay()
}

func weekStart(t time.Time) time.Time {
	wd := t.Weekday()
	offset := int(wd) - int(time.Monday)
	if offset < 0 {
		offset += 7
	}
	monday := t.AddDate(0, 0, -offset)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
}

func (it *iterator) advanceMonthly() time.Time {
	if !it.started {
		it.started = true
		return it.current
	}

	day := it.rule.ByMonthDay
	if day == 0 {
		day = it.baseStart.Day()
	}

	// Advance by interval months
	next := it.current.AddDate(0, it.rule.Interval, 0)

	// Set to desired day of month, clamping to last day of month
	year, month, _ := next.Date()
	lastDay := daysInMonth(year, month)
	actualDay := day
	if actualDay > lastDay {
		// Skip months that don't have this day
		for {
			next = next.AddDate(0, it.rule.Interval, 0)
			year, month, _ = next.Date()
			lastDay = daysInMonth(year, month)
			if day <= lastDay {
				actualDay = day
				break
			}
		}
	}

	it.current = time.Date(
		year, month, actualDay,
		it.baseStart.Hour(), it.baseStart.Minute(), it.baseStart.Second(), 0,
		it.baseStart.Location(),
	)
	return it.current
}

func (it *iterator) advanceYearly() time.Time {
	if !it.started {
		it.started = true
		return it.current
	}

	next := it.current.AddDate(it.rule.Interval, 0, 0)
	// Handle Feb 29 → non-leap years: skip to next occurrence
	if it.baseStart.Month() == time.February && it.baseStart.Day() == 29 {
		for next.Day() != 29 {
			next = next.AddDate(it.rule.Interval, 0, 0)
		}
	}

	it.current = next
	return it.current
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
