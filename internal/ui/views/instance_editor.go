package views

import (
	gocontext "context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/diff"
	"github.com/slayer/gcon/internal/ui/components/labeledit"
	"github.com/slayer/gcon/internal/ui/context"
)

// Editor state machine states
type editorState int

const (
	stateLoading editorState = iota
	stateForm
	stateDiff
	stateSaving
	stateError
)

// Internal message types for async operations
type labelsLoadedMsg struct {
	labelsFingerprint *gcp.InstanceLabelsFingerprint
}

type labelsErrorMsg struct {
	err error
}

type labelsSavedMsg struct{}

type labelsSaveErrorMsg struct {
	err error
}

// InstanceEditRequestMsg requests opening the instance editor.
// Used by views (like InstanceDetailsView) to request editing.
type InstanceEditRequestMsg struct {
	ProjectID    string
	Zone         string
	InstanceName string
	EditMode     string // "labels", "tags", "description", "machine_type"
}

// InstanceEditCompleteMsg indicates successful edit, triggers refresh
type InstanceEditCompleteMsg struct {
	InstanceName string
	EditType     string
}

// InstanceEditCancelledMsg indicates user cancelled editing
type InstanceEditCancelledMsg struct{}

// instanceEditorKeyMap defines key bindings for the editor
type instanceEditorKeyMap struct {
	Save    key.Binding
	Cancel  key.Binding
	Refresh key.Binding
}

func defaultInstanceEditorKeyMap() instanceEditorKeyMap {
	return instanceEditorKeyMap{
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "preview changes"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "retry"),
		),
	}
}

// InstanceEditorView allows editing instance properties like labels
type InstanceEditorView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	zone          string
	instanceName  string
	ctx           *context.ProgramContext

	// State machine
	state editorState

	// Labels editing
	originalLabels map[string]string
	fingerprint    string
	labelEditor    *labeledit.Editor
	diffViewer     *diff.Viewer

	// UI components
	spinner spinner.Model
	err     error
	width   int
	height  int
	keys    instanceEditorKeyMap
	ready   bool
}

// NewInstanceEditorView creates a new instance editor view
func NewInstanceEditorView(projectID, zone, instanceName string, computeClient *gcp.ComputeClient) *InstanceEditorView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &InstanceEditorView{
		computeClient: computeClient,
		projectID:     projectID,
		zone:          zone,
		instanceName:  instanceName,
		spinner:       s,
		state:         stateLoading,
		keys:          defaultInstanceEditorKeyMap(),
	}
}

// Init initializes the view and starts loading labels
func (v *InstanceEditorView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadLabels(),
	)
}

// loadLabels fetches current labels and fingerprint from GCP
func (v *InstanceEditorView) loadLabels() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return labelsErrorMsg{err: fmt.Errorf("compute client not initialized")}
		}
		ctx := gocontext.Background()

		lf, err := v.computeClient.GetInstanceLabelsFingerprint(ctx, v.projectID, v.zone, v.instanceName)
		if err != nil {
			return labelsErrorMsg{err: err}
		}

		return labelsLoadedMsg{labelsFingerprint: lf}
	}
}

// saveLabels saves the edited labels to GCP
func (v *InstanceEditorView) saveLabels() tea.Cmd {
	return func() tea.Msg {
		ctx := gocontext.Background()

		newLabels := v.labelEditor.GetLabels()

		err := v.computeClient.SetInstanceLabels(ctx, v.projectID, v.zone, v.instanceName, newLabels, v.fingerprint)
		if err != nil {
			return labelsSaveErrorMsg{err: err}
		}

		return labelsSavedMsg{}
	}
}

// Update handles messages for the instance editor view
func (v *InstanceEditorView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case labelsLoadedMsg:
		v.state = stateForm
		v.originalLabels = msg.labelsFingerprint.Labels
		v.fingerprint = msg.labelsFingerprint.Fingerprint
		v.labelEditor = labeledit.New(v.originalLabels)
		v.labelEditor.SetSize(v.width-4, v.height-8)
		return nil

	case labelsErrorMsg:
		v.state = stateError
		v.err = msg.err
		return nil

	case labelsSavedMsg:
		// Success - emit completion message
		return func() tea.Msg {
			return InstanceEditCompleteMsg{
				InstanceName: v.instanceName,
				EditType:     "labels",
			}
		}

	case labelsSaveErrorMsg:
		v.state = stateError
		v.err = msg.err
		return nil

	case diff.ConfirmMsg:
		// User confirmed changes, save them
		v.state = stateSaving
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.saveLabels())

	case diff.CancelMsg:
		// User cancelled from diff view, go back to form
		v.state = stateForm
		return nil

	case labeledit.SaveRequestedMsg:
		// Show diff preview
		return v.showDiffPreview()

	case spinner.TickMsg:
		if v.state == stateLoading || v.state == stateSaving {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		return v.handleKeyMsg(msg)
	}

	return nil
}

