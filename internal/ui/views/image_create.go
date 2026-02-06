package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

// ImageSourceType indicates the source type for image creation
type ImageSourceType int

const (
	ImageSourceDisk ImageSourceType = iota
	ImageSourceSnapshot
)

// Internal message types
type imageCreateSuccessMsg struct{}
type imageCreateErrorMsg struct{ err error }

// ImageCreateView allows creating images from disks or snapshots
type ImageCreateView struct {
	CreateViewBase

	computeClient *gcp.ComputeClient
	projectID     string

	// Source information - either disk or snapshot
	sourceType ImageSourceType
	sourceName string // disk name or snapshot name
	zone       string // only used for disk source
	attachedTo string // Instance name if disk is attached (only for disk source)
}

// NewImageCreateView creates a new image create view for creating from a disk
func NewImageCreateView(projectID, diskName, zone, attachedTo string, computeClient *gcp.ComputeClient) *ImageCreateView {
	v := &ImageCreateView{
		CreateViewBase: NewCreateViewBase("Creating image..."),
		computeClient:  computeClient,
		projectID:      projectID,
		sourceType:     ImageSourceDisk,
		sourceName:     diskName,
		zone:           zone,
		attachedTo:     attachedTo,
	}

	v.buildForm()
	return v
}

// NewImageCreateViewFromSnapshot creates a new image create view for creating from a snapshot
func NewImageCreateViewFromSnapshot(projectID, snapshotName string, computeClient *gcp.ComputeClient) *ImageCreateView {
	v := &ImageCreateView{
		CreateViewBase: NewCreateViewBase("Creating image..."),
		computeClient:  computeClient,
		projectID:      projectID,
		sourceType:     ImageSourceSnapshot,
		sourceName:     snapshotName,
	}

	v.buildForm()
	return v
}

// buildForm creates the image creation form
func (v *ImageCreateView) buildForm() {
	// Set subtitle based on source type
	var subtitle string
	if v.sourceType == ImageSourceDisk {
		subtitle = fmt.Sprintf("Create an image from disk '%s'", v.sourceName)
	} else {
		subtitle = fmt.Sprintf("Create an image from snapshot '%s'", v.sourceName)
	}

	v.Form = forms.NewForm("Create Image", forms.FormModeCreate).
		SetSubtitle(subtitle).
		EnableViewport()

	// Basic Settings section
	// Default name: truncate source name if needed to fit 63-char GCP limit
	defaultName := truncateForSuffix(v.sourceName, "-image", 63)
	basicSection := forms.NewSection("basic", "Basic Settings").
		AddField(forms.NewTextField("name", "Image Name").
			SetRequired(true).
			SetValue(defaultName).
			SetPlaceholder("my-image").
			SetHelpText("1-63 characters, lowercase letters, numbers, and hyphens").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateGCPResourceName,
			))).
		AddField(forms.NewTextAreaField("description", "Description").
			SetPlaceholder("Optional description for the image").
			SetRows(3).
			SetHelpText("Describe the purpose of this image")).
		AddField(forms.NewTextField("family", "Image Family").
			SetPlaceholder("my-image-family").
			SetHelpText("Optional family name for grouping related images").
			SetValidator(forms.ValidateGCPResourceName))

	v.Form.AddSection(basicSection)

	// Show warning if disk is attached (only for disk source)
	if v.sourceType == ImageSourceDisk && v.attachedTo != "" {
		warningSection := forms.NewSection("warning", "Warning").
			AddField(forms.NewReadOnlyField("warning_text", "",
				fmt.Sprintf("This disk is attached to instance '%s'.", v.attachedTo))).
			AddField(forms.NewToggleField("force_create", "Keep instance running (not recommended)").
				SetValue(false).
				SetHelpText("Create image without stopping the instance (may result in inconsistent data)"))

		v.Form.AddSection(warningSection)
	}

	// Storage Location section
	locationSection := forms.NewSection("location", "Storage Location").
		AddField(forms.NewDropdownField("storage_location", "Location").
			SetOptionsFromStrings(append([]string{"(default)"}, gcp.AllStorageLocations...)).
			SetHelpText("Where to store the image (multi-regional: us, eu, asia; or regional)"))

	v.Form.AddSection(locationSection)

	// Labels section (collapsible)
	labelsSection := forms.NewSection("labels", "Labels (Optional)").
		SetCollapsible(true).
		SetCollapsed(true).
		AddField(forms.NewTextAreaField("labels_text", "Labels").
			SetPlaceholder("key1=value1\nkey2=value2").
			SetRows(4).
			SetHelpText("Enter labels as key=value pairs, one per line"))

	v.Form.AddSection(labelsSection)
}

