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
	"github.com/slayer/gcon/internal/ui/components/sshdialog"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// InstancesView displays and manages Compute Engine instances in a table format
type InstancesView struct {
	TableClickDelegate
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	table         table.Model
	spinner       spinner.Model
	loading       bool
	actionLoading bool // True when performing start/stop action
	actionMsg     string
	err           error
	instances     []gcp.Instance
	keys          instanceKeyMap
	actionMenu    *actionmenu.ActionMenu
	menuOpen      bool

	// Stop confirmation state
	stopConfirm     *confirm.ConfirmDialog
	showStopConfirm bool
	pendingStop     *gcp.Instance // Instance pending stop

	// Delete confirmation state
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.Instance        // Instance pending deletion
	pendingDetails    *gcp.InstanceDetails // Details for deletion protection check

	// SSH dialog state
	sshDialog     *sshdialog.Dialog
	showSSHDialog bool
	sshErr        error

	// View dimensions for overlay rendering
	width  int
	height int
}

// instanceKeyMap defines instance-specific key bindings
type instanceKeyMap struct {
	Enter      key.Binding
	Create     key.Binding
	Start      key.Binding
	Stop       key.Binding
	Reset      key.Binding
	Suspend    key.Binding
	Resume     key.Binding
	Delete     key.Binding
	Refresh    key.Binding
	ActionMenu key.Binding
	SSH        key.Binding
}

func defaultInstanceKeyMap() instanceKeyMap {
	return instanceKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		Create: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "create"),
		),
		Start: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "start"),
		),
		Stop: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "stop"),
		),
		Reset: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "reset"),
		),
		Suspend: key.NewBinding(
			key.WithKeys("z"),
			key.WithHelp("z", "suspend"),
		),
		Resume: key.NewBinding(
			key.WithKeys("Z"),
			key.WithHelp("Z", "resume"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
		SSH: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "ssh"),
		),
	}
}

// Table column definitions
func instanceColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 28, Grow: true, Sortable: true},
		{Title: "Zone", Width: 20, Sortable: true},
		{Title: "Internal IP", Width: 15},
		{Title: "External IP", Width: 15},
		{Title: "Machine Type", Width: 15, Sortable: true},
	}
}

// NewInstancesView creates a new instances view with table display
func NewInstancesView(projectID string) *InstancesView {
	title := fmt.Sprintf("Compute Engine Instances - %s", projectID)
	t := table.NewWithColumns(instanceColumns(), title)

	s := components.NewGCPSpinner()

	v := &InstancesView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultInstanceKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init initializes the view and starts loading instances
func (v *InstancesView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initComputeClient(),
	)
}

// initComputeClient creates the compute client then loads instances
func (v *InstancesView) initComputeClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(gocontext.Background())
		if err != nil {
			return instancesErrorMsg{err: err}
		}
		return computeClientReadyMsg{client: client}
	}
}

// loadInstances fetches instances from GCP
func (v *InstancesView) loadInstances() tea.Cmd {
	return func() tea.Msg {
		instances, err := v.computeClient.ListInstances(gocontext.Background(), v.projectID)
		if err != nil {
			return instancesErrorMsg{err: err}
		}
		return instancesLoadedMsg{instances: instances}
	}
}

// Message types
type computeClientReadyMsg struct {
	client *gcp.ComputeClient
}

type instancesLoadedMsg struct {
	instances []gcp.Instance
}

type instancesErrorMsg struct {
	err error
}

type instanceActionMsg struct {
	action   string
	instance string
	err      error
}

// instanceDeleteDetailsMsg contains instance details fetched for delete confirmation
type instanceDeleteDetailsMsg struct {
	instance *gcp.Instance
	details  *gcp.InstanceDetails
	err      error
}

// statusIcon returns a symbol indicator for instance status
func statusIcon(status string) string {
	return symbols.GetStatusSymbol(status)
}

// instanceToRow converts a GCP instance to a table row
func instanceToRow(inst gcp.Instance) table.Row { //nolint:gocritic // Copying instance is acceptable
	externalIP := inst.ExternalIP
	if externalIP == "" {
		externalIP = "-"
	}

	// Combine status icon with name (like objects view)
	name := statusIcon(inst.Status) + " " + inst.Name

	return table.Row{
		Data: []string{
			name,
			inst.Zone,
			inst.InternalIP,
			externalIP,
			inst.MachineType,
		},
		FilterValue: inst.Name + " " + inst.Zone + " " + inst.Status + " " + inst.MachineType,
		ID:          inst.Name,
	}
}

