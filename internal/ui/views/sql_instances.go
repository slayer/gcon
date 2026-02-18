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

// SQLInstancesView displays Cloud SQL instances in a table format
type SQLInstancesView struct {
	TableClickDelegate
	sqlClient *gcp.SQLClient
	projectID string
	ctx       *context.ProgramContext
	table     table.Model
	spinner   spinner.Model
	loading   bool
	err       error
	instances []gcp.SQLInstance
	keys      sqlInstancesKeyMap

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.SQLInstance

	width  int
	height int
}

// sqlInstancesKeyMap defines SQL instances-specific key bindings
type sqlInstancesKeyMap struct {
	Select     key.Binding
	Refresh    key.Binding
	Start      key.Binding
	Stop       key.Binding
	Restart    key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
}

func defaultSQLInstancesKeyMap() sqlInstancesKeyMap {
	return sqlInstancesKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Start: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "start"),
		),
		Stop: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "stop"),
		),
		Restart: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "restart"),
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
func sqlInstanceColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 25, Grow: true, Sortable: true},
		{Title: "Version", Width: 14, Sortable: true},
		{Title: "State", Width: 10, Sortable: true},
		{Title: "Region", Width: 16, Sortable: true},
		{Title: "Tier", Width: 20, Sortable: true},
		{Title: "Primary IP", Width: 15},
	}
}

// Internal message types for async operations
type sqlClientReadyMsg struct{ client *gcp.SQLClient }
type sqlInstancesLoadedMsg struct{ instances []gcp.SQLInstance }
type sqlInstancesErrorMsg struct{ err error }

// NewSQLInstancesView creates a new Cloud SQL instances view with table display
func NewSQLInstancesView(projectID string) *SQLInstancesView {
	title := fmt.Sprintf("Cloud SQL Instances - %s", projectID)
	t := table.NewWithColumns(sqlInstanceColumns(), title)

	s := components.NewGCPSpinner()

	v := &SQLInstancesView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultSQLInstancesKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init initializes the view and starts loading SQL instances
func (v *SQLInstancesView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initSQLClient(),
	)
}

// initSQLClient creates the SQL client then loads instances
func (v *SQLInstancesView) initSQLClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewSQLClient(gocontext.Background())
		if err != nil {
			return sqlInstancesErrorMsg{err: err}
		}
		return sqlClientReadyMsg{client: client}
	}
}

// loadInstances fetches Cloud SQL instances from GCP
func (v *SQLInstancesView) loadInstances() tea.Cmd {
	return func() tea.Msg {
		if v.sqlClient == nil {
			return sqlInstancesErrorMsg{err: uierrors.ErrSQLClientNotInitialized}
		}
		instances, err := v.sqlClient.ListInstances(gocontext.Background(), v.projectID)
		if err != nil {
			return sqlInstancesErrorMsg{err: err}
		}
		return sqlInstancesLoadedMsg{instances: instances}
	}
}

// sqlInstanceToRow converts a GCP SQL instance to a table row
func sqlInstanceToRow(inst gcp.SQLInstance) table.Row { //nolint:gocritic // Copying instance is acceptable
	// State column: green for running, red for stopped, yellow for transitioning
	var status string
	switch inst.State {
	case "RUNNABLE":
		status = symbols.StatusRunning()
	case "STOPPED", "SUSPENDED", "FAILED":
		status = symbols.StatusStopped()
	default:
		status = symbols.StatusTransitioning()
	}

	return table.Row{
		Data: []string{
			inst.Name,
			gcp.FormatDatabaseVersion(inst.DatabaseVersion),
			status,
			inst.Region,
			inst.Tier,
			inst.PrimaryIP,
		},
		FilterValue: inst.Name + " " + inst.DatabaseVersion + " " + inst.State + " " + inst.Region + " " + inst.Tier,
		ID:          inst.Name,
	}
}