// Update handles messages for the view
func (v *ImageCreateView) Update(msg tea.Msg) tea.Cmd {
	// Determine cancel message based on source type
	var cancelMsg tea.Msg
	if v.sourceType == ImageSourceSnapshot {
		cancelMsg = ImageCreateFromSnapshotCanceledMsg{}
	} else {
		cancelMsg = ImageCreateCanceledMsg{}
	}

	// Let base handle spinner ticks and cancel-during-saving
	if cmd, handled := v.HandleBaseUpdate(msg, cancelMsg); handled {
		return cmd
	}

	switch msg := msg.(type) {
	case imageCreateSuccessMsg:
		// Return appropriate result based on source type
		if v.sourceType == ImageSourceSnapshot {
			return func() tea.Msg {
				return SnapshotActionResultMsg{
					Action:  "image",
					Success: true,
				}
			}
		}
		return func() tea.Msg {
			return DiskActionResultMsg{
				Action:  "image",
				Success: true,
			}
		}

	case imageCreateErrorMsg:
		v.SetError(msg.err)
		return nil

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg { return cancelMsg }
	}

	return v.UpdateForm(msg)
}

// handleSubmit processes form submission
func (v *ImageCreateView) handleSubmit() tea.Cmd {
	if errors := v.Form.Validate(); len(errors) > 0 {
		return nil // Form displays errors
	}

	data := v.Form.GetData()

	name := ""
	if n, ok := data["name"].(string); ok {
		name = strings.TrimSpace(n)
	}
	description := ""
	if desc, ok := data["description"].(string); ok {
		description = strings.TrimSpace(desc)
	}

	family := ""
	if fam, ok := data["family"].(string); ok {
		family = strings.TrimSpace(fam)
	}

	storageLocation := ""
	if loc, ok := data["storage_location"].(string); ok && loc != "(default)" {
		storageLocation = loc
	}

	labels := parseLabelsFromText(data["labels_text"])

	cmd := v.BeginSaving()

	// Return appropriate message based on source type
	if v.sourceType == ImageSourceSnapshot {
		return tea.Batch(
			cmd,
			func() tea.Msg {
				return CreateImageFromSnapshotMsg{
					SnapshotName:    v.sourceName,
					ImageName:       name,
					Description:     description,
					Family:          family,
					Labels:          labels,
					StorageLocation: storageLocation,
				}
			},
		)
	}

	// Disk source - check force create flag (only present if disk is attached)
	forceCreate := false
	if fc, ok := data["force_create"].(bool); ok {
		forceCreate = fc
	}

	return tea.Batch(
		cmd,
		func() tea.Msg {
			return CreateImageFromDiskMsg{
				DiskName:        v.sourceName,
				Zone:            v.zone,
				ImageName:       name,
				Description:     description,
				Family:          family,
				Labels:          labels,
				StorageLocation: storageLocation,
				ForceCreate:     forceCreate,
			}
		},
	)
}

// GetSourceName returns the source name (disk or snapshot) for breadcrumbs
func (v *ImageCreateView) GetSourceName() string {
	return v.sourceName
}

// GetDiskName returns the source disk name for breadcrumbs (backward compatibility)
func (v *ImageCreateView) GetDiskName() string {
	return v.sourceName
}

// GetSnapshotName returns the source snapshot name for breadcrumbs
func (v *ImageCreateView) GetSnapshotName() string {
	if v.sourceType == ImageSourceSnapshot {
		return v.sourceName
	}
	return ""
}

// IsFromSnapshot returns true if image is being created from a snapshot
func (v *ImageCreateView) IsFromSnapshot() bool {
	return v.sourceType == ImageSourceSnapshot
}

// GetComputeClient returns the compute client for reuse
func (v *ImageCreateView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}
