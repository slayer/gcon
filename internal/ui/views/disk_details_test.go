package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiskDetailsView_NewDiskDetailsView(t *testing.T) {
	v := NewDiskDetailsView("test-project", "us-central1-a", "test-disk", nil)

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.Equal(t, "us-central1-a", v.zone)
	assert.Equal(t, "test-disk", v.diskName)
	assert.True(t, v.loading, "View should start in loading state")
}

func TestDiskDetailsView_RenderLoading(t *testing.T) {
	v := NewDiskDetailsView("test-project", "us-central1-a", "test-disk", nil)

	output := renderLoading(v.spinner, "Loading disk details...")

	assert.Contains(t, output, "Loading disk details...")
	assert.Contains(t, output, v.spinner.View())
}

func TestDiskDetailsView_GetDiskName(t *testing.T) {
	v := NewDiskDetailsView("test-project", "us-central1-a", "my-disk", nil)

	assert.Equal(t, "my-disk", v.GetDiskName())
}

func TestFormatDiskType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"pd-standard", "Standard persistent disk"},
		{"pd-balanced", "Balanced persistent disk"},
		{"pd-ssd", "SSD persistent disk"},
		{"pd-extreme", "Extreme persistent disk"},
		{"hyperdisk-balanced", "Hyperdisk Balanced"},
		{"hyperdisk-extreme", "Hyperdisk Extreme"},
		{"hyperdisk-throughput", "Hyperdisk Throughput"},
		{"unknown-type", "unknown-type"},
		{"", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatDiskType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDiskDetailStatusIcon(t *testing.T) {
	// Attached disk should show running (green) indicator
	icon1 := diskDetailStatusIcon("READY", true)
	assert.NotEmpty(t, icon1)

	// Detached ready disk should show stopped (red) indicator
	icon2 := diskDetailStatusIcon("READY", false)
	assert.NotEmpty(t, icon2)

	// Creating disk should show transitioning indicator
	icon3 := diskDetailStatusIcon("CREATING", false)
	assert.NotEmpty(t, icon3)
}
