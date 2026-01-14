// Package timeutil provides time formatting helpers that respect local timezone.
package timeutil

import (
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
