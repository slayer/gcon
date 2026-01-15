package views

import (
	gocontext "context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// InstanceSelectedMsg is sent when an instance is selected from the list
type InstanceSelectedMsg struct {
	Instance gcp.Instance
}

// instanceDetailsLoadedMsg contains the fetched instance details
type instanceDetailsLoadedMsg struct {
	details *gcp.InstanceDetails
}

// instanceDetailsErrorMsg indicates an error loading details
type instanceDetailsErrorMsg struct {
	err error
}

// InstanceDetailsView displays comprehensive instance information
type InstanceDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	zone          string
	instanceName  string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	details       *gcp.InstanceDetails
	viewport      viewport.Model
	spinner       spinner.Model
	loading       bool
	actionLoading bool
	actionMsg     string
	err           error
	width         int
	height        int
	keys          instanceDetailsKeyMap
	ready         bool
}

type instanceDetailsKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Start   key.Binding
	Stop    key.Binding
	Reset   key.Binding
	Refresh key.Binding
}

func defaultInstanceDetailsKeyMap() instanceDetailsKeyMap {
	return instanceDetailsKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
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
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// NewInstanceDetailsView creates a new instance details view
func NewInstanceDetailsView(projectID, zone, instanceName string, computeClient *gcp.ComputeClient) *InstanceDetailsView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &InstanceDetailsView{
		computeClient: computeClient,
		projectID:     projectID,
		zone:          zone,
		instanceName:  instanceName,
		spinner:       s,
		loading:       true,
		keys:          defaultInstanceDetailsKeyMap(),
	}
}

// Init initializes the view and starts loading instance details
func (v *InstanceDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
	)
}

// loadDetails fetches instance details from GCP
func (v *InstanceDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		details, err := v.computeClient.GetInstanceDetails(gocontext.Background(), v.projectID, v.zone, v.instanceName)
		if err != nil {
			return instanceDetailsErrorMsg{err: err}
		}
		return instanceDetailsLoadedMsg{details: details}
	}
}

// Update handles messages for the instance details view
func (v *InstanceDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case instanceDetailsLoadedMsg:
		v.loading = false
		v.actionLoading = false
		v.actionMsg = ""
		v.details = msg.details
		v.updateViewportContent()
		return nil

	case instanceDetailsErrorMsg:
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
		// Refresh after action
		v.loading = true
		return tea.Batch(v.spinner.Tick, v.loadDetails())

	case spinner.TickMsg:
		if v.loading || v.actionLoading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		if v.actionLoading {
			return nil
		}

		switch {
		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadDetails())

		case key.Matches(msg, v.keys.Start):
			if v.details != nil && v.isInstanceStopped() {
				v.actionLoading = true
				v.actionMsg = fmt.Sprintf("Starting %s...", v.instanceName)
				return tea.Batch(v.spinner.Tick, v.startInstance())
			}

		case key.Matches(msg, v.keys.Stop):
			if v.details != nil && v.isInstanceRunning() {
				v.actionLoading = true
				v.actionMsg = fmt.Sprintf("Stopping %s...", v.instanceName)
				return tea.Batch(v.spinner.Tick, v.stopInstance())
			}

		case key.Matches(msg, v.keys.Reset):
			if v.details != nil && v.isInstanceRunning() {
				v.actionLoading = true
				v.actionMsg = fmt.Sprintf("Resetting %s...", v.instanceName)
				return tea.Batch(v.spinner.Tick, v.resetInstance())
			}
		}
	}

	// Handle viewport scrolling
	if v.ready {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return cmd
	}

	return nil
}

func (v *InstanceDetailsView) isInstanceRunning() bool {
	return v.details != nil && v.details.Status == "RUNNING"
}

func (v *InstanceDetailsView) isInstanceStopped() bool {
	return v.details != nil && (v.details.Status == "TERMINATED" || v.details.Status == "STOPPED")
}

func (v *InstanceDetailsView) startInstance() tea.Cmd {
	return func() tea.Msg {
		err := v.computeClient.StartInstance(gocontext.Background(), v.projectID, v.zone, v.instanceName)
		return instanceActionMsg{action: "Start", instance: v.instanceName, err: err}
	}
}

