package recurrence

import (
	"testing"
	"time"
)

func TestParseFreqOnly(t *testing.T) {
	tests := []struct {
		input string
		freq  Freq
	}{
		{"FREQ=DAILY", Daily},
		{"FREQ=WEEKLY", Weekly},
		{"FREQ=MONTHLY", Monthly},
		{"FREQ=YEARLY", Yearly},
	}

	for _, tt := range tests {
		r, err := Parse(tt.input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.input, err)
			continue
		}
		if r.Freq != tt.freq {
			t.Errorf("Parse(%q).Freq = %d, want %d", tt.input, r.Freq, tt.freq)
		}
		if r.Interval != 1 {
			t.Errorf("Parse(%q).Interval = %d, want 1", tt.input, r.Interval)
		}
	}
}

func TestParseWithInterval(t *testing.T) {
	r, err := Parse("FREQ=WEEKLY;INTERVAL=2")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if r.Freq != Weekly || r.Interval != 2 {
		t.Errorf("got Freq=%d Interval=%d, want Weekly Interval=2", r.Freq, r.Interval)
	}
}

func TestParseWithByDay(t *testing.T) {
	r, err := Parse("FREQ=WEEKLY;BYDAY=MO,WE,FR")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(r.ByDay) != 3 {
		t.Fatalf("ByDay len = %d, want 3", len(r.ByDay))
	}
	want := []time.Weekday{time.Monday, time.Wednesday, time.Friday}
	for i, d := range r.ByDay {
		if d != want[i] {
			t.Errorf("ByDay[%d] = %v, want %v", i, d, want[i])
		}
	}
}

func TestParseWithCount(t *testing.T) {
	r, err := Parse("FREQ=DAILY;COUNT=5")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if r.Count != 5 {
		t.Errorf("Count = %d, want 5", r.Count)
	}
}

func TestParseWithUntil(t *testing.T) {
	r, err := Parse("FREQ=WEEKLY;UNTIL=20260301T000000Z")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if r.Until == nil {
		t.Fatal("Until should not be nil")
	}
	want := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !r.Until.Equal(want) {
		t.Errorf("Until = %v, want %v", r.Until, want)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []string{
		"",
		"BYDAY=MO", // no FREQ
		"FREQ=HOURLY",
		"FREQ=WEEKLY;INTERVAL=0",
		"FREQ=WEEKLY;BYDAY=XX",
		"FREQ=DAILY;COUNT=0",
		"FREQ=DAILY;UNKNOWN=1",
	}

	for _, input := range tests {
		_, err := Parse(input)
		if err == nil {
			t.Errorf("Parse(%q) should error", input)
		}
	}
}

func TestRuleString(t *testing.T) {
	r := Rule{Freq: Weekly, Interval: 2, ByDay: []time.Weekday{time.Monday, time.Wednesday}}
	got := r.String()
	want := "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestRuleStringRoundTrip(t *testing.T) {
	inputs := []string{
		"FREQ=DAILY",
		"FREQ=WEEKLY",
		"FREQ=WEEKLY;INTERVAL=2",
		"FREQ=WEEKLY;BYDAY=MO,WE,FR",
		"FREQ=MONTHLY",
		"FREQ=YEARLY",
		"FREQ=DAILY;COUNT=5",
	}

	for _, input := range inputs {
		r, err := Parse(input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", input, err)
			continue
		}
		got := r.String()
		if got != input {
			t.Errorf("roundtrip %q -> %q", input, got)
		}
	}
}

func TestDescribe(t *testing.T) {
	tests := []struct {
		rule string
		want string
	}{
		{"FREQ=DAILY", "Repeats daily"},
		{"FREQ=WEEKLY", "Repeats weekly"},
		{"FREQ=WEEKLY;INTERVAL=2", "Repeats every 2 weeks"},
		{"FREQ=WEEKLY;BYDAY=MO,WE,FR", "Repeats weekly on Mon, Wed, Fri"},
		{"FREQ=MONTHLY", "Repeats monthly"},
		{"FREQ=YEARLY", "Repeats yearly"},
	}

	for _, tt := range tests {
		r, _ := Parse(tt.rule)
		got := r.Describe()
		if got != tt.want {
			t.Errorf("Describe(%q) = %q, want %q", tt.rule, got, tt.want)
		}
	}
}

// --- Expand tests ---

func d(year int, month time.Month, day, hour int) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.UTC)
}

