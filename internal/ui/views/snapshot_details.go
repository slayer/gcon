package views

import (
	gocontext "context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// snapshotDetailsLoadedMsg contains the fetched snapshot details
type snapshotDetailsLoadedMsg struct {
	details *gcp.SnapshotDetails
}

// snapshotDetailsErrorMsg indicates an error loading details
type snapshotDetailsErrorMsg struct {
	err error
}

// SnapshotDetailsView displays comprehensive snapshot information
type SnapshotDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	snapshotName  string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	details       *gcp.SnapshotDetails
	viewport      viewport.Model
	spinner       spinner.Model
	loading       bool
	err           error
	width         int
	height        int
	keys          snapshotDetailsKeyMap
	ready         bool
}

type snapshotDetailsKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Refresh key.Binding
	Delete  key.Binding
}

func defaultSnapshotDetailsKeyMap() snapshotDetailsKeyMap {
	return snapshotDetailsKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
	}
}

// NewSnapshotDetailsView creates a new snapshot details view
func NewSnapshotDetailsView(projectID, snapshotName string, computeClient *gcp.ComputeClient) *SnapshotDetailsView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &SnapshotDetailsView{
		computeClient: computeClient,
		projectID:     projectID,
		snapshotName:  snapshotName,
		spinner:       s,
		loading:       true,
		keys:          defaultSnapshotDetailsKeyMap(),
	}
}

// Init initializes the view and starts loading snapshot details
func (v *SnapshotDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
	)
}

// loadDetails fetches snapshot details from GCP
func (v *SnapshotDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		details, err := v.computeClient.GetSnapshotDetails(gocontext.Background(), v.projectID, v.snapshotName)
		if err != nil {
			return snapshotDetailsErrorMsg{err: err}
		}
		return snapshotDetailsLoadedMsg{details: details}
	}
}

// Update handles messages for the snapshot details view
func (v *SnapshotDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case snapshotDetailsLoadedMsg:
		v.loading = false
		v.details = msg.details
		v.updateViewportContent()
		return nil

	case snapshotDetailsErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		if key.Matches(msg, v.keys.Refresh) {
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadDetails())
		}
		if key.Matches(msg, v.keys.Delete) {
			// Emit delete request message for confirmation
			if v.details != nil {
				return func() tea.Msg {
					return DeleteSnapshotConfirmMsg{
						SnapshotName: v.snapshotName,
					}
				}
			}
		}
	}

	// Handle viewport scrolling
	if v.ready {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return cmd
	}

	return nil
}

// DeleteSnapshotConfirmMsg is emitted when user requests snapshot deletion
type DeleteSnapshotConfirmMsg struct {
	SnapshotName string
}

// View renders the snapshot details view
func (v *SnapshotDetailsView) View() string {
	if v.loading {
		return v.renderLoading("Loading snapshot details...")
	}

	if v.err != nil {
		return v.renderLoading(fmt.Sprintf("Error: %v\n  Press 'esc' to go back", v.err))
	}

	if v.details == nil {
		return v.renderLoading("No snapshot details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return v.renderLoading("Initializing view...")
	}

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", v.viewport.ScrollPercent()*100))
	help := helpStyle.Render("\n  ↑/↓: scroll • D: delete • r: refresh • esc: back") + " " + scrollInfo

	return v.viewport.View() + help
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *SnapshotDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// applySize applies the given dimensions to the viewport
func (v *SnapshotDetailsView) applySize(width, height int) {
	// Reserve space for footer
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

	if v.details != nil {
		v.updateViewportContent()
	}
}

// updateViewportContent renders the details content into the viewport
func (v *SnapshotDetailsView) updateViewportContent() {
	if v.details == nil || !v.ready {
		return
	}

	content := v.renderContent()
	v.viewport.SetContent(content)
}

// renderContent generates the full details content
func (v *SnapshotDetailsView) renderContent() string {
	d := v.details
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))

	// Header with status
	statusIcon := snapshotDetailStatusIcon(d.Status)
	b.WriteString(titleStyle.Render(fmt.Sprintf("Snapshot: %s  %s %s", d.Name, statusIcon, d.Status)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Basic Information
	b.WriteString(sectionStyle.Render("Basic Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Snapshot ID", strconv.FormatUint(d.ID, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Description", defaultIfEmpty(d.Description, "None")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Status", fmt.Sprintf("%s %s", statusIcon, d.Status)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	if d.AutoCreated {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Auto-Created", "Yes"))
	}
	b.WriteString("\n")

	// Labels
	if len(d.Labels) > 0 {
		b.WriteString(labelStyle.Render("Labels"))
		b.WriteString("\n")
		for k, val := range d.Labels {
			b.WriteString(fmt.Sprintf("    %s: %s\n", k, val))
		}
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Labels", "None"))
	}
	b.WriteString("\n")

	// Source Information
	b.WriteString(sectionStyle.Render("Source"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Disk", defaultIfEmpty(d.SourceDisk, "Unknown")))
	if d.SourceDiskZone != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Disk Zone", d.SourceDiskZone))
	}
	b.WriteString("\n")

	// Size and Storage
	b.WriteString(sectionStyle.Render("Size and Storage"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Disk Size", fmt.Sprintf("%d GB", d.DiskSizeGB)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Storage Used", fmt.Sprintf("%d GB (%.1f GB)", d.StorageBytesGb, float64(d.StorageBytes)/1024/1024/1024)))
	if len(d.StorageLocations) > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Storage Locations", strings.Join(d.StorageLocations, ", ")))
	}
	if d.SnapshotType != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Snapshot Type", d.SnapshotType))
	}
	b.WriteString("\n")

	// Encryption
	b.WriteString(sectionStyle.Render("Encryption"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Snapshot Encryption", defaultIfEmpty(d.SnapshotEncryptionKey, "Google-managed")))
	if d.SourceDiskEncryption != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Disk Encryption", d.SourceDiskEncryption))
	}
	b.WriteString("\n")

	// Additional Information
	if d.ChainName != "" || d.SatisfiesPZS {
		b.WriteString(sectionStyle.Render("Additional Information"))
		b.WriteString("\n")
		if d.ChainName != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Chain Name", d.ChainName))
		}
		if d.SatisfiesPZS {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Satisfies PZS", "Yes"))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// snapshotDetailStatusIcon returns an appropriate status icon for the snapshot details view
func snapshotDetailStatusIcon(status string) string {
	switch status {
	case "READY":
		return symbols.StatusRunning() // Green - ready to use
	case "CREATING", "UPLOADING":
		return symbols.StatusTransitioning() // Yellow - in progress
	case "FAILED":
		return symbols.StatusStopped() // Red - failed
	case "DELETING":
		return symbols.StatusTransitioning() // Yellow - being deleted
	default:
		return symbols.StatusUnknown() // Unknown status
	}
}

// renderLoading renders a loading message
func (v *SnapshotDetailsView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}

// GetSnapshotName returns the snapshot name for use in breadcrumbs
func (v *SnapshotDetailsView) GetSnapshotName() string {
	return v.snapshotName
}
