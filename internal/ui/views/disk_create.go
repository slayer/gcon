package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/slayer/gcon/internal/ui/context"
)

// diskCreateState represents the view's state machine
type diskCreateState int

const (
	diskCreateStateForm diskCreateState = iota
	diskCreateStateSaving
)

// Internal message types
type diskCreateSuccessMsg struct{}
type diskCreateErrorMsg struct{ err error }

// diskCreateKeyMap defines key bindings for the view
type diskCreateKeyMap struct {
	Submit key.Binding
	Cancel key.Binding
}

func defaultDiskCreateKeyMap() diskCreateKeyMap {
	return diskCreateKeyMap{
		Submit: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "create disk"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// DiskCreateView allows creating disks from snapshots
type DiskCreateView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	snapshotName  string
	snapshotSize  int64 // Size in GB of the source snapshot
	ctx           *context.ProgramContext

	// State machine
	state diskCreateState

	// UI components
	form    *forms.Form
	spinner spinner.Model
	err     error
	width   int
	height  int
	keys    diskCreateKeyMap
}

// NewDiskCreateView creates a new disk create view
func NewDiskCreateView(projectID, snapshotName string, snapshotSize int64, computeClient *gcp.ComputeClient) *DiskCreateView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	v := &DiskCreateView{
		computeClient: computeClient,
		projectID:     projectID,
		snapshotName:  snapshotName,
		snapshotSize:  snapshotSize,
		spinner:       s,
		state:         diskCreateStateForm,
		keys:          defaultDiskCreateKeyMap(),
	}

	v.buildForm()
	return v
}

// buildForm creates the disk creation form
func (v *DiskCreateView) buildForm() {
	v.form = forms.NewForm("Create Disk", forms.FormModeCreate).
		SetSubtitle(fmt.Sprintf("Create a disk from snapshot '%s'", v.snapshotName)).
		EnableViewport()

	// Basic Settings section
	// Default name: truncate snapshot name if needed to fit 63-char GCP limit
	defaultName := truncateForSuffix(v.snapshotName, "-disk", 63)
	basicSection := forms.NewSection("basic", "Basic Settings").
		AddField(forms.NewTextField("name", "Disk Name").
			SetRequired(true).
			SetValue(defaultName).
			SetPlaceholder("my-disk").
			SetHelpText("1-63 characters, lowercase letters, numbers, and hyphens").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateGCPResourceName,
			))).
		AddField(forms.NewTextAreaField("description", "Description").
			SetPlaceholder("Optional description for the disk").
			SetRows(3).
			SetHelpText("Describe the purpose of this disk"))

	v.form.AddSection(basicSection)

	// Location section - Zone selection
	zoneSection := forms.NewSection("location", "Location").
		AddField(forms.NewDropdownField("zone", "Zone").
			SetRequired(true).
			SetOptionsFromStrings(computeZones()).
			SetHelpText("Zone where the disk will be created"))

	v.form.AddSection(zoneSection)

	// Disk configuration section
	diskSection := forms.NewSection("disk", "Disk Configuration").
		AddField(forms.NewDropdownField("disk_type", "Disk Type").
			SetOptions([]forms.Option{
				{Value: "pd-standard", Label: "Standard persistent disk (HDD)"},
				{Value: "pd-balanced", Label: "Balanced persistent disk (SSD)"},
				{Value: "pd-ssd", Label: "SSD persistent disk"},
			}).
			SetValue("pd-balanced").
			SetHelpText("Type of persistent disk to create")).
		AddField(forms.NewNumberField("size_gb", "Size (GB)").
			SetRequired(true).
			SetValue(v.snapshotSize).
			SetValidator(forms.ValidateNumber(v.snapshotSize, 65536)). // Must be at least snapshot size, max 64TB
			SetHelpText(fmt.Sprintf("Minimum size: %d GB (snapshot size)", v.snapshotSize)))

	v.form.AddSection(diskSection)

	// Labels section (collapsible)
	labelsSection := forms.NewSection("labels", "Labels (Optional)").
		SetCollapsible(true).
		SetCollapsed(true).
		AddField(forms.NewTextAreaField("labels_text", "Labels").
			SetPlaceholder("key1=value1\nkey2=value2").
			SetRows(4).
			SetHelpText("Enter labels as key=value pairs, one per line"))

	v.form.AddSection(labelsSection)
}

