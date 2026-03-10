package views

import (
	"fmt"
	"strings"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

// buildInstanceForm creates the form shared by create and edit views.
// Fields marked read-only in edit mode cannot be changed after creation.
func buildInstanceForm(mode forms.FormMode, isEdit bool) *forms.Form {
	title := "Create VM Instance"
	if isEdit {
		title = "Edit VM Instance"
	}

	f := forms.NewForm(title, mode).EnableViewport()

	// Section 1: Basic Settings
	basicSection := forms.NewSection("basic", "Basic Settings")
	if isEdit {
		basicSection.AddField(forms.NewReadOnlyField("name", "Name", ""))
		basicSection.AddField(forms.NewReadOnlyField("zone", "Zone", ""))
	} else {
		basicSection.AddField(forms.NewTextField("name", "Name").
			SetRequired(true).
			SetPlaceholder("my-instance").
			SetHelpText("1-63 characters, lowercase letters, numbers, and hyphens").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateGCPResourceName,
			)))
		basicSection.AddField(forms.NewDropdownField("zone", "Zone").
			SetRequired(true).
			SetOptionsFromStrings(computeZones()).
			SetHelpText("Zone where the instance will be created"))
	}
	f.AddSection(basicSection)

	// Section 2: Machine Configuration
	machineSection := forms.NewSection("machine", "Machine Configuration")
	if isEdit {
		// Machine type can only be changed when instance is stopped
		machineSection.AddField(forms.NewDropdownField("machine_type", "Machine Type").
			SetRequired(true).
			SetHelpText("Instance must be stopped to change machine type"))
	} else {
		machineSection.AddField(forms.NewDropdownField("machine_type", "Machine Type").
			SetRequired(true).
			SetHelpText("Select zone first to see available types"))
	}
	machineSection.AddField(forms.NewTextField("custom_machine_type", "Custom Machine Type").
		SetPlaceholder("e.g., c3-standard-44").
		SetHelpText("Overrides dropdown if set"))
	f.AddSection(machineSection)

	// Section 3: Boot Disk
	diskSection := forms.NewSection("disk", "Boot Disk")
	if isEdit {
		diskSection.AddField(forms.NewReadOnlyField("image", "Image", ""))
		diskSection.AddField(forms.NewNumberField("disk_size_gb", "Disk Size (GB)").
			SetRequired(true).
			SetValidator(forms.ValidateNumber(10, 65536)).
			SetHelpText("Can only increase, not decrease"))
		diskSection.AddField(forms.NewReadOnlyField("disk_type", "Disk Type", ""))
	} else {
		diskSection.AddField(forms.NewDropdownField("image", "Image").
			SetRequired(true).
			SetOptions(imageDropdownOptions()).
			SetHelpText("OS image for the boot disk"))
		diskSection.AddField(forms.NewNumberField("disk_size_gb", "Disk Size (GB)").
			SetRequired(true).
			SetValue(int64(10)).
			SetValidator(forms.ValidateNumber(10, 65536)).
			SetHelpText("Minimum 10 GB"))
		diskSection.AddField(forms.NewDropdownField("disk_type", "Disk Type").
			SetRequired(true).
			SetOptions(diskTypeDropdownOptions()).
			SetValue("pd-balanced").
			SetHelpText("Type of persistent disk"))
	}
	f.AddSection(diskSection)

	// Section 4: Networking (collapsible in create, read-only in edit)
	netSection := forms.NewSection("networking", "Networking")
	if isEdit {
		netSection.SetCollapsible(true).SetCollapsed(true)
		netSection.AddField(forms.NewReadOnlyField("network", "Network", ""))
		netSection.AddField(forms.NewReadOnlyField("subnetwork", "Subnetwork", ""))
		netSection.AddField(forms.NewReadOnlyField("external_ip", "External IP", ""))
	} else {
		netSection.SetCollapsible(true).SetCollapsed(true)
		netSection.AddField(forms.NewDropdownField("network", "Network").
			SetRequired(true).
			SetOptionsFromStrings([]string{"default"}).
			SetValue("default").
			SetHelpText("VPC network"))
		netSection.AddField(forms.NewDropdownField("subnetwork", "Subnetwork").
			SetHelpText("Auto-selected for auto-mode networks"))
		netSection.AddField(forms.NewDropdownField("external_ip", "External IP").
			SetRequired(true).
			SetOptions([]forms.Option{
				{Value: "ephemeral", Label: "Ephemeral"},
				{Value: "none", Label: "None"},
			}).
			SetValue("ephemeral").
			SetHelpText("External IP allocation"))
	}
	f.AddSection(netSection)

	return f
}

