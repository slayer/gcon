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
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
)

// CustomRolesView displays custom IAM roles in a table format
type CustomRolesView struct {
	TableClickDelegate
	iamClient *gcp.IAMClient
	projectID string
	ctx       *context.ProgramContext
	table     table.Model
	spinner   spinner.Model
	loading   bool
	err       error
	roles     []gcp.CustomRole
	keys      customRolesKeyMap

	width  int
	height int
}

type customRolesKeyMap struct {
	Select  key.Binding
	Refresh key.Binding
}

func defaultCustomRolesKeyMap() customRolesKeyMap {
	return customRolesKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

func customRoleColumns() []table.Column {
	return []table.Column{
		{Title: "Title", Width: 30, Grow: true, Sortable: true},
		{Title: "Role ID", Width: 30, Sortable: true},
		{Title: "Stage", Width: 12, Sortable: true},
		{Title: "Permissions", Width: 12, Sortable: true},
	}
}

// Internal message types
type customRolesClientReadyMsg struct{ client *gcp.IAMClient }
type customRolesLoadedMsg struct{ roles []gcp.CustomRole }
type customRolesErrorMsg struct{ err error }

// NewCustomRolesView creates a new custom roles list view
func NewCustomRolesView(projectID string) *CustomRolesView {
	title := fmt.Sprintf("Custom Roles - %s", projectID)
	t := table.NewWithColumns(customRoleColumns(), title)
	s := components.NewGCPSpinner()

	v := &CustomRolesView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultCustomRolesKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init initializes the view
func (v *CustomRolesView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initIAMClient(),
	)
}

func (v *CustomRolesView) initIAMClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewIAMClient(gocontext.Background())
		if err != nil {
			return customRolesErrorMsg{err: err}
		}
		return customRolesClientReadyMsg{client: client}
	}
}

func (v *CustomRolesView) loadRoles() tea.Cmd {
	return func() tea.Msg {
		if v.iamClient == nil {
			return customRolesErrorMsg{err: uierrors.ErrIAMClientNotInitialized}
		}
		roles, err := v.iamClient.ListCustomRoles(gocontext.Background(), v.projectID)
		if err != nil {
			return customRolesErrorMsg{err: err}
		}
		return customRolesLoadedMsg{roles: roles}
	}
}

func customRoleToRow(role gcp.CustomRole) table.Row { //nolint:gocritic // Copying is acceptable
	return table.Row{
		Data: []string{
			role.Title,
			role.RoleID,
			role.Stage,
			fmt.Sprintf("%d", len(role.Permissions)),
		},
		FilterValue: role.Title + " " + role.RoleID + " " + role.Stage,
		ID:          role.RoleID,
	}
}

// Update handles messages
//
//nolint:gocognit // Bubble Tea Update pattern
func (v *CustomRolesView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case customRolesClientReadyMsg:
		v.iamClient = msg.client
		v.registerTask("load-custom-roles", "Loading custom roles...")
		return v.loadRoles()

	case customRolesLoadedMsg:
		v.loading = false
		v.roles = msg.roles
		v.clearTask("load-custom-roles")

		rows := make([]table.Row, len(msg.roles))
		for i, role := range msg.roles {
			rows[i] = customRoleToRow(role)
		}
		v.table.SetRows(rows)
		return nil

	case customRolesErrorMsg:
		v.loading = false
		v.err = msg.err
		return v.failTask("load-custom-roles", msg.err)

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case table.RowDoubleClickedMsg:
		if role, ok := v.findRoleByID(msg.RowID); ok {
			return func() tea.Msg { return CustomRoleSelectedMsg{Role: role} }
		}
		return nil

	case tea.KeyMsg:
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
		case key.Matches(msg, v.keys.Select):
			if row := v.table.SelectedRow(); row != nil {
				if role, ok := v.findRoleByID(row.ID); ok {
					return func() tea.Msg { return CustomRoleSelectedMsg{Role: role} }
				}
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.registerTask("load-custom-roles", "Refreshing...")
			if v.iamClient == nil {
				return tea.Batch(v.spinner.Tick, v.initIAMClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadRoles())
		}
	}

	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// View renders the custom roles view
func (v *CustomRolesView) View() string {
	if v.loading && v.iamClient == nil {
		return renderLoading(v.spinner, "Initializing IAM client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading custom roles...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.roles) == 0 {
		return "\n  No custom roles found in this project.\n  Press 'esc' to go back."
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • S: sort • /: filter • r: refresh • esc: back")

	return v.table.View() + help
}

// SetContext updates the view with shared program context
func (v *CustomRolesView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

// GetIAMClient returns the IAM client for reuse
func (v *CustomRolesView) GetIAMClient() *gcp.IAMClient {
	return v.iamClient
}

func (v *CustomRolesView) findRoleByID(roleID string) (gcp.CustomRole, bool) {
	for _, role := range v.roles {
		if role.RoleID == roleID {
			return role, true
		}
	}
	return gcp.CustomRole{}, false
}

func (v *CustomRolesView) registerTask(id, description string) {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskRunning,
			StartTime:   time.Now(),
		}
	}
}

func (v *CustomRolesView) clearTask(id string) {
	if v.ctx != nil {
		delete(v.ctx.Tasks, id)
	}
}

func (v *CustomRolesView) failTask(id string, err error) tea.Cmd {
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
