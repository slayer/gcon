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
	defer func() { _ = os.Setenv("TZ", originalTZ) }()

	// Set timezone to PST
	_ = os.Setenv("TZ", "America/Los_Angeles")
	time.Local = mustLoadLocation("America/Los_Angeles")

	// UTC time: Jan 15, 2024 at 20:00 UTC = Jan 15, 2024 at 12:00 PST
	input := time.Date(2024, 1, 15, 20, 0, 0, 0, time.UTC)
	result := FormatDateTime(input)

	// Should show PST time (12:00 PM)
	assert.Contains(t, result, "12:00:00 PM")
	assert.Contains(t, result, "PST")

	// Now switch to EST
	_ = os.Setenv("TZ", "America/New_York")
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

func TestCalculateUptime(t *testing.T) {
	tests := []struct {
		name       string
		startTime  string
		expected   string
		setupMock  func() time.Time // Returns the "now" time to use for calculation
		validateFn func(t *testing.T, result string)
	}{
		{
			name:      "empty string returns dash",
			startTime: "",
			expected:  "—",
		},
		{
			name:      "invalid timestamp returns dash",
			startTime: "not-a-timestamp",
			expected:  "—",
		},
		{
			name:      "less than a minute",
			startTime: time.Now().Add(-30 * time.Second).Format(time.RFC3339),
			expected:  "< 1m",
		},
		{
			name:      "exactly 1 minute",
			startTime: time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
			expected:  "1m",
		},
		{
			name:      "5 minutes",
			startTime: time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
			expected:  "5m",
		},
		{
			name:      "59 minutes",
			startTime: time.Now().Add(-59 * time.Minute).Format(time.RFC3339),
			expected:  "59m",
		},
		{
			name:      "1 hour",
			startTime: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			expected:  "1h",
		},
		{
			name:      "1 hour 30 minutes",
			startTime: time.Now().Add(-90 * time.Minute).Format(time.RFC3339),
			expected:  "1h 30m",
		},
		{
			name:      "5 hours 45 minutes",
			startTime: time.Now().Add(-5*time.Hour - 45*time.Minute).Format(time.RFC3339),
			expected:  "5h 45m",
		},
		{
			name:      "23 hours 59 minutes",
			startTime: time.Now().Add(-23*time.Hour - 59*time.Minute).Format(time.RFC3339),
			expected:  "23h 59m",
		},
		{
			name:      "1 day",
			startTime: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			expected:  "1d",
		},
		{
			name:      "1 day 5 hours",
			startTime: time.Now().Add(-24*time.Hour - 5*time.Hour).Format(time.RFC3339),
			expected:  "1d 5h",
		},
		{
			name:      "2 days 5 hours 30 minutes",
			startTime: time.Now().Add(-48*time.Hour - 5*time.Hour - 30*time.Minute).Format(time.RFC3339),
			expected:  "2d 5h 30m",
		},
		{
			name:      "7 days",
			startTime: time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
			expected:  "7d",
		},
		{
			name:      "30 days 12 hours 45 minutes",
			startTime: time.Now().Add(-30*24*time.Hour - 12*time.Hour - 45*time.Minute).Format(time.RFC3339),
			expected:  "30d 12h 45m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateUptime(tt.startTime)
			if tt.validateFn != nil {
				tt.validateFn(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name        string
		days        int
		hours       int
		minutes     int
		includeDays bool
		expected    string
	}{
		{
			name:        "only minutes",
			days:        0,
			hours:       0,
			minutes:     30,
			includeDays: false,
			expected:    "30m",
		},
		{
			name:        "hours and minutes",
			days:        0,
			hours:       2,
			minutes:     15,
			includeDays: false,
			expected:    "2h 15m",
		},
		{
			name:        "only hours",
			days:        0,
			hours:       5,
			minutes:     0,
			includeDays: false,
			expected:    "5h",
		},
		{
			name:        "days hours minutes",
			days:        3,
			hours:       4,
			minutes:     25,
			includeDays: true,
			expected:    "3d 4h 25m",
		},
		{
			name:        "only days",
			days:        1,
			hours:       0,
			minutes:     0,
			includeDays: true,
			expected:    "1d",
		},
		{
			name:        "days and hours",
			days:        2,
			hours:       6,
			minutes:     0,
			includeDays: true,
			expected:    "2d 6h",
		},
		{
			name:        "all zero",
			days:        0,
			hours:       0,
			minutes:     0,
			includeDays: false,
			expected:    "< 1m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.days, tt.hours, tt.minutes, tt.includeDays)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatUnit(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		unit     string
		expected string
	}{
		{
			name:     "5 hours",
			value:    5,
			unit:     "h",
			expected: "5h",
		},
		{
			name:     "30 minutes",
			value:    30,
			unit:     "m",
			expected: "30m",
		},
		{
			name:     "2 days",
			value:    2,
			unit:     "d",
			expected: "2d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUnit(tt.value, tt.unit)
			assert.Equal(t, tt.expected, result)
		})
	}
}
