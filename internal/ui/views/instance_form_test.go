package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildInstanceForm_CreateMode(t *testing.T) {
	f := buildInstanceForm(forms.FormModeCreate, false)
	require.NotNil(t, f)

	// Name is editable text field in create mode
	nameField := f.GetField("name")
	require.NotNil(t, nameField, "name field must exist")
	assert.True(t, nameField.Required)

	// Zone is editable dropdown in create mode
	zoneField := f.GetField("zone")
	require.NotNil(t, zoneField, "zone field must exist")
	assert.True(t, zoneField.Required)
	assert.NotEmpty(t, zoneField.Options, "zone should have options pre-populated")

	// Image is dropdown in create mode
	imageField := f.GetField("image")
	require.NotNil(t, imageField)
	assert.True(t, imageField.Required)
	assert.NotEmpty(t, imageField.Options)

	// Disk type is dropdown in create mode
	diskTypeField := f.GetField("disk_type")
	require.NotNil(t, diskTypeField)
	assert.True(t, diskTypeField.Required)

	// Network defaults
	netField := f.GetField("network")
	require.NotNil(t, netField)

	// External IP with options
	eipField := f.GetField("external_ip")
	require.NotNil(t, eipField)
	assert.Len(t, eipField.Options, 2)
}

func TestBuildInstanceForm_EditMode(t *testing.T) {
	f := buildInstanceForm(forms.FormModeEdit, true)
	require.NotNil(t, f)

	// Name is read-only in edit mode
	nameField := f.GetField("name")
	require.NotNil(t, nameField, "name field must exist in edit mode")
	assert.Equal(t, forms.FieldReadOnly, nameField.Type)

	// Zone is read-only in edit mode
	zoneField := f.GetField("zone")
	require.NotNil(t, zoneField)
	assert.Equal(t, forms.FieldReadOnly, zoneField.Type)

	// Image is read-only in edit mode
	imageField := f.GetField("image")
	require.NotNil(t, imageField)
	assert.Equal(t, forms.FieldReadOnly, imageField.Type)

	// Disk type is read-only in edit mode
	diskTypeField := f.GetField("disk_type")
	require.NotNil(t, diskTypeField)
	assert.Equal(t, forms.FieldReadOnly, diskTypeField.Type)

	// Disk size is still editable (can expand)
	diskSizeField := f.GetField("disk_size_gb")
	require.NotNil(t, diskSizeField)
	assert.Equal(t, forms.FieldNumber, diskSizeField.Type)

	// Machine type is editable dropdown in edit mode
	mtField := f.GetField("machine_type")
	require.NotNil(t, mtField)
	assert.Equal(t, forms.FieldDropdown, mtField.Type)
}

func TestDefaultDiskSizeForImage(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected int64
	}{
		{name: "debian", image: "debian-cloud/debian-12", expected: 10},
		{name: "windows", image: "windows-cloud/windows-2022", expected: 50},
		{name: "centos", image: "centos-cloud/centos-stream-9", expected: 20},
		{name: "unknown fallback", image: "unknown/image", expected: 10},
		{name: "empty", image: "", expected: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, defaultDiskSizeForImage(tt.image))
		})
	}
}

