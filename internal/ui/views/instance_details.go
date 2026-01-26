package views

import (
	gocontext "context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/links"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/focus"
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

// metricsLoadedMsg contains fetched observability metrics
type metricsLoadedMsg struct {
	metrics *gcp.ObservabilityMetrics
}

// metricsErrorMsg indicates an error loading metrics
type metricsErrorMsg struct {
	err error
}

// logsLoadedMsg contains fetched log entries
type logsLoadedMsg struct {
	logs []gcp.LogEntry
}

// logsErrorMsg indicates an error loading logs
type logsErrorMsg struct {
	err error
}

// refreshTickMsg triggers auto-refresh of metrics
type refreshTickMsg struct{}

// Recommendation represents an actionable insight based on metrics
type Recommendation struct {
	Severity string // "warning", "critical"
	Message  string
	Action   string
}

// Tab IDs for instance details view
const (
	tabIDDetails       = "details"
	tabIDObservability = "observability"
)

// Focus region IDs for instance details view
const (
	regionIDTabs     = "tabs"
	regionIDLinks    = "links"
	regionIDViewport = "viewport"
)

// Layout constants for viewport height calculation
const (
	// Lines reserved for: optional header (max 2) + tab bar (1) + separator (1) + help text (1)
	// We use 5 to account for the maximum case
	detailsViewportReservedLines = 5
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
	// Focus management for routing keys between regions
	focusMgr *focus.Manager
	// Observability tab state
	metrics           *gcp.ObservabilityMetrics
	logs              []gcp.LogEntry
	metricsLoading    bool
	logsLoading       bool
	metricsError      error
	logsError         error
	timeRange         time.Duration
	autoRefresh       bool
	autoRefreshTicker *time.Ticker
	gcpClient         *gcp.Client
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
func NewInstanceDetailsView(projectID, zone, instanceName string, computeClient *gcp.ComputeClient, gcpClient *gcp.Client) *InstanceDetailsView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	// Initialize tabs
	tabsComponent := tabs.New([]tabs.Tab{
		{ID: tabIDDetails, Label: "Details"},
		{ID: tabIDObservability, Label: "Observability"},
	})

	// Initialize focus manager with default regions
	// Links region starts disabled until we know disks exist
	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewRegion(regionIDTabs, focus.RegionTabs, "Tabs"),
		focus.NewDisabledRegion(regionIDLinks, focus.RegionLinks, "Disks"),
		focus.NewRegion(regionIDViewport, focus.RegionViewport, "Content"),
	})

	return &InstanceDetailsView{
		computeClient: computeClient,
		gcpClient:     gcpClient,
		projectID:     projectID,
		zone:          zone,
		instanceName:  instanceName,
		spinner:       s,
		loading:       true,
		keys:          defaultInstanceDetailsKeyMap(),
		tabs:          tabsComponent,
		tabViewports:  make([]viewport.Model, 2), // One viewport per tab
		diskLinks:     links.New(),
		focusMgr:      fm,
		timeRange:     6 * time.Hour, // Default to 6 hours
		autoRefresh:   true,          // Auto-refresh enabled by default
	}
}

// Init initializes the view and starts loading instance details
func (v *InstanceDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
	)
}

