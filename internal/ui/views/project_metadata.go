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

// projectMetadataLoadedMsg contains the fetched project metadata
type projectMetadataLoadedMsg struct {
	metadata *gcp.InstanceMetadata
}

// projectMetadataErrorMsg indicates an error loading metadata
type projectMetadataErrorMsg struct {
	err error
}

// projectMetadataSavedMsg indicates successful save
type projectMetadataSavedMsg struct{}

// projectMetadataSaveErrorMsg indicates an error saving metadata
type projectMetadataSaveErrorMsg struct {
	err error
}

// ProjectMetadataView displays and manages project-wide metadata
// This metadata applies to ALL instances in the project
type ProjectMetadataView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext

	// Metadata state
	metadata    *gcp.InstanceMetadata
	fingerprint string

	// UI state
	viewport    viewport.Model
	editor      components.MetadataEditor
	spinner     spinner.Model
	loading     bool
	saving      bool
	editMode    bool
	showWarning bool // Show warning before saving
	err         error
	saveSuccess bool
	width       int
	height      int
	keys        projectMetadataKeyMap
	ready       bool
}

type projectMetadataKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Edit    key.Binding
	Save    key.Binding
	Refresh key.Binding
}

func defaultProjectMetadataKeyMap() projectMetadataKeyMap {
	return projectMetadataKeyMap{
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

// NewProjectMetadataView creates a new project metadata view
func NewProjectMetadataView(projectID string, computeClient *gcp.ComputeClient) *ProjectMetadataView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &ProjectMetadataView{
		computeClient: computeClient,
		projectID:     projectID,
		spinner:       s,
		loading:       true,
		editor:        components.NewMetadataEditor(),
		keys:          defaultProjectMetadataKeyMap(),
	}
}

// Init initializes the view and starts loading metadata
func (v *ProjectMetadataView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadMetadata(),
	)
}

// loadMetadata fetches project metadata from GCP
func (v *ProjectMetadataView) loadMetadata() tea.Cmd {
	return func() tea.Msg {
		ctx := gocontext.Background()

		metadata, err := v.computeClient.GetProjectMetadata(ctx, v.projectID)
		if err != nil {
			return projectMetadataErrorMsg{err: err}
		}

		return projectMetadataLoadedMsg{
			metadata: metadata,
		}
	}
}

// saveMetadata saves the edited metadata back to GCP
func (v *ProjectMetadataView) saveMetadata() tea.Cmd {
	return func() tea.Msg {
		// Parse editor content
		content := v.editor.GetContent()
		metadata, err := components.ParseMetadata(content)
		if err != nil {
			return projectMetadataSaveErrorMsg{err: fmt.Errorf("parse error: %w", err)}
		}

		// Validate metadata
		validationErrors := components.Validate(metadata)
		if len(validationErrors) > 0 {
			return projectMetadataSaveErrorMsg{err: fmt.Errorf("validation failed: %s", strings.Join(validationErrors, "; "))}
		}

		// Save to GCP
		ctx := gocontext.Background()
		err = v.computeClient.SetProjectMetadata(ctx, v.projectID, metadata, v.fingerprint)
		if err != nil {
			return projectMetadataSaveErrorMsg{err: err}
		}

		return projectMetadataSavedMsg{}
	}
}

