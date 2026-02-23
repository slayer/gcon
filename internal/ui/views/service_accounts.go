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

// ServiceAccountsView displays service accounts in a table format
type ServiceAccountsView struct {
	TableClickDelegate
	iamClient *gcp.IAMClient
	projectID string
	ctx       *context.ProgramContext
	table     table.Model
	spinner   spinner.Model
	loading   bool
	err       error
	accounts  []gcp.ServiceAccount
	keys      serviceAccountsKeyMap

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.ServiceAccount

	width  int
	height int
}

type serviceAccountsKeyMap struct {
	Select     key.Binding
	Refresh    key.Binding
	Create     key.Binding
	Toggle     key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
}

func defaultServiceAccountsKeyMap() serviceAccountsKeyMap {
	return serviceAccountsKeyMap{
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

func serviceAccountColumns() []table.Column {
	return []table.Column{
		{Title: "Email", Width: 40, Grow: true, Sortable: true},
		{Title: "Display Name", Width: 25, Sortable: true},
		{Title: "Status", Width: 10, Sortable: true},
		{Title: "Unique ID", Width: 22, Sortable: true},
	}
}

// Internal message types for async operations
type iamClientReadyMsg struct{ client *gcp.IAMClient }
type serviceAccountsLoadedMsg struct{ accounts []gcp.ServiceAccount }
type serviceAccountsErrorMsg struct{ err error }

// NewServiceAccountsView creates a new service accounts list view
func NewServiceAccountsView(projectID string) *ServiceAccountsView {
	title := fmt.Sprintf("Service Accounts - %s", projectID)
	t := table.NewWithColumns(serviceAccountColumns(), title)

	s := components.NewGCPSpinner()

	v := &ServiceAccountsView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultServiceAccountsKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init initializes the view and starts loading service accounts
func (v *ServiceAccountsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initIAMClient(),
	)
}

// initIAMClient creates the IAM client then loads accounts
func (v *ServiceAccountsView) initIAMClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewIAMClient(gocontext.Background())
		if err != nil {
			return serviceAccountsErrorMsg{err: err}
		}
		return iamClientReadyMsg{client: client}
	}
}

// loadAccounts fetches service accounts from GCP
func (v *ServiceAccountsView) loadAccounts() tea.Cmd {
	return func() tea.Msg {
		if v.iamClient == nil {
			return serviceAccountsErrorMsg{err: uierrors.ErrIAMClientNotInitialized}
		}
		accounts, err := v.iamClient.ListServiceAccounts(gocontext.Background(), v.projectID)
		if err != nil {
			return serviceAccountsErrorMsg{err: err}
		}
		return serviceAccountsLoadedMsg{accounts: accounts}
	}
}

func serviceAccountToRow(sa gcp.ServiceAccount) table.Row { //nolint:gocritic // Copying is acceptable
	var status string
	if sa.Disabled {
		status = symbols.StatusStopped()
	} else {
		status = symbols.StatusRunning()
	}

	return table.Row{
		Data: []string{
			sa.Email,
			sa.DisplayName,
			status,
			sa.UniqueID,
		},
		FilterValue: sa.Email + " " + sa.DisplayName + " " + sa.UniqueID,
		ID:          sa.Email,
	}
}

