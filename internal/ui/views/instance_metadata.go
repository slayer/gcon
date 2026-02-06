package views

import (
	gocontext "context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/context"
)

// metadataLoadedMsg contains the fetched instance and project metadata
type metadataLoadedMsg struct {
	instanceMetadata *gcp.InstanceMetadata
	projectMetadata  *gcp.InstanceMetadata
}

// metadataErrorMsg indicates an error loading metadata
type metadataErrorMsg struct {
	err error
}

// metadataSavedMsg indicates successful save
type metadataSavedMsg struct{}

// metadataSaveErrorMsg indicates an error saving metadata
type metadataSaveErrorMsg struct {
	err error
}

// InstanceMetadataView displays and manages instance metadata
type InstanceMetadataView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	zone          string
	instanceName  string
	ctx           *context.ProgramContext

	// Metadata state
	instanceMetadata *gcp.InstanceMetadata
	projectMetadata  *gcp.InstanceMetadata
	fingerprint      string

	// UI state
	viewport    viewport.Model
	editor      components.MetadataEditor
	spinner     spinner.Model
	loading     bool
	saving      bool
	editMode    bool
	err         error
	saveSuccess bool
	width       int
	height      int
	keys        instanceMetadataKeyMap
	ready       bool
}

type instanceMetadataKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Edit    key.Binding
	Save    key.Binding
	Refresh key.Binding
}

func defaultInstanceMetadataKeyMap() instanceMetadataKeyMap {
	return instanceMetadataKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "save"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// NewInstanceMetadataView creates a new instance metadata view
func NewInstanceMetadataView(projectID, zone, instanceName string, computeClient *gcp.ComputeClient) *InstanceMetadataView {
	s := components.NewGCPSpinner()

	return &InstanceMetadataView{
		computeClient: computeClient,
		projectID:     projectID,
		zone:          zone,
		instanceName:  instanceName,
		spinner:       s,
		loading:       true,
		editor:        components.NewMetadataEditor(),
		keys:          defaultInstanceMetadataKeyMap(),
	}
}

// Init initializes the view and starts loading metadata
func (v *InstanceMetadataView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadMetadata(),
	)
}

// loadMetadata fetches instance and project metadata from GCP
func (v *InstanceMetadataView) loadMetadata() tea.Cmd {
	return func() tea.Msg {
		ctx := gocontext.Background()

		// Fetch instance metadata
		instanceMeta, err := v.computeClient.GetInstanceMetadata(ctx, v.projectID, v.zone, v.instanceName)
		if err != nil {
			return metadataErrorMsg{err: err}
		}

		// Fetch project metadata
		projectMeta, err := v.computeClient.GetProjectMetadata(ctx, v.projectID)
		if err != nil {
			return metadataErrorMsg{err: err}
		}

		return metadataLoadedMsg{
			instanceMetadata: instanceMeta,
			projectMetadata:  projectMeta,
		}
	}
}

// saveMetadata saves the edited metadata back to GCP
func (v *InstanceMetadataView) saveMetadata() tea.Cmd {
	return func() tea.Msg {
		// Parse editor content
		content := v.editor.GetContent()
		metadata, err := components.ParseMetadata(content)
		if err != nil {
			return metadataSaveErrorMsg{err: fmt.Errorf("parse error: %w", err)}
		}

		// Validate metadata
		validationErrors := components.Validate(metadata)
		if len(validationErrors) > 0 {
			//nolint:err113 // Validation errors need dynamic messages
			return metadataSaveErrorMsg{err: fmt.Errorf("validation failed: %s", strings.Join(validationErrors, "; "))}
		}

		// Save to GCP
		ctx := gocontext.Background()
		err = v.computeClient.SetInstanceMetadata(ctx, v.projectID, v.zone, v.instanceName, metadata, v.fingerprint)
		if err != nil {
			return metadataSaveErrorMsg{err: err}
		}

		return metadataSavedMsg{}
	}
}

