package views

import (
	gocontext "context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// SnapshotsView displays Compute Engine disk snapshots in a table format
type SnapshotsView struct {
	TableClickDelegate
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	snapshots     []gcp.Snapshot
	keys          snapshotKeyMap

	// Action menu state
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation state
	deleteConfirm     *confirm.ConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.Snapshot // Snapshot pending deletion

	// View dimensions for overlay rendering
	width  int
	height int
}

// snapshotKeyMap defines snapshot-specific key bindings
type snapshotKeyMap struct {
	Enter       key.Binding
	Refresh     key.Binding
	ActionMenu  key.Binding
	Delete      key.Binding
	CreateDisk  key.Binding
	CreateImage key.Binding
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
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
		CreateDisk: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "create disk"),
		),
		CreateImage: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "create image"),
		),
	}
}

// Table column definitions
func snapshotColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 28, Grow: true, Sortable: true},
		{Title: "Source Disk", Width: 22},
		{Title: "Size", Width: 10, Sortable: true},
		{Title: "Location", Width: 15},
		{Title: "Created", Width: 20, Sortable: true},
		{Title: "Status", Width: 12, Sortable: true},
	}
}

// NewSnapshotsView creates a new snapshots view with table display
func NewSnapshotsView(projectID string) *SnapshotsView {
	title := fmt.Sprintf("Disk Snapshots - %s", projectID)
	t := table.NewWithColumns(snapshotColumns(), title)

	s := components.NewGCPSpinner()

	v := &SnapshotsView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultSnapshotKeyMap(),
	}
	v.Table = &v.table
	return v
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
func snapshotStatusIcon(snapshot gcp.Snapshot) string { //nolint:gocritic // Copying snapshot is acceptable
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
func snapshotToRow(snapshot gcp.Snapshot) table.Row { //nolint:gocritic // Copying snapshot is acceptable
	sourceDisk := snapshot.SourceDisk
	if sourceDisk == "" {
		sourceDisk = "-"
	}

	// Combine status icon with name
	name := snapshotStatusIcon(snapshot) + " " + snapshot.Name

	// Format size with GB suffix
	size := fmt.Sprintf("%d GB", snapshot.SizeGB)

	// Format storage locations
	location := "-"
	if len(snapshot.StorageLocations) > 0 {
		if len(snapshot.StorageLocations) == 1 {
			location = snapshot.StorageLocations[0]
		} else {
			// Show first location and count of others
			location = fmt.Sprintf("%s +%d", snapshot.StorageLocations[0], len(snapshot.StorageLocations)-1)
		}
	}

	// Format created time
	created := timeutil.FormatTimestamp(snapshot.CreatedAt)

	return table.Row{
		Data: []string{
			name,
			sourceDisk,
			size,
			location,
			created,
			snapshot.Status,
		},
		FilterValue: snapshot.Name + " " + snapshot.SourceDisk + " " + snapshot.Status,
		ID:          snapshot.Name,
	}
}

// Update handles messages for the snapshots view
//
//nolint:gocognit // Bubble Tea Update pattern - complexity 45
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

	case table.RowDoubleClickedMsg:
		// Handle double-click on table row - navigate to details
		snapshot := v.findSnapshotByName(msg.RowID)
		if snapshot != nil {
			return func() tea.Msg {
				return SnapshotSelectedMsg{Snapshot: *snapshot}
			}
		}
		return nil

	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		return v.executeAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case confirm.ConfirmMsg:
		v.showDeleteConfirm = false
		if v.pendingDelete != nil {
			snapshot := v.pendingDelete
			v.pendingDelete = nil
			return func() tea.Msg {
				return DeleteSnapshotConfirmedMsg{
					SnapshotName: snapshot.Name,
				}
			}
		}
		return nil

	case confirm.CancelMsg:
		v.showDeleteConfirm = false
		v.pendingDelete = nil
		return nil

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		// Route to delete confirmation dialog when shown
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		// Route to action menu when open
		if v.menuOpen && v.actionMenu != nil {
			return v.actionMenu.Update(msg)
		}

		// Don't handle custom keys during loading
		if v.loading {
			return nil
		}

		// Delegate to table when sort menu is open
		if v.table.IsSortMenuOpen() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
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

		case key.Matches(msg, v.keys.ActionMenu):
			// Open action menu for selected snapshot
			if row := v.table.SelectedRow(); row != nil {
				snapshot := v.findSnapshotByName(row.ID)
				if snapshot != nil {
					v.actionMenu = actionmenu.New("Snapshot Actions", v.buildActions(snapshot))
					v.menuOpen = true
				}
			}
			return nil

		case key.Matches(msg, v.keys.Delete):
			// Direct hotkey for delete
			if row := v.table.SelectedRow(); row != nil {
				snapshot := v.findSnapshotByName(row.ID)
				if snapshot != nil {
					return v.showDeleteConfirmation(snapshot)
				}
			}
			return nil

		case key.Matches(msg, v.keys.CreateDisk):
			// Direct hotkey for create disk from snapshot
			if row := v.table.SelectedRow(); row != nil {
				snapshot := v.findSnapshotByName(row.ID)
				if snapshot != nil && snapshot.IsReady() {
					return func() tea.Msg {
						return DiskCreateFromSnapshotRequestMsg{
							SnapshotName: snapshot.Name,
							SnapshotSize: snapshot.SizeGB,
						}
					}
				}
			}
			return nil

		case key.Matches(msg, v.keys.CreateImage):
			// Direct hotkey for create image from snapshot
			if row := v.table.SelectedRow(); row != nil {
				snapshot := v.findSnapshotByName(row.ID)
				if snapshot != nil && snapshot.IsReady() {
					return func() tea.Msg {
						return ImageCreateFromSnapshotRequestMsg{
							SnapshotName: snapshot.Name,
						}
					}
				}
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			// Re-initialize client if previous attempt failed
			if v.computeClient == nil {
				return tea.Batch(v.spinner.Tick, v.initComputeClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadSnapshots())
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// buildActions creates the action menu items
func (v *SnapshotsView) buildActions(snapshot *gcp.Snapshot) []actionmenu.Action {
	return []actionmenu.Action{
		{Key: 'c', Label: "Create Disk", Enabled: snapshot.IsReady()},
		{Key: 'i', Label: "Create Image", Enabled: snapshot.IsReady()},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
	}
}

// executeAction performs the action selected from the menu
func (v *SnapshotsView) executeAction(actionKey rune) tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}

	snapshot := v.findSnapshotByName(row.ID)
	if snapshot == nil {
		return nil
	}

	switch actionKey {
	case 'c':
		// Create disk from snapshot
		if snapshot.IsReady() {
			return func() tea.Msg {
				return DiskCreateFromSnapshotRequestMsg{
					SnapshotName: snapshot.Name,
					SnapshotSize: snapshot.SizeGB,
				}
			}
		}
	case 'i':
		// Create image from snapshot
		if snapshot.IsReady() {
			return func() tea.Msg {
				return ImageCreateFromSnapshotRequestMsg{
					SnapshotName: snapshot.Name,
				}
			}
		}
	case 'D':
		return v.showDeleteConfirmation(snapshot)
	}

	return nil
}

// showDeleteConfirmation opens the delete confirmation dialog
func (v *SnapshotsView) showDeleteConfirmation(snapshot *gcp.Snapshot) tea.Cmd {
	v.deleteConfirm = confirm.New(
		"Delete Snapshot",
		fmt.Sprintf("Are you sure you want to delete snapshot '%s'?", snapshot.Name),
		[]string{
			fmt.Sprintf("Size: %d GB", snapshot.SizeGB),
			fmt.Sprintf("Source Disk: %s", snapshot.SourceDisk),
			"This action cannot be undone.",
		},
	)
	v.showDeleteConfirm = true
	v.pendingDelete = snapshot
	return nil
}

// IsMenuOpen returns true if the action menu or confirm dialog is currently open
func (v *SnapshotsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
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
		return renderLoading(v.spinner, "Initializing Compute Engine client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading snapshots...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.snapshots) == 0 {
		return "\n  No snapshots found in this project.\n  Press 'esc' to go back."
	}

	// Help text for actions
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • .: actions • c: create disk • i: create image • D: delete • S: sort • /: filter • r: refresh • esc: back")

	mainContent := v.table.View() + help

	// Overlay action menu if open
	if v.menuOpen && v.actionMenu != nil {
		return v.renderWithOverlay(mainContent, v.actionMenu.View())
	}

	// Overlay delete confirmation if shown
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.renderWithOverlay(mainContent, v.deleteConfirm.View())
	}

	return mainContent
}

// renderWithOverlay overlays a dialog centered on top of the content
func (v *SnapshotsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// GetComputeClient returns the compute client for reuse in detail views
func (v *SnapshotsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// HasTextInputFocused returns true if the table filter is active.
// Used to prevent global hotkeys (like 'q' for quit) from triggering while typing.
func (v *SnapshotsView) HasTextInputFocused() bool {
	return v.table.HasTextInputFocused()
}

// SetContext updates the view with shared program context
func (v *SnapshotsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

