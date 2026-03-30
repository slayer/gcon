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

// CloudRunServicesView displays Cloud Run services in a table
type CloudRunServicesView struct {
	TableClickDelegate
	runClient *gcp.CloudRunClient
	projectID string
	ctx       *context.ProgramContext
	table     table.Model
	spinner   spinner.Model
	loading   bool
	err       error
	services  []gcp.CloudRunService
	keys      cloudRunServicesKeyMap

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.CloudRunService

	width  int
	height int
}

type cloudRunServicesKeyMap struct {
	Select     key.Binding
	Refresh    key.Binding
	Create     key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
}

func defaultCloudRunServicesKeyMap() cloudRunServicesKeyMap {
	return cloudRunServicesKeyMap{
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
			key.WithHelp("c", "create service"),
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

func cloudRunServiceColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 16, Grow: true, Sortable: true},
		{Title: "Region", Width: 12, Sortable: true},
		{Title: "Status", Width: 8, Sortable: true},
		{Title: "URL", Width: 14, Grow: true, Sortable: true},
		{Title: "Revision", Width: 14, Sortable: true},
		{Title: "Updated", Width: 14, Sortable: true},
	}
}

// Internal messages for two-phase init
type cloudRunClientReadyMsg struct{ client *gcp.CloudRunClient }
type cloudRunServicesLoadedMsg struct{ services []gcp.CloudRunService }
type cloudRunServicesErrorMsg struct{ err error }

// NewCloudRunServicesView creates a new Cloud Run services list view
func NewCloudRunServicesView(projectID string) *CloudRunServicesView {
	title := fmt.Sprintf("Cloud Run Services - %s", projectID)
	t := table.NewWithColumns(cloudRunServiceColumns(), title)

	s := components.NewGCPSpinner()

	v := &CloudRunServicesView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultCloudRunServicesKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init initializes the view and starts loading services
func (v *CloudRunServicesView) Init() tea.Cmd {
	// Reset state — Init() may be called multiple times (e.g., after delete navigates back)
	v.loading = true
	v.err = nil

	return tea.Batch(
		v.spinner.Tick,
		v.initRunClient(),
	)
}

func (v *CloudRunServicesView) initRunClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewCloudRunClient(gocontext.Background())
		if err != nil {
			return cloudRunServicesErrorMsg{err: err}
		}
		return cloudRunClientReadyMsg{client: client}
	}
}

func (v *CloudRunServicesView) loadServices() tea.Cmd {
	return func() tea.Msg {
		if v.runClient == nil {
			return cloudRunServicesErrorMsg{err: uierrors.ErrCloudRunClientNotInitialized}
		}
		services, err := v.runClient.ListServices(gocontext.Background(), v.projectID)
		if err != nil {
			return cloudRunServicesErrorMsg{err: err}
		}
		return cloudRunServicesLoadedMsg{services: services}
	}
}

func cloudRunServiceToRow(svc gcp.CloudRunService) table.Row { //nolint:gocritic // Copying is acceptable
	var status string
	switch svc.Status {
	case "Ready":
		status = symbols.StatusRunning()
	case "Failed":
		status = symbols.StatusStopped()
	default:
		status = symbols.StatusTransitioning()
	}

	return table.Row{
		Data: []string{
			svc.Name,
			svc.Region,
			status,
			svc.URL,
			svc.LatestRevision,
			svc.UpdatedAt,
		},
		FilterValue: svc.Name + " " + svc.Region + " " + svc.Status + " " + svc.URL,
		ID:          svc.FullName, // Use full name as ID for navigation
	}
}

