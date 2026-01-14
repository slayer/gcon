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

// DiskSelectedMsg is sent when a disk is selected from the list
type DiskSelectedMsg struct {
	Disk gcp.Disk
}

// diskDetailsLoadedMsg contains the fetched disk details
type diskDetailsLoadedMsg struct {
	details *gcp.DiskDetails
}

// diskDetailsErrorMsg indicates an error loading details
type diskDetailsErrorMsg struct {
	err error
}

// DiskDetailsView displays comprehensive disk information
type DiskDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	zone          string
	diskName      string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	details       *gcp.DiskDetails
	viewport      viewport.Model
	spinner       spinner.Model
	loading       bool
	err           error
	width         int
	height        int
	keys          diskDetailsKeyMap
	ready         bool
}

type diskDetailsKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Refresh key.Binding
}

func defaultDiskDetailsKeyMap() diskDetailsKeyMap {
	return diskDetailsKeyMap{
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
	}
}

// NewDiskDetailsView creates a new disk details view
func NewDiskDetailsView(projectID, zone, diskName string, computeClient *gcp.ComputeClient) *DiskDetailsView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &DiskDetailsView{
		computeClient: computeClient,
		projectID:     projectID,
		zone:          zone,
		diskName:      diskName,
		spinner:       s,
		loading:       true,
		keys:          defaultDiskDetailsKeyMap(),
	}
}

// Init initializes the view and starts loading disk details
func (v *DiskDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
	)
}

// loadDetails fetches disk details from GCP
func (v *DiskDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		details, err := v.computeClient.GetDiskDetails(gocontext.Background(), v.projectID, v.zone, v.diskName)
		if err != nil {
			return diskDetailsErrorMsg{err: err}
		}
		return diskDetailsLoadedMsg{details: details}
	}
}

// Update handles messages for the disk details view
func (v *DiskDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case diskDetailsLoadedMsg:
		v.loading = false
		v.details = msg.details
		v.updateViewportContent()
		return nil

	case diskDetailsErrorMsg:
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
	}

	// Handle viewport scrolling
	if v.ready {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return cmd
	}

	return nil
}

// View renders the disk details view
func (v *DiskDetailsView) View() string {
	if v.loading {
		return v.renderLoading("Loading disk details...")
	}

	if v.err != nil {
		return v.renderLoading(fmt.Sprintf("Error: %v\n  Press 'esc' to go back", v.err))
	}

	if v.details == nil {
		return v.renderLoading("No disk details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return v.renderLoading("Initializing view...")
	}

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", v.viewport.ScrollPercent()*100))
	help := helpStyle.Render("\n  ↑/↓: scroll • r: refresh • esc: back") + " " + scrollInfo

	return v.viewport.View() + help
}

// SetSize updates the view dimensions (deprecated, use SetContext)
func (v *DiskDetailsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.applySize(width, height)
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *DiskDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// applySize applies the given dimensions to the viewport
func (v *DiskDetailsView) applySize(width, height int) {
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
func (v *DiskDetailsView) updateViewportContent() {
	if v.details == nil || !v.ready {
		return
	}

	content := v.renderContent()
	v.viewport.SetContent(content)
}

// renderContent generates the full details content
func (v *DiskDetailsView) renderContent() string {
	d := v.details
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))

	// Header with status
	statusIcon := diskDetailStatusIcon(d.Status, len(d.Users) > 0)
	b.WriteString(titleStyle.Render(fmt.Sprintf("Disk: %s  %s %s", d.Name, statusIcon, d.Status)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Basic Information
	b.WriteString(sectionStyle.Render("Basic Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Disk ID", strconv.FormatUint(d.ID, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Description", defaultIfEmpty(d.Description, "None")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Status", fmt.Sprintf("%s %s", statusIcon, d.Status)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Zone", d.Zone))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	if d.LastAttach != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Last Attached", timeutil.FormatTimestamp(d.LastAttach)))
	}
	if d.LastDetach != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Last Detached", timeutil.FormatTimestamp(d.LastDetach)))
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

	// Size and Type
	b.WriteString(sectionStyle.Render("Size and Performance"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Size", fmt.Sprintf("%d GB", d.SizeGB)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Type", formatDiskType(d.Type)))
	if d.ProvisionedIOPS > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Provisioned IOPS", fmt.Sprintf("%d", d.ProvisionedIOPS)))
	}
	if d.ProvisionedTPUT > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Provisioned Throughput", fmt.Sprintf("%d MB/s", d.ProvisionedTPUT)))
	}
	if d.PhysicalBlockSizeB > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Physical Block Size", fmt.Sprintf("%d bytes", d.PhysicalBlockSizeB)))
	}
	b.WriteString("\n")

	// Source
	b.WriteString(sectionStyle.Render("Source"))
	b.WriteString("\n")
	if d.SourceImage != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Image", d.SourceImage))
	}
	if d.SourceSnapshot != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Snapshot", d.SourceSnapshot))
	}
	if d.SourceDisk != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Disk", d.SourceDisk))
	}
	if d.SourceImage == "" && d.SourceSnapshot == "" && d.SourceDisk == "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source", "Blank disk"))
	}
	b.WriteString("\n")

	// Encryption
	b.WriteString(sectionStyle.Render("Encryption"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Encryption Type", d.DiskEncryptionKey))
	b.WriteString("\n")

	// Usage
	b.WriteString(sectionStyle.Render("Usage"))
	b.WriteString("\n")
	if len(d.Users) > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Attached To", strings.Join(d.Users, ", ")))
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Attached To", "Not attached"))
	}

	// Regional disk info
	if len(d.ReplicaZones) > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Replica Zones", strings.Join(d.ReplicaZones, ", ")))
	}

	return b.String()
}

// diskDetailStatusIcon returns an appropriate status icon for the disk details view
func diskDetailStatusIcon(status string, attached bool) string {
	if attached {
		return symbols.StatusRunning() // Green - in use
	}
	if status == "READY" {
		return symbols.StatusStopped() // Red - available but not attached
	}
	return symbols.StatusTransitioning() // Yellow - other states
}

// formatDiskType returns a human-readable disk type name
func formatDiskType(diskType string) string {
	switch diskType {
	case "pd-standard":
		return "Standard persistent disk"
	case "pd-balanced":
		return "Balanced persistent disk"
	case "pd-ssd":
		return "SSD persistent disk"
	case "pd-extreme":
		return "Extreme persistent disk"
	case "hyperdisk-balanced":
		return "Hyperdisk Balanced"
	case "hyperdisk-extreme":
		return "Hyperdisk Extreme"
	case "hyperdisk-throughput":
		return "Hyperdisk Throughput"
	default:
		return defaultIfEmpty(diskType, "Unknown")
	}
}

// renderLoading renders a loading message
func (v *DiskDetailsView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}

// GetDiskName returns the disk name for use in breadcrumbs
func (v *DiskDetailsView) GetDiskName() string {
	return v.diskName
}
