package views

import (
	gocontext "context"
	"fmt"
	"time"

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
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/overlay"
)

// SubnetsView displays subnets across all networks/regions in a table format
type SubnetsView struct {
	TableClickDelegate
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	subnets       []gcp.Subnet
	keys          subnetKeyMap

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.Subnet

	width  int
	height int
}

// subnetKeyMap defines subnet-specific key bindings
type subnetKeyMap struct {
	Select     key.Binding
	Refresh    key.Binding
	Create     key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
}

func defaultSubnetKeyMap() subnetKeyMap {
	return subnetKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Create: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "create"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
	}
}

// Table column definitions
func subnetColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 20, Grow: true, Sortable: true},
		{Title: "Network", Width: 15, Sortable: true},
		{Title: "Region", Width: 15, Sortable: true},
		{Title: "CIDR Range", Width: 18, Sortable: true},
		{Title: "Purpose", Width: 22, Sortable: true},
		{Title: "Google Access", Width: 14, Sortable: true},
		{Title: "Flow Logs", Width: 10, Sortable: true},
	}
}

// Internal message types for async operations
type subnetsClientReadyMsg struct{ client *gcp.ComputeClient }
type subnetsLoadedMsg struct{ subnets []gcp.Subnet }
type subnetsErrorMsg struct{ err error }

// NewSubnetsView creates a new subnets view with table display
func NewSubnetsView(projectID string) *SubnetsView {
	title := fmt.Sprintf("Subnets - %s", projectID)
	t := table.NewWithColumns(subnetColumns(), title)

	s := components.NewGCPSpinner()

	v := &SubnetsView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultSubnetKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init initializes the view and starts loading subnets
func (v *SubnetsView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	return tea.Batch(
		v.spinner.Tick,
		v.initComputeClient(),
	)
}

// initComputeClient creates the compute client then loads subnets
func (v *SubnetsView) initComputeClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(gocontext.Background())
		if err != nil {
			return subnetsErrorMsg{err: err}
		}
		return subnetsClientReadyMsg{client: client}
	}
}

// loadSubnets fetches subnets from GCP across all networks/regions
func (v *SubnetsView) loadSubnets() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return subnetsErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		subnets, err := v.computeClient.ListAllSubnets(gocontext.Background(), v.projectID)
		if err != nil {
			return subnetsErrorMsg{err: err}
		}
		return subnetsLoadedMsg{subnets: subnets}
	}
}

// subnetToRow converts a GCP subnet to a table row
func subnetToRow(s gcp.Subnet) table.Row { //nolint:gocritic // Copying subnet is acceptable
	return table.Row{
		Data: []string{
			s.Name,
			s.Network,
			s.Region,
			s.IPCidrRange,
			formatSubnetPurpose(s.Purpose),
			formatBoolCheck(s.PrivateIPGoogleAccess),
			formatBoolCheck(s.EnableFlowLogs),
		},
		FilterValue: s.Name + " " + s.Network + " " + s.Region + " " + s.IPCidrRange + " " + s.Purpose,
		// Unique ID: same name can exist in different regions
		ID: s.Name + "/" + s.Region,
	}
}

// formatBoolCheck returns a check mark or dash for boolean values
// formatSubnetPurpose is defined in network_details.go and shared across views.
func formatBoolCheck(b bool) string {
	if b {
		return "✓"
	}
	return "—"
}

