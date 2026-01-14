package views

import (
	"testing"

	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
	"github.com/stretchr/testify/assert"
)

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"RUNNING", symbols.StatusRunning()},
		{"TERMINATED", symbols.StatusStopped()},
		{"STOPPED", symbols.StatusStopped()},
		{"STAGING", symbols.StatusTransitioning()},
		{"PROVISIONING", symbols.StatusTransitioning()},
		{"STOPPING", symbols.StatusTransitioning()},
		{"SUSPENDING", symbols.StatusTransitioning()},
		{"UNKNOWN", symbols.StatusUnknown()},
		{"", symbols.StatusUnknown()},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := getStatusIcon(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // Check if output contains expected substring
	}{
		{
			name:     "valid RFC3339 timestamp",
			input:    "2025-01-11T14:30:00Z",
			contains: "Jan 11, 2025",
		},
		{
			name:     "empty string",
			input:    "",
			contains: "—",
		},
		{
			name:     "invalid timestamp",
			input:    "not-a-timestamp",
			contains: "not-a-timestamp", // Returns original on parse error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := timeutil.FormatTimestamp(tt.input)
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestFormatBool(t *testing.T) {
	assert.Equal(t, "Enabled", formatBool(true))
	assert.Equal(t, "Disabled", formatBool(false))
}

func TestFormatOnOff(t *testing.T) {
	assert.Equal(t, "On", formatOnOff(true))
	assert.Equal(t, "Off", formatOnOff(false))
}

func TestFormatMaintenance(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"MIGRATE", "Migrate VM instance"},
		{"TERMINATE", "Terminate VM instance"},
		{"CUSTOM", "CUSTOM"},
		{"", "—"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatMaintenance(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultIfEmpty(t *testing.T) {
	assert.Equal(t, "value", defaultIfEmpty("value", "default"))
	assert.Equal(t, "default", defaultIfEmpty("", "default"))
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "truncated",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "maxLen less than 3",
			input:    "hello world",
			maxLen:   2,
			expected: "he",
		},
		{
			name:     "maxLen equals 3",
			input:    "hello world",
			maxLen:   3,
			expected: "hel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMin(t *testing.T) {
	assert.Equal(t, 5, min(5, 10))
	assert.Equal(t, 5, min(10, 5))
	assert.Equal(t, 5, min(5, 5))
}
