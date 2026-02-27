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
)

// DisksView displays Compute Engine disks in a table format
type DisksView struct {
	TableClickDelegate
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	disks         []gcp.Disk
	keys          diskKeyMap

	// Action menu state
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation state
	deleteConfirm     *confirm.ConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.Disk // Disk pending deletion

	// View dimensions for overlay rendering
	width  int
	height int
}

// diskKeyMap defines disk-specific key bindings
type diskKeyMap struct {
	Enter      key.Binding
	Refresh    key.Binding
	ActionMenu key.Binding
	Snapshot   key.Binding
	Image      key.Binding
	Delete     key.Binding
}

func defaultDiskKeyMap() diskKeyMap {
	return diskKeyMap{
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
		Snapshot: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "snapshot"),
		),
		Image: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "image"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
	}
}

// Table column definitions
func diskColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 30, Grow: true, Sortable: true},
		{Title: "Zone", Width: 18, Sortable: true},
		{Title: "Size", Width: 8, Sortable: true},
		{Title: "Type", Width: 12, Sortable: true},
		{Title: "Attached To", Width: 20},
	}
}

// NewDisksView creates a new disks view with table display
func NewDisksView(projectID string) *DisksView {
	title := fmt.Sprintf("Persistent Disks - %s", projectID)
	t := table.NewWithColumns(diskColumns(), title)

	s := components.NewGCPSpinner()

	v := &DisksView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultDiskKeyMap(),
	}
	v.Table = &v.table
	return v
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
		client, err := gcp.NewComputeClient(gocontext.Background())
		if err != nil {
			return disksErrorMsg{err: err}
		}
		return disksClientReadyMsg{client: client}
	}
}

// loadDisks fetches disks from GCP
func (v *DisksView) loadDisks() tea.Cmd {
	return func() tea.Msg {
		disks, err := v.computeClient.ListDisks(gocontext.Background(), v.projectID)
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
func diskStatusIcon(disk gcp.Disk) string { //nolint:gocritic // Copying disk is acceptable
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
func diskToRow(disk gcp.Disk) table.Row { //nolint:gocritic // Copying disk is acceptable
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
//
//nolint:gocognit // Bubble Tea Update pattern - complexity 45
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

	case table.RowDoubleClickedMsg:
		// Handle double-click on table row - navigate to details
		disk := v.findDiskByName(msg.RowID)
		if disk != nil {
			return func() tea.Msg {
				return DiskSelectedMsg{Disk: *disk}
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
			disk := v.pendingDelete
			v.pendingDelete = nil
			return func() tea.Msg {
				return DeleteDiskConfirmedMsg{
					DiskName: disk.Name,
					Zone:     disk.Zone,
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
			// Navigate to disk details on Enter
			if row := v.table.SelectedRow(); row != nil {
				disk := v.findDiskByName(row.ID)
				if disk != nil {
					return func() tea.Msg {
						return DiskSelectedMsg{Disk: *disk}
					}
				}
			}

		case key.Matches(msg, v.keys.ActionMenu):
			// Open action menu for selected disk
			if row := v.table.SelectedRow(); row != nil {
				disk := v.findDiskByName(row.ID)
				if disk != nil {
					v.actionMenu = actionmenu.New("Disk Actions", v.buildActions(disk))
					v.menuOpen = true
				}
			}
			return nil

		case key.Matches(msg, v.keys.Snapshot):
			// Direct hotkey for snapshot
			if row := v.table.SelectedRow(); row != nil {
				disk := v.findDiskByName(row.ID)
				if disk != nil {
					return v.showSnapshotDialog(disk)
				}
			}
			return nil

		case key.Matches(msg, v.keys.Image):
			// Direct hotkey for image
			if row := v.table.SelectedRow(); row != nil {
				disk := v.findDiskByName(row.ID)
				if disk != nil {
					return v.showImageDialog(disk)
				}
			}
			return nil

		case key.Matches(msg, v.keys.Delete):
			// Direct hotkey for delete (only if detached)
			if row := v.table.SelectedRow(); row != nil {
				disk := v.findDiskByName(row.ID)
				if disk != nil && !disk.IsAttached() {
					return v.showDeleteConfirmation(disk)
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
			return tea.Batch(v.spinner.Tick, v.loadDisks())
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// buildActions creates the action menu items based on disk state
func (v *DisksView) buildActions(disk *gcp.Disk) []actionmenu.Action {
	isDetached := !disk.IsAttached()

	return []actionmenu.Action{
		{Key: 's', Label: "Create Snapshot", Enabled: true},
		{Key: 'i', Label: "Create Image", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: isDetached, Dangerous: true},
	}
}

// executeAction performs the action selected from the menu
func (v *DisksView) executeAction(actionKey rune) tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}

	disk := v.findDiskByName(row.ID)
	if disk == nil {
		return nil
	}

	switch actionKey {
	case 's':
		return v.showSnapshotDialog(disk)
	case 'i':
		return v.showImageDialog(disk)
	case 'D':
		if !disk.IsAttached() {
			return v.showDeleteConfirmation(disk)
		}
	}

	return nil
}

// showSnapshotDialog opens the snapshot creation form
func (v *DisksView) showSnapshotDialog(disk *gcp.Disk) tea.Cmd {
	return func() tea.Msg {
		return SnapshotCreateRequestMsg{
			DiskName:   disk.Name,
			Zone:       disk.Zone,
			AttachedTo: disk.AttachedTo,
		}
	}
}

// showImageDialog opens the image creation form
func (v *DisksView) showImageDialog(disk *gcp.Disk) tea.Cmd {
	return func() tea.Msg {
		return ImageCreateRequestMsg{
			DiskName:   disk.Name,
			Zone:       disk.Zone,
			AttachedTo: disk.AttachedTo,
		}
	}
}

// showDeleteConfirmation opens the delete confirmation dialog
func (v *DisksView) showDeleteConfirmation(disk *gcp.Disk) tea.Cmd {
	v.deleteConfirm = confirm.New(
		"Delete Disk",
		fmt.Sprintf("Are you sure you want to delete disk '%s'?", disk.Name),
		[]string{
			fmt.Sprintf("Zone: %s", disk.Zone),
			fmt.Sprintf("Size: %d GB", disk.SizeGB),
			"This action cannot be undone.",
		},
	)
	v.showDeleteConfirm = true
	v.pendingDelete = disk
	return nil
}

// IsMenuOpen returns true if the action menu is currently open
func (v *DisksView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// HasTextInputFocused returns true if the table filter is active.
// Used to prevent global hotkeys (like 'q' for quit) from triggering while typing.
func (v *DisksView) HasTextInputFocused() bool {
	return v.table.HasTextInputFocused()
}

// findDiskByName looks up a disk by name
func (v *DisksView) findDiskByName(name string) *gcp.Disk {
	for _, disk := range v.disks {
		if disk.Name == name {
			return &disk
		}
	}
	return nil
}

// View renders the disks view
func (v *DisksView) View() string {
	if v.loading && v.computeClient == nil {
		return renderLoading(v.spinner, "Initializing Compute Engine client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading disks...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.disks) == 0 {
		return "\n  No disks found in this project.\n  Press 'esc' to go back."
	}

	// Help text for actions
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • .: actions • s: snapshot • i: image • S: sort • /: filter • r: refresh • esc: back")

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
func (v *DisksView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// GetComputeClient returns the compute client for reuse in detail views
func (v *DisksView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *DisksView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}