// Update handles messages for the instance metadata view
func (v *InstanceMetadataView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case metadataLoadedMsg:
		v.loading = false
		v.instanceMetadata = msg.instanceMetadata
		v.projectMetadata = msg.projectMetadata
		v.fingerprint = msg.instanceMetadata.Fingerprint
		v.updateViewportContent()
		v.saveSuccess = false
		return nil

	case metadataErrorMsg:
		v.loading = false
		v.saving = false
		v.err = msg.err
		return nil

	case metadataSavedMsg:
		v.saving = false
		v.saveSuccess = true
		v.editMode = false
		v.editor.Blur()
		// Reload metadata to get updated fingerprint
		v.loading = true
		return tea.Batch(v.spinner.Tick, v.loadMetadata())

	case metadataSaveErrorMsg:
		v.saving = false
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.loading || v.saving {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		// Don't handle keys while loading or saving
		if v.loading || v.saving {
			return nil
		}

		// In edit mode, forward most keys to editor
		if v.editMode {
			switch {
			case key.Matches(msg, v.keys.Save):
				v.saving = true
				v.err = nil
				return tea.Batch(v.spinner.Tick, v.saveMetadata())
			case msg.Type == tea.KeyEsc:
				// Cancel edit mode
				v.exitEditMode()
				return nil
			default:
				// Forward to editor
				var cmd tea.Cmd
				v.editor, cmd = v.editor.Update(msg)
				return cmd
			}
		}

		// View mode key handling
		switch {
		case key.Matches(msg, v.keys.Edit):
			v.enterEditMode()
			return v.editor.Focus()

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.saveSuccess = false
			return tea.Batch(v.spinner.Tick, v.loadMetadata())
		}
	}

	// Handle viewport scrolling in view mode
	if v.ready && !v.editMode {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return cmd
	}

	return nil
}

// enterEditMode switches to edit mode
func (v *InstanceMetadataView) enterEditMode() {
	v.editMode = true
	v.err = nil
	v.saveSuccess = false

	// Prepare editor content (only custom metadata, not SSH keys)
	customMetadata := v.getCustomMetadata()
	content := components.SerializeMetadata(customMetadata)
	v.editor.SetContent(content)
	v.editor.SetSize(v.width-4, v.height-8)
}

// exitEditMode exits edit mode without saving
func (v *InstanceMetadataView) exitEditMode() {
	v.editMode = false
	v.editor.Blur()
}

// getCustomMetadata returns metadata excluding SSH keys
func (v *InstanceMetadataView) getCustomMetadata() map[string]string {
	if v.instanceMetadata == nil {
		return make(map[string]string)
	}

	custom := make(map[string]string)
	for k, val := range v.instanceMetadata.Items {
		// Exclude SSH keys metadata
		if k != "ssh-keys" && k != "sshKeys" && !strings.HasPrefix(k, "ssh-") {
			custom[k] = val
		}
	}
	return custom
}

// View renders the instance metadata view
func (v *InstanceMetadataView) View() string {
	if v.loading {
		return renderLoading(v.spinner,"Loading metadata...")
	}

	if v.saving {
		return renderLoading(v.spinner,"Saving metadata...")
	}

	if v.err != nil {
		return renderLoading(v.spinner,fmt.Sprintf("Error: %v\n  Press 'r' to retry or 'esc' to go back", v.err))
	}

	if v.instanceMetadata == nil {
		return renderLoading(v.spinner,"No metadata available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return renderLoading(v.spinner,"Initializing view...")
	}

	// Edit mode
	if v.editMode {
		return v.renderEditMode()
	}

	// View mode
	return v.renderViewMode()
}

// renderEditMode renders the metadata editor
func (v *InstanceMetadataView) renderEditMode() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  ctrl+s: save • esc: cancel\n")

	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	var errorMsg string
	if v.err != nil {
		errorMsg = errorStyle.Render(fmt.Sprintf("  Error: %v\n", v.err))
	}

	return errorMsg + v.editor.View() + help
}

// renderViewMode renders the metadata display
func (v *InstanceMetadataView) renderViewMode() string {
	var header string
	if v.saveSuccess {
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
		header = successStyle.Render("  Metadata saved successfully!") + "\n\n"
	}

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", v.viewport.ScrollPercent()*100))
	help := helpStyle.Render("\n  ↑/↓: scroll • e: edit • r: refresh • esc: back") + " " + scrollInfo

	return header + v.viewport.View() + help
}

