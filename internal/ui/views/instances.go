package views

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	btable "github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
)

// InstancesView displays and manages Compute Engine instances in a table format
type InstancesView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	table         table.Model
	spinner       spinner.Model
	loading       bool
	actionLoading bool // True when performing start/stop action
	actionMsg     string
	err           error
	width         int
	height        int
	instances     []gcp.Instance
	keys          instanceKeyMap
}

// instanceKeyMap defines instance-specific key bindings
type instanceKeyMap struct {
	Enter   key.Binding
	Start   key.Binding
	Stop    key.Binding
	Reset   key.Binding
	SSH     key.Binding
	Refresh key.Binding
}

func defaultInstanceKeyMap() instanceKeyMap {
	return instanceKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
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
		SSH: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "SSH"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// Table column definitions
func instanceColumns() []btable.Column {
	return []btable.Column{
		{Title: "Name", Width: 25},
		{Title: "Status", Width: 12},
		{Title: "Zone", Width: 20},
		{Title: "Internal IP", Width: 15},
		{Title: "External IP", Width: 15},
		{Title: "Machine Type", Width: 15},
	}
}

// NewInstancesView creates a new instances view with table display
func NewInstancesView(projectID string) *InstancesView {
	title := fmt.Sprintf("Compute Engine Instances - %s", projectID)
	t := table.New(instanceColumns(), title)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &InstancesView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultInstanceKeyMap(),
	}
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
		client, err := gcp.NewComputeClient(context.Background())
		if err != nil {
			return instancesErrorMsg{err: err}
		}
		return computeClientReadyMsg{client: client}
	}
}

// loadInstances fetches instances from GCP
func (v *InstancesView) loadInstances() tea.Cmd {
	return func() tea.Msg {
		instances, err := v.computeClient.ListInstances(context.Background(), v.projectID)
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

// statusIcon returns an emoji indicator for instance status
func statusIcon(status string) string {
	switch status {
	case "RUNNING":
		return "🟢"
	case "TERMINATED", "STOPPED":
		return "🔴"
	case "STAGING", "PROVISIONING", "STOPPING", "SUSPENDING":
		return "🟡"
	default:
		return "⚪"
	}
}

// instanceToRow converts a GCP instance to a table row
func instanceToRow(inst gcp.Instance) table.Row {
	externalIP := inst.ExternalIP
	if externalIP == "" {
		externalIP = "-"
	}

	return table.Row{
		Data: []string{
			inst.Name,
			statusIcon(inst.Status) + " " + inst.Status,
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
func (v *InstancesView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case computeClientReadyMsg:
		v.computeClient = msg.client
		return v.loadInstances()

	case instancesLoadedMsg:
		v.loading = false
		v.actionLoading = false
		v.actionMsg = ""
		v.instances = msg.instances

		// Convert instances to table rows
		rows := make([]table.Row, len(msg.instances))
		for i, inst := range msg.instances {
			rows[i] = instanceToRow(inst)
		}
		v.table.SetRows(rows)
		return nil

	case instancesErrorMsg:
		v.loading = false
		v.actionLoading = false
		v.err = msg.err
		return nil

	case instanceActionMsg:
		v.actionLoading = false
		if msg.err != nil {
			v.err = msg.err
			return nil
		}
		v.actionMsg = fmt.Sprintf("%s %s: success", msg.action, msg.instance)
		// Refresh the list after action
		v.loading = true
		return tea.Batch(v.spinner.Tick, v.loadInstances())

	case spinner.TickMsg:
		if v.loading || v.actionLoading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		// Don't handle custom keys during filtering, action, or loading
		if v.actionLoading || v.loading {
			return nil
		}

		// Let table handle filtering mode
		if v.table.IsFiltering() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
		}

		switch {
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
			return tea.Batch(v.spinner.Tick, v.loadInstances())

		case key.Matches(msg, v.keys.Start):
			if row := v.table.SelectedRow(); row != nil {
				inst := v.findInstanceByName(row.ID)
				if inst != nil && inst.IsStopped() {
					v.actionLoading = true
					v.actionMsg = fmt.Sprintf("Starting %s...", inst.Name)
					return tea.Batch(v.spinner.Tick, v.startInstance(*inst))
				}
			}

		case key.Matches(msg, v.keys.Stop):
			if row := v.table.SelectedRow(); row != nil {
				inst := v.findInstanceByName(row.ID)
				if inst != nil && inst.IsRunning() {
					v.actionLoading = true
					v.actionMsg = fmt.Sprintf("Stopping %s...", inst.Name)
					return tea.Batch(v.spinner.Tick, v.stopInstance(*inst))
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
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
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

func (v *InstancesView) startInstance(inst gcp.Instance) tea.Cmd {
	return func() tea.Msg {
		err := v.computeClient.StartInstance(context.Background(), v.projectID, inst.Zone, inst.Name)
		return instanceActionMsg{action: "Start", instance: inst.Name, err: err}
	}
}

func (v *InstancesView) stopInstance(inst gcp.Instance) tea.Cmd {
	return func() tea.Msg {
		err := v.computeClient.StopInstance(context.Background(), v.projectID, inst.Zone, inst.Name)
		return instanceActionMsg{action: "Stop", instance: inst.Name, err: err}
	}
}

func (v *InstancesView) resetInstance(inst gcp.Instance) tea.Cmd {
	return func() tea.Msg {
		err := v.computeClient.ResetInstance(context.Background(), v.projectID, inst.Zone, inst.Name)
		return instanceActionMsg{action: "Reset", instance: inst.Name, err: err}
	}
}

// View renders the instances view
func (v *InstancesView) View() string {
	if v.loading && v.computeClient == nil {
		return fmt.Sprintf("\n  %s Initializing Compute Engine client...\n", v.spinner.View())
	}

	if v.loading {
		return fmt.Sprintf("\n  %s Loading instances...\n", v.spinner.View())
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

	// Help text for actions
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • s: start • x: stop • R: reset • /: filter • r: refresh • esc: back")

	return header + v.table.View() + help
}

// SetSize updates the view dimensions
func (v *InstancesView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.table.SetSize(width, height-6) // Reserve space for header and help
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
