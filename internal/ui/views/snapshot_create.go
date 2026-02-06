package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

// Internal message types
type snapshotCreateSuccessMsg struct{}
type snapshotCreateErrorMsg struct{ err error }

// SnapshotCreateView allows creating snapshots from disks
type SnapshotCreateView struct {
	CreateViewBase

	computeClient *gcp.ComputeClient
	projectID     string
	diskName      string
	zone          string
	attachedTo    string // Instance name if disk is attached
}

// NewSnapshotCreateView creates a new snapshot create view
func NewSnapshotCreateView(projectID, diskName, zone, attachedTo string, computeClient *gcp.ComputeClient) *SnapshotCreateView {
	v := &SnapshotCreateView{
		CreateViewBase: NewCreateViewBase("Creating snapshot..."),
		computeClient:  computeClient,
		projectID:      projectID,
		diskName:       diskName,
		zone:           zone,
		attachedTo:     attachedTo,
	}

	v.buildForm()
	return v
}

// buildForm creates the snapshot creation form
func (v *SnapshotCreateView) buildForm() {
	v.Form = forms.NewForm("Create Snapshot", forms.FormModeCreate).
		SetSubtitle(fmt.Sprintf("Create a snapshot from disk '%s'", v.diskName)).
		EnableViewport()

	// Basic Settings section
	// Default name: truncate disk name if needed to fit 63-char GCP limit
	defaultName := truncateForSuffix(v.diskName, "-snapshot", 63)
	basicSection := forms.NewSection("basic", "Basic Settings").
		AddField(forms.NewTextField("name", "Snapshot Name").
			SetRequired(true).
			SetValue(defaultName).
			SetPlaceholder("my-snapshot").
			SetHelpText("1-63 characters, lowercase letters, numbers, and hyphens").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateGCPResourceName,
			))).
		AddField(forms.NewTextAreaField("description", "Description").
			SetPlaceholder("Optional description for the snapshot").
			SetRows(3).
			SetHelpText("Describe the purpose of this snapshot"))

	v.Form.AddSection(basicSection)

	// Storage Location section
	locationSection := forms.NewSection("location", "Storage Location").
		AddField(forms.NewDropdownField("storage_location", "Location").
			SetOptionsFromStrings(append([]string{"(default)"}, gcp.AllStorageLocations...)).
			SetHelpText("Where to store the snapshot (multi-regional: us, eu, asia; or regional)"))

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
func (v *SnapshotCreateView) Update(msg tea.Msg) tea.Cmd {
	// Let base handle spinner ticks and cancel-during-saving
	if cmd, handled := v.HandleBaseUpdate(msg, SnapshotCreateCanceledMsg{}); handled {
		return cmd
	}

	switch msg := msg.(type) {
	case snapshotCreateSuccessMsg:
		return func() tea.Msg {
			return DiskActionResultMsg{
				Action:  "snapshot",
				Success: true,
			}
		}

	case snapshotCreateErrorMsg:
		v.SetError(msg.err)
		return nil

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg {
			return SnapshotCreateCanceledMsg{}
		}
	}

	return v.UpdateForm(msg)
}

// handleSubmit processes form submission
func (v *SnapshotCreateView) handleSubmit() tea.Cmd {
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

	storageLocation := ""
	if loc, ok := data["storage_location"].(string); ok && loc != "(default)" {
		storageLocation = loc
	}

	labels := parseLabelsFromText(data["labels_text"])

	cmd := v.BeginSaving()

	return tea.Batch(
		cmd,
		func() tea.Msg {
			return CreateSnapshotFromDiskMsg{
				DiskName:        v.diskName,
				Zone:            v.zone,
				SnapshotName:    name,
				Description:     description,
				Labels:          labels,
				StorageLocation: storageLocation,
			}
		},
	)
}

// truncateForSuffix truncates a name to fit within maxLen when suffix is added.
// GCP resource names have a 63 character limit.
func truncateForSuffix(name, suffix string, maxLen int) string {
	combined := name + suffix
	if len(combined) <= maxLen {
		return combined
	}
	// Truncate name to fit suffix within maxLen
	maxNameLen := maxLen - len(suffix)
	if maxNameLen < 1 {
		maxNameLen = 1
	}
	return name[:maxNameLen] + suffix
}

// parseLabelsFromText parses labels from key=value text format
func parseLabelsFromText(data any) map[string]string {
	labels := make(map[string]string)
	text, ok := data.(string)
	if !ok || text == "" {
		return labels
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" {
				labels[key] = value
			}
		}
	}
	return labels
}

// GetDiskName returns the source disk name for breadcrumbs
func (v *SnapshotCreateView) GetDiskName() string {
	return v.diskName
}

// GetComputeClient returns the compute client for reuse
func (v *SnapshotCreateView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}
