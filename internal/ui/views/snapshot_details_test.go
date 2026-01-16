package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/links"
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

func TestSnapshotDetailsView_NavigateToDisk(t *testing.T) {
	// 1. Setup
	v := NewSnapshotDetailsView("test-project", "test-snapshot", nil)

	// 2. Load details
	details := &gcp.SnapshotDetails{
		Name:           "test-snapshot",
		SourceDisk:     "my-source-disk",
		SourceDiskZone: "us-central1-a",
		Status:         "READY",
		CreatedAt:      time.Now().Format("2006-01-02T15:04:05.999-07:00"),
	}
	v.Update(snapshotDetailsLoadedMsg{details: details})

	// 3. Simulate 'enter' key press which should produce a LinkSelectedMsg
	cmd1 := v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd1)

	msg1 := cmd1()
	linkMsg, ok := msg1.(links.LinkSelectedMsg)
	assert.True(t, ok, "message should be of type links.LinkSelectedMsg")
	assert.Equal(t, "my-source-disk", linkMsg.Link.ID)

	// 4. Pass the LinkSelectedMsg back into Update, which should produce the navigation message
	cmd2 := v.Update(linkMsg)
	assert.NotNil(t, cmd2)

	msg2 := cmd2()
	navMsg, ok := msg2.(SnapshotSourceDiskSelectedMsg)
	assert.True(t, ok, "message should be of type SnapshotSourceDiskSelectedMsg")

	assert.Equal(t, "my-source-disk", navMsg.DiskName)
	assert.Equal(t, "us-central1-a", navMsg.Zone)
}
