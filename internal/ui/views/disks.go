package views

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	btable "github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// DisksView displays Compute Engine disks in a table format
type DisksView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	width         int
	height        int
	disks         []gcp.Disk
	keys          diskKeyMap
}

// diskKeyMap defines disk-specific key bindings
type diskKeyMap struct {
	Refresh key.Binding
}

func defaultDiskKeyMap() diskKeyMap {
	return diskKeyMap{
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// Table column definitions
func diskColumns() []btable.Column {
	return []btable.Column{
		{Title: "Name", Width: 30},
		{Title: "Zone", Width: 18},
		{Title: "Size", Width: 8},
		{Title: "Type", Width: 12},
		{Title: "Attached To", Width: 20},
	}
}

// NewDisksView creates a new disks view with table display
func NewDisksView(projectID string) *DisksView {
	title := fmt.Sprintf("Persistent Disks - %s", projectID)
	t := table.New(diskColumns(), title)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &DisksView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultDiskKeyMap(),
	}
}

// Init initializes the view and starts loading disks
func (v *DisksView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initComputeClient(),
	)
}

// initComputeClient creates the compute client then loads disks
func (v *DisksView) initComputeClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(context.Background())
		if err != nil {
			return disksErrorMsg{err: err}
		}
		return disksClientReadyMsg{client: client}
	}
}

// loadDisks fetches disks from GCP
func (v *DisksView) loadDisks() tea.Cmd {
	return func() tea.Msg {
		disks, err := v.computeClient.ListDisks(context.Background(), v.projectID)
		if err != nil {
			return disksErrorMsg{err: err}
		}
		return disksLoadedMsg{disks: disks}
	}
}

// Message types
type disksClientReadyMsg struct {
	client *gcp.ComputeClient
}

type disksLoadedMsg struct {
	disks []gcp.Disk
}

type disksErrorMsg struct {
	err error
}

// diskStatusIcon returns a symbol indicator for disk status
func diskStatusIcon(disk gcp.Disk) string {
	// Use attachment status as primary indicator since most disks are READY
	if disk.IsAttached() {
		return symbols.StatusRunning() // Green - in use
	}
	if disk.IsReady() {
		return symbols.StatusStopped() // Red - available but not attached
	}
	// Other states (CREATING, FAILED, etc.)
	return symbols.StatusTransitioning()
}

// diskToRow converts a GCP disk to a table row
func diskToRow(disk gcp.Disk) table.Row {
	attachedTo := disk.AttachedTo
	if attachedTo == "" {
		attachedTo = "-"
	}

	// Combine status icon with name
	name := diskStatusIcon(disk) + " " + disk.Name

	// Format size with GB suffix
	size := fmt.Sprintf("%d GB", disk.SizeGB)

	return table.Row{
		Data: []string{
			name,
			disk.Zone,
			size,
			disk.Type,
			attachedTo,
		},
		FilterValue: disk.Name + " " + disk.Zone + " " + disk.Type + " " + disk.AttachedTo,
		ID:          disk.Name,
	}
}

// Update handles messages for the disks view
func (v *DisksView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case disksClientReadyMsg:
		v.computeClient = msg.client
		return v.loadDisks()

	case disksLoadedMsg:
		v.loading = false
		v.disks = msg.disks

		// Convert disks to table rows
		rows := make([]table.Row, len(msg.disks))
		for i, disk := range msg.disks {
			rows[i] = diskToRow(disk)
		}
		v.table.SetRows(rows)
		return nil

	case disksErrorMsg:
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
		// Don't handle custom keys during loading
		if v.loading {
			return nil
		}

		// Let table handle filtering mode
		if v.table.IsFiltering() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
		}

		if key.Matches(msg, v.keys.Refresh) {
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadDisks())
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// View renders the disks view
func (v *DisksView) View() string {
	if v.loading && v.computeClient == nil {
		return v.renderLoading("Initializing Compute Engine client...")
	}

	if v.loading {
		return v.renderLoading("Loading disks...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.disks) == 0 {
		return "\n  No disks found in this project.\n  Press 'esc' to go back."
	}

	// Help text for actions
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  /: filter • r: refresh • esc: back")

	return v.table.View() + help
}

// SetSize updates the view dimensions
func (v *DisksView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.table.SetSize(width, height-6) // Reserve space for header and help
}

// renderLoading renders a loading message
func (v *DisksView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}
