package gcp

import (
	"testing"
	"time"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	monitoredres "google.golang.org/genproto/googleapis/api/monitoredres"
)

func TestSeverityORClauses(t *testing.T) {
	t.Run("multiple severities", func(t *testing.T) {
		clauses := severityORClauses([]string{"INFO", "WARNING"})
		assert.Len(t, clauses, 2)
		assert.Equal(t, `severity="INFO"`, clauses[0])
		assert.Equal(t, `severity="WARNING"`, clauses[1])
	})

	t.Run("single severity", func(t *testing.T) {
		clauses := severityORClauses([]string{"ERROR"})
		assert.Len(t, clauses, 1)
		assert.Equal(t, `severity="ERROR"`, clauses[0])
	})

	t.Run("empty input", func(t *testing.T) {
		clauses := severityORClauses([]string{})
		assert.Empty(t, clauses)
	})
}

func TestLogEntryFlattenFields(t *testing.T) {
	entry := LogEntry{
		Timestamp:    time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC),
		Severity:     "ERROR",
		Message:      "something failed",
		LogName:      "projects/my-proj/logs/stderr",
		ResourceType: "gce_instance",
		ResourceLabels: map[string]string{
			"instance_id": "123456",
			"zone":        "us-central1-a",
		},
		Labels: map[string]string{
			"env": "prod",
		},
		JSONPayload: map[string]any{
			"message": "disk full",
			"details": map[string]any{
				"code":   404,
				"reason": "not found",
			},
		},
		TextPayload:    "raw text line",
		InsertID:       "abc-123",
		TraceID:        "projects/my-proj/traces/def456",
		SpanID:         "span-789",
		SourceLocation: "main.go:42",
	}

	fields := entry.FlattenFields()
	require.NotEmpty(t, fields)

	// Build a lookup map for easier assertions
	lookup := make(map[string]string, len(fields))
	for _, f := range fields {
		lookup[f.Key] = f.Value
	}

	assert.Equal(t, "gce_instance", lookup["resource.type"])
	assert.Equal(t, "123456", lookup["resource.labels.instance_id"])
	assert.Equal(t, "us-central1-a", lookup["resource.labels.zone"])
	assert.Equal(t, "projects/my-proj/logs/stderr", lookup["logName"])
	assert.Equal(t, "abc-123", lookup["insertId"])
	assert.Equal(t, "projects/my-proj/traces/def456", lookup["trace"])
	assert.Equal(t, "span-789", lookup["spanId"])
	assert.Equal(t, "main.go:42", lookup["sourceLocation"])
	assert.Equal(t, "prod", lookup["labels.env"])
	assert.Equal(t, "raw text line", lookup["textPayload"])
	assert.Equal(t, "disk full", lookup["jsonPayload.message"])
	assert.Equal(t, "404", lookup["jsonPayload.details.code"])
	assert.Equal(t, "not found", lookup["jsonPayload.details.reason"])
}

func TestLogEntryFlattenFieldsSorted(t *testing.T) {
	entry := LogEntry{
		ResourceType: "gce_instance",
		LogName:      "projects/p/logs/syslog",
		TraceID:      "trace-1",
		SpanID:       "span-1",
		InsertID:     "insert-1",
		Labels: map[string]string{
			"beta": "b",
			"alpha": "a",
		},
		ResourceLabels: map[string]string{
			"zone":        "us-east1-b",
			"instance_id": "999",
		},
		JSONPayload: map[string]any{
			"z_field": "last",
			"a_field": "first",
		},
	}

	fields := entry.FlattenFields()
	require.NotEmpty(t, fields)

	// Verify strict alphabetical ordering
	for i := 1; i < len(fields); i++ {
		assert.True(t, fields[i-1].Key < fields[i].Key,
			"fields not sorted: %q should come before %q", fields[i-1].Key, fields[i].Key)
	}
}

func TestLogEntryFlattenFieldsEmpty(t *testing.T) {
	entry := LogEntry{}
	fields := entry.FlattenFields()
	assert.Empty(t, fields, "empty entry should produce no flattened fields")
}

func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Default", "INFO"},
		{"Info", "INFO"},
		{"Warning", "WARNING"},
		{"Error", "ERROR"},
		{"Critical", "CRITICAL"},
		{"Debug", "DEBUG"},
		{"Notice", "NOTICE"},
		{"INFO", "INFO"},       // already uppercase
		{"WARNING", "WARNING"}, // already uppercase
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, normalizeSeverity(tc.input), "normalizeSeverity(%q)", tc.input)
	}
}

func TestConvertLogEntryString(t *testing.T) {
	entry := &logging.Entry{
		Timestamp: time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
		Severity:  logging.Error,
		Payload:   "simple text message",
		InsertID:  "ins-1",
		LogName:   "projects/test/logs/stderr",
		Trace:     "projects/test/traces/abc",
		SpanID:    "span-1",
	}

	result := convertLogEntry(entry)

	assert.Equal(t, "ERROR", result.Severity)
	assert.Equal(t, "simple text message", result.Message)
	assert.Equal(t, "simple text message", result.TextPayload)
	assert.Equal(t, "ins-1", result.InsertID)
	assert.Equal(t, "projects/test/logs/stderr", result.LogName)
	assert.Equal(t, "projects/test/traces/abc", result.TraceID)
	assert.Equal(t, "span-1", result.SpanID)
}

func TestConvertLogEntryJSON(t *testing.T) {
	entry := &logging.Entry{
		Timestamp: time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
		Severity:  logging.Warning,
		Payload:   map[string]any{"method": "GET", "status": float64(200)},
		InsertID:  "ins-2",
	}

	result := convertLogEntry(entry)

	assert.Equal(t, "WARNING", result.Severity)
	assert.NotNil(t, result.JSONPayload)
	assert.Equal(t, "GET", result.JSONPayload["method"])
	assert.Empty(t, result.TextPayload)
}

func TestConvertLogEntryWithResource(t *testing.T) {
	entry := &logging.Entry{
		Timestamp: time.Now(),
		Severity:  logging.Info,
		Payload:   "test",
		Resource: &monitoredres.MonitoredResource{
			Type:   "gce_instance",
			Labels: map[string]string{"instance_id": "123", "zone": "us-central1-a"},
		},
	}

	result := convertLogEntry(entry)

	assert.Equal(t, "gce_instance", result.ResourceType)
	assert.Equal(t, "123", result.ResourceLabels["instance_id"])
	assert.Equal(t, "us-central1-a", result.ResourceLabels["zone"])
}

func TestConvertLogEntryWithLabels(t *testing.T) {
	entry := &logging.Entry{
		Timestamp: time.Now(),
		Severity:  logging.Info,
		Payload:   "test",
		Labels:    map[string]string{"env": "prod", "app": "web"},
	}

	result := convertLogEntry(entry)

	assert.Equal(t, "prod", result.Labels["env"])
	assert.Equal(t, "web", result.Labels["app"])
}

func TestConvertLogEntryNilResource(t *testing.T) {
	entry := &logging.Entry{
		Timestamp: time.Now(),
		Severity:  logging.Info,
		Payload:   "no resource",
	}

	result := convertLogEntry(entry)

	assert.Empty(t, result.ResourceType)
	assert.Nil(t, result.ResourceLabels)
}
