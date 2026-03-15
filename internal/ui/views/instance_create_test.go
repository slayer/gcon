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
var errStaleZone = errors.New("stale zone error")
var errCurrentZone = errors.New("current zone error")

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

func TestInstanceCreateView_ExtractConfigStaticExternalIP(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)
	if f := v.Form.GetField("machine_type"); f != nil {
		f.SetOptions([]forms.Option{{Value: "e2-medium", Label: "e2-medium"}})
	}

	// Add static IP option to dropdown before SetData (dropdown ignores unknown values)
	addresses := []gcp.StaticAddress{
		{Name: "my-static-ip", Address: "35.192.0.1", AddressType: "EXTERNAL"},
	}
	if eipField := v.Form.GetField("external_ip"); eipField != nil {
		eipField.SetOptions(externalIPDropdownOptions(addresses))
	}
	v.lastLoadedAddresses = addresses

	v.Form.SetData(map[string]any{
		"name":         "static-ip-vm",
		"zone":         "us-central1-a",
		"machine_type": "e2-medium",
		"image":        "debian-cloud/debian-12",
		"disk_size_gb": int64(10),
		"disk_type":    "pd-balanced",
		"network":      "default",
		"external_ip":  "static:my-static-ip",
	})

	v.handleSubmit()
	require.NotNil(t, v.pendingConfig)

	assert.Equal(t, "my-static-ip", v.pendingConfig.ExternalIPType)
	assert.Equal(t, "35.192.0.1", v.pendingConfig.ExternalIPAddr)
}

func TestInstanceCreateView_ExtractConfigCustomInternalIP(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)

	// Populate machine type options for SetData to work
	if f := v.Form.GetField("machine_type"); f != nil {
		f.SetOptions([]forms.Option{{Value: "e2-medium", Label: "e2-medium"}})
	}
	// Populate internal_ip_type options (already set in buildForm)

	v.Form.SetData(map[string]any{
		"name":             "custom-ip-vm",
		"zone":             "us-central1-a",
		"machine_type":     "e2-medium",
		"image":            "debian-cloud/debian-12",
		"disk_size_gb":     int64(10),
		"disk_type":        "pd-balanced",
		"network":          "default",
		"external_ip":      "ephemeral",
		"internal_ip_type": "custom",
	})

	// Unhide and set the internal IP field
	if ipField := v.Form.GetField("internal_ip"); ipField != nil {
		ipField.SetHidden(false)
		ipField.SetValue("10.128.0.50")
	}

	v.handleSubmit()
	require.NotNil(t, v.pendingConfig)

	assert.Equal(t, "10.128.0.50", v.pendingConfig.InternalIP)
	assert.Equal(t, "ephemeral", v.pendingConfig.ExternalIPType)
}

func TestInstanceCreateView_ExtractConfigAutoInternalIP(t *testing.T) {
	v := setupCreateViewWithData(map[string]any{
		"name":             "auto-ip-vm",
		"zone":             "us-central1-a",
		"machine_type":     "e2-medium",
		"image":            "debian-cloud/debian-12",
		"disk_size_gb":     int64(10),
		"disk_type":        "pd-balanced",
		"network":          "default",
		"external_ip":      "ephemeral",
		"internal_ip_type": "auto",
	})

	v.handleSubmit()
	require.NotNil(t, v.pendingConfig)

	// Auto mode → empty internal IP
	assert.Empty(t, v.pendingConfig.InternalIP)
}

func TestInstanceCreateView_InternalIPTypeFieldChange(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)

	ipField := v.Form.GetField("internal_ip")
	require.NotNil(t, ipField)
	assert.True(t, ipField.Hidden, "should start hidden")

	// Simulate selecting "custom" in internal_ip_type dropdown
	v.handleFieldChanged(forms.FieldChangedMsg{FieldID: "internal_ip_type", Value: "custom"})
	assert.False(t, ipField.Hidden, "should be visible after selecting custom")

	// Switch back to auto
	v.handleFieldChanged(forms.FieldChangedMsg{FieldID: "internal_ip_type", Value: "auto"})
	assert.True(t, ipField.Hidden, "should be hidden again after selecting auto")
}

func TestInstanceCreateView_ConfirmShowsIPInfo(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)
	if f := v.Form.GetField("machine_type"); f != nil {
		f.SetOptions([]forms.Option{{Value: "e2-medium", Label: "e2-medium"}})
	}
	v.Form.SetData(map[string]any{
		"name":             "ip-test-vm",
		"zone":             "us-central1-a",
		"machine_type":     "e2-medium",
		"image":            "debian-cloud/debian-12",
		"disk_size_gb":     int64(10),
		"disk_type":        "pd-balanced",
		"network":          "default",
		"external_ip":      "none",
		"internal_ip_type": "custom",
	})
	if ipField := v.Form.GetField("internal_ip"); ipField != nil {
		ipField.SetHidden(false)
		ipField.SetValue("10.128.0.99")
	}

	v.handleSubmit()
	require.True(t, v.showConfirm)

	view := v.View()
	assert.Contains(t, view, "10.128.0.99", "custom internal IP should appear in confirmation")
	assert.Contains(t, view, "None", "external IP should show None")
}