// Close releases resources associated with the InstanceDetailsView.
// Should be called when the view is no longer active to prevent resource leaks.
func (v *InstanceDetailsView) Close() {
	if v == nil {
		return
	}
	if v.autoRefreshTicker != nil {
		v.autoRefreshTicker.Stop()
		v.autoRefreshTicker = nil
	}
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
//nolint:gocognit // Bubble Tea Update pattern - complexity 90
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

	case metricsLoadedMsg:
		v.metricsLoading = false
		v.metrics = msg.metrics
		v.metricsError = nil
		v.updateViewportContent()
		return nil

	case metricsErrorMsg:
		v.metricsLoading = false
		v.metricsError = msg.err
		v.updateViewportContent()
		return nil

	case logsLoadedMsg:
		v.logsLoading = false
		v.logs = msg.logs
		v.logsError = nil
		v.updateViewportContent()
		return nil

	case logsErrorMsg:
		v.logsLoading = false
		v.logsError = msg.err
		v.updateViewportContent()
		return nil

	case refreshTickMsg:
		// Auto-refresh triggered - schedule next tick to continue the cycle
		if v.autoRefresh && v.tabs.ActiveTab().ID == tabIDObservability {
			return tea.Batch(v.loadMetrics(), v.loadLogs(), v.tickAutoRefresh())
		}
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
		// Handle metrics/logs loading and auto-refresh when switching tabs
		if v.tabs.ActiveTab().ID == tabIDObservability {
			var cmds []tea.Cmd
			// Load metrics/logs when first switching to observability tab
			if v.metrics == nil && !v.metricsLoading {
				cmds = append(cmds, v.loadMetrics(), v.loadLogs())
			}
			// Start or continue auto-refresh ticker when auto-refresh is enabled
			if v.autoRefresh {
				if v.autoRefreshTicker == nil {
					v.autoRefreshTicker = time.NewTicker(30 * time.Second)
				}
				cmds = append(cmds, v.tickAutoRefresh())
			}
			if len(cmds) > 0 {
				return tea.Batch(cmds...)
			}
			return nil
		}
		// When leaving the observability tab, stop any running auto-refresh ticker
		if v.autoRefreshTicker != nil {
			v.autoRefreshTicker.Stop()
			v.autoRefreshTicker = nil
		}
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

	case focus.FocusChangedMsg:
		// Focus changed between regions - update rendering
		v.updateViewportContent()
		return nil

	case tea.KeyMsg:
		if v.actionLoading {
			return nil
		}

		// Route keys to action menu when open
		if v.menuOpen {
			return v.actionMenu.Update(msg)
		}

		// Handle Tab/Shift+Tab for cycling between focus regions
		if focusMsg := v.focusMgr.HandleKey(msg); focusMsg != nil {
			v.updateViewportContent()
			return func() tea.Msg { return focusMsg }
		}

		// Route keys based on currently focused region
		switch v.focusMgr.ActiveType() {
		case focus.RegionTabs:
			// When tabs region is focused, h/l/1-9 switch tabs
			if tabs.HandleKey(msg) {
				return v.tabs.Update(msg)
			}

		case focus.RegionLinks:
			// When links region is focused, j/k navigate links
			// Only available in Details tab with disk links
			if v.tabs.ActiveTab().ID == tabIDDetails && v.diskLinks.HasItems() {
				if links.HandleKey(msg) {
					cmd := v.diskLinks.Update(msg)
					v.updateViewportContent()
					return cmd
				}
			}

		case focus.RegionViewport:
			// When viewport region is focused, j/k scroll content
			activeIdx := v.tabs.ActiveIndex()
			if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
				var cmd tea.Cmd
				v.tabViewports[activeIdx], cmd = v.tabViewports[activeIdx].Update(msg)
				return cmd
			}
		}

		// In Observability tab, handle time range and refresh keys
		if v.tabs.ActiveTab().ID == tabIDObservability {
			switch msg.String() {
			case "1":
				v.timeRange = 1 * time.Hour
				v.metricsError = nil
				v.logsError = nil
				v.metricsLoading = true
				v.logsLoading = true
				return tea.Batch(v.spinner.Tick, v.loadMetrics(), v.loadLogs())
			case "2":
				v.timeRange = 6 * time.Hour
				v.metricsError = nil
				v.logsError = nil
				v.metricsLoading = true
				v.logsLoading = true
				return tea.Batch(v.spinner.Tick, v.loadMetrics(), v.loadLogs())
			case "3":
				v.timeRange = 24 * time.Hour
				v.metricsError = nil
				v.logsError = nil
				v.metricsLoading = true
				v.logsLoading = true
				return tea.Batch(v.spinner.Tick, v.loadMetrics(), v.loadLogs())
			case "4":
				v.timeRange = 7 * 24 * time.Hour
				v.metricsError = nil
				v.logsError = nil
				v.metricsLoading = true
				v.logsLoading = true
				return tea.Batch(v.spinner.Tick, v.loadMetrics(), v.loadLogs())
			case "5":
				v.timeRange = 30 * time.Hour
				v.metricsError = nil
				v.logsError = nil
				v.metricsLoading = true
				v.logsLoading = true
				return tea.Batch(v.spinner.Tick, v.loadMetrics(), v.loadLogs())
			case "a":
				v.autoRefresh = !v.autoRefresh
				if v.autoRefresh {
					// Start auto-refresh ticker
					v.autoRefreshTicker = time.NewTicker(30 * time.Second)
					return v.tickAutoRefresh()
				} else if v.autoRefreshTicker != nil {
					// Stop auto-refresh ticker. We intentionally do not set
					// v.autoRefreshTicker to nil here to avoid a race with the
					// tickAutoRefresh goroutine that may still read from it.
					v.autoRefreshTicker.Stop()
				}
				v.updateViewportContent()
				return nil
			}
		}

		// View-specific action keys (work regardless of focus)
		switch {
		case key.Matches(msg, v.keys.ActionMenu):
			if v.details != nil {
				v.actionMenu = actionmenu.New("Instance Actions", v.buildActions())
				v.menuOpen = true
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			// In observability tab, refresh metrics instead of details
			if v.tabs.ActiveTab().ID == tabIDObservability {
				v.metricsLoading = true
				v.logsLoading = true
				v.metricsError = nil
				v.logsError = nil
				return tea.Batch(v.spinner.Tick, v.loadMetrics(), v.loadLogs())
			}
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
		{Key: 'l', Label: "Edit Labels", Enabled: true},
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
	case 'l':
		// Edit Labels - emit message to app to navigate to editor
		return func() tea.Msg {
			return InstanceEditRequestMsg{
				ProjectID:    v.projectID,
				Zone:         v.zone,
				InstanceName: v.instanceName,
				EditMode:     "labels",
			}
		}
	case 'S':
		if v.isInstanceRunning() {
			// SSH to instance is a planned feature
			v.err = fmt.Errorf("%w for instance %s", uierrors.ErrSSHNotImplemented, v.instanceName)
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

// loadMetrics fetches observability metrics from Cloud Monitoring
func (v *InstanceDetailsView) loadMetrics() tea.Cmd {
	return func() tea.Msg {
		if v.gcpClient == nil || v.details == nil {
			return metricsErrorMsg{err: uierrors.ErrDetailsNotAvailable}
		}

		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 30*time.Second)
		defer cancel()

		monitoringClient, err := v.gcpClient.GetMonitoringClient(v.projectID)
		if err != nil {
			return metricsErrorMsg{err: fmt.Errorf("failed to initialize monitoring client: %w", err)}
		}

		// Extract instance ID from self link
		instanceID := fmt.Sprintf("%d", v.details.ID)

		// Fetch metrics
		metrics := &gcp.ObservabilityMetrics{
			LastFetch: time.Now(),
		}

		// CPU utilization
		cpuData, err := monitoringClient.GetCPUUtilization(ctx, instanceID, v.zone, v.timeRange)
		if err != nil {
			return metricsErrorMsg{err: fmt.Errorf("failed to fetch CPU metrics: %w", err)}
		}
		metrics.CPU = cpuData

		// Memory utilization (optional - requires Ops Agent)
		memData, _ := monitoringClient.GetMemoryUtilization(ctx, instanceID, v.zone, v.timeRange)
		metrics.Memory = memData

		// Network traffic
		networkData, err := monitoringClient.GetNetworkTraffic(ctx, instanceID, v.zone, v.timeRange)
		if err != nil {
			return metricsErrorMsg{err: fmt.Errorf("failed to fetch network metrics: %w", err)}
		}
		metrics.Network = networkData

		// Disk I/O
		diskData, err := monitoringClient.GetDiskIO(ctx, instanceID, v.zone, v.timeRange)
		if err != nil {
			return metricsErrorMsg{err: fmt.Errorf("failed to fetch disk metrics: %w", err)}
		}
		metrics.Disks = diskData

		// Calculate uptime (use CreatedAt as approximation if instance is running)
		if v.details.CreatedAt != "" && v.details.Status == "RUNNING" {
			startTime, err := time.Parse(time.RFC3339, v.details.CreatedAt)
			if err == nil {
				metrics.Uptime = time.Since(startTime)
			}
		}

		return metricsLoadedMsg{metrics: metrics}
	}
}

// loadLogs fetches recent logs from Cloud Logging
func (v *InstanceDetailsView) loadLogs() tea.Cmd {
	return func() tea.Msg {
		if v.gcpClient == nil || v.details == nil {
			return logsErrorMsg{err: uierrors.ErrDetailsNotAvailable}
		}

		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 30*time.Second)
		defer cancel()

		loggingClient, err := v.gcpClient.GetLoggingClient(v.projectID)
		if err != nil {
			return logsErrorMsg{err: fmt.Errorf("failed to initialize logging client: %w", err)}
		}

		// Extract instance ID from self link
		instanceID := fmt.Sprintf("%d", v.details.ID)

		// Fetch recent warning/error logs
		logs, err := loggingClient.GetRecentLogs(ctx, instanceID, v.zone, "WARNING", 10)
		if err != nil {
			return logsErrorMsg{err: fmt.Errorf("failed to fetch logs: %w", err)}
		}

		return logsLoadedMsg{logs: logs}
	}
}

// tickAutoRefresh creates a command for auto-refresh ticker
func (v *InstanceDetailsView) tickAutoRefresh() tea.Cmd {
	return func() tea.Msg {
		// If auto-refresh is disabled or the ticker is not available, do nothing.
		if v.autoRefreshTicker == nil || !v.autoRefresh {
			return nil
		}

		// Block until the next tick. If the ticker has been stopped, this command
		// should not be scheduled again when auto-refresh is turned off.
		<-v.autoRefreshTicker.C

		// Auto-refresh may have been turned off while waiting; in that case,
		// do not emit a refresh tick message.
		if !v.autoRefresh {
			return nil
		}
		return refreshTickMsg{}
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

	// Help text - context-sensitive based on focused region
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", scrollPercent*100))

	helpText := v.buildHelpText()
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
	viewportHeight := height - detailsViewportReservedLines
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

	// Update links focus state based on focus manager
	v.diskLinks.SetRegionFocused(v.focusMgr.IsActive(regionIDLinks))

	var content string
	switch v.tabs.ActiveTab().ID {
	case tabIDDetails:
		// Populate disk links from instance details
		v.populateDiskLinks()
		content = v.renderDetailsTab()
	case tabIDObservability:
		// Disable links region when not on Details tab
		v.focusMgr.DisableRegion(regionIDLinks)
		content = v.renderObservabilityTab()
	default:
		content = v.renderDetailsTab()
	}

	v.tabViewports[activeIdx].SetContent(content)
}

// populateDiskLinks creates link items from the instance's attached disks
// and enables/disables the links focus region accordingly
func (v *InstanceDetailsView) populateDiskLinks() {
	if v.details == nil || len(v.details.Disks) == 0 {
		v.diskLinks.SetItems(nil)
		v.focusMgr.DisableRegion(regionIDLinks)
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
	// Enable links region when in Details tab
	if v.tabs.ActiveTab().ID == tabIDDetails {
		v.focusMgr.EnableRegion(regionIDLinks)
	}
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

// renderObservabilityTab generates the Observability tab content with real-time metrics
//nolint:gocognit // Metrics rendering with multiple sections - complexity 31
func (v *InstanceDetailsView) renderObservabilityTab() string {
	d := v.details
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Bold(true)
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04")).Bold(true)

	// Header with status
	statusIcon := getStatusIcon(d.Status)
	b.WriteString(titleStyle.Render(fmt.Sprintf("Instance: %s  %s %s", d.Name, statusIcon, d.Status)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Time range selector
	if v.metrics != nil {
		selector := components.RenderTimeRangeSelector(v.timeRange, v.autoRefresh, v.metrics.LastFetch)
		b.WriteString(selector)
		b.WriteString("\n\n")
	}

	// Show loading state
	if v.metricsLoading {
		b.WriteString(fmt.Sprintf("  %s Loading metrics...\n\n", v.spinner.View()))
		return b.String()
	}

	// Show error state
	if v.metricsError != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  ✗ Error loading metrics: %s", v.metricsError.Error())))
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("  Press 'r' to retry"))
		b.WriteString("\n")
		return b.String()
	}

	// No metrics loaded yet
	if v.metrics == nil {
		b.WriteString(mutedStyle.Render("  Loading metrics for the first time..."))
		b.WriteString("\n")
		return b.String()
	}

	// Render metrics sections
	m := v.metrics

	// CPU utilization section
	b.WriteString(sectionStyle.Render("CPU Usage"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", min(v.width-4, 60)))
	b.WriteString("\n")
	if len(m.CPU) > 0 {
		// Extract values for sparkline
		cpuValues := make([]float64, len(m.CPU))
		for i, dp := range m.CPU {
			cpuValues[i] = dp.Value * 100 // Convert to percentage
		}

		// Calculate stats
		current := cpuValues[len(cpuValues)-1]
		avg, peak, peakTime := calculateStats(m.CPU)
		avg *= 100
		peak *= 100

		// Render sparkline
		sparkline := components.RenderSparkline(cpuValues, min(v.width-12, 50))
		b.WriteString(fmt.Sprintf("  Trend (%s): %s\n", formatDuration(v.timeRange), sparkline))

		// Render bar
		bar := components.RenderMetricBar("", current, avg, peak, peakTime, v.width-4)
		b.WriteString("  ")
		b.WriteString(bar)
		b.WriteString("\n")
	} else {
		b.WriteString(mutedStyle.Render("  No CPU data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Memory utilization section
	b.WriteString(sectionStyle.Render("Memory Usage"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", min(v.width-4, 60)))
	b.WriteString("\n")
	if len(m.Memory) > 0 {
		// Extract values for sparkline
		memValues := make([]float64, len(m.Memory))
		for i, dp := range m.Memory {
			memValues[i] = dp.Value
		}

		// Calculate stats
		current := memValues[len(memValues)-1]
		avg, peak, peakTime := calculateStats(m.Memory)

		// Render sparkline
		sparkline := components.RenderSparkline(memValues, min(v.width-12, 50))
		b.WriteString(fmt.Sprintf("  Trend (%s): %s\n", formatDuration(v.timeRange), sparkline))

		// Render bar
		bar := components.RenderMetricBar("", current, avg, peak, peakTime, v.width-4)
		b.WriteString("  ")
		b.WriteString(bar)
		b.WriteString("\n")
	} else {
		b.WriteString(warningStyle.Render("  ⚠ Cloud Monitoring (Ops) Agent not installed"))
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("  To enable memory metrics, install the Ops Agent:"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("    gcloud compute instances ops-agents policies create \\"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("      --agent-rules=\"type=ops-agent,version=latest,enable-autoupgrade=true\""))
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("  Learn more: https://cloud.google.com/stackdriver/docs/solutions/agents/ops-agent"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Network traffic section
	b.WriteString(sectionStyle.Render("Network Traffic"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", min(v.width-4, 60)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  ▲ Sent:     %.2f MB/s  |  Total: %.2f GB\n",
		m.Network.SentBytesPerSec/1024/1024, m.Network.TotalSentBytes/1024/1024/1024))
	b.WriteString(fmt.Sprintf("  ▼ Received: %.2f MB/s  |  Total: %.2f GB\n",
		m.Network.ReceivedBytesPerSec/1024/1024, m.Network.TotalReceivedBytes/1024/1024/1024))
	b.WriteString("\n")

	// Disk I/O section
	b.WriteString(sectionStyle.Render("Disk I/O"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", min(v.width-4, 60)))
	b.WriteString("\n")
	if len(m.Disks) > 0 {
		for _, disk := range m.Disks {
			b.WriteString(fmt.Sprintf("  %s\n", disk.DiskName))
			b.WriteString(fmt.Sprintf("    Read:  %.0f ops/s  |  %.2f MB/s  |  Total: %.2f GB\n",
				disk.ReadOpsPerSec, disk.ReadBytesPerSec/1024/1024, disk.TotalReadBytes/1024/1024/1024))
			b.WriteString(fmt.Sprintf("    Write: %.0f ops/s  |  %.2f MB/s  |  Total: %.2f GB\n",
				disk.WriteOpsPerSec, disk.WriteBytesPerSec/1024/1024, disk.TotalWriteBytes/1024/1024/1024))
		}
	} else {
		b.WriteString(mutedStyle.Render("  No disk data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Instance health section
	b.WriteString(sectionStyle.Render("Instance Health"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", min(v.width-4, 60)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Status:       %s %s\n", statusIcon, d.Status))
	if m.Uptime > 0 {
		b.WriteString(fmt.Sprintf("  Uptime:       %s\n", formatUptime(m.Uptime)))
	}
	if d.CreatedAt != "" {
		b.WriteString(fmt.Sprintf("  Created:      %s\n", formatTimestamp(d.CreatedAt)))
	}
	b.WriteString("\n")

	// Recommendations section
	recommendations := v.analyzeMetrics()
	if len(recommendations) > 0 {
		b.WriteString(sectionStyle.Render("Recommendations"))
		b.WriteString("\n")
		b.WriteString(strings.Repeat("━", min(v.width-4, 60)))
		b.WriteString("\n")
		for _, rec := range recommendations {
			if rec.Severity == "critical" {
				b.WriteString(errorStyle.Render("  ⚠ " + rec.Message))
			} else {
				b.WriteString(warningStyle.Render("  ⚠ " + rec.Message))
			}
			b.WriteString("\n")
			b.WriteString(mutedStyle.Render(fmt.Sprintf("    → %s", rec.Action)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Recent logs section
	b.WriteString(sectionStyle.Render("Recent Logs (warnings/errors)"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", min(v.width-4, 60)))
	b.WriteString("\n")
	switch {
	case v.logsLoading:
		b.WriteString(fmt.Sprintf("  %s Loading logs...\n", v.spinner.View()))
	case v.logsError != nil:
		b.WriteString(errorStyle.Render(fmt.Sprintf("  ✗ Error loading logs: %s", v.logsError.Error())))
		b.WriteString("\n")
	case len(v.logs) > 0:
		for _, log := range v.logs {
			var severityColor string
			switch log.Severity {
			case "ERROR", "CRITICAL":
				severityColor = "#EA4335"
			case "WARNING":
				severityColor = "#FBBC04"
			default:
				severityColor = "#9AA0A6"
			}
			severityStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(severityColor)).Bold(true)
			b.WriteString(fmt.Sprintf("  %s [%s] %s\n",
				log.Timestamp.Format("15:04:05"),
				severityStyle.Render(log.Severity),
				truncate(log.Message, v.width-30)))
		}
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  Showing severity >= WARNING"))
		b.WriteString("\n")
	default:
		b.WriteString(mutedStyle.Render("  No recent warnings or errors"))
		b.WriteString("\n")
	}

	return b.String()
}

// analyzeMetrics generates recommendations based on metric patterns
func (v *InstanceDetailsView) analyzeMetrics() []Recommendation {
	if v.metrics == nil {
		return nil
	}

	var recommendations []Recommendation
	m := v.metrics

	// Check CPU usage
	if len(m.CPU) > 0 {
		avg, _, _ := calculateStats(m.CPU)
		avg *= 100
		if avg > 80 {
			recommendations = append(recommendations, Recommendation{
				Severity: "warning",
				Message:  fmt.Sprintf("High CPU usage detected (avg %.0f%% over %s)", avg, formatDuration(v.timeRange)),
				Action:   "Consider upgrading to a larger machine type",
			})
		}
	}

	// Check memory usage
	if len(m.Memory) > 0 {
		avg, _, _ := calculateStats(m.Memory)
		if avg > 85 {
			recommendations = append(recommendations, Recommendation{
				Severity: "warning",
				Message:  fmt.Sprintf("High memory usage detected (avg %.0f%% over %s)", avg, formatDuration(v.timeRange)),
				Action:   "Increase memory or add swap space",
			})
		}
	}

	return recommendations
}

// Helper functions
// Shared helpers (renderRow, defaultIfEmpty, min) are now in helpers.go

// calculateStats calculates average, peak value and peak time from data points
func calculateStats(data []gcp.DataPoint) (avg, peak float64, peakTime time.Time) {
	if len(data) == 0 {
		return 0, 0, time.Time{}
	}

	sum := 0.0
	peak = data[0].Value
	peakTime = data[0].Timestamp

	for _, dp := range data {
		sum += dp.Value
		if dp.Value > peak {
			peak = dp.Value
			peakTime = dp.Timestamp
		}
	}

	avg = sum / float64(len(data))
	return avg, peak, peakTime
}

// formatDuration formats a time duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

// formatUptime formats an uptime duration
func formatUptime(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// formatTimestamp formats an RFC3339 timestamp in human-readable form
func formatTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Format("Jan 02, 2006 at 3:04 PM")
}

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

var diskSourceRegex = regexp.MustCompile(`projects/[^/]+/zones/([^/]+)/disks/([^/]+)`)

// extractDiskInfoFromSource parses a disk source URL and returns disk name and zone
// Source format: projects/{project}/zones/{zone}/disks/{diskName}
func extractDiskInfoFromSource(source string) (diskName, zone string) {
	matches := diskSourceRegex.FindStringSubmatch(source)
	if len(matches) == 3 {
		// matches[0] is the full string, [1] is the first group (zone), [2] is the second (diskName)
		return matches[2], matches[1]
	}
	return "", ""
}

// GetComputeClient returns the compute client for reuse in other detail views
func (v *InstanceDetailsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// GetInstanceName returns the instance name for use in breadcrumbs
func (v *InstanceDetailsView) GetInstanceName() string {
	return v.instanceName
}

// buildHelpText generates context-sensitive help text based on the focused region
func (v *InstanceDetailsView) buildHelpText() string {
	bindings := focus.HelpForRegion(v.focusMgr.ActiveType(), v.getRegionLabel())
	helpStr := focus.FormatHelp(bindings)
	return "\n  " + helpStr + " • .: actions"
}

// getRegionLabel returns a descriptive label for the current focus context
func (v *InstanceDetailsView) getRegionLabel() string {
	if v.focusMgr.ActiveType() == focus.RegionLinks {
		return "disk"
	}
	return ""
}

// renderLoading renders a loading message
// Height enforcement is handled by the app's View() method using lipgloss.MaxHeight()
func (v *InstanceDetailsView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}
