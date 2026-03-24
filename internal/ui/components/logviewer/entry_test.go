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

	result := RenderCompactEntry(entry, false, 80, "", true)

	assert.Contains(t, result, "▸")
	assert.Contains(t, result, "13:04:01")
	assert.Contains(t, result, "connection refused")
}

func TestRenderCompactEntryExpanded(t *testing.T) {
	entry := gcp.LogEntry{
		Timestamp:    time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
		Severity:     "WARNING",
		Message:      "slow query",
		ResourceType: "cloud_run_revision",
	}

	result := RenderCompactEntry(entry, true, 80, "", true)
	assert.Contains(t, result, "▾")
}

func TestRenderExpandedFields(t *testing.T) {
	entry := gcp.LogEntry{
		ResourceType:   "gce_instance",
		ResourceLabels: map[string]string{"instance_id": "123"},
		Labels:         map[string]string{"key": "val"},
		InsertID:       "abc",
	}

	result, lines := RenderExpandedFields(entry, -1, 80, true)

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

	result, _ := RenderExpandedFields(entry, 0, 80, true)
	// Should contain the filter hint on the cursor line
	assert.Contains(t, result, "[+f]")
}

func TestRenderExpandedFieldsEmpty(t *testing.T) {
	entry := gcp.LogEntry{}
	result, lines := RenderExpandedFields(entry, -1, 80, true)
	assert.Empty(t, result)
	assert.Equal(t, 0, lines)
}

