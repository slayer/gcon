package views

import (
	gocontext "context"
	"errors"
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/diff"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
)

var errNoConfigChanges = errors.New("no changes to apply")
var errMachineTypeRequiresStopped = errors.New("instance must be stopped before changing machine type")
var errDiskShrinkNotAllowed = errors.New("disk size can only be increased, not decreased")

// instanceConfigEditState represents the view lifecycle
type instanceConfigEditState int

const (
	instanceConfigEditStateLoading instanceConfigEditState = iota
	instanceConfigEditStateForm
	instanceConfigEditStateDiff
	instanceConfigEditStateSaving
)

// Internal messages for the config edit view
type instanceConfigLoadedMsg struct {
	details      *gcp.InstanceDetails
	machineTypes []gcp.MachineType
}
type instanceConfigLoadErrorMsg struct{ err error }

// instanceConfigEditKeyMap defines key bindings for the config edit view
type instanceConfigEditKeyMap struct {
	Cancel key.Binding
	Retry  key.Binding
}

func defaultInstanceConfigEditKeyMap() instanceConfigEditKeyMap {
	return instanceConfigEditKeyMap{
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel/back"),
		),
		Retry: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "retry"),
		),
	}
}

// InstanceConfigEditView allows editing VM instance configuration (machine type, disk size).
// Follows the Loading -> Form -> Diff -> Saving state machine like cloudrun_edit.go.
type InstanceConfigEditView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	instanceName  string
	zone          string
	ctx           *context.ProgramContext

	state    instanceConfigEditState
	original *gcp.InstanceDetails

	form       *forms.Form
	diffViewer *diff.Viewer
	spinner    spinner.Model
	err        error
	keys       instanceConfigEditKeyMap

	width, height int

	// Machine types loaded for the instance's zone
	machineTypes []gcp.MachineType
}

// NewInstanceConfigEditView creates a new instance config edit view.
func NewInstanceConfigEditView(projectID, instanceName, zone string, computeClient *gcp.ComputeClient) *InstanceConfigEditView {
	return &InstanceConfigEditView{
		computeClient: computeClient,
		projectID:     projectID,
		instanceName:  instanceName,
		zone:          zone,
		spinner:       components.NewGCPSpinner(),
		keys:          defaultInstanceConfigEditKeyMap(),
		state:         instanceConfigEditStateLoading,
	}
}

// Init starts loading instance details and machine types in parallel.
func (v *InstanceConfigEditView) Init() tea.Cmd {
	v.state = instanceConfigEditStateLoading
	v.err = nil
	return tea.Batch(v.spinner.Tick, v.loadInstanceConfig())
}

// loadInstanceConfig fetches instance details and machine types concurrently.
func (v *InstanceConfigEditView) loadInstanceConfig() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return instanceConfigLoadErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		ctx := gocontext.Background()

		details, err := v.computeClient.GetInstanceDetails(ctx, v.projectID, v.zone, v.instanceName)
		if err != nil {
			return instanceConfigLoadErrorMsg{err: err}
		}

		types, err := v.computeClient.ListMachineTypes(ctx, v.projectID, v.zone)
		if err != nil {
			return instanceConfigLoadErrorMsg{err: err}
		}

		return instanceConfigLoadedMsg{details: details, machineTypes: types}
	}
}

// Update handles messages for the config edit view.
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *InstanceConfigEditView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case instanceConfigLoadedMsg:
		v.original = msg.details
		v.machineTypes = msg.machineTypes
		v.buildForm()
		v.populateForm()
		v.state = instanceConfigEditStateForm
		v.applySize()
		if v.form != nil {
			return v.form.Init()
		}
		return nil

	case instanceConfigLoadErrorMsg:
		v.state = instanceConfigEditStateForm
		v.err = msg.err
		return nil

	case diff.ConfirmMsg:
		v.state = instanceConfigEditStateSaving
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.emitSubmit())

	case diff.CancelMsg:
		v.state = instanceConfigEditStateForm
		return nil

	case forms.FormSubmitMsg:
		return v.showDiffPreview()

	case forms.FormCancelMsg:
		return func() tea.Msg { return InstanceConfigEditCanceledMsg{} }

	case spinner.TickMsg:
		if v.state == instanceConfigEditStateLoading || v.state == instanceConfigEditStateSaving {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		return v.handleKeyMsg(msg)
	}

	// Forward remaining messages to form (e.g., cursor blink commands)
	if v.state == instanceConfigEditStateForm && v.form != nil {
		return v.form.Update(msg)
	}
	return nil
}