func TestPopulateInstanceFormFromDetails(t *testing.T) {
	t.Run("populates basic fields", func(t *testing.T) {
		f := buildInstanceForm(forms.FormModeEdit, true)

		// Pre-populate machine type options so SetValue can match
		mtField := f.GetField("machine_type")
		require.NotNil(t, mtField)
		mtField.SetOptions([]forms.Option{
			{Value: "e2-medium", Label: "e2-medium (2 vCPU, 4 GB RAM)"},
			{Value: "e2-small", Label: "e2-small (2 vCPU, 2 GB RAM)"},
		})

		details := &gcp.InstanceDetails{
			Name:        "test-vm",
			Zone:        "us-central1-a",
			MachineType: "e2-medium",
			Disks: []gcp.DiskInfo{
				{Name: "test-vm", SizeGB: 50, Type: "pd-ssd", Boot: true},
			},
			NetworkInterfaces: []gcp.NetworkInterfaceInfo{
				{Network: "default", Subnetwork: "default", ExternalIP: "34.1.2.3"},
			},
		}

		populateInstanceFormFromDetails(f, details)

		data := f.GetData()
		assert.Equal(t, "test-vm", data["name"])
		assert.Equal(t, "us-central1-a", data["zone"])
		assert.Equal(t, "e2-medium", data["machine_type"])
		assert.Equal(t, int64(50), data["disk_size_gb"])
	})

	t.Run("custom machine type falls back to custom field", func(t *testing.T) {
		f := buildInstanceForm(forms.FormModeEdit, true)

		// Options don't include the custom type
		mtField := f.GetField("machine_type")
		require.NotNil(t, mtField)
		mtField.SetOptions([]forms.Option{
			{Value: "e2-medium", Label: "e2-medium"},
		})

		details := &gcp.InstanceDetails{
			Name:        "custom-vm",
			Zone:        "us-central1-a",
			MachineType: "n2-custom-8-20480",
			Disks:       []gcp.DiskInfo{{Boot: true, SizeGB: 10, Type: "pd-balanced"}},
		}

		populateInstanceFormFromDetails(f, details)

		data := f.GetData()
		// Machine type dropdown should NOT be set to the custom type
		assert.NotEqual(t, "n2-custom-8-20480", data["machine_type"])
		// Custom field should have the value instead
		assert.Equal(t, "n2-custom-8-20480", data["custom_machine_type"])
	})

	t.Run("nil inputs are safe", func(t *testing.T) {
		// Should not panic
		populateInstanceFormFromDetails(nil, nil)
		populateInstanceFormFromDetails(buildInstanceForm(forms.FormModeEdit, true), nil)
		populateInstanceFormFromDetails(nil, &gcp.InstanceDetails{})
	})
}

func TestMachineTypeDropdownOptions(t *testing.T) {
	types := []gcp.MachineType{
		{Name: "e2-micro", Description: "2 vCPU, 1 GB RAM", CPUs: 2, MemoryMB: 1024},
		{Name: "e2-medium", Description: "2 vCPU, 4 GB RAM", CPUs: 2, MemoryMB: 4096},
	}

	opts := machineTypeDropdownOptions(types)
	require.Len(t, opts, 2)
	assert.Equal(t, "e2-micro", opts[0].Value)
	assert.Contains(t, opts[0].Label, "2 vCPU, 1 GB RAM")
	assert.Equal(t, "e2-medium", opts[1].Value)
}

func TestSubnetworkDropdownOptions(t *testing.T) {
	subnets := []gcp.SubnetworkInfo{
		{Name: "default", IPRange: "10.128.0.0/20", Network: "default"},
		{Name: "custom-subnet", IPRange: "10.0.0.0/24", Network: "my-vpc"},
	}

	opts := subnetworkDropdownOptions(subnets)
	require.Len(t, opts, 2)
	assert.Equal(t, "default", opts[0].Value)
	assert.Contains(t, opts[0].Label, "10.128.0.0/20")
	assert.Equal(t, "custom-subnet", opts[1].Value)
}

func TestImageLabelFromValue(t *testing.T) {
	assert.Equal(t, "Debian 12 (Bookworm)", imageLabelFromValue("debian-cloud/debian-12"))
	assert.Equal(t, "Windows Server 2022", imageLabelFromValue("windows-cloud/windows-2022"))
	assert.Equal(t, "unknown/image", imageLabelFromValue("unknown/image"))
}

func TestDiskTypeLabelFromValue(t *testing.T) {
	assert.Contains(t, diskTypeLabelFromValue("pd-balanced"), "Balanced")
	assert.Contains(t, diskTypeLabelFromValue("pd-ssd"), "SSD")
	assert.Equal(t, "pd-extreme", diskTypeLabelFromValue("pd-extreme"))
}

