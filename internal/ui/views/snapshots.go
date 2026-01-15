package views

import (
	gocontext "context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	btable "github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// SnapshotsView displays Compute Engine disk snapshots in a table format
type SnapshotsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	snapshots     []gcp.Snapshot
	keys          snapshotKeyMap
}

// snapshotKeyMap defines snapshot-specific key bindings
type snapshotKeyMap struct {
	Enter   key.Binding
	Refresh key.Binding
	Delete  key.Binding
}

func defaultSnapshotKeyMap() snapshotKeyMap {
	return snapshotKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
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

// Table column definitions
func snapshotColumns() []btable.Column {
	return []btable.Column{
		{Title: "Name", Width: 30},
		{Title: "Source Disk", Width: 25},
		{Title: "Size", Width: 10},
		{Title: "Created", Width: 20},
		{Title: "Status", Width: 12},
	}
}

// NewSnapshotsView creates a new snapshots view with table display
func NewSnapshotsView(projectID string) *SnapshotsView {
	title := fmt.Sprintf("Disk Snapshots - %s", projectID)
	t := table.New(snapshotColumns(), title)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &SnapshotsView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultSnapshotKeyMap(),
	}
}

// Init initializes the view and starts loading snapshots
func (v *SnapshotsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initComputeClient(),
	)
}

// initComputeClient creates the compute client then loads snapshots
func (v *SnapshotsView) initComputeClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(gocontext.Background())
		if err != nil {
			return snapshotsErrorMsg{err: err}
		}
		return snapshotsClientReadyMsg{client: client}
	}
}

// loadSnapshots fetches snapshots from GCP
func (v *SnapshotsView) loadSnapshots() tea.Cmd {
	return func() tea.Msg {
		snapshots, err := v.computeClient.ListSnapshots(gocontext.Background(), v.projectID)
		if err != nil {
			return snapshotsErrorMsg{err: err}
		}
		return snapshotsLoadedMsg{snapshots: snapshots}
	}
}

// Message types
type snapshotsClientReadyMsg struct {
	client *gcp.ComputeClient
}

type snapshotsLoadedMsg struct {
	snapshots []gcp.Snapshot
}

type snapshotsErrorMsg struct {
	err error
}

// SnapshotSelectedMsg is emitted when a snapshot is selected
type SnapshotSelectedMsg struct {
	Snapshot gcp.Snapshot
}

// snapshotStatusIcon returns a symbol indicator for snapshot status
func snapshotStatusIcon(snapshot gcp.Snapshot) string {
	if snapshot.IsReady() {
		return symbols.StatusRunning() // Green - ready
	}
	if snapshot.IsCreating() {
		return symbols.StatusTransitioning() // Yellow - creating
	}
	if snapshot.IsFailed() {
		return symbols.StatusStopped() // Red - failed
	}
	return symbols.StatusTransitioning()
}

// snapshotToRow converts a GCP snapshot to a table row
func snapshotToRow(snapshot gcp.Snapshot) table.Row {
	sourceDisk := snapshot.SourceDisk
	if sourceDisk == "" {
		sourceDisk = "-"
	}

	// Combine status icon with name
	name := snapshotStatusIcon(snapshot) + " " + snapshot.Name

	// Format size with GB suffix
	size := fmt.Sprintf("%d GB", snapshot.SizeGB)

	// Format created time
	created := timeutil.FormatTimestamp(snapshot.CreatedAt)

	return table.Row{
		Data: []string{
			name,
			sourceDisk,
			size,
			created,
			snapshot.Status,
		},
		FilterValue: snapshot.Name + " " + snapshot.SourceDisk + " " + snapshot.Status,
		ID:          snapshot.Name,
	}
}

// Update handles messages for the snapshots view
func (v *SnapshotsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case snapshotsClientReadyMsg:
		v.computeClient = msg.client
		return v.loadSnapshots()

	case snapshotsLoadedMsg:
		v.loading = false
		v.snapshots = msg.snapshots

		// Convert snapshots to table rows
		rows := make([]table.Row, len(msg.snapshots))
		for i, snapshot := range msg.snapshots {
			rows[i] = snapshotToRow(snapshot)
		}
		v.table.SetRows(rows)
		return nil

	case snapshotsErrorMsg:
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

		switch {
		case key.Matches(msg, v.keys.Enter):
			// Navigate to snapshot details on Enter
			if row := v.table.SelectedRow(); row != nil {
				snapshot := v.findSnapshotByName(row.ID)
				if snapshot != nil {
					return func() tea.Msg {
						return SnapshotSelectedMsg{Snapshot: *snapshot}
					}
				}
			}

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadSnapshots())

		case key.Matches(msg, v.keys.Delete):
			// Delete snapshot with confirmation
			if row := v.table.SelectedRow(); row != nil {
				snapshot := v.findSnapshotByName(row.ID)
				if snapshot != nil {
					// Emit delete request message
					return func() tea.Msg {
						return DeleteSnapshotRequestMsg{Snapshot: *snapshot}
					}
				}
			}
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// DeleteSnapshotRequestMsg is emitted when user requests to delete a snapshot
type DeleteSnapshotRequestMsg struct {
	Snapshot gcp.Snapshot
}

// findSnapshotByName looks up a snapshot by name
func (v *SnapshotsView) findSnapshotByName(name string) *gcp.Snapshot {
	for _, snapshot := range v.snapshots {
		if snapshot.Name == name {
			return &snapshot
		}
	}
	return nil
}

// View renders the snapshots view
func (v *SnapshotsView) View() string {
	if v.loading && v.computeClient == nil {
		return v.renderLoading("Initializing Compute Engine client...")
	}

	if v.loading {
		return v.renderLoading("Loading snapshots...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.snapshots) == 0 {
		return "\n  No snapshots found in this project.\n  Press 'esc' to go back."
	}

	// Help text for actions
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • D: delete • /: filter • r: refresh • esc: back")

	return v.table.View() + help
}

// GetComputeClient returns the compute client for reuse in detail views
func (v *SnapshotsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// SetContext updates the view with shared program context
func (v *SnapshotsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-6)
}

// renderLoading renders a loading message
func (v *SnapshotsView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}
