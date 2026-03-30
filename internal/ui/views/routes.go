package views

import (
	gocontext "context"
	"fmt"
	"strconv"
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
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// RoutesView displays routes in a project in a table format
type RoutesView struct {
	TableClickDelegate
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	routes        []gcp.Route
	keys          routeKeyMap

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation — only for Static routes
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.Route

	width  int
	height int
}

// routeKeyMap defines route-specific key bindings
type routeKeyMap struct {
	Select     key.Binding
	Refresh    key.Binding
	Create     key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
}

func defaultRouteKeyMap() routeKeyMap {
	return routeKeyMap{
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
func routeColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 20, Grow: true, Sortable: true},
		{Title: "Network", Width: 12, Sortable: true},
		{Title: "Dest Range", Width: 18, Sortable: true},
		{Title: "Priority", Width: 10, Sortable: true},
		{Title: "Next Hop", Width: 24, Sortable: true},
		{Title: "Type", Width: 10, Sortable: true},
		{Title: "Created", Width: 22, Sortable: true},
	}
}

// Internal message types for async operations
type routesClientReadyMsg struct{ client *gcp.ComputeClient }
type routesLoadedMsg struct{ routes []gcp.Route }
type routesErrorMsg struct{ err error }

// NewRoutesView creates a new routes view with table display
func NewRoutesView(projectID string) *RoutesView {
	title := fmt.Sprintf("Routes - %s", projectID)
	t := table.NewWithColumns(routeColumns(), title)

	s := components.NewGCPSpinner()

	v := &RoutesView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultRouteKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init initializes the view and starts loading routes
func (v *RoutesView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	return tea.Batch(
		v.spinner.Tick,
		v.initComputeClient(),
	)
}

// initComputeClient creates the compute client then loads routes
func (v *RoutesView) initComputeClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(gocontext.Background())
		if err != nil {
			return routesErrorMsg{err: err}
		}
		return routesClientReadyMsg{client: client}
	}
}

// loadRoutes fetches routes from GCP
func (v *RoutesView) loadRoutes() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return routesErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		routes, err := v.computeClient.ListRoutes(gocontext.Background(), v.projectID)
		if err != nil {
			return routesErrorMsg{err: err}
		}
		return routesLoadedMsg{routes: routes}
	}
}

// routeToRow converts a GCP route to a table row.
func routeToRow(r gcp.Route) table.Row { //nolint:gocritic // Copying route is acceptable
	return table.Row{
		Data: []string{
			r.Name,
			r.Network,
			r.DestRange,
			strconv.FormatInt(r.Priority, 10),
			r.NextHop,
			r.RouteType,
			timeutil.FormatTimestamp(r.CreatedAt),
		},
		FilterValue: r.Name + " " +
			r.Network + " " +
			r.DestRange + " " +
			strconv.FormatInt(r.Priority, 10) + " " +
			r.NextHop + " " +
			r.RouteType + " " +
			timeutil.FormatTimestamp(r.CreatedAt),
		// Routes are globally unique by name
		ID: r.Name,
	}
}