// Update handles messages for the service accounts view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *ServiceAccountsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case iamClientReadyMsg:
		v.iamClient = msg.client
		v.registerTask("load-service-accounts", "Loading service accounts...")
		return v.loadAccounts()

	case serviceAccountsLoadedMsg:
		v.loading = false
		v.accounts = msg.accounts
		v.clearTask("load-service-accounts")

		rows := make([]table.Row, len(msg.accounts))
		for i, sa := range msg.accounts {
			rows[i] = serviceAccountToRow(sa)
		}
		v.table.SetRows(rows)
		return nil

	case serviceAccountsErrorMsg:
		v.loading = false
		v.err = msg.err
		return v.failTask("load-service-accounts", msg.err)

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
			sa := v.pendingDelete
			v.pendingDelete = nil
			return func() tea.Msg {
				return DeleteServiceAccountConfirmedMsg{Email: sa.Email}
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
		if sa, ok := v.findAccountByEmail(msg.RowID); ok {
			return func() tea.Msg { return ServiceAccountSelectedMsg{ServiceAccount: sa} }
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
				if sa, ok := v.findAccountByEmail(row.ID); ok {
					v.actionMenu = actionmenu.New("Service Account Actions", v.buildActions(sa))
					v.menuOpen = true
				}
			}
			return nil

		case key.Matches(msg, v.keys.Select):
			if row := v.table.SelectedRow(); row != nil {
				if sa, ok := v.findAccountByEmail(row.ID); ok {
					return func() tea.Msg { return ServiceAccountSelectedMsg{ServiceAccount: sa} }
				}
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.registerTask("load-service-accounts", "Refreshing...")
			if v.iamClient == nil {
				return tea.Batch(v.spinner.Tick, v.initIAMClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadAccounts())

		case key.Matches(msg, v.keys.Create):
			return func() tea.Msg {
				return ServiceAccountCreateRequestMsg{ProjectID: v.projectID}
			}

		case key.Matches(msg, v.keys.Toggle):
			if row := v.table.SelectedRow(); row != nil {
				if sa, ok := v.findAccountByEmail(row.ID); ok {
					return func() tea.Msg {
						return ToggleServiceAccountMsg{Email: sa.Email, Disable: !sa.Disabled}
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

// buildActions creates the action menu items based on account state
func (v *ServiceAccountsView) buildActions(sa gcp.ServiceAccount) []actionmenu.Action { //nolint:gocritic // Copying is acceptable
	actions := []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
		{Key: 'c', Label: "Create", Enabled: true},
	}

	if sa.Disabled {
		actions = append(actions, actionmenu.Action{Key: 't', Label: "Enable", Enabled: true})
	} else {
		actions = append(actions, actionmenu.Action{Key: 't', Label: "Disable", Enabled: true})
	}

	actions = append(actions, actionmenu.Action{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true})
	return actions
}

// executeAction performs the action selected from the menu
func (v *ServiceAccountsView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'r':
		v.loading = true
		v.err = nil
		v.registerTask("load-service-accounts", "Refreshing...")
		if v.iamClient == nil {
			return tea.Batch(v.spinner.Tick, v.initIAMClient())
		}
		return tea.Batch(v.spinner.Tick, v.loadAccounts())
	case 'c':
		return func() tea.Msg {
			return ServiceAccountCreateRequestMsg{ProjectID: v.projectID}
		}
	case 't':
		if row := v.table.SelectedRow(); row != nil {
			if sa, ok := v.findAccountByEmail(row.ID); ok {
				return func() tea.Msg {
					return ToggleServiceAccountMsg{Email: sa.Email, Disable: !sa.Disabled}
				}
			}
		}
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

// initiateDelete shows the delete confirmation dialog for the selected account
func (v *ServiceAccountsView) initiateDelete() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	sa, ok := v.findAccountByEmail(row.ID)
	if !ok {
		return nil
	}

	v.pendingDelete = &sa

	detailLines := []string{
		fmt.Sprintf("Display Name: %s", sa.DisplayName),
		fmt.Sprintf("Unique ID: %s", sa.UniqueID),
	}
	if sa.Disabled {
		detailLines = append(detailLines, "Status: Disabled")
	} else {
		detailLines = append(detailLines, "Status: Active")
	}
	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Service Account", sa.Email, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

// View renders the service accounts view
func (v *ServiceAccountsView) View() string {
	if v.loading && v.iamClient == nil {
		return renderLoading(v.spinner, "Initializing IAM client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading service accounts...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.accounts) == 0 {
		return "\n  No service accounts found in this project.\n  Press 'c' to create one, or 'esc' to go back."
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • .: actions • c: create • t: toggle • D: delete • S: sort • /: filter • r: refresh • esc: back")

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
func (v *ServiceAccountsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// HasTextInputFocused returns true if the delete confirm input or table filter is active
func (v *ServiceAccountsView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return v.table.HasTextInputFocused()
}

// IsMenuOpen returns true if the action menu or delete confirm is open
func (v *ServiceAccountsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetIAMClient returns the IAM client for reuse in detail views
func (v *ServiceAccountsView) GetIAMClient() *gcp.IAMClient {
	return v.iamClient
}

// SetContext updates the view with shared program context
func (v *ServiceAccountsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

// findAccountByEmail finds a service account by email in the loaded list
func (v *ServiceAccountsView) findAccountByEmail(email string) (gcp.ServiceAccount, bool) {
	for _, sa := range v.accounts {
		if sa.Email == email {
			return sa, true
		}
	}
	return gcp.ServiceAccount{}, false
}

// Task registration helpers for status bar integration
func (v *ServiceAccountsView) registerTask(id, description string) {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskRunning,
			StartTime:   time.Now(),
		}
	}
}

func (v *ServiceAccountsView) clearTask(id string) {
	if v.ctx != nil {
		delete(v.ctx.Tasks, id)
	}
}

func (v *ServiceAccountsView) failTask(id string, err error) tea.Cmd {
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
