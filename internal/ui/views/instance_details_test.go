package views

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
	"github.com/stretchr/testify/assert"
)

// errTestSSH is a sentinel used by SSH tests to avoid dynamic error construction.
var errTestSSH = errors.New("connection refused")

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

func TestInstanceDetails_TKey_OpensSSHDialog_WhenRunning(t *testing.T) {
	v := NewInstanceDetailsView("proj", "us-central1-a", "vm", nil, nil)
	v.details = &gcp.InstanceDetails{
		Name:   "vm",
		Status: "RUNNING",
		NetworkInterfaces: []gcp.NetworkInterfaceInfo{
			{InternalIP: "10.0.0.5", ExternalIP: "34.1.2.3"},
		},
	}
	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.NotNil(t, cmd, "expected dialog Init cmd")
	assert.True(t, v.showSSHDialog, "dialog should be visible after t key")
	assert.NotNil(t, v.sshDialog)
}

func TestInstanceDetails_TKey_NoOp_WhenStopped(t *testing.T) {
	v := NewInstanceDetailsView("proj", "us-central1-a", "vm", nil, nil)
	v.details = &gcp.InstanceDetails{Name: "vm", Status: "TERMINATED"}
	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.False(t, v.showSSHDialog, "dialog must not open on stopped instance")
}

func TestInstanceDetails_SSHExited_StoresError(t *testing.T) {
	v := NewInstanceDetailsView("proj", "us-central1-a", "vm", nil, nil)
	v.SetSSHError(errTestSSH)
	assert.EqualError(t, v.sshErr, "connection refused")
}

func TestInstanceDetails_HasTextInputFocused_WhenDialogOpen(t *testing.T) {
	v := NewInstanceDetailsView("proj", "us-central1-a", "vm", nil, nil)
	v.details = &gcp.InstanceDetails{
		Name:   "vm",
		Status: "RUNNING",
		NetworkInterfaces: []gcp.NetworkInterfaceInfo{
			{InternalIP: "10.0.0.5", ExternalIP: "34.1.2.3"},
		},
	}
	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.True(t, v.HasTextInputFocused())
}

func TestInstanceDetails_IsMenuOpen_TrueWhenSSHDialogOpen(t *testing.T) {
	v := NewInstanceDetailsView("proj", "us-central1-a", "vm", nil, nil)
	v.details = &gcp.InstanceDetails{
		Name:   "vm",
		Status: "RUNNING",
		NetworkInterfaces: []gcp.NetworkInterfaceInfo{
			{InternalIP: "10.0.0.5"},
		},
	}
	// Pre-condition: menu is closed.
	assert.False(t, v.IsMenuOpen())
	// Open the SSH dialog via the t key.
	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.True(t, v.IsMenuOpen(), "IsMenuOpen must include showSSHDialog so Esc routes to the dialog")
}