// SetContext updates the view with shared program context
func (v *InstanceMetadataView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// applySize applies the given dimensions to the viewport
func (v *InstanceMetadataView) applySize(width, height int) {
	// Reserve space for header and footer
	viewportHeight := height - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	if !v.ready {
		v.viewport = viewport.New(width, viewportHeight)
		v.viewport.Style = lipgloss.NewStyle().Padding(0, 2)
		v.ready = true
	} else {
		v.viewport.Width = width
		v.viewport.Height = viewportHeight
	}

	if v.instanceMetadata != nil {
		v.updateViewportContent()
	}
}

// updateViewportContent renders the metadata content into the viewport
func (v *InstanceMetadataView) updateViewportContent() {
	if v.instanceMetadata == nil || !v.ready {
		return
	}

	content := v.renderContent()
	v.viewport.SetContent(content)
}

// renderContent generates the full metadata content
func (v *InstanceMetadataView) renderContent() string {
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(20)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04")).Italic(true)

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Instance Metadata: %s", v.instanceName)))
	b.WriteString("\n")
	separatorWidth := min(v.width-4, 80)
	if separatorWidth < 0 {
		separatorWidth = 60 // Default width
	}
	b.WriteString(strings.Repeat("─", separatorWidth))
	b.WriteString("\n\n")

	// Custom Metadata section
	b.WriteString(sectionStyle.Render("Custom Metadata"))
	b.WriteString("\n")
	customMeta := v.getCustomMetadata()
	if len(customMeta) == 0 {
		b.WriteString(mutedStyle.Render("  No custom metadata defined"))
		b.WriteString("\n")
	} else {
		// Sort keys for consistent display
		keys := make([]string, 0, len(customMeta))
		for k := range customMeta {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			val := customMeta[k]
			// Truncate long values
			displayVal := val
			if len(val) > 60 {
				displayVal = val[:57] + "..."
			}
			b.WriteString(fmt.Sprintf("  %s %s\n",
				labelStyle.Render(k+":"),
				valueStyle.Render(displayVal)))
		}
	}
	b.WriteString("\n")

	// SSH Keys (Project) - read-only
	b.WriteString(sectionStyle.Render("SSH Keys (Project-wide)"))
	b.WriteString("\n")
	b.WriteString(infoStyle.Render("  Read-only. Managed at the project level."))
	b.WriteString("\n")

	projectSSHKeys := v.parseProjectSSHKeys()
	if len(projectSSHKeys) == 0 {
		b.WriteString(mutedStyle.Render("  No project-wide SSH keys defined"))
		b.WriteString("\n")
	} else {
		for _, sshKey := range projectSSHKeys {
			// Display: username, key type, truncated key data
			keyPreview := sshKey.KeyData
			if len(keyPreview) > 40 {
				keyPreview = keyPreview[:20] + "..." + keyPreview[len(keyPreview)-17:]
			}
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				labelStyle.Render(sshKey.Username+":"),
				valueStyle.Render(sshKey.KeyType),
				mutedStyle.Render(keyPreview)))
		}
	}
	b.WriteString("\n")

	// SSH Keys (Instance) - editable
	b.WriteString(sectionStyle.Render("SSH Keys (Instance-specific)"))
	b.WriteString("\n")

	instanceSSHKeys := v.parseInstanceSSHKeys()
	if len(instanceSSHKeys) == 0 {
		b.WriteString(mutedStyle.Render("  No instance-specific SSH keys defined"))
		b.WriteString("\n")
	} else {
		for _, sshKey := range instanceSSHKeys {
			keyPreview := sshKey.KeyData
			if len(keyPreview) > 40 {
				keyPreview = keyPreview[:20] + "..." + keyPreview[len(keyPreview)-17:]
			}
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				labelStyle.Render(sshKey.Username+":"),
				valueStyle.Render(sshKey.KeyType),
				mutedStyle.Render(keyPreview)))
		}
	}
	b.WriteString("\n")

	// Info about editing
	b.WriteString(infoStyle.Render("  Press 'e' to edit custom metadata"))
	b.WriteString("\n")

	return b.String()
}

// parseProjectSSHKeys extracts SSH keys from project metadata
func (v *InstanceMetadataView) parseProjectSSHKeys() []gcp.SSHKey {
	if v.projectMetadata == nil {
		return nil
	}

	sshKeysValue, ok := v.projectMetadata.Items["ssh-keys"]
	if !ok {
		// Try alternate key
		sshKeysValue, ok = v.projectMetadata.Items["sshKeys"]
		if !ok {
			return nil
		}
	}

	return gcp.ParseSSHKeys(sshKeysValue)
}

// parseInstanceSSHKeys extracts SSH keys from instance metadata
func (v *InstanceMetadataView) parseInstanceSSHKeys() []gcp.SSHKey {
	if v.instanceMetadata == nil {
		return nil
	}

	sshKeysValue, ok := v.instanceMetadata.Items["ssh-keys"]
	if !ok {
		// Try alternate key
		sshKeysValue, ok = v.instanceMetadata.Items["sshKeys"]
		if !ok {
			return nil
		}
	}

	return gcp.ParseSSHKeys(sshKeysValue)
}