// imageDropdownOptions builds dropdown options from the curated BootDiskImages list.
func imageDropdownOptions() []forms.Option {
	opts := make([]forms.Option, 0, len(gcp.BootDiskImages))
	for _, img := range gcp.BootDiskImages {
		opts = append(opts, forms.Option{
			Value: img.Project + "/" + img.Family,
			Label: img.Label,
		})
	}
	return opts
}

// diskTypeDropdownOptions builds dropdown options from the standard DiskTypes list.
func diskTypeDropdownOptions() []forms.Option {
	opts := make([]forms.Option, 0, len(gcp.DiskTypes))
	for _, dt := range gcp.DiskTypes {
		opts = append(opts, forms.Option{
			Value: dt.Value,
			Label: dt.Label,
		})
	}
	return opts
}

// defaultDiskSizeForImage returns the recommended minimum disk size for a given image value.
// imageValue is "project/family" format (e.g., "debian-cloud/debian-12").
func defaultDiskSizeForImage(imageValue string) int64 {
	for _, img := range gcp.BootDiskImages {
		if img.Project+"/"+img.Family == imageValue {
			return img.DefaultSizeGB
		}
	}
	return 10
}

// populateInstanceFormFromDetails fills a form with values from an existing instance.
// Used by the edit view to pre-populate the form with current configuration.
func populateInstanceFormFromDetails(f *forms.Form, details *gcp.InstanceDetails) {
	if f == nil || details == nil {
		return
	}

	// Basic Settings — read-only in edit mode
	if field := f.GetField("name"); field != nil {
		field.SetValue(details.Name)
	}
	if field := f.GetField("zone"); field != nil {
		field.SetValue(details.Zone)
	}

	// Machine Type — try to find in dropdown options, fall back to custom field
	if field := f.GetField("machine_type"); field != nil {
		field.SetValue(details.MachineType)
	}

	// Boot Disk — first disk marked as boot
	for _, disk := range details.Disks {
		if disk.Boot {
			if field := f.GetField("disk_size_gb"); field != nil {
				field.SetValue(disk.SizeGB)
			}
			if field := f.GetField("disk_type"); field != nil {
				field.SetValue(disk.Type)
			}
			if field := f.GetField("image"); field != nil {
				// In edit mode this is read-only, show the source disk name
				field.SetValue(fmt.Sprintf("%s (%s)", disk.Name, disk.Type))
			}
			break
		}
	}

	// Networking — first interface
	if len(details.NetworkInterfaces) > 0 {
		nic := details.NetworkInterfaces[0]
		if field := f.GetField("network"); field != nil {
			field.SetValue(nic.Network)
		}
		if field := f.GetField("subnetwork"); field != nil {
			sub := nic.Subnetwork
			if sub == "" {
				sub = "(auto)"
			}
			field.SetValue(sub)
		}
		if field := f.GetField("external_ip"); field != nil {
			if nic.ExternalIP != "" {
				field.SetValue(nic.ExternalIP)
			} else {
				field.SetValue("None")
			}
		}
	}
}

// machineTypeDropdownOptions converts a list of MachineType structs to form options.
func machineTypeDropdownOptions(types []gcp.MachineType) []forms.Option {
	opts := make([]forms.Option, 0, len(types))
	for _, mt := range types {
		label := mt.Name
		if mt.Description != "" {
			label = fmt.Sprintf("%s (%s)", mt.Name, mt.Description)
		}
		opts = append(opts, forms.Option{
			Value: mt.Name,
			Label: label,
		})
	}
	return opts
}

// networkDropdownOptions converts a list of Network structs to simple string options.
func networkDropdownOptions(networks []gcp.Network) []string {
	names := make([]string, 0, len(networks))
	for _, n := range networks {
		names = append(names, n.Name)
	}
	return names
}

// subnetworkDropdownOptions converts a list of SubnetworkInfo structs to form options.
func subnetworkDropdownOptions(subnets []gcp.SubnetworkInfo) []forms.Option {
	opts := make([]forms.Option, 0, len(subnets)+1)
	// Empty option for auto-selection
	opts = append(opts, forms.Option{Value: "", Label: "(auto)"})
	for _, s := range subnets {
		label := s.Name
		if s.IPRange != "" {
			label = fmt.Sprintf("%s (%s)", s.Name, s.IPRange)
		}
		opts = append(opts, forms.Option{
			Value: s.Name,
			Label: label,
		})
	}
	return opts
}

// regionFromZoneName extracts region from zone name (e.g., "us-central1-a" -> "us-central1").
func regionFromZoneName(zone string) string {
	lastDash := strings.LastIndex(zone, "-")
	if lastDash == -1 {
		return zone
	}
	return zone[:lastDash]
}