// computeZones returns a list of GCP compute zones
func computeZones() []string {
	// Standard zones across all regions
	// Each region typically has zones a, b, c (some have more)
	zones := []string{}

	// US zones
	usRegions := []string{
		"us-central1", "us-east1", "us-east4", "us-east5",
		"us-south1", "us-west1", "us-west2", "us-west3", "us-west4",
	}
	for _, r := range usRegions {
		zones = append(zones, r+"-a", r+"-b", r+"-c")
	}

	// North/South America zones
	americaRegions := []string{
		"northamerica-northeast1", "northamerica-northeast2",
		"southamerica-east1", "southamerica-west1",
	}
	for _, r := range americaRegions {
		zones = append(zones, r+"-a", r+"-b", r+"-c")
	}

	// Europe zones
	europeRegions := []string{
		"europe-central2", "europe-north1", "europe-southwest1",
		"europe-west1", "europe-west2", "europe-west3",
		"europe-west4", "europe-west6", "europe-west8",
		"europe-west9", "europe-west10", "europe-west12",
	}
	for _, r := range europeRegions {
		zones = append(zones, r+"-a", r+"-b", r+"-c")
	}

	// Asia Pacific zones
	asiaRegions := []string{
		"asia-east1", "asia-east2",
		"asia-northeast1", "asia-northeast2", "asia-northeast3",
		"asia-south1", "asia-south2",
		"asia-southeast1", "asia-southeast2",
	}
	for _, r := range asiaRegions {
		zones = append(zones, r+"-a", r+"-b", r+"-c")
	}

	// Australia zones
	australiaRegions := []string{"australia-southeast1", "australia-southeast2"}
	for _, r := range australiaRegions {
		zones = append(zones, r+"-a", r+"-b", r+"-c")
	}

	// Middle East zones
	meRegions := []string{"me-central1", "me-central2", "me-west1"}
	for _, r := range meRegions {
		zones = append(zones, r+"-a", r+"-b", r+"-c")
	}

	// Africa zones
	africaRegions := []string{"africa-south1"}
	for _, r := range africaRegions {
		zones = append(zones, r+"-a", r+"-b", r+"-c")
	}

	return zones
}

// Init initializes the view
func (v *DiskCreateView) Init() tea.Cmd {
	return v.form.Init()
}

// Update handles messages for the view
func (v *DiskCreateView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case diskCreateSuccessMsg:
		// Return to previous view via result message
		return func() tea.Msg {
			return SnapshotActionResultMsg{
				Action:  "create_disk",
				Success: true,
			}
		}

	case diskCreateErrorMsg:
		v.state = diskCreateStateForm
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.state == diskCreateStateSaving {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg {
			return DiskCreateCanceledMsg{}
		}

	case tea.KeyMsg:
		// Allow cancel during saving
		if v.state == diskCreateStateSaving {
			if key.Matches(msg, v.keys.Cancel) {
				return func() tea.Msg {
					return DiskCreateCanceledMsg{}
				}
			}
			return nil
		}
	}

	// Update form
	return v.form.Update(msg)
}

// handleSubmit processes form submission
func (v *DiskCreateView) handleSubmit() tea.Cmd {
	// Validate form
	if errors := v.form.Validate(); len(errors) > 0 {
		return nil // Form displays errors
	}

	data := v.form.GetData()

	// Extract values
	name := ""
	if n, ok := data["name"].(string); ok {
		name = strings.TrimSpace(n)
	}
	description := ""
	if desc, ok := data["description"].(string); ok {
		description = strings.TrimSpace(desc)
	}

	zone := ""
	if z, ok := data["zone"].(string); ok {
		zone = z
	}

	diskType := "pd-balanced"
	if dt, ok := data["disk_type"].(string); ok {
		diskType = dt
	}

	sizeGB := v.snapshotSize
	if size, ok := data["size_gb"].(int64); ok && size >= v.snapshotSize {
		sizeGB = size
	}

	// Parse labels from text
	labels := parseLabelsFromText(data["labels_text"])

	v.state = diskCreateStateSaving
	v.err = nil

	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			return CreateDiskFromSnapshotMsg{
				SnapshotName: v.snapshotName,
				DiskName:     name,
				Description:  description,
				Zone:         zone,
				DiskType:     diskType,
				SizeGB:       sizeGB,
				Labels:       labels,
			}
		},
	)
}

// View renders the view
func (v *DiskCreateView) View() string {
	if v.state == diskCreateStateSaving {
		return v.renderSaving()
	}

	content := v.form.View()

	// Show error if any
	if v.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		content += "\n\n" + errorStyle.Render("Error: "+v.err.Error())
	}

	return content
}

// renderSaving renders the saving state
func (v *DiskCreateView) renderSaving() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), style.Render("Creating disk..."))
}

// SetContext updates the view with shared program context
func (v *DiskCreateView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.form.SetSize(ctx.ContentWidth-4, ctx.ContentHeight-4)
}

// HasTextInputFocused returns true if a text input field is focused
func (v *DiskCreateView) HasTextInputFocused() bool {
	if v.form != nil {
		return v.form.HasTextInputFocused()
	}
	return false
}

// GetSnapshotName returns the source snapshot name for breadcrumbs
func (v *DiskCreateView) GetSnapshotName() string {
	return v.snapshotName
}

// GetComputeClient returns the compute client for reuse
func (v *DiskCreateView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}
