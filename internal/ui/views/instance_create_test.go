package views

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/diff"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

var errTestCreateFailed = errors.New("create failed")

func TestInstanceCreateView_SetErrorResetsState(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)
	v.State = createViewStateSaving

	v.SetError(errTestCreateFailed)

	assert.Equal(t, createViewStateForm, v.State)
	assert.Equal(t, errTestCreateFailed, v.Err)
	assert.Contains(t, v.View(), errTestCreateFailed.Error())
}

func TestInstanceCreateView_HasTextInputFocused(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)
	// Form is built in constructor, HasTextInputFocused should not panic
	result := v.HasTextInputFocused()
	// At init, form exists — result depends on form state
	_ = result
}

// setupCreateViewWithData populates a create view with valid form data.
func setupCreateViewWithData(data map[string]any) *InstanceCreateView {
	v := NewInstanceCreateView("project-id", nil)
	if f := v.Form.GetField("machine_type"); f != nil {
		f.SetOptions([]forms.Option{{Value: "e2-medium", Label: "e2-medium"}})
	}
	v.Form.SetData(data)
	return v
}

func TestInstanceCreateView_SubmitShowsConfirmation(t *testing.T) {
	v := setupCreateViewWithData(map[string]any{
		"name":         "test-instance",
		"zone":         "us-central1-a",
		"machine_type": "e2-medium",
		"image":        "debian-cloud/debian-12",
		"disk_size_gb": int64(20),
		"disk_type":    "pd-balanced",
		"network":      "default",
		"subnetwork":   "",
		"external_ip":  "ephemeral",
	})

	cmd := v.handleSubmit()
	assert.Nil(t, cmd, "handleSubmit should return nil (shows confirmation inline)")

	// Confirmation state should be active
	assert.True(t, v.showConfirm)
	assert.NotNil(t, v.diffViewer)
	assert.NotNil(t, v.pendingConfig)

	// Pending config should have extracted values
	assert.Equal(t, "test-instance", v.pendingConfig.Name)
	assert.Equal(t, "us-central1-a", v.pendingConfig.Zone)
	assert.Equal(t, "e2-medium", v.pendingConfig.MachineType)
	assert.Equal(t, "debian-cloud", v.pendingConfig.ImageProject)
	assert.Equal(t, "debian-12", v.pendingConfig.ImageFamily)
	assert.Equal(t, int64(20), v.pendingConfig.DiskSizeGB)
	assert.Equal(t, "ephemeral", v.pendingConfig.ExternalIPType)

	// View should render the diff viewer, not the form
	view := v.View()
	assert.Contains(t, view, "Confirm VM Instance Creation")
	assert.Contains(t, view, "test-instance")
}

func TestInstanceCreateView_ConfirmProceeds(t *testing.T) {
	v := setupCreateViewWithData(map[string]any{
		"name":         "test-vm",
		"zone":         "us-central1-a",
		"machine_type": "e2-medium",
		"image":        "debian-cloud/debian-12",
		"disk_size_gb": int64(10),
		"disk_type":    "pd-balanced",
		"network":      "default",
		"external_ip":  "ephemeral",
	})

	// Show confirmation
	v.handleSubmit()
	require.True(t, v.showConfirm)

	// Simulate user confirming
	cmd := v.Update(diff.ConfirmMsg{})
	require.NotNil(t, cmd, "confirm should return a batch command")

	// Should exit confirmation state and enter saving
	assert.False(t, v.showConfirm)
	assert.True(t, v.IsSaving())

	// Execute the batch and find the CreateInstanceMsg
	batchMsg := cmd()
	if batch, ok := batchMsg.(tea.BatchMsg); ok {
		var foundCreate bool
		for _, c := range batch {
			if c == nil {
				continue
			}
			msg := c()
			if createMsg, ok := msg.(CreateInstanceMsg); ok {
				foundCreate = true
				assert.Equal(t, "test-vm", createMsg.Config.Name)
				assert.Equal(t, "e2-medium", createMsg.Config.MachineType)
			}
		}
		assert.True(t, foundCreate, "batch should contain CreateInstanceMsg")
	}
}

