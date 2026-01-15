package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnapshotDetailsView_NewSnapshotDetailsView(t *testing.T) {
	v := NewSnapshotDetailsView("test-project", "test-snapshot", nil)

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.Equal(t, "test-snapshot", v.snapshotName)
	assert.True(t, v.loading, "View should start in loading state")
}

func TestSnapshotDetailsView_RenderLoading(t *testing.T) {
	v := NewSnapshotDetailsView("test-project", "test-snapshot", nil)

	output := v.renderLoading("Loading snapshot details...")

	assert.Contains(t, output, "Loading snapshot details...")
	assert.Contains(t, output, v.spinner.View())
}

func TestSnapshotDetailsView_GetSnapshotName(t *testing.T) {
	v := NewSnapshotDetailsView("test-project", "my-snapshot", nil)

	assert.Equal(t, "my-snapshot", v.GetSnapshotName())
}

func TestSnapshotDetailStatusIcon(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"Ready snapshot", "READY"},
		{"Creating snapshot", "CREATING"},
		{"Uploading snapshot", "UPLOADING"},
		{"Failed snapshot", "FAILED"},
		{"Deleting snapshot", "DELETING"},
		{"Unknown status", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon := snapshotDetailStatusIcon(tt.status)
			assert.NotEmpty(t, icon, "Status icon should not be empty for status: %s", tt.status)
		})
	}
}

func TestSnapshotDetailStatusIcon_Ready(t *testing.T) {
	// Ready snapshot should show running (green) indicator
	icon := snapshotDetailStatusIcon("READY")
	assert.NotEmpty(t, icon)
}

func TestSnapshotDetailStatusIcon_Creating(t *testing.T) {
	// Creating snapshot should show transitioning indicator
	icon := snapshotDetailStatusIcon("CREATING")
	assert.NotEmpty(t, icon)
}

func TestSnapshotDetailStatusIcon_Failed(t *testing.T) {
	// Failed snapshot should show stopped (red) indicator
	icon := snapshotDetailStatusIcon("FAILED")
	assert.NotEmpty(t, icon)
}
