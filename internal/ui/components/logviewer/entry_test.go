package logviewer

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}


func TestRenderCompactEntry(t *testing.T) {
	entry := gcp.LogEntry{
		Timestamp:    time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
		Severity:     "ERROR",
		Message:      "connection refused to database",
		ResourceType: "gce_instance",
	}

	result := RenderCompactEntry(entry, false, 80, "")

	assert.Contains(t, result, "▸")
	assert.Contains(t, result, "13:04:01")
	assert.Contains(t, result, "gce_instance")
	assert.Contains(t, result, "connection refused")
}

func TestRenderCompactEntryExpanded(t *testing.T) {
	entry := gcp.LogEntry{
		Timestamp:    time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
		Severity:     "WARNING",
		Message:      "slow query",
		ResourceType: "cloud_run_revision",
	}

	result := RenderCompactEntry(entry, true, 80, "")
	assert.Contains(t, result, "▾")
}

func TestRenderExpandedFields(t *testing.T) {
	entry := gcp.LogEntry{
		ResourceType:   "gce_instance",
		ResourceLabels: map[string]string{"instance_id": "123"},
		Labels:         map[string]string{"key": "val"},
		InsertID:       "abc",
	}

	result, lines := RenderExpandedFields(entry, -1, 80)

	assert.Contains(t, result, "resource.type")
	assert.Contains(t, result, "gce_instance")
	assert.Contains(t, result, "resource.labels.instance_id")
	assert.Contains(t, result, "123")
	assert.Greater(t, lines, 0)
}

func TestRenderExpandedFieldsWithCursor(t *testing.T) {
	entry := gcp.LogEntry{
		ResourceType: "gce_instance",
		InsertID:     "abc",
	}

	result, _ := RenderExpandedFields(entry, 0, 80)
	// Should contain the filter hint on the cursor line
	assert.Contains(t, result, "[+f]")
}

func TestRenderExpandedFieldsEmpty(t *testing.T) {
	entry := gcp.LogEntry{}
	result, lines := RenderExpandedFields(entry, -1, 80)
	assert.Empty(t, result)
	assert.Equal(t, 0, lines)
}

func TestRenderExpandedFieldsWrapsLongValues(t *testing.T) {
	// Value longer than available width should wrap to multiple lines
	longMsg := strings.Repeat("x", 200)
	entry := gcp.LogEntry{
		TextPayload: longMsg,
	}

	result, lines := RenderExpandedFields(entry, -1, 80)
	assert.Contains(t, result, "textPayload")
	assert.Greater(t, lines, 1, "long value should wrap to multiple lines")
	// All content should be present (no truncation) — join lines to verify
	plainResult := strings.ReplaceAll(stripANSI(result), "\n", "")
	plainResult = strings.ReplaceAll(plainResult, " ", "")
	assert.Contains(t, plainResult, longMsg, "full value should be present, not truncated")
}

func TestSeverityAbbrev(t *testing.T) {
	tests := []struct {
		severity string
		expected string
	}{
		{"INFO", "I"},
		{"WARNING", "W"},
		{"ERROR", "E"},
		{"CRITICAL", "C"},
		{"DEBUG", "D"},
		{"DEFAULT", "D"},
		{"NOTICE", "N"},
		{"ALERT", "A"},
		{"EMERGENCY", "!"},
		{"UNKNOWN", "?"},
	}
	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			assert.Equal(t, tt.expected, SeverityAbbrev(tt.severity))
		})
	}
}

func TestTruncateEntry(t *testing.T) {
	assert.Equal(t, "hello", truncateEntry("hello", 10))
	assert.Equal(t, "hel...", truncateEntry("hello world", 6))
	assert.Equal(t, "hello world", truncateEntry("hello\nworld", 20))
}