func TestInstanceCreateView_ConfirmCancelReturnsToForm(t *testing.T) {
	v := setupCreateViewWithData(map[string]any{
		"name":         "test-vm",
		"zone":         "us-central1-a",
		"machine_type": "e2-medium",
		"image":        "debian-cloud/debian-12",
		"disk_size_gb": int64(10),
		"disk_type":    "pd-balanced",
		"network":      "default",
		"external_ip":  "ephemeral",
	})

	v.handleSubmit()
	require.True(t, v.showConfirm)

	// Simulate user canceling
	cmd := v.Update(diff.CancelMsg{})
	assert.Nil(t, cmd)
	assert.False(t, v.showConfirm)
	assert.Nil(t, v.diffViewer)
	assert.Nil(t, v.pendingConfig)
	assert.False(t, v.IsSaving())
}

func TestInstanceCreateView_ConfirmCustomMachineType(t *testing.T) {
	v := setupCreateViewWithData(map[string]any{
		"name":                "custom-vm",
		"zone":                "us-central1-a",
		"machine_type":        "e2-medium",
		"custom_machine_type": "n2-custom-8-20480",
		"image":               "debian-cloud/debian-12",
		"disk_size_gb":        int64(10),
		"disk_type":           "pd-standard",
		"network":             "default",
		"external_ip":         "none",
	})

	v.handleSubmit()
	require.NotNil(t, v.pendingConfig)

	// Custom machine type overrides dropdown
	assert.Equal(t, "n2-custom-8-20480", v.pendingConfig.MachineType)
	assert.Equal(t, "none", v.pendingConfig.ExternalIPType)

	// Confirmation view should show the custom type
	view := v.View()
	assert.Contains(t, view, "n2-custom-8-20480")
}

func TestInstanceCreateView_ConfirmShowsHumanReadableLabels(t *testing.T) {
	v := setupCreateViewWithData(map[string]any{
		"name":         "my-vm",
		"zone":         "us-central1-a",
		"machine_type": "e2-medium",
		"image":        "debian-cloud/debian-12",
		"disk_size_gb": int64(20),
		"disk_type":    "pd-balanced",
		"network":      "default",
		"external_ip":  "ephemeral",
	})

	v.handleSubmit()
	require.True(t, v.showConfirm)

	view := v.View()
	// Should show human-readable image label, not raw "debian-cloud/debian-12"
	assert.Contains(t, view, "Debian 12 (Bookworm)")
	assert.Contains(t, view, "20 GB")
	assert.Contains(t, view, "Balanced persistent disk")
	assert.Contains(t, view, "Ephemeral")
}

func TestInstanceCreateView_HandleSubmitValidationFails(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)

	// Leave required fields empty — validation should fail
	cmd := v.handleSubmit()
	assert.Nil(t, cmd, "should return nil when validation fails")
	assert.False(t, v.showConfirm, "should not show confirmation on validation failure")
}

func TestInstanceCreateView_MachineTypeCache(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)

	types := []gcp.MachineType{
		{Name: "e2-micro", Description: "2 vCPU, 1 GB RAM"},
		{Name: "e2-medium", Description: "2 vCPU, 4 GB RAM"},
	}

	// Simulate loading machine types
	v.lastZone = "us-central1-a"
	v.machineTypeCache["us-central1-a"] = types

	// Cache hit should update dropdown without fetching
	cmd := v.onZoneChanged("us-central1-a")

	// Should still return a command for subnetwork fetching
	// but machine types should be from cache
	_ = cmd

	mtField := v.Form.GetField("machine_type")
	require.NotNil(t, mtField)
	assert.Len(t, mtField.Options, 2)
	assert.Equal(t, "e2-micro", mtField.Options[0].Value)
}

func TestInstanceCreateView_OnImageChangedUpdatesDiskSize(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)

	// Select Windows image — should set disk size to 50
	v.onImageChanged("windows-cloud/windows-2022")

	data := v.Form.GetData()
	assert.Equal(t, int64(50), data["disk_size_gb"])

	// Switch to Debian — should set to 10
	v.onImageChanged("debian-cloud/debian-12")
	data = v.Form.GetData()
	assert.Equal(t, int64(10), data["disk_size_gb"])
}

func TestInstanceCreateView_CancelEmitsMessage(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)

	cmd := v.Update(forms.FormCancelMsg{})
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(InstanceCreateCanceledMsg)
	assert.True(t, ok, "cancel should emit InstanceCreateCanceledMsg")
}
