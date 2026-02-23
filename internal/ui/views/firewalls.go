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
	"github.com/slayer/gcon/internal/ui/symbols"
)

// FirewallsView displays firewall rules in a table format
type FirewallsView struct {
	TableClickDelegate
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	firewalls     []gcp.FirewallRule
	keys          firewallKeyMap

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.FirewallRule

	width  int
	height int
}

// firewallKeyMap defines firewall-specific key bindings
type firewallKeyMap struct {
	Select     key.Binding
	Refresh    key.Binding
	Toggle     key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
}

func defaultFirewallKeyMap() firewallKeyMap {
	return firewallKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "enable/disable"),
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
func firewallColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 25, Grow: true, Sortable: true},
		{Title: "Direction", Width: 10, Sortable: true},
		{Title: "Priority", Width: 8, Sortable: true},
		{Title: "Action", Width: 8, Sortable: true},
		{Title: "Protocols", Width: 25},
		{Title: "Network", Width: 15},
		{Title: "Status", Width: 8, Sortable: true},
	}
}

// Internal message types for async operations
type firewallsClientReadyMsg struct{ client *gcp.ComputeClient }
type firewallsLoadedMsg struct{ firewalls []gcp.FirewallRule }
type firewallsErrorMsg struct{ err error }

// NewFirewallsView creates a new firewall rules view with table display
func NewFirewallsView(projectID string) *FirewallsView {
	title := fmt.Sprintf("Firewall Rules - %s", projectID)
	t := table.NewWithColumns(firewallColumns(), title)

	s := components.NewGCPSpinner()

	v := &FirewallsView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultFirewallKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init initializes the view and starts loading firewall rules
func (v *FirewallsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initComputeClient(),
	)
}

// initComputeClient creates the compute client then loads firewall rules
func (v *FirewallsView) initComputeClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(gocontext.Background())
		if err != nil {
			return firewallsErrorMsg{err: err}
		}
		return firewallsClientReadyMsg{client: client}
	}
}

// loadFirewalls fetches firewall rules from GCP
func (v *FirewallsView) loadFirewalls() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return firewallsErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		firewalls, err := v.computeClient.ListFirewallRules(gocontext.Background(), v.projectID)
		if err != nil {
			return firewallsErrorMsg{err: err}
		}
		return firewallsLoadedMsg{firewalls: firewalls}
	}
}

// firewallToRow converts a GCP firewall rule to a table row
func firewallToRow(fw gcp.FirewallRule) table.Row { //nolint:gocritic // Copying firewall is acceptable
	// Status column: green for enabled, red for disabled
	status := symbols.StatusRunning()
	if fw.Disabled {
		status = symbols.StatusStopped()
	}

	return table.Row{
		Data: []string{
			fw.Name,
			fw.Direction,
			fmt.Sprintf("%d", fw.Priority),
			fw.Action,
			fw.Protocols,
			fw.Network,
			status,
		},
		FilterValue: fw.Name + " " + fw.Direction + " " + fw.Action + " " + fw.Network + " " + fw.Protocols,
		ID:          fw.Name,
	}
}

