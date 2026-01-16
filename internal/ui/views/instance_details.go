package views

import (
	gocontext "context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/links"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// InstanceSelectedMsg is sent when an instance is selected from the list
type InstanceSelectedMsg struct {
	Instance gcp.Instance
}

// InstanceDiskSelectedMsg is sent when a disk link is selected in instance details
// Contains disk info extracted from DiskInfo.Source URL
type InstanceDiskSelectedMsg struct {
	DiskName string
	Zone     string
}

// instanceDetailsLoadedMsg contains the fetched instance details
type instanceDetailsLoadedMsg struct {
	details *gcp.InstanceDetails
}

// instanceDetailsErrorMsg indicates an error loading details
type instanceDetailsErrorMsg struct {
	err error
}

// Tab IDs for instance details view
const (
	tabIDDetails       = "details"
	tabIDObservability = "observability"
)

// InstanceDetailsView displays comprehensive instance information
type InstanceDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	zone          string
	instanceName  string
	ctx           *context.ProgramContext // Shared context for dimensions and styles
	details       *gcp.InstanceDetails
	spinner       spinner.Model
	loading       bool
	actionLoading bool
	actionMsg     string
	err           error
	width         int
	height        int
	keys          instanceDetailsKeyMap
	ready         bool
	actionMenu    *actionmenu.ActionMenu
	menuOpen      bool
	// Tab navigation
	tabs         *tabs.Tabs
	tabViewports []viewport.Model // Separate viewport per tab to preserve scroll
	// Navigable links (e.g., disks)
	diskLinks *links.Links
}

type instanceDetailsKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Start      key.Binding
	Stop       key.Binding
	Reset      key.Binding
	Refresh    key.Binding
	ActionMenu key.Binding
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
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
	}
}

// NewInstanceDetailsView creates a new instance details view
func NewInstanceDetailsView(projectID, zone, instanceName string, computeClient *gcp.ComputeClient) *InstanceDetailsView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	// Initialize tabs
	tabsComponent := tabs.New([]tabs.Tab{
		{ID: tabIDDetails, Label: "Details"},
		{ID: tabIDObservability, Label: "Observability"},
	})

	return &InstanceDetailsView{
		computeClient: computeClient,
		projectID:     projectID,
		zone:          zone,
		instanceName:  instanceName,
		spinner:       s,
		loading:       true,
		keys:          defaultInstanceDetailsKeyMap(),
		tabs:          tabsComponent,
		tabViewports:  make([]viewport.Model, 2), // One viewport per tab
		diskLinks:     links.New(),
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

	case actionmenu.ActionSelectedMsg:
		// Handle action menu selection
		v.menuOpen = false
		return v.executeAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case spinner.TickMsg:
		if v.loading || v.actionLoading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tabs.TabChangedMsg:
		// Tab changed - update active viewport content
		v.updateViewportContent()
		return nil

	case links.LinkSelectedMsg:
		// Handle disk link selection - navigate to disk details
		if msg.Link.Type == "disk" {
			// Extract zone and disk name from the link data
			if diskInfo, ok := msg.Link.Data.(gcp.DiskInfo); ok {
				diskName, zone := extractDiskInfoFromSource(diskInfo.Source)
				if diskName != "" && zone != "" {
					return func() tea.Msg {
						return InstanceDiskSelectedMsg{DiskName: diskName, Zone: zone}
					}
				}
			}
		}
		return nil

	case tea.KeyMsg:
		if v.actionLoading {
			return nil
		}

		// Route keys to action menu when open
		if v.menuOpen {
			return v.actionMenu.Update(msg)
		}

		// Handle tab navigation keys first
		if tabs.HandleKey(msg) {
			return v.tabs.Update(msg)
		}

		// In Details tab, route j/k/Enter to disk links if available
		if v.tabs.ActiveTab().ID == tabIDDetails && v.diskLinks.HasItems() {
			if links.HandleKey(msg) {
				cmd := v.diskLinks.Update(msg)
				// Re-render content to update highlighting
				v.updateViewportContent()
				return cmd
			}
		}

		switch {
		case key.Matches(msg, v.keys.ActionMenu):
			// Toggle action menu
			if v.details != nil {
				v.actionMenu = actionmenu.New("Instance Actions", v.buildActions())
				v.menuOpen = true
			}
			return nil

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

	return nil
}

// buildActions creates the action menu items based on instance state
func (v *InstanceDetailsView) buildActions() []actionmenu.Action {
	isRunning := v.isInstanceRunning()
	isStopped := v.isInstanceStopped()

	return []actionmenu.Action{
		{Key: 's', Label: "Start", Enabled: isStopped},
		{Key: 'x', Label: "Stop", Enabled: isRunning},
		{Key: 'R', Label: "Reset", Enabled: isRunning, Dangerous: true},
		{Key: 'S', Label: "SSH", Enabled: isRunning},
		{Key: 'r', Label: "Refresh", Enabled: true},
	}
}

// executeAction performs the action selected from the menu
func (v *InstanceDetailsView) executeAction(actionKey rune) tea.Cmd {
	if v.details == nil {
		return nil
	}

	switch actionKey {
	case 's':
		if v.isInstanceStopped() {
			v.actionLoading = true
			v.actionMsg = fmt.Sprintf("Starting %s...", v.instanceName)
			return tea.Batch(v.spinner.Tick, v.startInstance())
		}
	case 'x':
		if v.isInstanceRunning() {
			v.actionLoading = true
			v.actionMsg = fmt.Sprintf("Stopping %s...", v.instanceName)
			return tea.Batch(v.spinner.Tick, v.stopInstance())
		}
	case 'R':
		if v.isInstanceRunning() {
			v.actionLoading = true
			v.actionMsg = fmt.Sprintf("Resetting %s...", v.instanceName)
			return tea.Batch(v.spinner.Tick, v.resetInstance())
		}
	case 'r':
		v.loading = true
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.loadDetails())
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

	// Render tab bar
	tabBar := "  " + v.tabs.View()

	// Get active tab viewport
	activeIdx := v.tabs.ActiveIndex()
	var viewportContent string
	var scrollPercent float64
	if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
		viewportContent = v.tabViewports[activeIdx].View()
		scrollPercent = v.tabViewports[activeIdx].ScrollPercent()
	}

	// Help text - context-sensitive based on active tab and available links
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", scrollPercent*100))

	var helpText string
	if v.tabs.ActiveTab().ID == tabIDDetails && v.diskLinks.HasItems() {
		// Details tab with disk links - show disk navigation hint
		helpText = "\n  j/k: select disk • enter: view disk • tab: switch tabs • .: actions"
	} else {
		helpText = "\n  tab/1-2: switch tabs • .: actions • r: refresh"
	}
	help := helpStyle.Render(helpText) + " " + scrollInfo

	mainContent := header + tabBar + "\n" + viewportContent + help

	// Overlay action menu if open
	if v.menuOpen && v.actionMenu != nil {
		return v.renderWithActionMenu(mainContent)
	}

	return mainContent
}

