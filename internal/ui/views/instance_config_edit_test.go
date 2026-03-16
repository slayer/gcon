package views

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

var errTestAPIError = errors.New("API error")

func TestFieldLabelToKey(t *testing.T) {
	tests := []struct {
		label    string
		expected string
	}{
		{"Machine Type", "machine_type"},
		{"Boot Disk Size (GB)", "disk_size"},
		{"Something Else", "unknown_Something Else"},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			assert.Equal(t, tt.expected, fieldLabelToKey(tt.label))
		})
	}
}

func TestInstanceConfigEditView_SetError(t *testing.T) {
	v := NewInstanceConfigEditView("proj", "my-vm", "us-central1-a", nil)
	v.state = instanceConfigEditStateSaving

	v.SetError(errTestAPIError)

	assert.Equal(t, instanceConfigEditStateForm, v.state)
	assert.Equal(t, errTestAPIError, v.err)
	assert.Contains(t, v.View(), errTestAPIError.Error())
}

func TestInstanceConfigEditView_HasTextInputFocused(t *testing.T) {
	v := NewInstanceConfigEditView("proj", "my-vm", "us-central1-a", nil)

	// No form loaded yet
	assert.False(t, v.HasTextInputFocused())

	// Simulate loading complete
	v.state = instanceConfigEditStateForm
	v.form = buildInstanceForm(forms.FormModeEdit, true)
	// Form exists but text input may or may not be focused depending on form state
	// At minimum, should not panic
	_ = v.HasTextInputFocused()
}

func TestInstanceConfigEditView_BuildDiffFields(t *testing.T) {
	v := NewInstanceConfigEditView("proj", "my-vm", "us-central1-a", nil)

	// Simulate loaded state with original details
	v.original = &gcp.InstanceDetails{
		Name:        "my-vm",
		Zone:        "us-central1-a",
		MachineType: "e2-medium",
		Status:      "TERMINATED",
		Disks: []gcp.DiskInfo{
			{Name: "my-vm", SizeGB: 20, Type: "pd-balanced", Boot: true},
		},
	}

	v.machineTypes = []gcp.MachineType{
		{Name: "e2-medium", Description: "2 vCPU, 4 GB RAM"},
		{Name: "e2-standard-4", Description: "4 vCPU, 16 GB RAM"},
	}
	v.buildForm()
	v.populateForm()

	// Change machine type in form
	mtField := v.form.GetField("machine_type")
	require.NotNil(t, mtField)
	mtField.SetValue("e2-standard-4")

	fields := v.buildDiffFields()
	require.Len(t, fields, 2)

	// Machine type should show the change
	assert.Equal(t, "Machine Type", fields[0].Label)
	assert.Equal(t, "e2-medium", fields[0].OldValue)
	assert.Equal(t, "e2-standard-4", fields[0].NewValue)
	assert.True(t, fields[0].IsChanged())

	// Disk size unchanged
	assert.Equal(t, "Boot Disk Size (GB)", fields[1].Label)
	assert.False(t, fields[1].IsChanged())
}

func TestInstanceConfigEditView_BlocksMachineTypeChangeOnRunningInstance(t *testing.T) {
	v := NewInstanceConfigEditView("proj", "my-vm", "us-central1-a", nil)

	v.original = &gcp.InstanceDetails{
		Name:        "my-vm",
		Zone:        "us-central1-a",
		MachineType: "e2-medium",
		Status:      "RUNNING",
		Disks: []gcp.DiskInfo{
			{Name: "my-vm", SizeGB: 20, Type: "pd-balanced", Boot: true},
		},
	}

	v.machineTypes = []gcp.MachineType{
		{Name: "e2-medium", Description: "2 vCPU, 4 GB RAM"},
		{Name: "e2-standard-4", Description: "4 vCPU, 16 GB RAM"},
	}
	v.buildForm()
	v.populateForm()

	// Change machine type on a running instance
	mtField := v.form.GetField("machine_type")
	require.NotNil(t, mtField)
	mtField.SetValue("e2-standard-4")

	// Should block and set error instead of showing diff
	cmd := v.showDiffPreview()
	assert.Nil(t, cmd)
	assert.Equal(t, instanceConfigEditStateForm, v.state)
	require.NotNil(t, v.err)
	assert.Contains(t, v.err.Error(), "must be stopped")
}