// Update handles messages for the instances view
//
//nolint:gocognit // Bubble Tea Update pattern - complexity 57
func (v *InstancesView) Update(msg tea.Msg) tea.Cmd {
	// Route all messages through the SSH dialog while it is open, except for
	// the dialog's own terminal messages (ConnectMsg / CancelMsg).
	if v.showSSHDialog && v.sshDialog != nil {
		if _, isConnect := msg.(sshdialog.ConnectMsg); !isConnect {
			if _, isCancel := msg.(sshdialog.CancelMsg); !isCancel {
				var cmd tea.Cmd
				v.sshDialog, cmd = v.sshDialog.Update(msg)
				return cmd
			}
		}
	}

	switch msg := msg.(type) {
	case sshdialog.ConnectMsg:
		v.showSSHDialog = false
		opts := msg.Options
		return func() tea.Msg {
			return SSHRequestMsg{Options: opts, OriginView: v}
		}

	case sshdialog.CancelMsg:
		v.showSSHDialog = false
		return nil

	case computeClientReadyMsg:
		v.computeClient = msg.client
		// Register loading task in status bar
		v.registerTask("load-instances", "Loading instances...")
		return v.loadInstances()

	case instancesLoadedMsg:
		v.loading = false
		v.actionLoading = false
		v.actionMsg = ""
		v.instances = msg.instances
		// Clear the loading task
		v.clearTask("load-instances")

		// Convert instances to table rows
		rows := make([]table.Row, len(msg.instances))
		for i, inst := range msg.instances {
			rows[i] = instanceToRow(inst)
		}
		v.table.SetRows(rows)
		// Default sort by name ascending
		v.table.SortBy(0, true)
		return nil

	case instancesErrorMsg:
		v.loading = false
		v.actionLoading = false
		v.err = msg.err
		// Mark task as error and schedule cleanup
		return v.failTask("load-instances", msg.err)

	case instanceActionMsg:
		v.actionLoading = false
		if msg.err != nil {
			v.err = msg.err
			// Mark task as error and schedule cleanup
			return v.failTask("action-"+msg.instance, msg.err)
		}
		v.actionMsg = fmt.Sprintf("%s %s: success", msg.action, msg.instance)
		// Mark action as successful and schedule cleanup
		clearCmd := v.finishTask("action-"+msg.instance, msg.action+" "+msg.instance)
		// Refresh the list after action
		v.loading = true
		v.registerTask("load-instances", "Refreshing...")
		return tea.Batch(clearCmd, v.spinner.Tick, v.loadInstances())

	case actionmenu.ActionSelectedMsg:
		// Handle action menu selection
		v.menuOpen = false
		return v.executeAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case instanceDeleteDetailsMsg:
		// Details fetched for delete confirmation
		v.actionLoading = false
		v.clearTask("fetch-delete-details")
		if msg.err != nil {
			v.err = msg.err
			return nil
		}
		v.pendingDelete = msg.instance
		v.pendingDetails = msg.details
		return v.showDeleteConfirmation()

	case confirm.ConfirmMsg:
		// Stop confirmation accepted
		v.showStopConfirm = false
		if v.pendingStop != nil {
			inst := v.pendingStop
			v.pendingStop = nil
			v.actionLoading = true
			v.actionMsg = fmt.Sprintf("Stopping %s...", inst.Name)
			v.registerTask("action-"+inst.Name, "Stopping "+inst.Name+"...")
			return tea.Batch(v.spinner.Tick, v.stopInstance(*inst))
		}
		return nil

	case confirm.CancelMsg:
		// Stop confirmation canceled
		v.showStopConfirm = false
		v.pendingStop = nil
		return nil

	case confirm.TypeConfirmMsg:
		v.showDeleteConfirm = false
		if v.pendingDelete != nil && v.pendingDetails != nil {
			inst := v.pendingDelete
			v.pendingDelete = nil
			v.pendingDetails = nil
			return func() tea.Msg {
				return DeleteInstanceConfirmedMsg{
					InstanceName: inst.Name,
					Zone:         inst.Zone,
				}
			}
		}
		return nil

	case confirm.TypeCancelMsg:
		v.showDeleteConfirm = false
		v.pendingDelete = nil
		v.pendingDetails = nil
		return nil

	case table.RowDoubleClickedMsg:
		// Handle double-click on table row - navigate to details
		inst := v.findInstanceByName(msg.RowID)
		if inst != nil {
			return func() tea.Msg {
				return InstanceSelectedMsg{Instance: *inst}
			}
		}
		return nil

	case spinner.TickMsg:
		if v.loading || v.actionLoading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		// Route to stop confirmation dialog when shown
		if v.showStopConfirm && v.stopConfirm != nil {
			return v.stopConfirm.Update(msg)
		}

		// Route to delete confirmation dialog when shown
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		// Don't handle custom keys during filtering, action, or loading
		if v.actionLoading || v.loading {
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
			// Toggle action menu
			if row := v.table.SelectedRow(); row != nil {
				inst := v.findInstanceByName(row.ID)
				if inst != nil {
					v.actionMenu = actionmenu.New("Instance Actions", v.buildActions(*inst))
					v.menuOpen = true
				}
			}
			return nil

		case key.Matches(msg, v.keys.Create):
			return func() tea.Msg {
				return InstanceCreateRequestMsg{ProjectID: v.projectID}
			}

		case key.Matches(msg, v.keys.Enter):
			// Navigate to instance details on Enter
			if row := v.table.SelectedRow(); row != nil {
				inst := v.findInstanceByName(row.ID)
				if inst != nil {
					return func() tea.Msg {
						return InstanceSelectedMsg{Instance: *inst}
					}
				}
			}

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.registerTask("load-instances", "Refreshing...")
			// Re-initialize client if previous attempt failed
			if v.computeClient == nil {
				return tea.Batch(v.spinner.Tick, v.initComputeClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadInstances())

		case key.Matches(msg, v.keys.Start):
			if row := v.table.SelectedRow(); row != nil {
				inst := v.findInstanceByName(row.ID)
				if inst != nil && inst.IsStopped() {
					v.actionLoading = true
					v.actionMsg = fmt.Sprintf("Starting %s...", inst.Name)
					v.registerTask("action-"+inst.Name, "Starting "+inst.Name+"...")
					return tea.Batch(v.spinner.Tick, v.startInstance(*inst))
				}
			}

		case key.Matches(msg, v.keys.Stop):
			if row := v.table.SelectedRow(); row != nil {
				inst := v.findInstanceByName(row.ID)
				if inst != nil && inst.IsRunning() {
					return v.showStopConfirmation(inst)
				}
			}

		case key.Matches(msg, v.keys.Reset):
			if row := v.table.SelectedRow(); row != nil {
				inst := v.findInstanceByName(row.ID)
				if inst != nil && inst.IsRunning() {
					v.actionLoading = true
					v.actionMsg = fmt.Sprintf("Resetting %s...", inst.Name)
					return tea.Batch(v.spinner.Tick, v.resetInstance(*inst))
				}
			}

		case key.Matches(msg, v.keys.Suspend):
			if row := v.table.SelectedRow(); row != nil {
				inst := v.findInstanceByName(row.ID)
				if inst != nil && inst.IsRunning() {
					v.actionLoading = true
					v.actionMsg = fmt.Sprintf("Suspending %s...", inst.Name)
					v.registerTask("action-"+inst.Name, "Suspending "+inst.Name+"...")
					return tea.Batch(v.spinner.Tick, v.suspendInstance(*inst))
				}
			}

		case key.Matches(msg, v.keys.Resume):
			if row := v.table.SelectedRow(); row != nil {
				inst := v.findInstanceByName(row.ID)
				if inst != nil && inst.IsSuspended() {
					v.actionLoading = true
					v.actionMsg = fmt.Sprintf("Resuming %s...", inst.Name)
					v.registerTask("action-"+inst.Name, "Resuming "+inst.Name+"...")
					return tea.Batch(v.spinner.Tick, v.resumeInstance(*inst))
				}
			}

		case key.Matches(msg, v.keys.Delete):
			// Delete requires fetching details first to check deletion protection
			if row := v.table.SelectedRow(); row != nil {
				inst := v.findInstanceByName(row.ID)
				if inst != nil {
					v.actionLoading = true
					v.actionMsg = fmt.Sprintf("Checking %s...", inst.Name)
					v.registerTask("fetch-delete-details", "Checking deletion protection...")
					return tea.Batch(v.spinner.Tick, v.fetchDeleteDetails(inst))
				}
			}
			return nil

		case key.Matches(msg, v.keys.SSH):
			if inst := v.cursorInstance(); inst != nil && inst.IsRunning() {
				return v.openSSHDialog(inst)
			}
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// buildActions creates the action menu items based on instance state
func (v *InstancesView) buildActions(inst gcp.Instance) []actionmenu.Action { //nolint:gocritic // Copying instance is acceptable
	isRunning := inst.IsRunning()
	isStopped := inst.IsStopped()
	isSuspended := inst.IsSuspended()

	return []actionmenu.Action{
		{Key: 'c', Label: "Create Instance", Enabled: true},
		{Key: 't', Label: "SSH", Enabled: isRunning},
		{Key: 's', Label: "Start", Enabled: isStopped},
		{Key: 'x', Label: "Stop", Enabled: isRunning},
		{Key: 'z', Label: "Suspend", Enabled: isRunning},
		{Key: 'Z', Label: "Resume", Enabled: isSuspended},
		{Key: 'R', Label: "Reset", Enabled: isRunning, Dangerous: true},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
		{Key: 'r', Label: "Refresh", Enabled: true},
	}
}

// executeAction performs the action selected from the menu
func (v *InstancesView) executeAction(actionKey rune) tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	inst := v.findInstanceByName(row.ID)
	if inst == nil {
		return nil
	}

	switch actionKey {
	case 'c':
		return func() tea.Msg {
			return InstanceCreateRequestMsg{ProjectID: v.projectID}
		}
	case 't':
		if inst.IsRunning() {
			return v.openSSHDialog(inst)
		}
	case 's':
		if inst.IsStopped() {
			v.actionLoading = true
			v.actionMsg = fmt.Sprintf("Starting %s...", inst.Name)
			return tea.Batch(v.spinner.Tick, v.startInstance(*inst))
		}
	case 'x':
		if inst.IsRunning() {
			return v.showStopConfirmation(inst)
		}
	case 'z':
		if inst.IsRunning() {
			v.actionLoading = true
			v.actionMsg = fmt.Sprintf("Suspending %s...", inst.Name)
			v.registerTask("action-"+inst.Name, "Suspending "+inst.Name+"...")
			return tea.Batch(v.spinner.Tick, v.suspendInstance(*inst))
		}
	case 'Z':
		if inst.IsSuspended() {
			v.actionLoading = true
			v.actionMsg = fmt.Sprintf("Resuming %s...", inst.Name)
			v.registerTask("action-"+inst.Name, "Resuming "+inst.Name+"...")
			return tea.Batch(v.spinner.Tick, v.resumeInstance(*inst))
		}
	case 'R':
		if inst.IsRunning() {
			v.actionLoading = true
			v.actionMsg = fmt.Sprintf("Resetting %s...", inst.Name)
			return tea.Batch(v.spinner.Tick, v.resetInstance(*inst))
		}
	case 'D':
		// Delete requires fetching details first to check deletion protection
		v.actionLoading = true
		v.actionMsg = fmt.Sprintf("Checking %s...", inst.Name)
		v.registerTask("fetch-delete-details", "Checking deletion protection...")
		return tea.Batch(v.spinner.Tick, v.fetchDeleteDetails(inst))
	case 'r':
		v.loading = true
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.loadInstances())
	}

	return nil
}

// findInstanceByName looks up an instance by name
func (v *InstancesView) findInstanceByName(name string) *gcp.Instance {
	for _, inst := range v.instances {
		if inst.Name == name {
			return &inst
		}
	}
	return nil
}

// cursorInstance returns the instance under the current table cursor, or nil
// if no row is selected or the row cannot be matched to an instance.
func (v *InstancesView) cursorInstance() *gcp.Instance {
	if row := v.table.SelectedRow(); row != nil {
		return v.findInstanceByName(row.ID)
	}
	return nil
}

// openSSHDialog opens the SSH options modal for the given running instance.
// Caller must have verified inst is non-nil and running.
func (v *InstancesView) openSSHDialog(inst *gcp.Instance) tea.Cmd {
	v.sshErr = nil
	v.sshDialog = sshdialog.New(sshdialog.Params{
		Project:    v.projectID,
		Zone:       inst.Zone,
		Instance:   inst.Name,
		InternalIP: inst.InternalIP,
		ExternalIP: inst.ExternalIP,
		OriginView: v,
	})
	v.sshDialog.SetSize(v.width, v.height)
	v.showSSHDialog = true
	return v.sshDialog.Init()
}

// SetSSHError stores an error from a finished SSH session for inline display.
func (v *InstancesView) SetSSHError(err error) {
	v.sshErr = err
}

func (v *InstancesView) startInstance(inst gcp.Instance) tea.Cmd { //nolint:gocritic // Copying instance is acceptable
	return func() tea.Msg {
		err := v.computeClient.StartInstance(gocontext.Background(), v.projectID, inst.Zone, inst.Name)
		return instanceActionMsg{action: "Start", instance: inst.Name, err: err}
	}
}

//nolint:gocritic // hugeParam: Instance struct passed by value for clarity
func (v *InstancesView) stopInstance(inst gcp.Instance) tea.Cmd {
	return func() tea.Msg {
		err := v.computeClient.StopInstance(gocontext.Background(), v.projectID, inst.Zone, inst.Name)
		return instanceActionMsg{action: "Stop", instance: inst.Name, err: err}
	}
}

//nolint:gocritic // hugeParam: Instance struct passed by value for clarity
func (v *InstancesView) resetInstance(inst gcp.Instance) tea.Cmd {
	return func() tea.Msg {
		err := v.computeClient.ResetInstance(gocontext.Background(), v.projectID, inst.Zone, inst.Name)
		return instanceActionMsg{action: "Reset", instance: inst.Name, err: err}
	}
}

//nolint:gocritic // hugeParam: Instance struct passed by value for clarity
func (v *InstancesView) suspendInstance(inst gcp.Instance) tea.Cmd {
	return func() tea.Msg {
		err := v.computeClient.SuspendInstance(gocontext.Background(), v.projectID, inst.Zone, inst.Name)
		return instanceActionMsg{action: "Suspend", instance: inst.Name, err: err}
	}
}

//nolint:gocritic // hugeParam: Instance struct passed by value for clarity
func (v *InstancesView) resumeInstance(inst gcp.Instance) tea.Cmd {
	return func() tea.Msg {
		err := v.computeClient.ResumeInstance(gocontext.Background(), v.projectID, inst.Zone, inst.Name)
		return instanceActionMsg{action: "Resume", instance: inst.Name, err: err}
	}
}

// fetchDeleteDetails fetches instance details to check deletion protection before showing dialog
func (v *InstancesView) fetchDeleteDetails(inst *gcp.Instance) tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return instanceDeleteDetailsMsg{err: uierrors.ErrClientNotInitialized}
		}
		details, err := v.computeClient.GetInstanceDetails(gocontext.Background(), v.projectID, inst.Zone, inst.Name)
		if err != nil {
			return instanceDeleteDetailsMsg{err: err}
		}
		return instanceDeleteDetailsMsg{instance: inst, details: details}
	}
}