func TestBuildInstanceForm_CreateMode_IPFields(t *testing.T) {
	f := buildInstanceForm(forms.FormModeCreate, false)

	// Internal IP type dropdown exists with auto/custom options
	ipTypeField := f.GetField("internal_ip_type")
	require.NotNil(t, ipTypeField, "internal_ip_type field must exist in create mode")
	assert.Equal(t, forms.FieldDropdown, ipTypeField.Type)
	assert.Len(t, ipTypeField.Options, 2)
	assert.Equal(t, "auto", ipTypeField.Options[0].Value)
	assert.Equal(t, "custom", ipTypeField.Options[1].Value)

	// Custom internal IP field exists but is hidden
	ipField := f.GetField("internal_ip")
	require.NotNil(t, ipField, "internal_ip field must exist in create mode")
	assert.True(t, ipField.Hidden, "internal_ip should start hidden")
	assert.Equal(t, forms.FieldText, ipField.Type)
}

func TestBuildInstanceForm_EditMode_IPFields(t *testing.T) {
	f := buildInstanceForm(forms.FormModeEdit, true)

	// Internal IP is read-only in edit mode
	ipField := f.GetField("internal_ip")
	require.NotNil(t, ipField, "internal_ip field must exist in edit mode")
	assert.Equal(t, forms.FieldReadOnly, ipField.Type)

	// No internal_ip_type field in edit mode (not needed)
	assert.Nil(t, f.GetField("internal_ip_type"))
}

func TestPopulateInstanceFormFromDetails_InternalIP(t *testing.T) {
	f := buildInstanceForm(forms.FormModeEdit, true)

	details := &gcp.InstanceDetails{
		Name:        "test-vm",
		Zone:        "us-central1-a",
		MachineType: "e2-medium",
		Disks:       []gcp.DiskInfo{{Boot: true, SizeGB: 10, Type: "pd-balanced"}},
		NetworkInterfaces: []gcp.NetworkInterfaceInfo{
			{Network: "default", Subnetwork: "default", InternalIP: "10.128.0.42", ExternalIP: "35.1.2.3"},
		},
	}

	// Pre-populate machine type options
	if mt := f.GetField("machine_type"); mt != nil {
		mt.SetOptions([]forms.Option{{Value: "e2-medium", Label: "e2-medium"}})
	}

	populateInstanceFormFromDetails(f, details)

	data := f.GetData()
	assert.Equal(t, "10.128.0.42", data["internal_ip"])
	assert.Equal(t, "35.1.2.3", data["external_ip"])
}

func TestExternalIPDropdownOptions(t *testing.T) {
	t.Run("no static addresses", func(t *testing.T) {
		opts := externalIPDropdownOptions(nil)
		assert.Len(t, opts, 2)
		assert.Equal(t, "ephemeral", opts[0].Value)
		assert.Equal(t, "none", opts[1].Value)
	})

	t.Run("with static external addresses", func(t *testing.T) {
		addrs := []gcp.StaticAddress{
			{Name: "web-ip", Address: "35.192.0.1", AddressType: "EXTERNAL"},
			{Name: "internal-ip", Address: "10.0.0.5", AddressType: "INTERNAL"},
			{Name: "api-ip", Address: "35.192.0.2", AddressType: "EXTERNAL"},
		}
		opts := externalIPDropdownOptions(addrs)
		// ephemeral + none + 2 external static addresses (internal filtered out)
		assert.Len(t, opts, 4)
		assert.Equal(t, "ephemeral", opts[0].Value)
		assert.Equal(t, "none", opts[1].Value)
		assert.Equal(t, "static:web-ip", opts[2].Value)
		assert.Contains(t, opts[2].Label, "35.192.0.1")
		assert.Equal(t, "static:api-ip", opts[3].Value)
	})
}