func (v *InstanceConfigEditView) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	// Allow cancel during loading/saving
	if v.state == instanceConfigEditStateLoading || v.state == instanceConfigEditStateSaving {
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return InstanceConfigEditCanceledMsg{} }
		}
		return nil
	}

	// Error state with no form — retry or cancel
	if v.err != nil && v.state == instanceConfigEditStateForm && v.original == nil {
		if key.Matches(msg, v.keys.Retry) {
			v.state = instanceConfigEditStateLoading
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadInstanceConfig())
		}
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return InstanceConfigEditCanceledMsg{} }
		}
		return nil
	}

	// Diff state: delegate to diff viewer
	if v.state == instanceConfigEditStateDiff {
		return v.diffViewer.Update(msg)
	}

	// Form state: delegate to form
	if v.state == instanceConfigEditStateForm && v.form != nil {
		return v.form.Update(msg)
	}

	return nil
}

// View renders the config edit view.
func (v *InstanceConfigEditView) View() string {
	switch v.state {
	case instanceConfigEditStateLoading:
		return renderLoading(v.spinner, "Loading instance configuration...")

	case instanceConfigEditStateSaving:
		return renderSaving(v.spinner, "Updating instance configuration...")

	case instanceConfigEditStateDiff:
		if v.diffViewer != nil {
			return v.diffViewer.View()
		}

	case instanceConfigEditStateForm:
		// Error while loading (no form available yet)
		if v.err != nil && v.form == nil {
			return "\n" + components.RenderError(v.err)
		}

		content := ""
		if v.form != nil {
			content = v.form.View()
		}

		if v.err != nil {
			content += components.RenderInlineError(v.err)
		}

		return content
	}

	return ""
}

// SetContext updates the view with shared program context.
func (v *InstanceConfigEditView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize()
}

// SetError allows the app to propagate async errors back to the view.
func (v *InstanceConfigEditView) SetError(err error) {
	v.state = instanceConfigEditStateForm
	v.err = err
}

// HasTextInputFocused returns true if a text input is active.
func (v *InstanceConfigEditView) HasTextInputFocused() bool {
	if v.state == instanceConfigEditStateForm && v.form != nil {
		return v.form.HasTextInputFocused()
	}
	return false
}

func (v *InstanceConfigEditView) applySize() {
	if v.form != nil {
		v.form.SetSize(v.width-formWidthPadding, v.height-formHeightPadding)
	}
	if v.diffViewer != nil {
		v.diffViewer.SetSize(v.width-8, v.height-10)
	}
}

func (v *InstanceConfigEditView) buildForm() {
	v.form = buildInstanceForm(forms.FormModeEdit, true)

	// Populate machine type dropdown with loaded types
	if field := v.form.GetField("machine_type"); field != nil {
		field.SetOptions(machineTypeDropdownOptions(v.machineTypes))
	}
}

func (v *InstanceConfigEditView) populateForm() {
	if v.original == nil || v.form == nil {
		return
	}
	populateInstanceFormFromDetails(v.form, v.original)

	// Warn if instance isn't stopped — machine type changes require TERMINATED/STOPPED
	if !v.original.IsStopped() {
		v.form.SetSubtitle("⚠ Stop the instance before changing machine type")
	}
}

