// Package timeutil provides time formatting helpers that respect local timezone.
package timeutil

import (
	"fmt"
	"time"
)

// FormatDate formats a time.Time as "2006-01-02" in local timezone.
func FormatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("2006-01-02")
}

// FormatDateTime formats a time.Time as "Jan 2, 2006, 3:04:05 PM MST" in local timezone.
func FormatDateTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("Jan 2, 2006, 3:04:05 PM MST")
}

// FormatTimestamp parses an RFC3339 timestamp string and formats it
// as "Jan 2, 2006, 3:04:05 PM MST" in local timezone.
func FormatTimestamp(ts string) string {
	if ts == "" {
		return "—"
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return FormatDateTime(t)
}

// FormatTimestampDate parses an RFC3339 timestamp string and formats it
// as "2006-01-02" in local timezone.
func FormatTimestampDate(ts string) string {
	if ts == "" {
		return "—"
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return FormatDate(t)
}

// FormatTimestampWithAge renders the absolute timestamp with the relative
// age appended in parentheses — e.g. "May 16, 2026, 4:31:17 PM EEST (3 days 2 hours)".
// Useful for "Created" / "Last started" fields where users want both the
// exact moment and a quick sense of how long ago that was.
func FormatTimestampWithAge(ts string) string {
	if ts == "" {
		return "—"
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return fmt.Sprintf("%s (%s)", FormatDateTime(t), HumanAge(time.Since(t)))
}

// HumanAge formats a duration as a coarse age string — the two largest
// non-zero units. Examples: "3 days 2 hours", "5 hours 12 minutes",
// "42 seconds", "just now". Negative durations (future timestamps) are
// rendered as "in <age>".
func HumanAge(d time.Duration) string {
	if d < 0 {
		return "in " + HumanAge(-d)
	}
	const (
		day  = 24 * time.Hour
		hour = time.Hour
		min  = time.Minute
	)
	switch {
	case d >= day:
		days := int(d / day)
		hours := int((d % day) / hour)
		if hours == 0 {
			return fmt.Sprintf("%d %s", days, plural(days, "day", "days"))
		}
		return fmt.Sprintf("%d %s %d %s",
			days, plural(days, "day", "days"),
			hours, plural(hours, "hour", "hours"))
	case d >= hour:
		hours := int(d / hour)
		mins := int((d % hour) / min)
		if mins == 0 {
			return fmt.Sprintf("%d %s", hours, plural(hours, "hour", "hours"))
		}
		return fmt.Sprintf("%d %s %d %s",
			hours, plural(hours, "hour", "hours"),
			mins, plural(mins, "minute", "minutes"))
	case d >= min:
		mins := int(d / min)
		return fmt.Sprintf("%d %s", mins, plural(mins, "minute", "minutes"))
	case d >= time.Second:
		secs := int(d / time.Second)
		return fmt.Sprintf("%d %s", secs, plural(secs, "second", "seconds"))
	default:
		return "just now"
	}
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}
