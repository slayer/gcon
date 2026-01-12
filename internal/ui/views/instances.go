package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
)

// instanceItem implements list.Item for VM instances
type instanceItem struct {
	instance gcp.Instance
}

func (i instanceItem) Title() string {
	// Show status indicator with color
	var statusIcon string
	switch i.instance.Status {
	case "RUNNING":
		statusIcon = "🟢"
	case "TERMINATED", "STOPPED":
		statusIcon = "🔴"
	case "STAGING", "PROVISIONING", "STOPPING", "SUSPENDING":
		statusIcon = "🟡"
	default:
		statusIcon = "⚪"
	}
	return fmt.Sprintf("%s %s", statusIcon, i.instance.Name)
}

func (i instanceItem) Description() string {
	ip := i.instance.InternalIP
	if i.instance.ExternalIP != "" {
		ip = fmt.Sprintf("%s (ext: %s)", i.instance.InternalIP, i.instance.ExternalIP)
	}
	return fmt.Sprintf("%s • %s • %s", i.instance.Zone, i.instance.MachineType, ip)
}

func (i instanceItem) FilterValue() string {
	return i.instance.Name + " " + i.instance.Zone + " " + i.instance.Status
}

// InstancesView displays and manages Compute Engine instances
type InstancesView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	list          list.Model
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

// NewInstancesView creates a new instances view
func NewInstancesView(projectID string) *InstancesView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#4285F4")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#CCCCCC")).
		Background(lipgloss.Color("#4285F4"))

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false) // Title shown in app header instead
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	// Add additional help keys
	l.AdditionalShortHelpKeys = func() []key.Binding {
		km := defaultInstanceKeyMap()
		return []key.Binding{km.Start, km.Stop, km.Refresh}
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &InstancesView{
		projectID: projectID,
		list:      l,
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

		items := make([]list.Item, len(msg.instances))
		for i, inst := range msg.instances {
			items[i] = instanceItem{instance: inst}
		}
		v.list.SetItems(items)
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
		// Don't handle keys during action or loading
		if v.actionLoading || v.loading {
			return nil
		}

		switch {
		case key.Matches(msg, v.keys.Enter):
			// Navigate to instance details on Enter
			if item, ok := v.list.SelectedItem().(instanceItem); ok {
				return func() tea.Msg {
					return InstanceSelectedMsg{Instance: item.instance}
				}
			}

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadInstances())

		case key.Matches(msg, v.keys.Start):
			if item, ok := v.list.SelectedItem().(instanceItem); ok {
				if item.instance.IsStopped() {
					v.actionLoading = true
					v.actionMsg = fmt.Sprintf("Starting %s...", item.instance.Name)
					return tea.Batch(v.spinner.Tick, v.startInstance(item.instance))
				}
			}

		case key.Matches(msg, v.keys.Stop):
			if item, ok := v.list.SelectedItem().(instanceItem); ok {
				if item.instance.IsRunning() {
					v.actionLoading = true
					v.actionMsg = fmt.Sprintf("Stopping %s...", item.instance.Name)
					return tea.Batch(v.spinner.Tick, v.stopInstance(item.instance))
				}
			}

		case key.Matches(msg, v.keys.Reset):
			if item, ok := v.list.SelectedItem().(instanceItem); ok {
				if item.instance.IsRunning() {
					v.actionLoading = true
					v.actionMsg = fmt.Sprintf("Resetting %s...", item.instance.Name)
					return tea.Batch(v.spinner.Tick, v.resetInstance(item.instance))
				}
			}
		}
	}

	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return cmd
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
		return v.renderLoading("Initializing Compute Engine client...")
	}

	if v.loading {
		return v.renderLoading("Loading instances...")
	}

	if v.actionLoading {
		return fmt.Sprintf("\n  %s %s\n\n%s", v.spinner.View(), v.actionMsg, v.list.View())
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
	help := helpStyle.Render("\n  enter: details • s: start • x: stop • R: reset • r: refresh • esc: back")

	return header + v.list.View() + help
}

// SetSize updates the view dimensions
func (v *InstancesView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.list.SetSize(width, height-4)
}

// SelectedInstance returns the currently selected instance
func (v *InstancesView) SelectedInstance() *gcp.Instance {
	if item, ok := v.list.SelectedItem().(instanceItem); ok {
		return &item.instance
	}
	return nil
}

// GetComputeClient returns the compute client for reuse in detail views
func (v *InstancesView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// renderLoading renders a loading message that fills the view height
// to prevent rendering artifacts when transitioning to loaded state
func (v *InstancesView) renderLoading(msg string) string {
	content := fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
	// Pad with empty lines to fill the view height
	lines := strings.Count(content, "\n")
	for i := lines; i < v.height; i++ {
		content += "\n"
	}
	return content
}