func TestExpandDaily(t *testing.T) {
	rule, _ := Parse("FREQ=DAILY")
	start := d(2026, 2, 1, 10)
	end := d(2026, 2, 1, 11)
	rangeStart := d(2026, 2, 1, 0)
	rangeEnd := d(2026, 2, 5, 0)

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	if len(occs) != 4 {
		t.Fatalf("got %d occurrences, want 4", len(occs))
	}
	for i, occ := range occs {
		wantStart := d(2026, 2, 1+i, 10)
		if !occ.Start.Equal(wantStart) {
			t.Errorf("occ[%d].Start = %v, want %v", i, occ.Start, wantStart)
		}
	}
}

func TestExpandWeeklyTuesday(t *testing.T) {
	// "Every Tuesday" starting Feb 3, 2026 (a Tuesday)
	rule, _ := Parse("FREQ=WEEKLY")
	start := d(2026, 2, 3, 10) // Tuesday
	end := d(2026, 2, 3, 11)
	rangeStart := d(2026, 2, 1, 0)
	rangeEnd := d(2026, 3, 1, 0) // 1 month

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	if len(occs) != 4 {
		t.Fatalf("got %d occurrences, want 4 (Feb 3, 10, 17, 24)", len(occs))
	}

	expected := []int{3, 10, 17, 24}
	for i, occ := range occs {
		if occ.Start.Day() != expected[i] {
			t.Errorf("occ[%d] day = %d, want %d", i, occ.Start.Day(), expected[i])
		}
	}
}

func TestExpandBiweekly(t *testing.T) {
	rule, _ := Parse("FREQ=WEEKLY;INTERVAL=2")
	start := d(2026, 2, 3, 10) // Tuesday
	end := d(2026, 2, 3, 11)
	rangeStart := d(2026, 2, 1, 0)
	rangeEnd := d(2026, 3, 15, 0) // ~6 weeks

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	if len(occs) != 3 {
		t.Fatalf("got %d occurrences, want 3 (Feb 3, 17, Mar 3)", len(occs))
	}

	// Biweekly: Feb 3, Feb 17, Mar 3
	if occs[0].Start.Day() != 3 || occs[0].Start.Month() != time.February {
		t.Errorf("occ[0] = %v", occs[0].Start)
	}
	if occs[1].Start.Day() != 17 || occs[1].Start.Month() != time.February {
		t.Errorf("occ[1] = %v", occs[1].Start)
	}
	if occs[2].Start.Day() != 3 || occs[2].Start.Month() != time.March {
		t.Errorf("occ[2] = %v", occs[2].Start)
	}
}

func TestExpandWeeklyByDay(t *testing.T) {
	rule, _ := Parse("FREQ=WEEKLY;BYDAY=TU,TH")
	start := d(2026, 2, 3, 16) // Tuesday at 4pm
	end := d(2026, 2, 3, 17)
	rangeStart := d(2026, 2, 1, 0)
	rangeEnd := d(2026, 2, 15, 0) // 2 weeks

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	// Week of Feb 2: Tue Feb 3, Thu Feb 5
	// Week of Feb 9: Tue Feb 10, Thu Feb 12
	if len(occs) != 4 {
		t.Fatalf("got %d occurrences, want 4", len(occs))
	}

	expected := []int{3, 5, 10, 12}
	for i, occ := range occs {
		if occ.Start.Day() != expected[i] {
			t.Errorf("occ[%d] day = %d, want %d", i, occ.Start.Day(), expected[i])
		}
		if occ.Start.Hour() != 16 {
			t.Errorf("occ[%d] hour = %d, want 16", i, occ.Start.Hour())
		}
	}
}

func TestExpandMonthly(t *testing.T) {
	rule, _ := Parse("FREQ=MONTHLY")
	start := d(2026, 1, 15, 10)
	end := d(2026, 1, 15, 11)
	rangeStart := d(2026, 1, 1, 0)
	rangeEnd := d(2026, 4, 1, 0)

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	if len(occs) != 3 {
		t.Fatalf("got %d occurrences, want 3 (Jan 15, Feb 15, Mar 15)", len(occs))
	}

	for i, occ := range occs {
		if occ.Start.Day() != 15 {
			t.Errorf("occ[%d] day = %d, want 15", i, occ.Start.Day())
		}
	}
}