// Update handles messages for the routes view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *RoutesView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case routesClientReadyMsg:
		v.computeClient = msg.client
		v.registerTask("load-routes", "Loading routes...")
		return v.loadRoutes()

	case routesLoadedMsg:
		v.loading = false
		v.routes = msg.routes
		v.clearTask("load-routes")

		rows := make([]table.Row, len(msg.routes))
		for i := range msg.routes {
			rows[i] = routeToRow(msg.routes[i])
		}
		v.table.SetRows(rows)
		return nil

	case routesErrorMsg:
		v.loading = false
		v.err = msg.err
		return v.failTask("load-routes", msg.err)

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
			r := v.pendingDelete
			v.pendingDelete = nil
			return func() tea.Msg {
				return RouteDeleteRequestMsg{Name: r.Name}
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
		if r, ok := v.findRouteByName(msg.RowID); ok {
			return func() tea.Msg { return RouteSelectedMsg{Route: r} }
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
			v.actionMenu = actionmenu.New("Route Actions", v.buildActions())
			v.menuOpen = true
			return nil

		case key.Matches(msg, v.keys.Select):
			if row := v.table.SelectedRow(); row != nil {
				if r, ok := v.findRouteByName(row.ID); ok {
					return func() tea.Msg { return RouteSelectedMsg{Route: r} }
				}
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.registerTask("load-routes", "Refreshing...")
			// Re-initialize client if previous attempt failed
			if v.computeClient == nil {
				return tea.Batch(v.spinner.Tick, v.initComputeClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadRoutes())

		case key.Matches(msg, v.keys.Create):
			return func() tea.Msg { return RouteCreateRequestMsg{} }

		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// buildActions creates the action menu items.
// Delete is only enabled when the selected route is a static route.
func (v *RoutesView) buildActions() []actionmenu.Action {
	deleteEnabled := false
	if row := v.table.SelectedRow(); row != nil {
		if r, ok := v.findRouteByName(row.ID); ok {
			deleteEnabled = r.RouteType == "Static"
		}
	}

	return []actionmenu.Action{
		{Key: 'c', Label: "Create Route", Enabled: true},
		{Key: 'r', Label: "Refresh", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: deleteEnabled, Dangerous: true},
	}
}

// executeAction performs the action selected from the menu
func (v *RoutesView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'c':
		return func() tea.Msg { return RouteCreateRequestMsg{} }
	case 'r':
		v.loading = true
		v.err = nil
		v.registerTask("load-routes", "Refreshing...")
		if v.computeClient == nil {
			return tea.Batch(v.spinner.Tick, v.initComputeClient())
		}
		return tea.Batch(v.spinner.Tick, v.loadRoutes())
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

// initiateDelete shows the delete confirmation dialog for the selected route.
// Only static routes can be deleted — system/subnet/peering routes are managed by GCP.
func (v *RoutesView) initiateDelete() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	r, ok := v.findRouteByName(row.ID)
	if !ok {
		return nil
	}

	// Only static routes can be deleted
	if r.RouteType != "Static" {
		return nil
	}

	v.pendingDelete = &r

	detailLines := []string{
		fmt.Sprintf("Network: %s", r.Network),
		fmt.Sprintf("Destination: %s", r.DestRange),
		fmt.Sprintf("Priority: %d", r.Priority),
	}
	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Route", r.Name, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

// View renders the routes view
func (v *RoutesView) View() string {
	if v.loading && v.computeClient == nil {
		return renderLoading(v.spinner, "Initializing Compute Engine client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading routes...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.routes) == 0 {
		return "\n  No routes found. Press 'c' to create a route or 'esc' to go back."
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
func (v *RoutesView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// HasTextInputFocused returns true if the delete confirm input or table filter is active.
// Prevents global hotkeys (like 'q' for quit) from triggering while typing.
func (v *RoutesView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return v.table.HasTextInputFocused()
}

// IsMenuOpen returns true if the action menu or delete confirm is open
func (v *RoutesView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetComputeClient returns the compute client for reuse in detail views
func (v *RoutesView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// SetContext updates the view with shared program context.
func (v *RoutesView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

// findRouteByName finds a route by name in the loaded slice
func (v *RoutesView) findRouteByName(name string) (gcp.Route, bool) {
	for i := range v.routes {
		if v.routes[i].Name == name {
			return v.routes[i], true
		}
	}
	return gcp.Route{}, false
}

// Task registration helpers for status bar integration
func (v *RoutesView) registerTask(id, description string) {
	if v.ctx != nil && v.ctx.Tasks != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskRunning,
			StartTime:   time.Now(),
		}
	}
}

func (v *RoutesView) clearTask(id string) {
	if v.ctx != nil && v.ctx.Tasks != nil {
		delete(v.ctx.Tasks, id)
	}
}

// failTask marks a task as failed and returns a command to clear it after a delay
func (v *RoutesView) failTask(id string, err error) tea.Cmd {
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