func TestInstanceConfigEditView_BlocksMachineTypeChangeOnSuspendedInstance(t *testing.T) {
	v := NewInstanceConfigEditView("proj", "my-vm", "us-central1-a", nil)

	v.original = &gcp.InstanceDetails{
		Name:        "my-vm",
		Zone:        "us-central1-a",
		MachineType: "e2-medium",
		Status:      "SUSPENDED",
		Disks: []gcp.DiskInfo{
			{Name: "my-vm", SizeGB: 20, Type: "pd-balanced", Boot: true},
		},
	}

	v.machineTypes = []gcp.MachineType{
		{Name: "e2-medium", Description: "2 vCPU, 4 GB RAM"},
		{Name: "e2-standard-4", Description: "4 vCPU, 16 GB RAM"},
	}
	v.buildForm()
	v.populateForm()

	mtField := v.form.GetField("machine_type")
	require.NotNil(t, mtField)
	mtField.SetValue("e2-standard-4")

	// SUSPENDED is not a stopped state — should block machine type changes
	cmd := v.showDiffPreview()
	assert.Nil(t, cmd)
	assert.Equal(t, instanceConfigEditStateForm, v.state)
	require.NotNil(t, v.err)
	assert.Contains(t, v.err.Error(), "must be stopped")
}

func TestInstanceConfigEditView_AllowsMachineTypeChangeOnStoppedInstance(t *testing.T) {
	v := NewInstanceConfigEditView("proj", "my-vm", "us-central1-a", nil)

	v.original = &gcp.InstanceDetails{
		Name:        "my-vm",
		Zone:        "us-central1-a",
		MachineType: "e2-medium",
		Status:      "TERMINATED",
		Disks: []gcp.DiskInfo{
			{Name: "my-vm", SizeGB: 20, Type: "pd-balanced", Boot: true},
		},
	}

	v.machineTypes = []gcp.MachineType{
		{Name: "e2-medium", Description: "2 vCPU, 4 GB RAM"},
		{Name: "e2-standard-4", Description: "4 vCPU, 16 GB RAM"},
	}
	v.buildForm()
	v.populateForm()
	v.width = 80
	v.height = 40

	mtField := v.form.GetField("machine_type")
	require.NotNil(t, mtField)
	mtField.SetValue("e2-standard-4")

	cmd := v.showDiffPreview()
	assert.Nil(t, cmd)
	assert.Equal(t, instanceConfigEditStateDiff, v.state)
	assert.Nil(t, v.err)
}

func TestInstanceConfigEditView_BlocksDiskShrink(t *testing.T) {
	v := NewInstanceConfigEditView("proj", "my-vm", "us-central1-a", nil)

	v.original = &gcp.InstanceDetails{
		Name:        "my-vm",
		Zone:        "us-central1-a",
		MachineType: "e2-medium",
		Status:      "TERMINATED",
		Disks: []gcp.DiskInfo{
			{Name: "my-vm", SizeGB: 50, Type: "pd-balanced", Boot: true},
		},
	}

	v.machineTypes = []gcp.MachineType{
		{Name: "e2-medium", Description: "2 vCPU, 4 GB RAM"},
	}
	v.buildForm()
	v.populateForm()

	// Try to shrink disk from 50 to 20 GB
	diskField := v.form.GetField("disk_size_gb")
	require.NotNil(t, diskField)
	diskField.SetValue(int64(20))

	cmd := v.showDiffPreview()
	assert.Nil(t, cmd)
	assert.Equal(t, instanceConfigEditStateForm, v.state)
	require.NotNil(t, v.err)
	assert.ErrorIs(t, v.err, errDiskShrinkNotAllowed)
	assert.Contains(t, v.err.Error(), "50")
}

func TestInstanceConfigEditView_AllowsDiskExpand(t *testing.T) {
	v := NewInstanceConfigEditView("proj", "my-vm", "us-central1-a", nil)

	v.original = &gcp.InstanceDetails{
		Name:        "my-vm",
		Zone:        "us-central1-a",
		MachineType: "e2-medium",
		Status:      "TERMINATED",
		Disks: []gcp.DiskInfo{
			{Name: "my-vm", SizeGB: 20, Type: "pd-balanced", Boot: true},
		},
	}

	v.machineTypes = []gcp.MachineType{
		{Name: "e2-medium", Description: "2 vCPU, 4 GB RAM"},
	}
	v.buildForm()
	v.populateForm()
	v.width = 80
	v.height = 40

	// Expand disk from 20 to 50 GB
	diskField := v.form.GetField("disk_size_gb")
	require.NotNil(t, diskField)
	diskField.SetValue(int64(50))

	cmd := v.showDiffPreview()
	assert.Nil(t, cmd)
	assert.Equal(t, instanceConfigEditStateDiff, v.state)
	assert.Nil(t, v.err)
}