// showStopConfirmation creates and shows the stop confirmation dialog
func (v *InstancesView) showStopConfirmation(inst *gcp.Instance) tea.Cmd {
	v.pendingStop = inst
	v.stopConfirm = confirm.New(
		"Stop Instance",
		fmt.Sprintf("Are you sure you want to stop instance %q?", inst.Name),
		[]string{
			fmt.Sprintf("Zone: %s", inst.Zone),
			fmt.Sprintf("Machine type: %s", inst.MachineType),
		},
	)
	v.showStopConfirm = true
	return v.stopConfirm.Init()
}

// showDeleteConfirmation creates and shows the delete confirmation dialog
func (v *InstancesView) showDeleteConfirmation() tea.Cmd {
	if v.pendingDelete == nil || v.pendingDetails == nil {
		return nil
	}

	inst := v.pendingDelete
	details := v.pendingDetails

	// Build detail lines for the dialog
	detailLines := []string{
		fmt.Sprintf("Zone: %s", inst.Zone),
		fmt.Sprintf("Machine type: %s", inst.MachineType),
		fmt.Sprintf("Status: %s", inst.Status),
	}

	v.deleteConfirm = confirm.NewTypeConfirmDialog(
		"Delete Instance",
		inst.Name,
		detailLines,
	)

	// Check deletion protection
	if details.DeletionProtection {
		v.deleteConfirm.SetWarning("Deletion protection is enabled. Disable it in the GCP Console before deleting.")
		v.deleteConfirm.SetCannotConfirm(true)
	}

	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

// View renders the instances view
func (v *InstancesView) View() string {
	if v.loading && v.computeClient == nil {
		return renderLoading(v.spinner, "Initializing Compute Engine client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading instances...")
	}

	if v.actionLoading {
		return fmt.Sprintf("\n  %s %s\n\n%s", v.spinner.View(), v.actionMsg, v.table.View())
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.instances) == 0 {
		return "\n  No instances found in this project.\n  Press 'esc' to go back."
	}

	// Show action result if any
	var header string
	if v.actionMsg != "" {
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
		header = successStyle.Render("  ✓ "+v.actionMsg) + "\n\n"
	}

	// Help text for actions - include '.' for action menu
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • c: create • .: actions • s: start • x: stop • z: suspend • Z: resume • S: sort • /: filter • r: refresh")

	mainContent := header + v.table.View() + help

	if v.sshErr != nil {
		mainContent += "\n" + components.RenderInlineError(v.sshErr)
	}

	// Overlay SSH dialog — highest priority
	if v.showSSHDialog && v.sshDialog != nil {
		return v.renderWithOverlay(mainContent, v.sshDialog.View())
	}

	// Overlay action menu if open
	if v.menuOpen && v.actionMenu != nil {
		return v.renderWithOverlay(mainContent, v.actionMenu.View())
	}

	// Overlay stop confirmation if shown
	if v.showStopConfirm && v.stopConfirm != nil {
		return v.renderWithOverlay(mainContent, v.stopConfirm.View())
	}

	// Overlay delete confirmation if shown
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.renderWithOverlay(mainContent, v.deleteConfirm.View())
	}

	return mainContent
}

