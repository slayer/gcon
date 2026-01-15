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

// CalculateUptime calculates the duration from startTimeISO to now and formats it
// as a human-readable string like "2d 5h 30m", "5h 30m", "30m", or "< 1m".
func CalculateUptime(startTimeISO string) string {
	if startTimeISO == "" {
		return "—"
	}

	startTime, err := time.Parse(time.RFC3339, startTimeISO)
	if err != nil {
		return "—"
	}

	duration := time.Since(startTime)

	// Less than a minute
	if duration < time.Minute {
		return "< 1m"
	}

	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	// Format based on duration length
	if days > 0 {
		return formatDuration(days, hours, minutes, true)
	}
	if hours > 0 {
		return formatDuration(0, hours, minutes, false)
	}
	return formatDuration(0, 0, minutes, false)
}

// formatDuration formats duration components into a string
func formatDuration(days, hours, minutes int, includeDays bool) string {
	var parts []string

	if includeDays && days > 0 {
		parts = append(parts, formatUnit(days, "d"))
	}
	if hours > 0 {
		parts = append(parts, formatUnit(hours, "h"))
	}
	if minutes > 0 {
		parts = append(parts, formatUnit(minutes, "m"))
	}

	if len(parts) == 0 {
		return "< 1m"
	}

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += " "
		}
		result += part
	}
	return result
}

// formatUnit formats a time unit (e.g., 5, "h" -> "5h")
func formatUnit(value int, unit string) string {
	return fmt.Sprintf("%d%s", value, unit)
}