func TestExpandMonthly31st(t *testing.T) {
	// Monthly on the 31st — should skip months without 31 days
	rule, _ := Parse("FREQ=MONTHLY")
	start := d(2026, 1, 31, 10)
	end := d(2026, 1, 31, 11)
	rangeStart := d(2026, 1, 1, 0)
	rangeEnd := d(2026, 8, 1, 0) // 7 months

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	// Jan 31, Mar 31, May 31, Jul 31
	if len(occs) != 4 {
		t.Fatalf("got %d occurrences, want 4 (Jan 31, Mar 31, May 31, Jul 31)", len(occs))
	}

	expected := []time.Month{time.January, time.March, time.May, time.July}
	for i, occ := range occs {
		if occ.Start.Month() != expected[i] || occ.Start.Day() != 31 {
			t.Errorf("occ[%d] = %v, want %v 31", i, occ.Start, expected[i])
		}
	}
}

func TestExpandYearly(t *testing.T) {
	rule, _ := Parse("FREQ=YEARLY")
	start := d(2026, 6, 15, 0)
	end := d(2026, 6, 16, 0) // all-day birthday
	rangeStart := d(2026, 1, 1, 0)
	rangeEnd := d(2030, 1, 1, 0)

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	if len(occs) != 4 {
		t.Fatalf("got %d occurrences, want 4 (2026-2029)", len(occs))
	}
	for i, occ := range occs {
		if occ.Start.Year() != 2026+i {
			t.Errorf("occ[%d] year = %d, want %d", i, occ.Start.Year(), 2026+i)
		}
	}
}

func TestExpandCount(t *testing.T) {
	rule, _ := Parse("FREQ=DAILY;COUNT=5")
	start := d(2026, 2, 1, 10)
	end := d(2026, 2, 1, 11)
	rangeStart := d(2026, 1, 1, 0)
	rangeEnd := d(2027, 1, 1, 0) // very wide range

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	if len(occs) != 5 {
		t.Fatalf("got %d occurrences, want 5 (COUNT=5)", len(occs))
	}
}

func TestExpandUntil(t *testing.T) {
	until := d(2026, 2, 10, 0)
	rule := Rule{Freq: Daily, Interval: 1, Until: &until}
	start := d(2026, 2, 1, 10)
	end := d(2026, 2, 1, 11)
	rangeStart := d(2026, 1, 1, 0)
	rangeEnd := d(2027, 1, 1, 0)

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	if len(occs) != 9 {
		t.Fatalf("got %d occurrences, want 9 (Feb 1-9)", len(occs))
	}
	// Last occurrence should be Feb 9 (10am on Feb 10 is after UNTIL of midnight Feb 10)
	last := occs[len(occs)-1]
	if last.Start.Day() != 9 {
		t.Errorf("last occurrence day = %d, want 9", last.Start.Day())
	}
}

func TestExpandRangeFiltering(t *testing.T) {
	// Daily event starting Jan 1, but we only query Feb 5-10
	rule, _ := Parse("FREQ=DAILY")
	start := d(2026, 1, 1, 10)
	end := d(2026, 1, 1, 11)
	rangeStart := d(2026, 2, 5, 0)
	rangeEnd := d(2026, 2, 10, 0)

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	if len(occs) != 5 {
		t.Fatalf("got %d occurrences, want 5 (Feb 5-9)", len(occs))
	}
	if occs[0].Start.Day() != 5 {
		t.Errorf("first occurrence day = %d, want 5", occs[0].Start.Day())
	}
}

func TestExpandMultipleDaysPerWeek(t *testing.T) {
	rule, _ := Parse("FREQ=WEEKLY;BYDAY=MO,WE,FR")
	start := d(2026, 2, 2, 10) // Monday
	end := d(2026, 2, 2, 11)
	rangeStart := d(2026, 2, 2, 0)
	rangeEnd := d(2026, 2, 9, 0) // 1 week

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	// Feb 2 (Mon), Feb 4 (Wed), Feb 6 (Fri)
	if len(occs) != 3 {
		t.Fatalf("got %d occurrences, want 3", len(occs))
	}

	expected := []int{2, 4, 6}
	for i, occ := range occs {
		if occ.Start.Day() != expected[i] {
			t.Errorf("occ[%d] day = %d, want %d", i, occ.Start.Day(), expected[i])
		}
	}
}

func TestExpandPreservesDuration(t *testing.T) {
	rule, _ := Parse("FREQ=DAILY")
	start := d(2026, 2, 1, 10)
	end := d(2026, 2, 1, 12) // 2 hour event
	rangeStart := d(2026, 2, 1, 0)
	rangeEnd := d(2026, 2, 3, 0)

	occs := Expand(rule, start, end, rangeStart, rangeEnd)
	for i, occ := range occs {
		dur := occ.End.Sub(occ.Start)
		if dur != 2*time.Hour {
			t.Errorf("occ[%d] duration = %v, want 2h", i, dur)
		}
	}
}
