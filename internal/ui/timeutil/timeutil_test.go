package timeutil

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time returns dash",
			input:    time.Time{},
			expected: "—",
		},
		{
			name:     "valid time formats correctly",
			input:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			expected: "2024-01-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDateTime(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time returns dash",
			input:    time.Time{},
			expected: "—",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDateTime(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test that valid time includes timezone
	t.Run("valid time includes timezone", func(t *testing.T) {
		input := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		result := FormatDateTime(input)
		assert.Contains(t, result, "2024")
		assert.Contains(t, result, "Jan")
		assert.Contains(t, result, "15")
	})
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string returns dash",
			input:    "",
			expected: "—",
		},
		{
			name:     "invalid timestamp returns original",
			input:    "not-a-timestamp",
			expected: "not-a-timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimestamp(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test valid RFC3339 timestamp
	t.Run("valid RFC3339 timestamp", func(t *testing.T) {
		input := "2024-01-15T10:30:00Z"
		result := FormatTimestamp(input)
		assert.Contains(t, result, "Jan")
		assert.Contains(t, result, "15")
		assert.Contains(t, result, "2024")
	})
}

func TestFormatTimestampDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string returns dash",
			input:    "",
			expected: "—",
		},
		{
			name:     "invalid timestamp returns original",
			input:    "not-a-timestamp",
			expected: "not-a-timestamp",
		},
		{
			name:     "valid RFC3339 timestamp",
			input:    "2024-01-15T10:30:00Z",
			expected: "2024-01-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimestampDate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTimezoneRespected(t *testing.T) {
	// Save current TZ and restore after test
	originalTZ := os.Getenv("TZ")
	defer func() { _ = os.Setenv("TZ", originalTZ) }() //nolint:errcheck // Test cleanup

	// Set timezone to PST
	_ = os.Setenv("TZ", "America/Los_Angeles") //nolint:errcheck // Test setup
	time.Local = mustLoadLocation("America/Los_Angeles")

	// UTC time: Jan 15, 2024 at 20:00 UTC = Jan 15, 2024 at 12:00 PST
	input := time.Date(2024, 1, 15, 20, 0, 0, 0, time.UTC)
	result := FormatDateTime(input)

	// Should show PST time (12:00 PM)
	assert.Contains(t, result, "12:00:00 PM")
	assert.Contains(t, result, "PST")

	// Now switch to EST
	_ = os.Setenv("TZ", "America/New_York") //nolint:errcheck // Test setup
	time.Local = mustLoadLocation("America/New_York")

	result = FormatDateTime(input)
	// UTC 20:00 = EST 15:00
	assert.Contains(t, result, "3:00:00 PM")
	assert.Contains(t, result, "EST")
}

func TestHumanAge(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{"sub-second", 500 * time.Millisecond, "just now"},
		{"one second", time.Second, "1 second"},
		{"42 seconds", 42 * time.Second, "42 seconds"},
		{"one minute", time.Minute, "1 minute"},
		{"5 minutes", 5 * time.Minute, "5 minutes"},
		{"one hour exactly", time.Hour, "1 hour"},
		{"1 hour 1 minute", time.Hour + time.Minute, "1 hour 1 minute"},
		{"5 hours 12 minutes", 5*time.Hour + 12*time.Minute, "5 hours 12 minutes"},
		{"one day exactly", 24 * time.Hour, "1 day"},
		{"1 day 1 hour", 24*time.Hour + time.Hour, "1 day 1 hour"},
		{"3 days 2 hours", 3*24*time.Hour + 2*time.Hour, "3 days 2 hours"},
		{"7 days flat", 7 * 24 * time.Hour, "7 days"},
		{"future", -time.Hour - 30*time.Minute, "in 1 hour 30 minutes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HumanAge(tt.input))
		})
	}
}

func TestFormatTimestampWithAge(t *testing.T) {
	t.Run("empty returns dash", func(t *testing.T) {
		assert.Equal(t, "—", FormatTimestampWithAge(""))
	})
	t.Run("invalid returns original", func(t *testing.T) {
		assert.Equal(t, "garbage", FormatTimestampWithAge("garbage"))
	})
	t.Run("valid timestamp includes both absolute and relative", func(t *testing.T) {
		// Use a timestamp ~2 days in the past relative to "now".
		ts := time.Now().Add(-(2*24*time.Hour + 3*time.Hour)).UTC().Format(time.RFC3339)
		got := FormatTimestampWithAge(ts)
		assert.Contains(t, got, "2 days 3 hours",
			"output must include the parenthetical relative age")
		assert.Contains(t, got, "(")
		assert.Contains(t, got, ")")
	})
}

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}