// Update handles messages for the SQL instances view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *SQLInstancesView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case sqlClientReadyMsg:
		v.sqlClient = msg.client
		v.registerTask("load-sql-instances", "Loading Cloud SQL instances...")
		return v.loadInstances()

	case sqlInstancesLoadedMsg:
		v.loading = false
		v.instances = msg.instances
		v.clearTask("load-sql-instances")

		rows := make([]table.Row, len(msg.instances))
		for i, inst := range msg.instances {
			rows[i] = sqlInstanceToRow(inst)
		}
		v.table.SetRows(rows)
		return nil

	case sqlInstancesErrorMsg:
		v.loading = false
		v.err = msg.err
		return v.failTask("load-sql-instances", msg.err)

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
			inst := v.pendingDelete
			v.pendingDelete = nil
			return func() tea.Msg {
				return DeleteSQLInstanceConfirmedMsg{InstanceName: inst.Name}
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
		if inst, ok := v.findInstanceByName(msg.RowID); ok {
			return func() tea.Msg { return SQLInstanceSelectedMsg{Instance: inst} }
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
				if inst, ok := v.findInstanceByName(row.ID); ok {
					v.actionMenu = actionmenu.New("SQL Instance Actions", v.buildActions(inst))
					v.menuOpen = true
				}
			}
			return nil

		case key.Matches(msg, v.keys.Select):
			if row := v.table.SelectedRow(); row != nil {
				if inst, ok := v.findInstanceByName(row.ID); ok {
					return func() tea.Msg { return SQLInstanceSelectedMsg{Instance: inst} }
				}
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.registerTask("load-sql-instances", "Refreshing...")
			// Re-initialize client if previous attempt failed
			if v.sqlClient == nil {
				return tea.Batch(v.spinner.Tick, v.initSQLClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadInstances())

		case key.Matches(msg, v.keys.Start):
			if row := v.table.SelectedRow(); row != nil {
				if inst, ok := v.findInstanceByName(row.ID); ok && inst.IsStopped() {
					return func() tea.Msg {
						return SQLInstanceActionMsg{InstanceName: inst.Name, Action: "start"}
					}
				}
			}
			return nil

		case key.Matches(msg, v.keys.Stop):
			if row := v.table.SelectedRow(); row != nil {
				if inst, ok := v.findInstanceByName(row.ID); ok && inst.IsRunnable() {
					return func() tea.Msg {
						return SQLInstanceActionMsg{InstanceName: inst.Name, Action: "stop"}
					}
				}
			}
			return nil

		case key.Matches(msg, v.keys.Restart):
			if row := v.table.SelectedRow(); row != nil {
				if inst, ok := v.findInstanceByName(row.ID); ok && inst.IsRunnable() {
					return func() tea.Msg {
						return SQLInstanceActionMsg{InstanceName: inst.Name, Action: "restart"}
					}
				}
			}
			return nil

		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// buildActions creates the action menu items based on instance state
func (v *SQLInstancesView) buildActions(inst gcp.SQLInstance) []actionmenu.Action { //nolint:gocritic // Copying instance is acceptable
	actions := []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
	}

	if inst.IsStopped() {
		actions = append(actions, actionmenu.Action{Key: 's', Label: "Start", Enabled: true})
	}
	if inst.IsRunnable() {
		actions = append(actions,
			actionmenu.Action{Key: 'x', Label: "Stop", Enabled: true},
			actionmenu.Action{Key: 'R', Label: "Restart", Enabled: true},
		)
	}

	actions = append(actions, actionmenu.Action{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true})
	return actions
}

// executeAction performs the action selected from the menu
func (v *SQLInstancesView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'r':
		v.loading = true
		v.err = nil
		v.registerTask("load-sql-instances", "Refreshing...")
		if v.sqlClient == nil {
			return tea.Batch(v.spinner.Tick, v.initSQLClient())
		}
		return tea.Batch(v.spinner.Tick, v.loadInstances())
	case 's':
		if row := v.table.SelectedRow(); row != nil {
			if inst, ok := v.findInstanceByName(row.ID); ok && inst.IsStopped() {
				return func() tea.Msg {
					return SQLInstanceActionMsg{InstanceName: inst.Name, Action: "start"}
				}
			}
		}
	case 'x':
		if row := v.table.SelectedRow(); row != nil {
			if inst, ok := v.findInstanceByName(row.ID); ok && inst.IsRunnable() {
				return func() tea.Msg {
					return SQLInstanceActionMsg{InstanceName: inst.Name, Action: "stop"}
				}
			}
		}
	case 'R':
		if row := v.table.SelectedRow(); row != nil {
			if inst, ok := v.findInstanceByName(row.ID); ok && inst.IsRunnable() {
				return func() tea.Msg {
					return SQLInstanceActionMsg{InstanceName: inst.Name, Action: "restart"}
				}
			}
		}
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

// initiateDelete shows the delete confirmation dialog for the selected instance
func (v *SQLInstancesView) initiateDelete() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	inst, ok := v.findInstanceByName(row.ID)
	if !ok {
		return nil
	}

	v.pendingDelete = &inst

	detailLines := []string{
		fmt.Sprintf("Version: %s", gcp.FormatDatabaseVersion(inst.DatabaseVersion)),
		fmt.Sprintf("Region: %s", inst.Region),
		fmt.Sprintf("State: %s", inst.State),
	}
	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete SQL Instance", inst.Name, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

// View renders the SQL instances view
func (v *SQLInstancesView) View() string {
	if v.loading && v.sqlClient == nil {
		return renderLoading(v.spinner, "Initializing Cloud SQL client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading Cloud SQL instances...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.instances) == 0 {
		return "\n  No Cloud SQL instances found in this project.\n  Press 'esc' to go back."
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • .: actions • s: start • x: stop • R: restart • D: delete • S: sort • /: filter • r: refresh • esc: back")

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
func (v *SQLInstancesView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// HasTextInputFocused returns true if the delete confirm input or table filter is active.
// Prevents global hotkeys (like 'q' for quit) from triggering while typing.
func (v *SQLInstancesView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return v.table.HasTextInputFocused()
}

// IsMenuOpen returns true if the action menu or delete confirm is open
func (v *SQLInstancesView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetSQLClient returns the SQL client for reuse in detail views
func (v *SQLInstancesView) GetSQLClient() *gcp.SQLClient {
	return v.sqlClient
}

// SetContext updates the view with shared program context.
func (v *SQLInstancesView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

// findInstanceByName finds a SQL instance by name in the loaded list
func (v *SQLInstancesView) findInstanceByName(name string) (gcp.SQLInstance, bool) {
	for _, inst := range v.instances {
		if inst.Name == name {
			return inst, true
		}
	}
	return gcp.SQLInstance{}, false
}

// Task registration helpers for status bar integration
func (v *SQLInstancesView) registerTask(id, description string) {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskRunning,
			StartTime:   time.Now(),
		}
	}
}

func (v *SQLInstancesView) clearTask(id string) {
	if v.ctx != nil {
		delete(v.ctx.Tasks, id)
	}
}

// failTask marks a task as failed and returns a command to clear it after a delay
func (v *SQLInstancesView) failTask(id string, err error) tea.Cmd {
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