// showDiffPreview builds a diff view showing what will change.
func (v *InstanceConfigEditView) showDiffPreview() tea.Cmd {
	if errs := v.form.Validate(); len(errs) > 0 {
		return nil
	}

	fields := v.buildDiffFields()

	viewer := diff.New("Apply Configuration Changes", fields)
	if !viewer.HasChanges() {
		v.err = errNoConfigChanges
		return nil
	}

	// Pre-submit validations that the form-level validators can't catch
	// (they require comparing against the original instance state)
	for _, f := range fields {
		if f.Label == "Machine Type" && f.IsChanged() && v.original != nil && !v.original.IsStopped() {
			v.err = fmt.Errorf("%w (current status: %s)", errMachineTypeRequiresStopped, v.original.Status)
			v.state = instanceConfigEditStateForm
			return nil
		}
		if f.Label == "Boot Disk Size (GB)" && f.IsChanged() && v.original != nil {
			if newSize, ok := v.form.GetData()["disk_size_gb"].(int64); ok {
				for _, disk := range v.original.Disks {
					if disk.Boot && newSize < disk.SizeGB {
						v.err = fmt.Errorf("%w (current: %d GB)", errDiskShrinkNotAllowed, disk.SizeGB)
						v.state = instanceConfigEditStateForm
						return nil
					}
				}
			}
		}
	}

	v.diffViewer = viewer
	v.diffViewer.SetSize(v.width-8, v.height-10)
	v.state = instanceConfigEditStateDiff
	return nil
}

// buildDiffFields compares form data against the original instance details.
func (v *InstanceConfigEditView) buildDiffFields() []diff.Field {
	data := v.form.GetData()

	// Resolve machine type: custom overrides dropdown
	newMachineType := ""
	if custom, ok := data["custom_machine_type"].(string); ok && custom != "" {
		newMachineType = custom
	} else if mt, ok := data["machine_type"].(string); ok {
		newMachineType = mt
	}

	oldMachineType := ""
	if v.original != nil {
		oldMachineType = v.original.MachineType
	}

	// Disk size — compare against boot disk
	oldDiskSize := ""
	newDiskSize := ""
	if v.original != nil {
		for _, disk := range v.original.Disks {
			if disk.Boot {
				oldDiskSize = strconv.FormatInt(disk.SizeGB, 10)
				break
			}
		}
	}
	if size, ok := data["disk_size_gb"].(int64); ok {
		newDiskSize = strconv.FormatInt(size, 10)
	}

	return []diff.Field{
		{Label: "Machine Type", OldValue: oldMachineType, NewValue: newMachineType},
		{Label: "Boot Disk Size (GB)", OldValue: oldDiskSize, NewValue: newDiskSize},
	}
}

// emitSubmit builds the changes list and emits InstanceConfigEditSubmitMsg.
func (v *InstanceConfigEditView) emitSubmit() tea.Cmd {
	fields := v.buildDiffFields()
	var changes []InstanceConfigEditChange
	for _, f := range fields {
		if f.IsChanged() {
			changes = append(changes, InstanceConfigEditChange{
				Field:    fieldLabelToKey(f.Label),
				OldValue: f.OldValue,
				NewValue: f.NewValue,
			})
		}
	}

	// Resolve boot disk name from loaded instance details
	bootDiskName := v.instanceName
	if v.original != nil {
		for _, disk := range v.original.Disks {
			if disk.Boot {
				bootDiskName = disk.Name
				break
			}
		}
	}

	return func() tea.Msg {
		return InstanceConfigEditSubmitMsg{
			ProjectID:    v.projectID,
			InstanceName: v.instanceName,
			Zone:         v.zone,
			BootDiskName: bootDiskName,
			Changes:      changes,
		}
	}
}

// GetComputeClient returns the compute client for reuse
func (v *InstanceConfigEditView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// GetInstanceName returns the instance name for breadcrumbs
func (v *InstanceConfigEditView) GetInstanceName() string {
	return v.instanceName
}

// fieldLabelToKey converts a human-readable field label to an API field key.
func fieldLabelToKey(label string) string {
	switch label {
	case "Machine Type":
		return "machine_type"
	case "Boot Disk Size (GB)":
		return "disk_size"
	default:
		return fmt.Sprintf("unknown_%s", label)
	}
}
