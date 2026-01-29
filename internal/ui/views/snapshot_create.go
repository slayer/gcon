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

// snapshotCreateState represents the view's state machine
type snapshotCreateState int

const (
	snapshotCreateStateForm snapshotCreateState = iota
	snapshotCreateStateSaving
)

// Internal message types
type snapshotCreateSuccessMsg struct{}
type snapshotCreateErrorMsg struct{ err error }

// snapshotCreateKeyMap defines key bindings for the view
type snapshotCreateKeyMap struct {
	Submit key.Binding
	Cancel key.Binding
}

func defaultSnapshotCreateKeyMap() snapshotCreateKeyMap {
	return snapshotCreateKeyMap{
		Submit: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "create snapshot"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// SnapshotCreateView allows creating snapshots from disks
type SnapshotCreateView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	diskName      string
	zone          string
	attachedTo    string // Instance name if disk is attached
	ctx           *context.ProgramContext

	// State machine
	state snapshotCreateState

	// UI components
	form    *forms.Form
	spinner spinner.Model
	err     error
	width   int
	height  int
	keys    snapshotCreateKeyMap
}

// NewSnapshotCreateView creates a new snapshot create view
func NewSnapshotCreateView(projectID, diskName, zone, attachedTo string, computeClient *gcp.ComputeClient) *SnapshotCreateView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	v := &SnapshotCreateView{
		computeClient: computeClient,
		projectID:     projectID,
		diskName:      diskName,
		zone:          zone,
		attachedTo:    attachedTo,
		spinner:       s,
		state:         snapshotCreateStateForm,
		keys:          defaultSnapshotCreateKeyMap(),
	}

	v.buildForm()
	return v
}

// buildForm creates the snapshot creation form
func (v *SnapshotCreateView) buildForm() {
	v.form = forms.NewForm("Create Snapshot", forms.FormModeCreate).
		SetSubtitle(fmt.Sprintf("Create a snapshot from disk '%s'", v.diskName)).
		EnableViewport()

	// Basic Settings section
	basicSection := forms.NewSection("basic", "Basic Settings").
		AddField(forms.NewTextField("name", "Snapshot Name").
			SetRequired(true).
			SetValue(v.diskName+"-snapshot").
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

	v.form.AddSection(basicSection)

	// Storage Location section
	locationSection := forms.NewSection("location", "Storage Location").
		AddField(forms.NewDropdownField("storage_location", "Location").
			SetOptionsFromStrings(append([]string{"(default)"}, gcp.AllStorageLocations...)).
			SetHelpText("Where to store the snapshot (multi-regional: us, eu, asia; or regional)"))

	v.form.AddSection(locationSection)

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

// Init initializes the view
func (v *SnapshotCreateView) Init() tea.Cmd {
	return v.form.Init()
}

// Update handles messages for the view
func (v *SnapshotCreateView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case snapshotCreateSuccessMsg:
		// Return to previous view
		return func() tea.Msg {
			return DiskActionResultMsg{
				Action:  "snapshot",
				Success: true,
			}
		}

	case snapshotCreateErrorMsg:
		v.state = snapshotCreateStateForm
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.state == snapshotCreateStateSaving {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg {
			return SnapshotCreateCanceledMsg{}
		}

	case tea.KeyMsg:
		// Allow cancel during saving
		if v.state == snapshotCreateStateSaving {
			if key.Matches(msg, v.keys.Cancel) {
				return func() tea.Msg {
					return SnapshotCreateCanceledMsg{}
				}
			}
			return nil
		}
	}

	// Update form
	return v.form.Update(msg)
}

// handleSubmit processes form submission
func (v *SnapshotCreateView) handleSubmit() tea.Cmd {
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

	storageLocation := ""
	if loc, ok := data["storage_location"].(string); ok && loc != "(default)" {
		storageLocation = loc
	}

	// Parse labels from text
	labels := parseLabelsFromText(data["labels_text"])

	v.state = snapshotCreateStateSaving
	v.err = nil

	return tea.Batch(
		v.spinner.Tick,
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

// View renders the view
func (v *SnapshotCreateView) View() string {
	if v.state == snapshotCreateStateSaving {
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
func (v *SnapshotCreateView) renderSaving() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), style.Render("Creating snapshot..."))
}

// SetContext updates the view with shared program context
func (v *SnapshotCreateView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.form.SetSize(ctx.ContentWidth-4, ctx.ContentHeight-4)
}

// HasTextInputFocused returns true if a text input field is focused
func (v *SnapshotCreateView) HasTextInputFocused() bool {
	if v.form != nil {
		return v.form.HasTextInputFocused()
	}
	return false
}

// GetDiskName returns the source disk name for breadcrumbs
func (v *SnapshotCreateView) GetDiskName() string {
	return v.diskName
}

// GetComputeClient returns the compute client for reuse
func (v *SnapshotCreateView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}