// Update handles messages for the Cloud Run services view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *CloudRunServicesView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case cloudRunClientReadyMsg:
		v.runClient = msg.client
		v.registerTask("load-cloudrun-services", "Loading Cloud Run services...")
		return v.loadServices()

	case cloudRunServicesLoadedMsg:
		v.loading = false
		v.services = msg.services
		v.clearTask("load-cloudrun-services")

		rows := make([]table.Row, len(msg.services))
		for i, svc := range msg.services {
			rows[i] = cloudRunServiceToRow(svc)
		}
		v.table.SetRows(rows)
		return nil

	case cloudRunServicesErrorMsg:
		v.loading = false
		v.err = msg.err
		return v.failTask("load-cloudrun-services", msg.err)

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
			svc := v.pendingDelete
			v.pendingDelete = nil
			return func() tea.Msg {
				return DeleteCloudRunServiceConfirmedMsg{
					FullName: svc.FullName,
					Name:     svc.Name,
				}
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
		if svc, ok := v.findServiceByFullName(msg.RowID); ok {
			return func() tea.Msg { return CloudRunServiceSelectedMsg{Service: svc} }
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
				if _, ok := v.findServiceByFullName(row.ID); ok {
					v.actionMenu = actionmenu.New("Cloud Run Actions", v.buildActions())
					v.menuOpen = true
				}
			}
			return nil

		case key.Matches(msg, v.keys.Select):
			if row := v.table.SelectedRow(); row != nil {
				if svc, ok := v.findServiceByFullName(row.ID); ok {
					return func() tea.Msg { return CloudRunServiceSelectedMsg{Service: svc} }
				}
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.registerTask("load-cloudrun-services", "Refreshing...")
			if v.runClient == nil {
				return tea.Batch(v.spinner.Tick, v.initRunClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadServices())

		case key.Matches(msg, v.keys.Create):
			return func() tea.Msg {
				return CloudRunCreateRequestMsg{ProjectID: v.projectID}
			}

		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

func (v *CloudRunServicesView) buildActions() []actionmenu.Action {
	return []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
		{Key: 'c', Label: "Create Service", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
	}
}

func (v *CloudRunServicesView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'r':
		v.loading = true
		v.err = nil
		v.registerTask("load-cloudrun-services", "Refreshing...")
		if v.runClient == nil {
			return tea.Batch(v.spinner.Tick, v.initRunClient())
		}
		return tea.Batch(v.spinner.Tick, v.loadServices())
	case 'c':
		return func() tea.Msg {
			return CloudRunCreateRequestMsg{ProjectID: v.projectID}
		}
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

func (v *CloudRunServicesView) initiateDelete() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	svc, ok := v.findServiceByFullName(row.ID)
	if !ok {
		return nil
	}

	v.pendingDelete = &svc

	detailLines := []string{
		fmt.Sprintf("Region: %s", svc.Region),
		fmt.Sprintf("Status: %s", svc.Status),
	}
	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Cloud Run Service", svc.Name, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

// View renders the Cloud Run services view
func (v *CloudRunServicesView) View() string {
	if v.loading && v.runClient == nil {
		return renderLoading(v.spinner, "Initializing Cloud Run client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading Cloud Run services...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.services) == 0 {
		return "\n  No Cloud Run services found in this project.\n  Press 'esc' to go back."
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • .: actions • D: delete • S: sort • /: filter • r: refresh • esc: back")

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

func (v *CloudRunServicesView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// HasTextInputFocused returns true if the delete confirm input or table filter is active
func (v *CloudRunServicesView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return v.table.HasTextInputFocused()
}

// IsMenuOpen returns true if the action menu or delete confirm is open
func (v *CloudRunServicesView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetCloudRunClient returns the Cloud Run client for reuse in detail views
func (v *CloudRunServicesView) GetCloudRunClient() *gcp.CloudRunClient {
	return v.runClient
}

// SetContext updates the view with shared program context
func (v *CloudRunServicesView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

func (v *CloudRunServicesView) findServiceByFullName(fullName string) (gcp.CloudRunService, bool) {
	for _, svc := range v.services {
		if svc.FullName == fullName {
			return svc, true
		}
	}
	return gcp.CloudRunService{}, false
}

// Task registration helpers for status bar integration
func (v *CloudRunServicesView) registerTask(id, description string) {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskRunning,
			StartTime:   time.Now(),
		}
	}
}

func (v *CloudRunServicesView) clearTask(id string) {
	if v.ctx != nil {
		delete(v.ctx.Tasks, id)
	}
}

func (v *CloudRunServicesView) failTask(id string, err error) tea.Cmd {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: err.Error(),
			State:       context.TaskError,
			Error:       err,
		}
	}
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return context.TaskClearMsg{TaskID: id}
	})
}
