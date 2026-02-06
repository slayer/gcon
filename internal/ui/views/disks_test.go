package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
)

func TestDisksView_NewDisksView(t *testing.T) {
	v := NewDisksView("test-project")

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.True(t, v.loading, "View should start in loading state")
}

func TestDisksView_RenderLoading(t *testing.T) {
	v := NewDisksView("test-project")
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30}
	v.SetContext(ctx)

	output := renderLoading(v.spinner, "Loading disks...")

	assert.Contains(t, output, "Loading disks...")
	assert.Contains(t, output, v.spinner.View())
}

func TestDiskToRow(t *testing.T) {
	tests := []struct {
		name     string
		disk     gcp.Disk
		validate func(t *testing.T, row table.Row)
	}{
		{
			name: "attached disk",
			disk: gcp.Disk{
				Name:       "boot-disk",
				Zone:       "us-central1-a",
				SizeGB:     100,
				Type:       "pd-ssd",
				Status:     "READY",
				AttachedTo: "my-instance",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Contains(t, row.Data[0], "boot-disk")
				assert.Equal(t, "us-central1-a", row.Data[1])
				assert.Equal(t, "100 GB", row.Data[2])
				assert.Equal(t, "pd-ssd", row.Data[3])
				assert.Equal(t, "my-instance", row.Data[4])
				assert.Equal(t, "boot-disk", row.ID)
			},
		},
		{
			name: "detached disk",
			disk: gcp.Disk{
				Name:       "data-disk",
				Zone:       "us-east1-b",
				SizeGB:     500,
				Type:       "pd-balanced",
				Status:     "READY",
				AttachedTo: "",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Contains(t, row.Data[0], "data-disk")
				assert.Equal(t, "-", row.Data[4]) // Not attached shows "-"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := diskToRow(tt.disk)
			tt.validate(t, row)
		})
	}
}

func TestDiskStatusIcon(t *testing.T) {
	// Attached disk should show running (green) indicator
	attachedDisk := gcp.Disk{Status: "READY", AttachedTo: "vm-1"}
	icon := diskStatusIcon(attachedDisk)
	assert.NotEmpty(t, icon)

	// Detached ready disk should show stopped (red) indicator
	detachedDisk := gcp.Disk{Status: "READY", AttachedTo: ""}
	icon2 := diskStatusIcon(detachedDisk)
	assert.NotEmpty(t, icon2)

	// Creating disk should show transitioning indicator
	creatingDisk := gcp.Disk{Status: "CREATING", AttachedTo: ""}
	icon3 := diskStatusIcon(creatingDisk)
	assert.NotEmpty(t, icon3)
}