func TestInstanceCreateView_AddressesLoadedUpdatesDropdown(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)
	v.lastRegion = "us-central1" // must match so handler doesn't drop as stale

	eipField := v.Form.GetField("external_ip")
	require.NotNil(t, eipField)
	assert.Len(t, eipField.Options, 2, "should start with ephemeral + none")

	// Simulate addresses loaded for matching region
	v.Update(instanceAddressesLoadedMsg{
		region: "us-central1",
		addresses: []gcp.StaticAddress{
			{Name: "web-ip", Address: "35.1.2.3", AddressType: "EXTERNAL"},
		},
	})

	assert.Len(t, eipField.Options, 3, "should now have ephemeral + none + static")
	assert.Equal(t, "static:web-ip", eipField.Options[2].Value)
}

func TestInstanceCreateView_StaleRegionResponseDropped(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)
	v.lastRegion = "europe-west1" // current region

	// Stale response from a different region should be dropped
	v.Update(instanceSubnetworksLoadedMsg{
		region:  "us-central1",
		subnets: []gcp.SubnetworkInfo{{Name: "stale-subnet", IPRange: "10.0.0.0/20", Network: "default"}},
	})
	assert.Nil(t, v.lastLoadedSubnets, "stale subnets should not be applied")

	v.Update(instanceAddressesLoadedMsg{
		region:    "us-central1",
		addresses: []gcp.StaticAddress{{Name: "stale-ip", Address: "35.1.2.3"}},
	})
	assert.Nil(t, v.lastLoadedAddresses, "stale addresses should not be applied")
}

func TestInstanceCreateView_StaleZoneErrorDropped(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)
	v.lastZone = "europe-west1-b"

	// Error from an old zone should not surface
	v.Update(instanceMachineTypesErrorMsg{zone: "us-central1-a", err: errStaleZone})
	assert.Nil(t, v.Err, "stale machine type error should be dropped")

	// Error from current zone should surface
	v.Update(instanceMachineTypesErrorMsg{zone: "europe-west1-b", err: errCurrentZone})
	assert.ErrorIs(t, v.Err, errCurrentZone)
}

func TestInstanceCreateView_StaticIPUnresolvedError(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)
	if f := v.Form.GetField("machine_type"); f != nil {
		f.SetOptions([]forms.Option{{Value: "e2-medium", Label: "e2-medium"}})
	}

	// Add static IP option but do NOT populate lastLoadedAddresses
	if eipField := v.Form.GetField("external_ip"); eipField != nil {
		eipField.SetOptions([]forms.Option{
			{Value: "ephemeral", Label: "Ephemeral"},
			{Value: "none", Label: "None"},
			{Value: "static:missing-ip", Label: "missing-ip"},
		})
	}

	v.Form.SetData(map[string]any{
		"name":         "bad-static-vm",
		"zone":         "us-central1-a",
		"machine_type": "e2-medium",
		"image":        "debian-cloud/debian-12",
		"disk_size_gb": int64(10),
		"disk_type":    "pd-balanced",
		"network":      "default",
		"external_ip":  "static:missing-ip",
	})

	cmd := v.handleSubmit()
	assert.Nil(t, cmd, "should return nil on error")
	assert.False(t, v.showConfirm, "should not show confirmation")
	assert.ErrorIs(t, v.Err, errStaticIPUnresolved)
	assert.Contains(t, v.Err.Error(), "missing-ip")
}

func TestInstanceCreateView_ConfirmPassesProjectID(t *testing.T) {
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

	cmd := v.Update(diff.ConfirmMsg{})
	require.NotNil(t, cmd)

	batchMsg := cmd()
	if batch, ok := batchMsg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			msg := c()
			if createMsg, ok := msg.(CreateInstanceMsg); ok {
				assert.Equal(t, "project-id", createMsg.ProjectID)
				return
			}
		}
		t.Fatal("CreateInstanceMsg not found in batch")
	}
}

func TestRestoreDropdownSelection_NoMatch(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)
	v.lastRegion = "us-central1"

	eipField := v.Form.GetField("external_ip")
	require.NotNil(t, eipField)

	// Select a static IP
	eipField.SetOptions([]forms.Option{
		{Value: "ephemeral", Label: "Ephemeral"},
		{Value: "none", Label: "None"},
		{Value: "static:old-ip", Label: "old-ip (35.1.2.3)"},
	})
	eipField.SetValue("static:old-ip")
	assert.Equal(t, "static:old-ip", eipField.GetValue())

	// New region has different addresses — old static IP gone
	v.Update(instanceAddressesLoadedMsg{
		region: "us-central1",
		addresses: []gcp.StaticAddress{
			{Name: "new-ip", Address: "35.5.6.7", AddressType: "EXTERNAL"},
		},
	})

	// Should NOT silently select "static:new-ip" — should fall back to
	// option[0] ("ephemeral") since old value doesn't exist in new options
	val := eipField.GetValue()
	assert.Equal(t, "ephemeral", val, "should fall back to first option, not silently remap")
}

func TestInstanceCreateView_CancelEmitsMessage(t *testing.T) {
	v := NewInstanceCreateView("project-id", nil)

	cmd := v.Update(forms.FormCancelMsg{})
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(InstanceCreateCanceledMsg)
	assert.True(t, ok, "cancel should emit InstanceCreateCanceledMsg")
}