// handleKeyMsg handles key presses based on current state
func (v *InstanceEditorView) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	// Handle loading/saving states - allow cancel
	if v.state == stateLoading || v.state == stateSaving {
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return InstanceEditCancelledMsg{} }
		}
		return nil
	}

	// Handle error state - allow retry or cancel
	if v.state == stateError {
		if key.Matches(msg, v.keys.Refresh) {
			v.state = stateLoading
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadLabels())
		}
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return InstanceEditCancelledMsg{} }
		}
		return nil
	}

	// Handle diff state - delegate to diff viewer
	if v.state == stateDiff {
		return v.diffViewer.Update(msg)
	}

	// Handle form state
	if v.state == stateForm {
		// Check for save shortcut
		if key.Matches(msg, v.keys.Save) {
			return v.showDiffPreview()
		}

		// Check for cancel
		if key.Matches(msg, v.keys.Cancel) {
			// If editing, let label editor handle it first
			if v.labelEditor != nil && v.labelEditor.IsEditing() {
				return v.labelEditor.Update(msg)
			}
			// Otherwise cancel the whole editor
			return func() tea.Msg { return InstanceEditCancelledMsg{} }
		}

		// Delegate to label editor
		if v.labelEditor != nil {
			return v.labelEditor.Update(msg)
		}
	}

	return nil
}

// showDiffPreview shows the diff preview before saving
func (v *InstanceEditorView) showDiffPreview() tea.Cmd {
	if v.labelEditor == nil {
		return nil
	}

	// Check if there are any changes - stay in form if none
	if !v.labelEditor.IsDirty() {
		return nil
	}

	// Build diff fields from labels with deterministic ordering
	newLabels := v.labelEditor.GetLabels()
	var fields []diff.Field

	// Collect all unique keys and sort them for deterministic output
	keySet := make(map[string]bool)
	for k := range v.originalLabels {
		keySet[k] = true
	}
	for k := range newLabels {
		keySet[k] = true
	}
	allKeys := make([]string, 0, len(keySet))
	for k := range keySet {
		allKeys = append(allKeys, k)
	}
	sort.Strings(allKeys)

	// Build diff fields in sorted order
	for _, key := range allKeys {
		oldVal, hadOld := v.originalLabels[key]
		newVal, hasNew := newLabels[key]

		switch {
		case !hasNew:
			// Label was removed
			fields = append(fields, diff.Field{
				Label:    key,
				OldValue: oldVal,
				NewValue: "",
			})
		case !hadOld:
			// Label was added
			fields = append(fields, diff.Field{
				Label:    key,
				OldValue: "",
				NewValue: newVal,
			})
		case oldVal != newVal:
			// Label was modified
			fields = append(fields, diff.Field{
				Label:    key,
				OldValue: oldVal,
				NewValue: newVal,
			})
		}
	}

	// Create diff viewer
	v.diffViewer = diff.New("Confirm Label Changes", fields)
	v.diffViewer.SetSize(v.width-8, v.height-10)
	v.state = stateDiff

	return nil
}

// View renders the instance editor view
func (v *InstanceEditorView) View() string {
	switch v.state {
	case stateLoading:
		return v.renderLoading("Loading labels...")

	case stateSaving:
		return v.renderLoading("Saving labels...")

	case stateError:
		return v.renderError()

	case stateDiff:
		return v.renderDiff()

	case stateForm:
		return v.renderForm()
	}

	return v.renderLoading("Initializing...")
}

// renderLoading renders a loading state with spinner
func (v *InstanceEditorView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}

// renderError renders an error state with retry option
func (v *InstanceEditorView) renderError() string {
	var b strings.Builder

	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	b.WriteString("\n")
	b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", v.err)))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  r: retry  esc: go back"))
	b.WriteString("\n")

	return b.String()
}

// renderDiff renders the diff confirmation view
func (v *InstanceEditorView) renderDiff() string {
	if v.diffViewer == nil {
		return v.renderLoading("Preparing diff...")
	}

	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	b.WriteString(titleStyle.Render(fmt.Sprintf("Edit Labels: %s", v.instanceName)))
	b.WriteString("\n\n")

	// Diff viewer
	b.WriteString(v.diffViewer.View())

	return b.String()
}

// renderForm renders the label editor form
func (v *InstanceEditorView) renderForm() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	b.WriteString(titleStyle.Render(fmt.Sprintf("Edit Labels: %s", v.instanceName)))
	b.WriteString("\n")

	// Zone info
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString(mutedStyle.Render(fmt.Sprintf("Zone: %s", v.zone)))
	b.WriteString("\n\n")

	// Label editor
	if v.labelEditor != nil {
		b.WriteString(v.labelEditor.View())
	} else {
		b.WriteString(mutedStyle.Render("  Loading..."))
	}

	return b.String()
}

// SetContext updates the view with shared program context
func (v *InstanceEditorView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// applySize applies the given dimensions
func (v *InstanceEditorView) applySize(width, height int) {
	v.width = width
	v.height = height
	v.ready = true

	if v.labelEditor != nil {
		v.labelEditor.SetSize(width-4, height-8)
	}
	if v.diffViewer != nil {
		v.diffViewer.SetSize(width-8, height-10)
	}
}

// GetComputeClient returns the compute client for passing to sub-views
func (v *InstanceEditorView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}