// Update handles messages for the subnets view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *SubnetsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case subnetsClientReadyMsg:
		v.computeClient = msg.client
		v.registerTask("load-subnets", "Loading subnets...")
		return v.loadSubnets()

	case subnetsLoadedMsg:
		v.loading = false
		v.subnets = msg.subnets
		v.clearTask("load-subnets")

		rows := make([]table.Row, len(msg.subnets))
		for i, s := range msg.subnets {
			rows[i] = subnetToRow(s)
		}
		v.table.SetRows(rows)
		return nil

	case subnetsErrorMsg:
		v.loading = false
		v.err = msg.err
		return v.failTask("load-subnets", msg.err)

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case confirm.TypeConfirmMsg:
		v.showDeleteConfirm = false
		if v.pendingDelete != nil {
			s := v.pendingDelete
			v.pendingDelete = nil
			return func() tea.Msg {
				return DeleteSubnetConfirmedMsg{SubnetName: s.Name, Region: s.Region}
			}
		}
		return nil

	case confirm.TypeCancelMsg:
		v.showDeleteConfirm = false
		v.pendingDelete = nil
		return nil

	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		return v.executeAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case table.RowDoubleClickedMsg:
		if s, ok := v.findSubnetByID(msg.RowID); ok {
			return func() tea.Msg { return SubnetSelectedMsg{SubnetName: s.Name, Region: s.Region} }
		}
		return nil

	case tea.KeyMsg:
		// Route to delete confirmation dialog when shown
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		if v.loading {
			return nil
		}

		// Route keys to action menu when open
		if v.menuOpen {
			return v.actionMenu.Update(msg)
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
		case key.Matches(msg, v.keys.ActionMenu):
			v.actionMenu = actionmenu.New("Subnet Actions", v.buildActions())
			v.menuOpen = true
			return nil

		case key.Matches(msg, v.keys.Select):
			if row := v.table.SelectedRow(); row != nil {
				if s, ok := v.findSubnetByID(row.ID); ok {
					return func() tea.Msg { return SubnetSelectedMsg{SubnetName: s.Name, Region: s.Region} }
				}
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.registerTask("load-subnets", "Refreshing...")
			// Re-initialize client if previous attempt failed
			if v.computeClient == nil {
				return tea.Batch(v.spinner.Tick, v.initComputeClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadSubnets())

		case key.Matches(msg, v.keys.Create):
			return func() tea.Msg { return SubnetCreateRequestMsg{} }

		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// buildActions creates the action menu items
func (v *SubnetsView) buildActions() []actionmenu.Action {
	return []actionmenu.Action{
		{Key: 'c', Label: "Create Subnet", Enabled: true},
		{Key: 'r', Label: "Refresh", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
	}
}

// executeAction performs the action selected from the menu
func (v *SubnetsView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'c':
		return func() tea.Msg { return SubnetCreateRequestMsg{} }
	case 'r':
		v.loading = true
		v.err = nil
		v.registerTask("load-subnets", "Refreshing...")
		if v.computeClient == nil {
			return tea.Batch(v.spinner.Tick, v.initComputeClient())
		}
		return tea.Batch(v.spinner.Tick, v.loadSubnets())
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

// initiateDelete shows the delete confirmation dialog for the selected subnet
func (v *SubnetsView) initiateDelete() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	s, ok := v.findSubnetByID(row.ID)
	if !ok {
		return nil
	}

	v.pendingDelete = &s

	detailLines := []string{
		fmt.Sprintf("Network: %s", s.Network),
		fmt.Sprintf("Region: %s", s.Region),
		fmt.Sprintf("CIDR: %s", s.IPCidrRange),
	}
	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Subnet", s.Name, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

// View renders the subnets view
func (v *SubnetsView) View() string {
	if v.loading && v.computeClient == nil {
		return renderLoading(v.spinner, "Initializing Compute Engine client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading subnets...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.subnets) == 0 {
		return "\n  No subnets found.\n  Press 'c' to create a subnet or 'esc' to go back."
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • .: actions • c: create • D: delete • S: sort • /: filter • r: refresh • esc: back")

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
func (v *SubnetsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// HasTextInputFocused returns true if the delete confirm input or table filter is active.
// Prevents global hotkeys (like 'q' for quit) from triggering while typing.
func (v *SubnetsView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return v.table.HasTextInputFocused()
}

// IsMenuOpen returns true if the action menu or delete confirm is open
func (v *SubnetsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetComputeClient returns the compute client for reuse in detail views
func (v *SubnetsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// SetContext updates the view with shared program context.
func (v *SubnetsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

// findSubnetByID finds a subnet by its composite ID (Name/Region) in the loaded list
func (v *SubnetsView) findSubnetByID(id string) (gcp.Subnet, bool) {
	for _, s := range v.subnets {
		if s.Name+"/"+s.Region == id {
			return s, true
		}
	}
	return gcp.Subnet{}, false
}

// selectSubnet emits a SubnetSelectedMsg for the selected subnet
func (v *SubnetsView) selectSubnet() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	s, ok := v.findSubnetByID(row.ID)
	if !ok {
		return nil
	}
	return func() tea.Msg { return SubnetSelectedMsg{SubnetName: s.Name, Region: s.Region} }
}

// Task registration helpers for status bar integration
func (v *SubnetsView) registerTask(id, description string) {
	if v.ctx != nil && v.ctx.Tasks != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskRunning,
			StartTime:   time.Now(),
		}
	}
}

func (v *SubnetsView) clearTask(id string) {
	if v.ctx != nil && v.ctx.Tasks != nil {
		delete(v.ctx.Tasks, id)
	}
}

// failTask marks a task as failed and returns a command to clear it after a delay
func (v *SubnetsView) failTask(id string, err error) tea.Cmd {
	if v.ctx != nil && v.ctx.Tasks != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: err.Error(),
			State:       context.TaskError,
			Error:       err,
		}
	}
	// Schedule task removal after 5 seconds to give user time to see the error
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return context.TaskClearMsg{TaskID: id}
	})
}
