package recurrence

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Freq int

const (
	Daily Freq = iota
	Weekly
	Monthly
	Yearly
)

var freqNames = map[Freq]string{
	Daily:   "DAILY",
	Weekly:  "WEEKLY",
	Monthly: "MONTHLY",
	Yearly:  "YEARLY",
}

var freqFromName = map[string]Freq{
	"DAILY":   Daily,
	"WEEKLY":  Weekly,
	"MONTHLY": Monthly,
	"YEARLY":  Yearly,
}

var dayNames = map[string]time.Weekday{
	"SU": time.Sunday,
	"MO": time.Monday,
	"TU": time.Tuesday,
	"WE": time.Wednesday,
	"TH": time.Thursday,
	"FR": time.Friday,
	"SA": time.Saturday,
}

var dayAbbrev = map[time.Weekday]string{
	time.Sunday:    "SU",
	time.Monday:    "MO",
	time.Tuesday:   "TU",
	time.Wednesday: "WE",
	time.Thursday:  "TH",
	time.Friday:    "FR",
	time.Saturday:  "SA",
}

type Rule struct {
	Freq       Freq
	Interval   int            // default 1; 2 = biweekly when Freq=Weekly
	ByDay      []time.Weekday // for WEEKLY: which days (empty = same weekday as start)
	ByMonthDay int            // for MONTHLY: day of month (0 = same as start)
	Count      int            // max occurrences (0 = unlimited)
	Until      *time.Time     // stop after this date (nil = no limit)
}

// Parse parses an RRULE string like "FREQ=WEEKLY;BYDAY=MO,WE;INTERVAL=2".
func Parse(rule string) (Rule, error) {
	if rule == "" {
		return Rule{}, fmt.Errorf("empty rule")
	}

	r := Rule{Interval: 1}
	var hasFreq bool

	parts := strings.Split(rule, ";")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return Rule{}, fmt.Errorf("invalid rule part: %q", part)
		}
		key, val := kv[0], kv[1]

		switch key {
		case "FREQ":
			f, ok := freqFromName[val]
			if !ok {
				return Rule{}, fmt.Errorf("unknown frequency: %q", val)
			}
			r.Freq = f
			hasFreq = true

		case "INTERVAL":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return Rule{}, fmt.Errorf("invalid interval: %q", val)
			}
			r.Interval = n

		case "BYDAY":
			days := strings.Split(val, ",")
			for _, d := range days {
				wd, ok := dayNames[strings.TrimSpace(d)]
				if !ok {
					return Rule{}, fmt.Errorf("unknown day: %q", d)
				}
				r.ByDay = append(r.ByDay, wd)
			}

		case "BYMONTHDAY":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 31 {
				return Rule{}, fmt.Errorf("invalid BYMONTHDAY: %q", val)
			}
			r.ByMonthDay = n

		case "COUNT":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return Rule{}, fmt.Errorf("invalid count: %q", val)
			}
			r.Count = n

		case "UNTIL":
			t, err := time.Parse("20060102T150405Z", val)
			if err != nil {
				t, err = time.Parse("20060102", val)
				if err != nil {
					return Rule{}, fmt.Errorf("invalid UNTIL: %q", val)
				}
			}
			r.Until = &t

		default:
			return Rule{}, fmt.Errorf("unsupported rule key: %q", key)
		}
	}

	if !hasFreq {
		return Rule{}, fmt.Errorf("FREQ is required")
	}

	return r, nil
}

// String serializes the rule back to an RRULE string.
func (r Rule) String() string {
	var parts []string
	parts = append(parts, "FREQ="+freqNames[r.Freq])

	if r.Interval > 1 {
		parts = append(parts, fmt.Sprintf("INTERVAL=%d", r.Interval))
	}

	if len(r.ByDay) > 0 {
		var days []string
		for _, d := range r.ByDay {
			days = append(days, dayAbbrev[d])
		}
		parts = append(parts, "BYDAY="+strings.Join(days, ","))
	}

	if r.ByMonthDay > 0 {
		parts = append(parts, fmt.Sprintf("BYMONTHDAY=%d", r.ByMonthDay))
	}

	if r.Count > 0 {
		parts = append(parts, fmt.Sprintf("COUNT=%d", r.Count))
	}

	if r.Until != nil {
		parts = append(parts, "UNTIL="+r.Until.Format("20060102T150405Z"))
	}

	return strings.Join(parts, ";")
}

// Describe returns a human-readable description of the rule.
func (r Rule) Describe() string {
	switch r.Freq {
	case Daily:
		if r.Interval > 1 {
			return fmt.Sprintf("Repeats every %d days", r.Interval)
		}
		return "Repeats daily"
	case Weekly:
		prefix := "Repeats weekly"
		if r.Interval == 2 {
			prefix = "Repeats every 2 weeks"
		} else if r.Interval > 2 {
			prefix = fmt.Sprintf("Repeats every %d weeks", r.Interval)
		}
		if len(r.ByDay) > 0 {
			var names []string
			for _, d := range r.ByDay {
				names = append(names, d.String()[:3])
			}
			return prefix + " on " + strings.Join(names, ", ")
		}
		return prefix
	case Monthly:
		if r.Interval > 1 {
			return fmt.Sprintf("Repeats every %d months", r.Interval)
		}
		return "Repeats monthly"
	case Yearly:
		if r.Interval > 1 {
			return fmt.Sprintf("Repeats every %d years", r.Interval)
		}
		return "Repeats yearly"
	}
	return ""
}
