package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/stretchr/testify/assert"
)

func TestSnapshotStatusIcon(t *testing.T) {
	tests := []struct {
		name     string
		snapshot gcp.Snapshot
	}{
		{
			name:     "ready snapshot",
			snapshot: gcp.Snapshot{Status: "READY"},
		},
		{
			name:     "creating snapshot",
			snapshot: gcp.Snapshot{Status: "CREATING"},
		},
		{
			name:     "uploading snapshot",
			snapshot: gcp.Snapshot{Status: "UPLOADING"},
		},
		{
			name:     "failed snapshot",
			snapshot: gcp.Snapshot{Status: "FAILED"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snapshotStatusIcon(tt.snapshot)
			// Just verify the icon is not empty
			assert.NotEmpty(t, got, "status icon should not be empty")
		})
	}
}

func TestSnapshotToRow(t *testing.T) {
	tests := []struct {
		name     string
		snapshot gcp.Snapshot
		validate func(t *testing.T, row table.Row)
	}{
		{
			name: "basic snapshot",
			snapshot: gcp.Snapshot{
				Name:       "snapshot-1",
				SourceDisk: "my-disk",
				SizeGB:     100,
				Status:     "READY",
				CreatedAt:  "2025-01-15T10:00:00Z",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Len(t, row.Data, 5)
				assert.Contains(t, row.Data[0], "snapshot-1")
				// Status icon is prepended to name
				assert.NotEqual(t, "snapshot-1", row.Data[0], "status icon should be prepended")
				assert.Equal(t, "my-disk", row.Data[1])
				assert.Equal(t, "100 GB", row.Data[2])
				assert.Equal(t, "READY", row.Data[4])
				assert.Equal(t, "snapshot-1", row.ID)
				assert.Contains(t, row.FilterValue, "snapshot-1")
				assert.Contains(t, row.FilterValue, "my-disk")
			},
		},
		{
			name: "snapshot without source disk",
			snapshot: gcp.Snapshot{
				Name:       "snapshot-2",
				SourceDisk: "",
				SizeGB:     500,
				Status:     "CREATING",
				CreatedAt:  "2025-01-15T11:00:00Z",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "-", row.Data[1]) // Source disk should be "-"
				assert.Equal(t, "500 GB", row.Data[2])
				assert.Equal(t, "CREATING", row.Data[4])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := snapshotToRow(tt.snapshot)
			tt.validate(t, row)
		})
	}
}

func TestNewSnapshotsView(t *testing.T) {
	projectID := "test-project"
	view := NewSnapshotsView(projectID)

	assert.NotNil(t, view)
	assert.Equal(t, projectID, view.projectID)
	assert.True(t, view.loading)
	assert.Nil(t, view.err)
	assert.Empty(t, view.snapshots)
}

func TestSnapshotsViewLoadedState(t *testing.T) {
	view := NewSnapshotsView("test-project")

	// Simulate snapshots loaded
	snapshots := []gcp.Snapshot{
		{Name: "snap-1", SourceDisk: "disk-1", SizeGB: 100, Status: "READY"},
		{Name: "snap-2", SourceDisk: "disk-2", SizeGB: 200, Status: "CREATING"},
	}
	msg := snapshotsLoadedMsg{snapshots: snapshots}
	view.Update(msg)

	assert.False(t, view.loading)
	assert.Nil(t, view.err)
	assert.Len(t, view.snapshots, 2)
	assert.Equal(t, "snap-1", view.snapshots[0].Name)
	assert.Equal(t, "snap-2", view.snapshots[1].Name)
}

func TestSnapshotsViewErrorState(t *testing.T) {
	view := NewSnapshotsView("test-project")

	// Simulate error
	testErr := assert.AnError
	msg := snapshotsErrorMsg{err: testErr}
	view.Update(msg)

	assert.False(t, view.loading)
	assert.Equal(t, testErr, view.err)
}

func TestFindSnapshotByName(t *testing.T) {
	view := NewSnapshotsView("test-project")
	view.snapshots = []gcp.Snapshot{
		{Name: "snap-1", SourceDisk: "disk-1"},
		{Name: "snap-2", SourceDisk: "disk-2"},
	}

	t.Run("existing snapshot", func(t *testing.T) {
		snapshot := view.findSnapshotByName("snap-1")
		assert.NotNil(t, snapshot)
		assert.Equal(t, "snap-1", snapshot.Name)
		assert.Equal(t, "disk-1", snapshot.SourceDisk)
	})

	t.Run("non-existing snapshot", func(t *testing.T) {
		snapshot := view.findSnapshotByName("snap-999")
		assert.Nil(t, snapshot)
	})
}

func TestSnapshotsViewRendering(t *testing.T) {
	view := NewSnapshotsView("test-project")

	t.Run("loading with no client", func(t *testing.T) {
		view.loading = true
		view.computeClient = nil
		output := view.View()
		assert.Contains(t, output, "Initializing Compute Engine client")
	})

	t.Run("loading with client", func(t *testing.T) {
		view.loading = true
		view.computeClient = &gcp.ComputeClient{}
		output := view.View()
		assert.Contains(t, output, "Loading snapshots")
	})

	t.Run("error state", func(t *testing.T) {
		view.loading = false
		view.err = assert.AnError
		output := view.View()
		assert.NotEmpty(t, output)
	})

	t.Run("empty snapshots", func(t *testing.T) {
		view.loading = false
		view.err = nil
		view.snapshots = []gcp.Snapshot{}
		output := view.View()
		assert.Contains(t, output, "No snapshots found")
	})
}