// Update handles messages for the firewalls view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *FirewallsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case firewallsClientReadyMsg:
		v.computeClient = msg.client
		v.registerTask("load-firewalls", "Loading firewall rules...")
		return v.loadFirewalls()

	case firewallsLoadedMsg:
		v.loading = false
		v.firewalls = msg.firewalls
		v.clearTask("load-firewalls")

		rows := make([]table.Row, len(msg.firewalls))
		for i, fw := range msg.firewalls {
			rows[i] = firewallToRow(fw)
		}
		v.table.SetRows(rows)
		return nil

	case firewallsErrorMsg:
		v.loading = false
		v.err = msg.err
		return v.failTask("load-firewalls", msg.err)

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
			fw := v.pendingDelete
			v.pendingDelete = nil
			return func() tea.Msg {
				return DeleteFirewallConfirmedMsg{RuleName: fw.Name}
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
		if fw, ok := v.findFirewallByName(msg.RowID); ok {
			return func() tea.Msg { return FirewallSelectedMsg{Firewall: fw} }
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
			if row := v.table.SelectedRow(); row != nil {
				if fw, ok := v.findFirewallByName(row.ID); ok {
					v.actionMenu = actionmenu.New("Firewall Actions", v.buildActions(fw))
					v.menuOpen = true
				}
			}
			return nil

		case key.Matches(msg, v.keys.Select):
			if row := v.table.SelectedRow(); row != nil {
				if fw, ok := v.findFirewallByName(row.ID); ok {
					return func() tea.Msg { return FirewallSelectedMsg{Firewall: fw} }
				}
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.registerTask("load-firewalls", "Refreshing...")
			// Re-initialize client if previous attempt failed
			if v.computeClient == nil {
				return tea.Batch(v.spinner.Tick, v.initComputeClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadFirewalls())

		case key.Matches(msg, v.keys.Toggle):
			return v.toggleSelectedFirewall()

		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// buildActions creates the action menu items based on firewall state
func (v *FirewallsView) buildActions(fw gcp.FirewallRule) []actionmenu.Action { //nolint:gocritic // Copying firewall is acceptable
	toggleLabel := "Disable"
	if fw.Disabled {
		toggleLabel = "Enable"
	}

	return []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
		{Key: 't', Label: toggleLabel, Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
	}
}

// executeAction performs the action selected from the menu
func (v *FirewallsView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'r':
		v.loading = true
		v.err = nil
		v.registerTask("load-firewalls", "Refreshing...")
		if v.computeClient == nil {
			return tea.Batch(v.spinner.Tick, v.initComputeClient())
		}
		return tea.Batch(v.spinner.Tick, v.loadFirewalls())
	case 't':
		return v.toggleSelectedFirewall()
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

// toggleSelectedFirewall emits a ToggleFirewallMsg for the selected rule
func (v *FirewallsView) toggleSelectedFirewall() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	fw, ok := v.findFirewallByName(row.ID)
	if !ok {
		return nil
	}

	// Invert current state: if currently disabled, we want to enable (Disable=false)
	return func() tea.Msg {
		return ToggleFirewallMsg{
			RuleName: fw.Name,
			Disable:  !fw.Disabled,
		}
	}
}

// initiateDelete shows the delete confirmation dialog for the selected rule
func (v *FirewallsView) initiateDelete() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	fw, ok := v.findFirewallByName(row.ID)
	if !ok {
		return nil
	}

	v.pendingDelete = &fw

	detailLines := []string{
		fmt.Sprintf("Direction: %s", fw.Direction),
		fmt.Sprintf("Priority: %d", fw.Priority),
		fmt.Sprintf("Action: %s", fw.Action),
	}
	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Firewall Rule", fw.Name, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

// View renders the firewalls view
func (v *FirewallsView) View() string {
	if v.loading && v.computeClient == nil {
		return renderLoading(v.spinner, "Initializing Compute Engine client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading firewall rules...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.firewalls) == 0 {
		return "\n  No firewall rules found in this project.\n  Press 'esc' to go back."
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • .: actions • t: enable/disable • D: delete • S: sort • /: filter • r: refresh • esc: back")

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
func (v *FirewallsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// HasTextInputFocused returns true if the delete confirm input or table filter is active.
// Prevents global hotkeys (like 'q' for quit) from triggering while typing.
func (v *FirewallsView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return v.table.HasTextInputFocused()
}

// IsMenuOpen returns true if the action menu or delete confirm is open
func (v *FirewallsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetComputeClient returns the compute client for reuse in detail views
func (v *FirewallsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// SetContext updates the view with shared program context.
func (v *FirewallsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

// findFirewallByName finds a firewall rule by name in the loaded list
func (v *FirewallsView) findFirewallByName(name string) (gcp.FirewallRule, bool) {
	for _, fw := range v.firewalls {
		if fw.Name == name {
			return fw, true
		}
	}
	return gcp.FirewallRule{}, false
}

// Task registration helpers for status bar integration
func (v *FirewallsView) registerTask(id, description string) {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskRunning,
			StartTime:   time.Now(),
		}
	}
}

func (v *FirewallsView) clearTask(id string) {
	if v.ctx != nil {
		delete(v.ctx.Tasks, id)
	}
}

// failTask marks a task as failed and returns a command to clear it after a delay
func (v *FirewallsView) failTask(id string, err error) tea.Cmd {
	if v.ctx != nil {
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
