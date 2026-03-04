package views

import (
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudRunObservability_InitialState(t *testing.T) {
	obs := newCloudRunObservability("test-project", "my-service", nil, nil)

	assert.True(t, obs.autoRefresh, "auto-refresh should default to true")
	assert.Equal(t, time.Hour, obs.timeRange, "time range should default to 1h")
	assert.False(t, obs.initialized, "should not be initialized before Init()")
	assert.Nil(t, obs.metrics, "metrics should be nil before Init()")
	assert.Nil(t, obs.logs, "logs should be nil before Init()")

	// Severity filters should all be enabled
	assert.True(t, obs.severityEnabled["INFO"])
	assert.True(t, obs.severityEnabled["WARNING"])
	assert.True(t, obs.severityEnabled["ERROR"])

	// After Init, loading flags should be set
	obs.Init()
	assert.True(t, obs.metricsLoading, "metricsLoading should be true after Init()")
	assert.True(t, obs.logsLoading, "logsLoading should be true after Init()")
	assert.True(t, obs.initialized, "initialized should be true after Init()")
}

func TestCloudRunObservability_SeverityFilter(t *testing.T) {
	obs := newCloudRunObservability("test-project", "my-service", nil, nil)

	// All enabled by default
	assert.True(t, obs.severityEnabled["INFO"])

	// Toggle off
	obs.toggleSeverity("INFO")
	assert.False(t, obs.severityEnabled["INFO"])

	// Toggle back on
	obs.toggleSeverity("INFO")
	assert.True(t, obs.severityEnabled["INFO"])

	// Toggle ERROR off
	obs.toggleSeverity("ERROR")
	assert.False(t, obs.severityEnabled["ERROR"])
}

func TestCloudRunObservability_ActiveSeverities(t *testing.T) {
	tests := []struct {
		name     string
		enabled  map[string]bool
		expected []string
	}{
		{
			name:     "all enabled includes CRITICAL with ERROR",
			enabled:  map[string]bool{"INFO": true, "WARNING": true, "ERROR": true},
			expected: []string{"INFO", "WARNING", "ERROR", "CRITICAL"},
		},
		{
			name:     "only INFO",
			enabled:  map[string]bool{"INFO": true, "WARNING": false, "ERROR": false},
			expected: []string{"INFO"},
		},
		{
			name:     "ERROR includes CRITICAL",
			enabled:  map[string]bool{"INFO": false, "WARNING": false, "ERROR": true},
			expected: []string{"ERROR", "CRITICAL"},
		},
		{
			name:     "none enabled",
			enabled:  map[string]bool{"INFO": false, "WARNING": false, "ERROR": false},
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obs := newCloudRunObservability("p", "s", nil, nil)
			obs.severityEnabled = tc.enabled
			assert.Equal(t, tc.expected, obs.activeSeverities())
		})
	}
}

func TestCloudRunObservability_FilteredLogs(t *testing.T) {
	obs := newCloudRunObservability("p", "s", nil, nil)
	now := time.Now()

	obs.logs = []gcp.LogEntry{
		{Timestamp: now, Severity: "INFO", Message: "info msg"},
		{Timestamp: now, Severity: "WARNING", Message: "warn msg"},
		{Timestamp: now, Severity: "ERROR", Message: "error msg"},
		{Timestamp: now, Severity: "CRITICAL", Message: "critical msg"},
	}

	// All enabled: should get all 4
	filtered := obs.filteredLogs()
	assert.Len(t, filtered, 4)

	// Disable INFO
	obs.toggleSeverity("INFO")
	filtered = obs.filteredLogs()
	assert.Len(t, filtered, 3)
	for _, entry := range filtered {
		assert.NotEqual(t, "INFO", entry.Severity)
	}

	// Disable ERROR (also hides CRITICAL)
	obs.toggleSeverity("ERROR")
	filtered = obs.filteredLogs()
	assert.Len(t, filtered, 1)
	assert.Equal(t, "WARNING", filtered[0].Severity)
}

func TestCloudRunObservability_RenderView_Loading(t *testing.T) {
	obs := newCloudRunObservability("p", "s", nil, nil)
	obs.width = 80
	obs.metricsLoading = true

	view := obs.View()

	assert.Contains(t, view, "Observability")
	assert.Contains(t, view, "Loading metrics")
}

func TestCloudRunObservability_RenderView_WithMetrics(t *testing.T) {
	obs := newCloudRunObservability("p", "s", nil, nil)
	obs.width = 80
	obs.metricsLoading = false
	obs.logsLoading = false

	obs.metrics = &gcp.CloudRunMetrics{
		RequestCount: []gcp.DataPoint{
			{Timestamp: time.Now(), Value: 10.0},
			{Timestamp: time.Now(), Value: 15.0},
		},
		CPU: []gcp.DataPoint{
			{Timestamp: time.Now(), Value: 0.25},
			{Timestamp: time.Now(), Value: 0.50},
		},
		Memory: []gcp.DataPoint{
			{Timestamp: time.Now(), Value: 0.40},
		},
		InstanceCount: []gcp.DataPoint{
			{Timestamp: time.Now(), Value: 3.0},
		},
		LastFetch: time.Now(),
	}

	view := obs.View()

	assert.Contains(t, view, "Request Count")
	assert.Contains(t, view, "CPU Usage")
	assert.Contains(t, view, "Memory Usage")
	assert.Contains(t, view, "Instance Count")
	assert.Contains(t, view, "Logs")
}

func TestCloudRunObservability_ExtractValues(t *testing.T) {
	data := []gcp.DataPoint{
		{Value: 1.0},
		{Value: 2.5},
		{Value: 3.7},
	}
	values := extractValues(data)
	require.Len(t, values, 3)
	assert.Equal(t, 1.0, values[0])
	assert.Equal(t, 2.5, values[1])
	assert.Equal(t, 3.7, values[2])
}

func TestCloudRunObservability_RenderView_Error(t *testing.T) {
	obs := newCloudRunObservability("p", "s", nil, nil)
	obs.width = 80
	obs.metricsLoading = false
	obs.metricsError = assert.AnError

	view := obs.View()

	assert.Contains(t, view, "Error loading metrics")
	assert.Contains(t, view, "retry")
}

func TestFormatLatency(t *testing.T) {
	tests := []struct {
		ms       float64
		expected string
	}{
		{0.5, "0.50ms"},
		{42, "42ms"},
		{999, "999ms"},
		{1500, "1.5s"},
		{59999, "60.0s"},
		{60000, "1m"},
		{90000, "1m 30s"},
		{372343, "6m 12s"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatLatency(tt.ms), "formatLatency(%f)", tt.ms)
	}
}

func TestCloudRunObservability_AutoRefreshToggle(t *testing.T) {
	obs := newCloudRunObservability("p", "s", nil, nil)

	assert.True(t, obs.autoRefresh)

	// Simulate key 'a' to toggle off
	obs.autoRefresh = false
	assert.False(t, obs.autoRefresh)

	// Toggle back on
	obs.autoRefresh = true
	assert.True(t, obs.autoRefresh)
}