// renderWithOverlay overlays a dialog centered on top of the content
func (v *InstancesView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *InstancesView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

// SelectedInstance returns the currently selected instance
func (v *InstancesView) SelectedInstance() *gcp.Instance {
	if row := v.table.SelectedRow(); row != nil {
		return v.findInstanceByName(row.ID)
	}
	return nil
}

// GetComputeClient returns the compute client for reuse in detail views
func (v *InstancesView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// IsMenuOpen returns true if the action menu, a confirmation dialog, or the SSH dialog is open.
func (v *InstancesView) IsMenuOpen() bool {
	return v.menuOpen || v.showStopConfirm || v.showDeleteConfirm || v.showSSHDialog
}

// HasTextInputFocused returns true if the table filter, delete confirm input, or SSH dialog is active.
// Used to prevent global hotkeys (like 'q' for quit) from triggering while typing.
func (v *InstancesView) HasTextInputFocused() bool {
	if v.showSSHDialog && v.sshDialog != nil {
		return true
	}
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return v.table.HasTextInputFocused()
}

// Task registration helpers for status bar integration
func (v *InstancesView) registerTask(id, description string) {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskRunning,
			StartTime:   time.Now(),
		}
	}
}

func (v *InstancesView) clearTask(id string) {
	if v.ctx != nil {
		delete(v.ctx.Tasks, id)
	}
}

// finishTask marks a task as finished and returns a command to clear it after a delay
func (v *InstancesView) finishTask(id, description string) tea.Cmd {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskFinished,
		}
	}
	// Schedule task removal after 2 seconds to prevent memory leaks
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return context.TaskClearMsg{TaskID: id}
	})
}

// failTask marks a task as failed and returns a command to clear it after a delay
func (v *InstancesView) failTask(id string, err error) tea.Cmd {
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

// InstancesLoadedMsgForTest creates an instancesLoadedMsg for cross-package testing.
func InstancesLoadedMsgForTest(instances []gcp.Instance) instancesLoadedMsg {
	return instancesLoadedMsg{instances: instances}
}
