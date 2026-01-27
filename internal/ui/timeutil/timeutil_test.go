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

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}