// Update handles messages for the project metadata view
func (v *ProjectMetadataView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectMetadataLoadedMsg:
		v.loading = false
		v.metadata = msg.metadata
		v.fingerprint = msg.metadata.Fingerprint
		v.updateViewportContent()
		v.saveSuccess = false
		return nil

	case projectMetadataErrorMsg:
		v.loading = false
		v.saving = false
		v.err = msg.err
		return nil

	case projectMetadataSavedMsg:
		v.saving = false
		v.saveSuccess = true
		v.editMode = false
		v.showWarning = false
		v.editor.Blur()
		// Reload metadata to get updated fingerprint
		v.loading = true
		return tea.Batch(v.spinner.Tick, v.loadMetadata())

	case projectMetadataSaveErrorMsg:
		v.saving = false
		v.showWarning = false
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

		// Warning mode - only accept save or cancel
		if v.showWarning {
			switch {
			case key.Matches(msg, v.keys.Save):
				// Confirmed - proceed with save
				v.saving = true
				v.err = nil
				return tea.Batch(v.spinner.Tick, v.saveMetadata())
			case msg.Type == tea.KeyEsc:
				// Cancel warning
				v.showWarning = false
				return nil
			}
			return nil
		}

		// In edit mode, forward most keys to editor
		if v.editMode {
			switch {
			case key.Matches(msg, v.keys.Save):
				// Show warning before saving
				v.showWarning = true
				v.err = nil
				return nil
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
	if v.ready && !v.editMode && !v.showWarning {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return cmd
	}

	return nil
}

// enterEditMode switches to edit mode
func (v *ProjectMetadataView) enterEditMode() {
	v.editMode = true
	v.err = nil
	v.saveSuccess = false

	// Prepare editor content with all metadata
	content := components.SerializeMetadata(v.metadata.Items)
	v.editor.SetContent(content)
	v.editor.SetSize(v.width-4, v.height-8)
}

// exitEditMode exits edit mode without saving
func (v *ProjectMetadataView) exitEditMode() {
	v.editMode = false
	v.showWarning = false
	v.editor.Blur()
}

// View renders the project metadata view
func (v *ProjectMetadataView) View() string {
	if v.loading {
		return v.renderLoading("Loading project metadata...")
	}

	if v.saving {
		return v.renderLoading("Saving project metadata...")
	}

	if v.err != nil {
		return v.renderLoading(fmt.Sprintf("Error: %v\n  Press 'r' to retry or 'esc' to go back", v.err))
	}

	if v.metadata == nil {
		return v.renderLoading("No metadata available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return v.renderLoading("Initializing view...")
	}

	// Warning mode - show confirmation dialog
	if v.showWarning {
		return v.renderWarning()
	}

	// Edit mode
	if v.editMode {
		return v.renderEditMode()
	}

	// View mode
	return v.renderViewMode()
}

// renderWarning renders the save confirmation dialog
func (v *ProjectMetadataView) renderWarning() string {
	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FBBC04")).
		Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	warning := warningStyle.Render("⚠️  WARNING")
	message := "\n\nThis will affect ALL instances in project: " + v.projectID
	question := "\n\nAre you sure you want to save these changes?"
	help := helpStyle.Render("\n\nctrl+s: confirm and save • esc: cancel")

	content := "\n\n  " + warning + message + question + help + "\n"

	// Add vertical padding to center the dialog
	// Calculate how many newlines we need to center it
	contentLines := strings.Count(content, "\n")
	targetHeight := v.height
	if targetHeight < 10 {
		targetHeight = 10
	}

	// Add padding above to center vertically
	paddingAbove := (targetHeight - contentLines) / 2
	if paddingAbove > 0 {
		content = strings.Repeat("\n", paddingAbove) + content
	}

	return content
}

// renderEditMode renders the metadata editor
func (v *ProjectMetadataView) renderEditMode() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04")).Italic(true)

	info := infoStyle.Render("\n  This metadata applies to ALL instances in the project.\n")
	help := helpStyle.Render("\n  ctrl+s: save • esc: cancel\n")

	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	var errorMsg string
	if v.err != nil {
		errorMsg = errorStyle.Render(fmt.Sprintf("  Error: %v\n", v.err))
	}

	return info + errorMsg + v.editor.View() + help
}

// renderViewMode renders the metadata display
func (v *ProjectMetadataView) renderViewMode() string {
	var header string
	if v.saveSuccess {
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
		header = successStyle.Render("  Project metadata saved successfully!") + "\n"
		header += successStyle.Render("  This affects all instances in the project.") + "\n\n"
	}

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04")).Italic(true)
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	info := infoStyle.Render("  This metadata applies to ALL instances in the project.\n  Instance-specific metadata can override these values.\n\n")
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", v.viewport.ScrollPercent()*100))
	help := helpStyle.Render("\n  ↑/↓: scroll • e: edit • r: refresh • esc: back") + " " + scrollInfo

	return header + info + v.viewport.View() + help
}

// SetContext updates the view with shared program context
func (v *ProjectMetadataView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// applySize applies the given dimensions to the viewport
func (v *ProjectMetadataView) applySize(width, height int) {
	// Reserve space for header and footer
	viewportHeight := height - 6
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

	if v.metadata != nil {
		v.updateViewportContent()
	}
}

// updateViewportContent renders the metadata content into the viewport
func (v *ProjectMetadataView) updateViewportContent() {
	if v.metadata == nil || !v.ready {
		return
	}

	content := v.renderContent()
	v.viewport.SetContent(content)
}

// renderContent generates the full metadata content
func (v *ProjectMetadataView) renderContent() string {
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(20)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04")).Italic(true)

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Project Metadata: %s", v.projectID)))
	b.WriteString("\n")
	separatorWidth := min(v.width-4, 80)
	if separatorWidth < 0 {
		separatorWidth = 60
	}
	b.WriteString(strings.Repeat("─", separatorWidth))
	b.WriteString("\n\n")

	// SSH Keys section
	b.WriteString(sectionStyle.Render("SSH Keys (Project-wide)"))
	b.WriteString("\n")
	b.WriteString(infoStyle.Render("  All instances in the project inherit these SSH keys."))
	b.WriteString("\n")

	sshKeys := v.parseSSHKeys()
	if len(sshKeys) == 0 {
		b.WriteString(mutedStyle.Render("  No project-wide SSH keys defined"))
		b.WriteString("\n")
	} else {
		for _, sshKey := range sshKeys {
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

	// Info about editing
	b.WriteString(infoStyle.Render("  Press 'e' to edit metadata"))
	b.WriteString("\n")

	return b.String()
}

// parseSSHKeys extracts SSH keys from project metadata
func (v *ProjectMetadataView) parseSSHKeys() []gcp.SSHKey {
	if v.metadata == nil {
		return nil
	}

	sshKeysValue, ok := v.metadata.Items["ssh-keys"]
	if !ok {
		// Try alternate key
		sshKeysValue, ok = v.metadata.Items["sshKeys"]
		if !ok {
			return nil
		}
	}

	return gcp.ParseSSHKeys(sshKeysValue)
}

// getCustomMetadata returns metadata excluding SSH keys
func (v *ProjectMetadataView) getCustomMetadata() map[string]string {
	if v.metadata == nil {
		return make(map[string]string)
	}

	custom := make(map[string]string)
	for k, val := range v.metadata.Items {
		// Exclude SSH keys metadata
		if k != "ssh-keys" && k != "sshKeys" && !strings.HasPrefix(k, "ssh-") {
			custom[k] = val
		}
	}
	return custom
}

// renderLoading renders a loading message
func (v *ProjectMetadataView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}