func (v *InstanceDetailsView) stopInstance() tea.Cmd {
	return func() tea.Msg {
		err := v.computeClient.StopInstance(gocontext.Background(), v.projectID, v.zone, v.instanceName)
		return instanceActionMsg{action: "Stop", instance: v.instanceName, err: err}
	}
}

func (v *InstanceDetailsView) resetInstance() tea.Cmd {
	return func() tea.Msg {
		err := v.computeClient.ResetInstance(gocontext.Background(), v.projectID, v.zone, v.instanceName)
		return instanceActionMsg{action: "Reset", instance: v.instanceName, err: err}
	}
}

// View renders the instance details view
func (v *InstanceDetailsView) View() string {
	if v.loading {
		return v.renderLoading("Loading instance details...")
	}

	if v.actionLoading {
		return v.renderLoading(v.actionMsg)
	}

	if v.err != nil {
		return v.renderLoading(fmt.Sprintf("Error: %v\n  Press 'esc' to go back", v.err))
	}

	if v.details == nil {
		return v.renderLoading("No instance details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return v.renderLoading("Initializing view...")
	}

	// Show action result if any
	var header string
	if v.actionMsg != "" {
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
		header = successStyle.Render("  "+v.actionMsg) + "\n\n"
	}

	// Help text for actions
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", v.viewport.ScrollPercent()*100))
	help := helpStyle.Render("\n  ↑/↓: scroll • s: start • x: stop • R: reset • r: refresh • esc: back") + " " + scrollInfo

	return header + v.viewport.View() + help
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *InstanceDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// applySize applies the given dimensions to the viewport
func (v *InstanceDetailsView) applySize(width, height int) {
	// Reserve space for header and footer
	viewportHeight := height - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	if !v.ready {
		v.viewport = viewport.New(width, viewportHeight)
		v.viewport.Style = lipgloss.NewStyle().Padding(0, 2)
		v.ready = true
	} else {
		v.viewport.Width = width
		v.viewport.Height = viewportHeight
	}

	if v.details != nil {
		v.updateViewportContent()
	}
}

// updateViewportContent renders the details content into the viewport
func (v *InstanceDetailsView) updateViewportContent() {
	if v.details == nil || !v.ready {
		return
	}

	content := v.renderContent()
	v.viewport.SetContent(content)
}

// renderContent generates the full details content
func (v *InstanceDetailsView) renderContent() string {
	d := v.details
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))

	// Header with status
	statusIcon := getStatusIcon(d.Status)
	b.WriteString(titleStyle.Render(fmt.Sprintf("Instance: %s  %s %s", d.Name, statusIcon, d.Status)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Basic Information
	b.WriteString(sectionStyle.Render("Basic Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Instance ID", strconv.FormatUint(d.ID, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Description", defaultIfEmpty(d.Description, "None")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Status", fmt.Sprintf("%s %s", getStatusIcon(d.Status), d.Status)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Zone", d.Zone))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Deletion protection", formatBool(d.DeletionProtection)))
	b.WriteString("\n")

	// Labels
	if len(d.Labels) > 0 {
		b.WriteString(labelStyle.Render("Labels"))
		b.WriteString("\n")
		for k, val := range d.Labels {
			b.WriteString(fmt.Sprintf("    %s: %s\n", k, val))
		}
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Labels", "None"))
	}

	// Tags
	if len(d.Tags) > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Tags", strings.Join(d.Tags, ", ")))
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Tags", "None"))
	}
	b.WriteString("\n")

	// Machine Configuration
	b.WriteString(sectionStyle.Render("Machine Configuration"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Machine Type", d.MachineType))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "CPU Platform", defaultIfEmpty(d.CpuPlatform, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Min CPU Platform", defaultIfEmpty(d.MinCpuPlatform, "None")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Display Device", formatBool(d.DisplayDevice)))

	// GPUs
	if len(d.GPUs) > 0 {
		gpuStrs := make([]string, len(d.GPUs))
		for i, gpu := range d.GPUs {
			gpuStrs[i] = fmt.Sprintf("%s x%d", gpu.Type, gpu.Count)
		}
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "GPUs", strings.Join(gpuStrs, ", ")))
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "GPUs", "None"))
	}
	b.WriteString("\n")

	// Networking
	b.WriteString(sectionStyle.Render("Networking"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "IP Forwarding", formatOnOff(d.CanIPForward)))
	b.WriteString("\n")

	// Network interfaces table
	if len(d.NetworkInterfaces) > 0 {
		b.WriteString("  Network Interfaces:\n")
		b.WriteString(fmt.Sprintf("  %-8s %-15s %-15s %-10s %-16s %-16s\n",
			"Name", "Network", "Subnetwork", "Type", "Internal IP", "External IP"))
		b.WriteString("  " + strings.Repeat("─", 84) + "\n")
		for _, nic := range d.NetworkInterfaces {
			extIP := defaultIfEmpty(nic.ExternalIP, "—")
			nicType := defaultIfEmpty(nic.NicType, "—")
			b.WriteString(fmt.Sprintf("  %-8s %-15s %-15s %-10s %-16s %-16s\n",
				nic.Name,
				truncate(nic.Network, 15),
				truncate(nic.Subnetwork, 15),
				nicType,
				nic.InternalIP,
				extIP))
		}
	}
	b.WriteString("\n")

	// Storage
	b.WriteString(sectionStyle.Render("Storage"))
	b.WriteString("\n")
	if len(d.Disks) > 0 {
		b.WriteString(fmt.Sprintf("  %-25s %-10s %-12s %-12s %-10s\n",
			"Name", "Size", "Type", "Mode", "Boot"))
		b.WriteString("  " + strings.Repeat("─", 72) + "\n")
		for _, disk := range d.Disks {
			bootStr := "—"
			if disk.Boot {
				bootStr = "Yes"
			}
			b.WriteString(fmt.Sprintf("  %-25s %-10s %-12s %-12s %-10s\n",
				truncate(disk.Name, 25),
				fmt.Sprintf("%d GB", disk.SizeGB),
				defaultIfEmpty(disk.Type, "—"),
				disk.Mode,
				bootStr))
		}
	} else {
		b.WriteString("  No disks attached\n")
	}
	b.WriteString("\n")

	// Security & Access
	b.WriteString(sectionStyle.Render("Security & Access"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Secure Boot", formatOnOff(d.ShieldedVM.SecureBoot)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "vTPM", formatOnOff(d.ShieldedVM.VTPM)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Integrity Monitoring", formatOnOff(d.ShieldedVM.IntegrityMonitoring)))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Service Account", defaultIfEmpty(d.ServiceAccount, "None")))

	// Scopes
	if len(d.Scopes) > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Access Scopes", fmt.Sprintf("%d scopes", len(d.Scopes))))
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Access Scopes", "None"))
	}
	b.WriteString("\n")

	// Availability Policies
	b.WriteString(sectionStyle.Render("Availability Policies"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Provisioning Model", defaultIfEmpty(d.Scheduling.ProvisioningModel, "Standard")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Preemptible", formatOnOff(d.Scheduling.Preemptible)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "On Host Maintenance", formatMaintenance(d.Scheduling.OnHostMaintenance)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Automatic Restart", formatOnOff(d.Scheduling.AutomaticRestart)))
	b.WriteString("\n")

	// Metadata
	if len(d.Metadata) > 0 {
		b.WriteString(sectionStyle.Render("Custom Metadata"))
		b.WriteString("\n")
		for k, val := range d.Metadata {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, truncate(val, 50)))
		}
	}

	return b.String()
}

// Helper functions
// Shared helpers (renderRow, defaultIfEmpty, min) are now in helpers.go

func getStatusIcon(status string) string {
	return symbols.GetStatusSymbol(status)
}

func formatBool(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}

func formatOnOff(b bool) string {
	if b {
		return "On"
	}
	return "Off"
}

func formatMaintenance(m string) string {
	switch m {
	case "MIGRATE":
		return "Migrate VM instance"
	case "TERMINATE":
		return "Terminate VM instance"
	default:
		return defaultIfEmpty(m, "—")
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// renderLoading renders a loading message
// Height enforcement is handled by the app's View() method using lipgloss.MaxHeight()
func (v *InstanceDetailsView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}