func TestRenderExpandedFieldsWrapsLongValues(t *testing.T) {
	// Value longer than available width should wrap to multiple lines
	longMsg := strings.Repeat("x", 200)
	entry := gcp.LogEntry{
		TextPayload: longMsg,
	}

	result, lines := RenderExpandedFields(entry, -1, 80, true)
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

func TestColorizeMessage(t *testing.T) {
	t.Run("logfmt key=value pairs", func(t *testing.T) {
		msg := `level=info msg="GetRepoObjs stats" count=42 ratio=3.14`
		result := colorizeMessage(msg, "")
		plain := stripANSI(result)
		assert.Equal(t, msg, plain, "colorized output should preserve all text")
	})

	t.Run("boolean and null values", func(t *testing.T) {
		msg := `enabled=true deleted=false value=null`
		result := colorizeMessage(msg, "")
		plain := stripANSI(result)
		assert.Equal(t, msg, plain)
	})

	t.Run("plain text without logfmt", func(t *testing.T) {
		msg := "just a regular log message"
		result := colorizeMessage(msg, "")
		plain := stripANSI(result)
		assert.Equal(t, msg, plain)
	})

	t.Run("with background color", func(t *testing.T) {
		msg := `level=error code=500`
		result := colorizeMessage(msg, "#3C4043")
		plain := stripANSI(result)
		assert.Equal(t, msg, plain)
	})

	t.Run("empty string", func(t *testing.T) {
		assert.Equal(t, "", colorizeMessage("", ""))
	})

	t.Run("negative numbers", func(t *testing.T) {
		msg := `offset=-10 temp=-3.5`
		result := colorizeMessage(msg, "")
		plain := stripANSI(result)
		assert.Equal(t, msg, plain)
	})

	t.Run("bracketed text", func(t *testing.T) {
		msg := `[GIN] 2026/03/19 - 13:04:05 [Recovery] panic recovered`
		result := colorizeMessage(msg, "")
		plain := stripANSI(result)
		assert.Equal(t, msg, plain)
	})

	t.Run("mixed brackets and logfmt", func(t *testing.T) {
		msg := `[INFO] level=info msg="hello" [tag] count=5`
		result := colorizeMessage(msg, "")
		plain := stripANSI(result)
		assert.Equal(t, msg, plain)
	})

	t.Run("protobuf key:value pairs", func(t *testing.T) {
		msg := `service_name:"k8s.io"  method_name:"sh.keda.v1alpha1.scaledobjects.status.patch"`
		result := colorizeMessage(msg, "")
		plain := stripANSI(result)
		assert.Equal(t, msg, plain)
	})

	t.Run("mixed logfmt and protobuf", func(t *testing.T) {
		msg := `level=info service_name:"k8s.io" count=42`
		result := colorizeMessage(msg, "")
		plain := stripANSI(result)
		assert.Equal(t, msg, plain)
	})

	t.Run("preserves existing ANSI colors", func(t *testing.T) {
		msg := "\x1b[32mINFO\x1b[0m some message level=info"
		result := colorizeMessage(msg, "")
		// Should pass through unchanged — no double-styling
		assert.Equal(t, msg, result)
	})
}

func TestTruncateEntryWithANSI(t *testing.T) {
	// ANSI codes should not count toward visible width
	msg := "\x1b[32mgreen text\x1b[0m and more"
	result := truncateEntry(msg, 20)
	plain := stripANSI(result)
	// "green text and more" is 19 visible chars — fits in 20
	assert.Equal(t, "green text and more", plain)
	assert.Contains(t, result, "\x1b[32m", "should preserve ANSI codes")

	// Truncation should happen based on visible width
	result2 := truncateEntry(msg, 12)
	plain2 := stripANSI(result2)
	assert.Equal(t, 12, len([]rune(plain2)), "should truncate to 12 visible chars including ...")
	assert.Contains(t, plain2, "...")
}

func TestTruncateEntryPlainUnchanged(t *testing.T) {
	// Plain text should still work as before
	assert.Equal(t, "hello", truncateEntry("hello", 10))
	assert.Equal(t, "hel...", truncateEntry("hello world", 6))
}

func TestRenderWrappedEntryMultiLineContent(t *testing.T) {
	// Long message that wraps to multiple lines should preserve full content
	// through the styling pipeline (plainStyle.Render or colorizeMessage).
	longMsg := strings.Repeat("x", 200)
	entry := gcp.LogEntry{
		Timestamp:    time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
		Severity:     "INFO",
		Message:      longMsg,
		ResourceType: "gce_instance",
	}

	t.Run("plain style preserves content on all lines", func(t *testing.T) {
		rendered, lineCount := RenderWrappedEntry(entry, false, 80, "", false)
		assert.Greater(t, lineCount, 1, "should wrap to multiple lines")
		// Full message content should be present across all wrapped lines
		plain := stripANSI(rendered)
		plain = strings.ReplaceAll(plain, "\n", "")
		plain = strings.ReplaceAll(plain, " ", "")
		assert.Contains(t, plain, longMsg, "all chunks should be rendered")
	})

	t.Run("colorize path produces content on all lines", func(t *testing.T) {
		msgEntry := gcp.LogEntry{
			Timestamp:    time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
			Severity:     "INFO",
			Message:      strings.Repeat("level=info ", 20),
			ResourceType: "gce_instance",
		}
		rendered, lineCount := RenderWrappedEntry(msgEntry, false, 80, "", true)
		assert.Greater(t, lineCount, 1, "should wrap to multiple lines")
		// All continuation lines should have content (not empty/whitespace-only)
		lines := strings.Split(rendered, "\n")
		for i, line := range lines {
			if line == "" {
				continue
			}
			trimmed := strings.TrimSpace(stripANSI(line))
			assert.NotEmpty(t, trimmed, "line %d should have visible content", i)
		}
	})
}

func TestIsNumeric(t *testing.T) {
	assert.True(t, isNumeric("42"))
	assert.True(t, isNumeric("3.14"))
	assert.True(t, isNumeric("-10"))
	assert.True(t, isNumeric("+5"))
	assert.False(t, isNumeric(""))
	assert.False(t, isNumeric("abc"))
	assert.False(t, isNumeric("1.2.3"))
	assert.False(t, isNumeric("-"))
}
