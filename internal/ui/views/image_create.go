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

// ImageSourceType indicates the source type for image creation
type ImageSourceType int

const (
	ImageSourceDisk ImageSourceType = iota
	ImageSourceSnapshot
)

// imageCreateState represents the view's state machine
type imageCreateState int

const (
	imageCreateStateForm imageCreateState = iota
	imageCreateStateSaving
)

// Internal message types
type imageCreateSuccessMsg struct{}
type imageCreateErrorMsg struct{ err error }

// imageCreateKeyMap defines key bindings for the view
type imageCreateKeyMap struct {
	Submit key.Binding
	Cancel key.Binding
}

func defaultImageCreateKeyMap() imageCreateKeyMap {
	return imageCreateKeyMap{
		Submit: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "create image"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// ImageCreateView allows creating images from disks or snapshots
type ImageCreateView struct {
	computeClient *gcp.ComputeClient
	projectID     string

	// Source information - either disk or snapshot
	sourceType   ImageSourceType
	sourceName   string // disk name or snapshot name
	zone         string // only used for disk source
	attachedTo   string // Instance name if disk is attached (only for disk source)

	ctx *context.ProgramContext

	// State machine
	state imageCreateState

	// UI components
	form    *forms.Form
	spinner spinner.Model
	err     error
	width   int
	height  int
	keys    imageCreateKeyMap
}

// NewImageCreateView creates a new image create view for creating from a disk
func NewImageCreateView(projectID, diskName, zone, attachedTo string, computeClient *gcp.ComputeClient) *ImageCreateView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	v := &ImageCreateView{
		computeClient: computeClient,
		projectID:     projectID,
		sourceType:    ImageSourceDisk,
		sourceName:    diskName,
		zone:          zone,
		attachedTo:    attachedTo,
		spinner:       s,
		state:         imageCreateStateForm,
		keys:          defaultImageCreateKeyMap(),
	}

	v.buildForm()
	return v
}

// NewImageCreateViewFromSnapshot creates a new image create view for creating from a snapshot
func NewImageCreateViewFromSnapshot(projectID, snapshotName string, computeClient *gcp.ComputeClient) *ImageCreateView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	v := &ImageCreateView{
		computeClient: computeClient,
		projectID:     projectID,
		sourceType:    ImageSourceSnapshot,
		sourceName:    snapshotName,
		spinner:       s,
		state:         imageCreateStateForm,
		keys:          defaultImageCreateKeyMap(),
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

	v.form = forms.NewForm("Create Image", forms.FormModeCreate).
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

	v.form.AddSection(basicSection)

	// Show warning if disk is attached (only for disk source)
	if v.sourceType == ImageSourceDisk && v.attachedTo != "" {
		warningSection := forms.NewSection("warning", "Warning").
			AddField(forms.NewReadOnlyField("warning_text", "",
				fmt.Sprintf("This disk is attached to instance '%s'.", v.attachedTo))).
			AddField(forms.NewToggleField("force_create", "Keep instance running (not recommended)").
				SetValue(false).
				SetHelpText("Create image without stopping the instance (may result in inconsistent data)"))

		v.form.AddSection(warningSection)
	}

	// Storage Location section
	locationSection := forms.NewSection("location", "Storage Location").
		AddField(forms.NewDropdownField("storage_location", "Location").
			SetOptionsFromStrings(append([]string{"(default)"}, gcp.AllStorageLocations...)).
			SetHelpText("Where to store the image (multi-regional: us, eu, asia; or regional)"))

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
func (v *ImageCreateView) Init() tea.Cmd {
	return v.form.Init()
}

// Update handles messages for the view
func (v *ImageCreateView) Update(msg tea.Msg) tea.Cmd {
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
		v.state = imageCreateStateForm
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.state == imageCreateStateSaving {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		// Return appropriate cancel message based on source type
		if v.sourceType == ImageSourceSnapshot {
			return func() tea.Msg {
				return ImageCreateFromSnapshotCanceledMsg{}
			}
		}
		return func() tea.Msg {
			return ImageCreateCanceledMsg{}
		}

	case tea.KeyMsg:
		// Allow cancel during saving
		if v.state == imageCreateStateSaving {
			if key.Matches(msg, v.keys.Cancel) {
				if v.sourceType == ImageSourceSnapshot {
					return func() tea.Msg {
						return ImageCreateFromSnapshotCanceledMsg{}
					}
				}
				return func() tea.Msg {
					return ImageCreateCanceledMsg{}
				}
			}
			return nil
		}
	}

	// Update form
	return v.form.Update(msg)
}

// handleSubmit processes form submission
func (v *ImageCreateView) handleSubmit() tea.Cmd {
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

	family := ""
	if fam, ok := data["family"].(string); ok {
		family = strings.TrimSpace(fam)
	}

	storageLocation := ""
	if loc, ok := data["storage_location"].(string); ok && loc != "(default)" {
		storageLocation = loc
	}

	// Parse labels from text
	labels := parseLabelsFromText(data["labels_text"])

	v.state = imageCreateStateSaving
	v.err = nil

	// Return appropriate message based on source type
	if v.sourceType == ImageSourceSnapshot {
		return tea.Batch(
			v.spinner.Tick,
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
		v.spinner.Tick,
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

// View renders the view
func (v *ImageCreateView) View() string {
	if v.state == imageCreateStateSaving {
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
func (v *ImageCreateView) renderSaving() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), style.Render("Creating image..."))
}

// SetContext updates the view with shared program context
func (v *ImageCreateView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.form.SetSize(ctx.ContentWidth-4, ctx.ContentHeight-4)
}

// HasTextInputFocused returns true if a text input field is focused
func (v *ImageCreateView) HasTextInputFocused() bool {
	if v.form != nil {
		return v.form.HasTextInputFocused()
	}
	return false
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