// renderWithActionMenu overlays the action menu centered on top of the content
func (v *InstanceDetailsView) renderWithActionMenu(content string) string {
	menuView := v.actionMenu.View()

	// Use stored width for consistent centering (like command palette)
	// Content width varies due to viewport padding, but we want centered in the full area
	contentHeight := lipgloss.Height(content)

	// Use overlay helper to composite menu on top of content
	return overlay.Center(content, menuView, v.width, contentHeight)
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *InstanceDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// IsMenuOpen returns true if the action menu is currently open
func (v *InstanceDetailsView) IsMenuOpen() bool {
	return v.menuOpen
}

// applySize applies the given dimensions to the viewports
func (v *InstanceDetailsView) applySize(width, height int) {
	// Reserve space for header, tab bar, and footer
	viewportHeight := height - 5
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	if !v.ready {
		// Initialize viewport for each tab
		for i := range v.tabViewports {
			v.tabViewports[i] = viewport.New(width, viewportHeight)
			v.tabViewports[i].Style = lipgloss.NewStyle().Padding(0, 2)
		}
		v.ready = true
	} else {
		// Update dimensions for all tab viewports
		for i := range v.tabViewports {
			v.tabViewports[i].Width = width
			v.tabViewports[i].Height = viewportHeight
		}
	}

	if v.details != nil {
		v.updateViewportContent()
	}
}

// updateViewportContent renders the content for the active tab's viewport
func (v *InstanceDetailsView) updateViewportContent() {
	if v.details == nil || !v.ready {
		return
	}

	activeIdx := v.tabs.ActiveIndex()
	if activeIdx < 0 || activeIdx >= len(v.tabViewports) {
		return
	}

	var content string
	switch v.tabs.ActiveTab().ID {
	case tabIDDetails:
		// Populate disk links from instance details
		v.populateDiskLinks()
		content = v.renderDetailsTab()
	case tabIDObservability:
		content = v.renderObservabilityTab()
	default:
		content = v.renderDetailsTab()
	}

	v.tabViewports[activeIdx].SetContent(content)
}

// populateDiskLinks creates link items from the instance's attached disks
func (v *InstanceDetailsView) populateDiskLinks() {
	if v.details == nil || len(v.details.Disks) == 0 {
		v.diskLinks.SetItems(nil)
		return
	}

	items := make([]links.Link, len(v.details.Disks))
	for i, disk := range v.details.Disks {
		items[i] = links.Link{
			ID:    disk.Name,
			Label: disk.Name,
			Type:  "disk",
			Data:  disk, // Store the DiskInfo for navigation
		}
	}
	v.diskLinks.SetItems(items)
}

// renderDetailsTab generates the Details tab content
func (v *InstanceDetailsView) renderDetailsTab() string {
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

	// Labels (sorted alphabetically for consistent display)
	if len(d.Labels) > 0 {
		b.WriteString(labelStyle.Render("Labels"))
		b.WriteString("\n")
		// Sort label keys for consistent ordering
		labelKeys := make([]string, 0, len(d.Labels))
		for k := range d.Labels {
			labelKeys = append(labelKeys, k)
		}
		sort.Strings(labelKeys)
		for _, k := range labelKeys {
			b.WriteString(fmt.Sprintf("    %s: %s\n", k, d.Labels[k]))
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

	// Storage - with navigable disk links
	b.WriteString(sectionStyle.Render("Storage"))
	b.WriteString("\n")
	if len(d.Disks) > 0 {
		// Render header using links component
		header := fmt.Sprintf("%-25s %-10s %-12s %-12s %-10s",
			"Name", "Size", "Type", "Mode", "Boot")
		b.WriteString(v.diskLinks.RenderHeader(header))
		b.WriteString("\n")
		b.WriteString(v.diskLinks.RenderDivider(72))
		b.WriteString("\n")

		// Render each disk row with link highlighting
		for i, disk := range d.Disks {
			bootStr := "—"
			if disk.Boot {
				bootStr = "Yes"
			}
			row := fmt.Sprintf("%-25s %-10s %-12s %-12s %-10s",
				truncate(disk.Name, 25),
				fmt.Sprintf("%d GB", disk.SizeGB),
				defaultIfEmpty(disk.Type, "—"),
				disk.Mode,
				bootStr)
			b.WriteString(v.diskLinks.RenderRow(i, row))
			b.WriteString("\n")
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

	// Metadata (sorted alphabetically for consistent display)
	if len(d.Metadata) > 0 {
		b.WriteString(sectionStyle.Render("Custom Metadata"))
		b.WriteString("\n")
		// Sort metadata keys for consistent ordering
		metaKeys := make([]string, 0, len(d.Metadata))
		for k := range d.Metadata {
			metaKeys = append(metaKeys, k)
		}
		sort.Strings(metaKeys)
		for _, k := range metaKeys {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, truncate(d.Metadata[k], 50)))
		}
	}

	return b.String()
}

// renderObservabilityTab generates the Observability tab content
// Placeholder for future metrics/monitoring integration
func (v *InstanceDetailsView) renderObservabilityTab() string {
	d := v.details
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	// Header with status
	statusIcon := getStatusIcon(d.Status)
	b.WriteString(titleStyle.Render(fmt.Sprintf("Instance: %s  %s %s", d.Name, statusIcon, d.Status)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Observability section
	b.WriteString(sectionStyle.Render("Observability"))
	b.WriteString("\n\n")

	b.WriteString(mutedStyle.Render("  Metrics and monitoring data will be available in a future update."))
	b.WriteString("\n\n")

	b.WriteString(mutedStyle.Render("  Planned features:"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  • CPU utilization"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  • Memory usage"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  • Network traffic"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  • Disk I/O"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  • Recent logs"))
	b.WriteString("\n")

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

// extractDiskInfoFromSource parses a disk source URL and returns disk name and zone
// Source format: projects/{project}/zones/{zone}/disks/{diskName}
func extractDiskInfoFromSource(source string) (diskName, zone string) {
	parts := strings.Split(source, "/")
	// Need at least: projects/X/zones/Y/disks/Z (6 parts)
	if len(parts) < 6 {
		return "", ""
	}

	// Find the indices for zones and disks
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "zones" && i+1 < len(parts) {
			zone = parts[i+1]
		}
		if parts[i] == "disks" && i+1 < len(parts) {
			diskName = parts[i+1]
		}
	}
	return diskName, zone
}

// GetComputeClient returns the compute client for reuse in other detail views
func (v *InstanceDetailsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// renderLoading renders a loading message
// Height enforcement is handled by the app's View() method using lipgloss.MaxHeight()
func (v *InstanceDetailsView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}
