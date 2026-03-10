package views

import (
	gocontext "context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/focus"
	"github.com/slayer/gcon/internal/ui/mouse"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// Tab IDs for Cloud Run service details
const (
	runTabIDDetails       = "details"
	runTabIDRevisions     = "revisions"
	runTabIDYAML          = "yaml"
	runTabIDObservability = "observability"
)

// Focus region IDs
const (
	runRegionIDTabs     = "tabs"
	runRegionIDViewport = "viewport"
)

// Lines reserved for tab bar + help text
const runDetailsViewportReservedLines = 5

// Internal messages for async data loading
type crServiceDetailsLoadedMsg struct{ details *gcp.CloudRunServiceDetails }
type crServiceDetailsErrorMsg struct{ err error }
type crRevisionsLoadedMsg struct{ revisions []gcp.CloudRunRevision }
type crRevisionsErrorMsg struct{ err error }

// CloudRunServiceDetailsView shows comprehensive service information with tabs
type CloudRunServiceDetailsView struct {
	runClient   *gcp.CloudRunClient
	projectID   string
	serviceName string // Short name for breadcrumbs
	fullName    string // Full resource name for API calls
	ctx         *context.ProgramContext

	// Data — each dataset loads independently
	details   *gcp.CloudRunServiceDetails
	revisions []gcp.CloudRunRevision

	// Separate loading/error state per dataset
	detailsLoading   bool
	revisionsLoading bool
	detailsErr       error
	revisionsErr     error

	// UI state
	spinner spinner.Model
	width   int
	height  int
	ready   bool

	// Tab navigation (Details / Revisions / YAML)
	tabs         *tabs.Tabs
	tabViewports []viewport.Model

	// Focus management
	focusMgr  *focus.Manager
	regionMgr *mouse.RegionManager

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool

	// Traffic split dialog
	trafficDialog     *trafficSplitDialog
	showTrafficDialog bool

	// Observability tab
	observability *cloudRunObservability
	gcpClient     *gcp.Client

	keys crServiceDetailsKeyMap
}

type crServiceDetailsKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Refresh      key.Binding
	Delete       key.Binding
	Edit         key.Binding
	TrafficSplit key.Binding
	ActionMenu   key.Binding
}

func defaultCRServiceDetailsKeyMap() crServiceDetailsKeyMap {
	return crServiceDetailsKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit service"),
		),
		TrafficSplit: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "edit traffic"),
		),
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
	}
}

// NewCloudRunServiceDetailsView creates a new Cloud Run service details view
func NewCloudRunServiceDetailsView(projectID, serviceName, fullName string, runClient *gcp.CloudRunClient, gcpClient *gcp.Client) *CloudRunServiceDetailsView {
	s := components.NewGCPSpinner()

	tabsComponent := tabs.New([]tabs.Tab{
		{ID: runTabIDDetails, Label: "Details"},
		{ID: runTabIDRevisions, Label: "Revisions"},
		{ID: runTabIDYAML, Label: "YAML"},
		{ID: runTabIDObservability, Label: "Observability"},
	})

	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewRegion(runRegionIDTabs, focus.RegionTabs, "Tabs"),
		focus.NewRegion(runRegionIDViewport, focus.RegionViewport, "Content"),
	})

	return &CloudRunServiceDetailsView{
		runClient:        runClient,
		gcpClient:        gcpClient,
		projectID:        projectID,
		serviceName:      serviceName,
		fullName:         fullName,
		spinner:          s,
		detailsLoading:   true,
		revisionsLoading: true,
		keys:             defaultCRServiceDetailsKeyMap(),
		tabs:             tabsComponent,
		tabViewports:     make([]viewport.Model, 4),
		focusMgr:         fm,
		regionMgr:        mouse.NewRegionManager(),
	}
}

// Close releases resources associated with the view.
// Must be called before nil-ing the view to prevent goroutine/ticker leaks.
func (v *CloudRunServiceDetailsView) Close() {
	if v == nil {
		return
	}
	if v.observability != nil {
		v.observability.StopAutoRefresh()
	}
}

// Init starts loading all datasets in parallel
func (v *CloudRunServiceDetailsView) Init() tea.Cmd {
	// Reset state — Init() may be called multiple times (e.g., after traffic update refreshes)
	v.detailsLoading = true
	v.revisionsLoading = true
	v.detailsErr = nil
	v.revisionsErr = nil

	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
		v.loadRevisions(),
	)
}

func (v *CloudRunServiceDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		if v.runClient == nil {
			return crServiceDetailsErrorMsg{err: uierrors.ErrCloudRunClientNotInitialized}
		}
		details, err := v.runClient.GetService(gocontext.Background(), v.fullName)
		if err != nil {
			return crServiceDetailsErrorMsg{err: err}
		}
		return crServiceDetailsLoadedMsg{details: details}
	}
}

func (v *CloudRunServiceDetailsView) loadRevisions() tea.Cmd {
	return func() tea.Msg {
		if v.runClient == nil {
			return crRevisionsErrorMsg{err: uierrors.ErrCloudRunClientNotInitialized}
		}
		revisions, err := v.runClient.ListRevisions(gocontext.Background(), v.fullName)
		if err != nil {
			return crRevisionsErrorMsg{err: err}
		}
		return crRevisionsLoadedMsg{revisions: revisions}
	}
}

// applyTrafficIfReady enriches revisions with traffic data once both datasets are loaded.
// Details and revisions load in parallel, so whichever finishes second triggers this.
func (v *CloudRunServiceDetailsView) applyTrafficIfReady() {
	if v.details != nil && len(v.revisions) > 0 {
		gcp.ApplyTrafficToRevisions(v.revisions, v.details.Traffic)
	}
}

// Update handles messages for the Cloud Run service details view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern - complexity expected
func (v *CloudRunServiceDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case crServiceDetailsLoadedMsg:
		v.detailsLoading = false
		v.details = msg.details
		v.applyTrafficIfReady()
		v.updateViewportContent()
		return nil

	case crServiceDetailsErrorMsg:
		v.detailsLoading = false
		v.detailsErr = msg.err
		v.updateViewportContent()
		return nil

	case crRevisionsLoadedMsg:
		v.revisionsLoading = false
		v.revisions = msg.revisions
		v.applyTrafficIfReady()
		v.updateViewportContent()
		return nil

	case crRevisionsErrorMsg:
		v.revisionsLoading = false
		v.revisionsErr = msg.err
		v.updateViewportContent()
		return nil

	case crMetricsLoadedMsg, crMetricsErrorMsg, crLogsLoadedMsg, crLogsErrorMsg, crRefreshTickMsg:
		if v.observability != nil {
			cmd, _ := v.observability.Update(msg)
			v.updateViewportContent()
			return cmd
		}
		return nil

	case spinner.TickMsg:
		if v.detailsLoading || v.revisionsLoading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		// Forward spinner ticks to observability when on that tab
		if v.observability != nil && v.tabs.ActiveTab().ID == runTabIDObservability {
			cmd, handled := v.observability.Update(msg)
			if handled {
				v.updateViewportContent()
				return cmd
			}
		}
		return nil

	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		return v.executeAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case confirm.TypeConfirmMsg:
		v.showDeleteConfirm = false
		if v.details != nil {
			return func() tea.Msg {
				return DeleteCloudRunServiceConfirmedMsg{
					FullName: v.fullName,
					Name:     v.serviceName,
				}
			}
		}
		return nil

	case confirm.TypeCancelMsg:
		v.showDeleteConfirm = false
		return nil

	case tabs.TabChangedMsg:
		// Lazy-load observability on first tab switch; manage auto-refresh lifecycle
		if v.tabs.ActiveTab().ID == runTabIDObservability {
			if v.observability == nil {
				v.observability = newCloudRunObservability(v.projectID, v.serviceName, v.gcpClient)
				v.observability.width = max(1, v.width-1) // match applySize viewport width
				v.observability.resizeCharts()
			}
			// Update viewport after observability exists so first frame shows loading state
			v.updateViewportContent()
			if v.observability.metricsLoading || v.observability.metrics == nil {
				return tea.Batch(v.observability.Init(), v.observability.StartAutoRefresh())
			}
			return v.observability.StartAutoRefresh()
		}
		// Stop auto-refresh when leaving observability tab
		if v.observability != nil {
			v.observability.StopAutoRefresh()
		}
		v.updateViewportContent()
		return nil

	case focus.FocusChangedMsg:
		v.updateViewportContent()
		return nil

	case tea.KeyMsg:
		// Route to traffic dialog first (highest priority)
		if v.showTrafficDialog && v.trafficDialog != nil {
			cmd, done := v.trafficDialog.Update(msg)
			if done {
				v.showTrafficDialog = false
				if targets := v.trafficDialog.Result(); targets != nil {
					return func() tea.Msg {
						return CloudRunTrafficUpdateMsg{
							FullName: v.fullName,
							Targets:  targets,
						}
					}
				}
			}
			return cmd
		}

		// Route to action menu when open
		if v.menuOpen {
			return v.actionMenu.Update(msg)
		}

		// Route to delete confirm when shown
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		// Handle Tab/Shift+Tab for cycling between focus regions
		if focusMsg := v.focusMgr.HandleKey(msg); focusMsg != nil {
			v.updateViewportContent()
			return func() tea.Msg { return focusMsg }
		}

		// Action keys work regardless of focused region
		switch {
		case key.Matches(msg, v.keys.ActionMenu):
			if v.details != nil {
				v.actionMenu = actionmenu.New("Cloud Run Actions", v.buildActions())
				v.menuOpen = true
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			return v.refresh()

		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()

		case key.Matches(msg, v.keys.Edit):
			if v.details != nil {
				return func() tea.Msg {
					return CloudRunEditRequestMsg{
						ProjectID:   v.projectID,
						ServiceName: v.serviceName,
						FullName:    v.fullName,
					}
				}
			}
			return nil

		case key.Matches(msg, v.keys.TrafficSplit):
			// Only allow traffic editing from the Revisions tab
			if v.tabs.ActiveTab().ID == runTabIDRevisions && v.details != nil && len(v.revisions) > 0 {
				return v.openTrafficDialog()
			}
			return nil
		}

		// Route remaining keys based on currently focused region
		switch v.focusMgr.ActiveType() {
		case focus.RegionTabs:
			if tabs.HandleKey(msg) {
				return v.tabs.Update(msg)
			}

		case focus.RegionViewport:
			// Route observability keys (time range 1-5, severity, auto-refresh) only
			// when viewport is focused, so numeric tab switching still works from tabs region
			if v.tabs.ActiveTab().ID == runTabIDObservability && v.observability != nil {
				cmd, handled := v.observability.Update(msg)
				if handled {
					v.updateViewportContent()
					return cmd
				}
			}

			activeIdx := v.tabs.ActiveIndex()
			if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
				var cmd tea.Cmd
				v.tabViewports[activeIdx], cmd = v.tabViewports[activeIdx].Update(msg)
				return cmd
			}
		}
	}

	return nil
}

func (v *CloudRunServiceDetailsView) buildActions() []actionmenu.Action {
	actions := []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
		{Key: 'e', Label: "Edit Service", Enabled: true},
	}

	// Traffic editing is only available on the Revisions tab
	if v.tabs.ActiveTab().ID == runTabIDRevisions && len(v.revisions) > 0 {
		actions = append(actions, actionmenu.Action{Key: 't', Label: "Edit Traffic Split", Enabled: true})
	}

	actions = append(actions, actionmenu.Action{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true})
	return actions
}

func (v *CloudRunServiceDetailsView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'r':
		return v.refresh()
	case 't':
		if v.tabs.ActiveTab().ID == runTabIDRevisions && v.details != nil && len(v.revisions) > 0 {
			return v.openTrafficDialog()
		}
	case 'e':
		if v.details != nil {
			return func() tea.Msg {
				return CloudRunEditRequestMsg{
					ProjectID:   v.projectID,
					ServiceName: v.serviceName,
					FullName:    v.fullName,
				}
			}
		}
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

func (v *CloudRunServiceDetailsView) refresh() tea.Cmd {
	v.detailsLoading = true
	v.revisionsLoading = true
	v.detailsErr = nil
	v.revisionsErr = nil
	cmds := []tea.Cmd{v.spinner.Tick, v.loadDetails(), v.loadRevisions()}
	// Also refresh observability data when on that tab
	if v.observability != nil && v.tabs.ActiveTab().ID == runTabIDObservability {
		cmds = append(cmds, v.observability.Init())
	}
	return tea.Batch(cmds...)
}

func (v *CloudRunServiceDetailsView) initiateDelete() tea.Cmd {
	if v.details == nil {
		return nil
	}

	detailLines := []string{
		fmt.Sprintf("Region: %s", v.details.Region),
		fmt.Sprintf("Status: %s", v.details.Status),
	}

	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Cloud Run Service", v.serviceName, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

func (v *CloudRunServiceDetailsView) openTrafficDialog() tea.Cmd {
	v.trafficDialog = newTrafficSplitDialog(v.revisions, v.details.Traffic)
	v.showTrafficDialog = true
	return nil
}

// View renders the Cloud Run service details view
func (v *CloudRunServiceDetailsView) View() string {
	// Show loading until at least details are loaded
	if v.detailsLoading && v.details == nil {
		return renderLoading(v.spinner, "Loading Cloud Run service details...")
	}

	if v.detailsErr != nil && v.details == nil {
		return "\n" + components.RenderError(v.detailsErr)
	}

	if v.details == nil {
		return renderLoading(v.spinner, "No Cloud Run service details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return renderLoading(v.spinner, "Initializing view...")
	}

	// Render tab bar with focus accent
	tabBar := focus.RenderAccent("  "+v.tabs.View(), v.focusMgr.IsActive(runRegionIDTabs))

	// Get active tab viewport with focus accent
	activeIdx := v.tabs.ActiveIndex()
	var viewportContent string
	var scrollPercent float64
	if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
		viewportContent = focus.RenderAccent(v.tabViewports[activeIdx].View(), v.focusMgr.IsActive(runRegionIDViewport))
		scrollPercent = v.tabViewports[activeIdx].ScrollPercent()
	}

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", scrollPercent*100))

	helpText := v.buildHelpText()
	help := helpStyle.Render(helpText) + " " + scrollInfo

	mainContent := tabBar + "\n" + viewportContent + help

	// Render overlays in z-order: traffic dialog > delete confirm > action menu
	if v.showTrafficDialog && v.trafficDialog != nil {
		return v.renderWithOverlay(mainContent, v.trafficDialog.View())
	}

	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.renderWithOverlay(mainContent, v.deleteConfirm.View())
	}

	if v.menuOpen && v.actionMenu != nil {
		return v.renderWithOverlay(mainContent, v.actionMenu.View())
	}

	return mainContent
}

func (v *CloudRunServiceDetailsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// SetContext updates the view with shared program context
func (v *CloudRunServiceDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// SetError allows the app to propagate async errors back to the view
func (v *CloudRunServiceDetailsView) SetError(err error) {
	v.detailsErr = err
}

// IsMenuOpen returns true if the action menu, delete confirm, or traffic dialog is open
func (v *CloudRunServiceDetailsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm || v.showTrafficDialog
}

// GetServiceName returns the service name for breadcrumbs
func (v *CloudRunServiceDetailsView) GetServiceName() string {
	return v.serviceName
}

// GetCloudRunClient returns the Cloud Run client for reuse
func (v *CloudRunServiceDetailsView) GetCloudRunClient() *gcp.CloudRunClient {
	return v.runClient
}

// HasTextInputFocused returns true when text input in delete confirm or traffic dialog is active
func (v *CloudRunServiceDetailsView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	if v.showTrafficDialog && v.trafficDialog != nil {
		return true // Traffic dialog always has text inputs focused
	}
	return false
}

func (v *CloudRunServiceDetailsView) applySize(width, height int) {
	viewportHeight := height - runDetailsViewportReservedLines
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	viewportWidth := width - 1 // Reserve 1 char for focus accent bar
	if viewportWidth < 1 {
		viewportWidth = 1
	}

	if !v.ready {
		for i := range v.tabViewports {
			v.tabViewports[i] = viewport.New(viewportWidth, viewportHeight)
			v.tabViewports[i].Style = lipgloss.NewStyle().Padding(0, 2)
		}
		v.ready = true
	} else {
		for i := range v.tabViewports {
			v.tabViewports[i].Width = viewportWidth
			v.tabViewports[i].Height = viewportHeight
		}
	}

	if v.observability != nil {
		v.observability.width = viewportWidth
		v.observability.resizeCharts()
	}

	if v.details != nil {
		v.updateViewportContent()
	}
}

func (v *CloudRunServiceDetailsView) updateViewportContent() {
	if !v.ready {
		return
	}

	activeIdx := v.tabs.ActiveIndex()
	if activeIdx < 0 || activeIdx >= len(v.tabViewports) {
		return
	}

	var content string
	switch v.tabs.ActiveTab().ID {
	case runTabIDDetails:
		content = v.renderDetailsTab()
	case runTabIDRevisions:
		content = v.renderRevisionsTab()
	case runTabIDYAML:
		content = v.renderYAMLTab()
	case runTabIDObservability:
		if v.observability != nil {
			content = v.observability.View()
		}
	default:
		content = v.renderDetailsTab()
	}

	v.tabViewports[activeIdx].SetContent(content)
}

func (v *CloudRunServiceDetailsView) renderDetailsTab() string {
	if v.details == nil {
		return ""
	}

	d := v.details
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Cloud Run Service: %s", d.Name)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Service Information
	b.WriteString(sectionStyle.Render("Service Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Region", d.Region))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "URL", defaultIfEmpty(d.URL, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Description", defaultIfEmpty(d.Description, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Ingress", defaultIfEmpty(d.Ingress, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Service Account", defaultIfEmpty(d.ServiceAccount, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Creator", defaultIfEmpty(d.Creator, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Last Modifier", defaultIfEmpty(d.LastModifier, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Updated", timeutil.FormatTimestamp(d.UpdatedAt)))

	// Status with color
	var statusStr string
	switch d.Status {
	case "Ready":
		statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")).Render("Ready")
	case "Failed":
		statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Render("Failed")
	default:
		statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04")).Render(d.Status)
	}
	b.WriteString(labelStyle.Render("Status:") + " " + statusStr + "\n")
	if d.StatusMessage != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Status Message", d.StatusMessage))
	}
	b.WriteString("\n")

	// Container configuration
	b.WriteString(sectionStyle.Render("Container"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Image", defaultIfEmpty(d.ContainerImage, "—")))
	if d.ContainerPort > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Port", strconv.FormatInt(d.ContainerPort, 10)))
	}
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "CPU", defaultIfEmpty(d.CPU, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Memory", defaultIfEmpty(d.Memory, "—")))
	b.WriteString("\n")

	// Scaling
	b.WriteString(sectionStyle.Render("Scaling"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Min Instances", strconv.FormatInt(d.MinInstances, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Max Instances", strconv.FormatInt(d.MaxInstances, 10)))
	if d.Concurrency > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Concurrency", strconv.FormatInt(d.Concurrency, 10)))
	}
	if d.TimeoutSeconds > 0 {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Timeout", fmt.Sprintf("%ds", d.TimeoutSeconds)))
	}
	b.WriteString("\n")

	// Environment Variables (sorted)
	if len(d.EnvVars) > 0 {
		b.WriteString(sectionStyle.Render("Environment Variables"))
		b.WriteString("\n")

		keys := make([]string, 0, len(d.EnvVars))
		for k := range d.EnvVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, k, d.EnvVars[k]))
		}
		b.WriteString("\n")
	}

	// Labels (sorted)
	if len(d.Labels) > 0 {
		b.WriteString(sectionStyle.Render("Labels"))
		b.WriteString("\n")

		keys := make([]string, 0, len(d.Labels))
		for k := range d.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, k, d.Labels[k]))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (v *CloudRunServiceDetailsView) renderRevisionsTab() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	trafficStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	if v.revisionsLoading {
		b.WriteString(fmt.Sprintf("\n  %s Loading revisions...\n", v.spinner.View()))
		return b.String()
	}

	if v.revisionsErr != nil {
		b.WriteString("\n  " + errorStyle.Render(fmt.Sprintf("Error: %v", v.revisionsErr)) + "\n")
		return b.String()
	}

	if len(v.revisions) == 0 {
		b.WriteString("\n  " + mutedStyle.Render("No revisions found") + "\n")
		return b.String()
	}

	for _, rev := range v.revisions {
		// Status indicator
		var statusIcon string
		switch rev.Status {
		case "Ready":
			statusIcon = symbols.StatusRunning()
		case "Failed":
			statusIcon = symbols.StatusStopped()
		default:
			statusIcon = symbols.StatusTransitioning()
		}

		// Traffic percentage (highlighted if non-zero)
		trafficStr := ""
		if rev.TrafficPercent > 0 {
			trafficStr = trafficStyle.Render(fmt.Sprintf(" %d%%", rev.TrafficPercent))
		}

		b.WriteString(fmt.Sprintf("  %s %s%s\n",
			statusIcon,
			nameStyle.Render(rev.ShortName),
			trafficStr))

		// Image and creation time on a second line
		b.WriteString(fmt.Sprintf("    %s  •  %s\n",
			mutedStyle.Render(rev.ContainerImage),
			mutedStyle.Render(timeutil.FormatTimestamp(rev.CreatedAt))))
		b.WriteString("\n")
	}

	b.WriteString("  " + helpStyle.Render("t: edit traffic split") + "\n")

	return b.String()
}

func (v *CloudRunServiceDetailsView) renderYAMLTab() string {
	if v.details == nil || v.details.RawYAML == "" {
		return "\n  No YAML data available.\n"
	}

	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	return mutedStyle.Render(v.details.RawYAML)
}

func (v *CloudRunServiceDetailsView) buildHelpText() string {
	bindings := focus.HelpForRegion(v.focusMgr.ActiveType(), "")
	helpStr := focus.FormatHelp(bindings)
	badge := focus.FormatRegionBadge(v.focusMgr.Active())

	var actionHints string
	switch v.tabs.ActiveTab().ID {
	case runTabIDObservability:
		actionHints = "1-5: range • a: auto-refresh • I/W/E: logs • r: refresh"
	case runTabIDRevisions:
		actionHints = ".: actions • D: delete"
		if len(v.revisions) > 0 {
			actionHints += " • t: traffic"
		}
	default:
		actionHints = ".: actions • D: delete"
	}

	if badge != "" {
		return "\n  " + badge + " • " + helpStr + " • " + actionHints
	}
	return "\n  " + helpStr + " • " + actionHints
}

// UpdateRegions calculates clickable regions for tabs and viewport
func (v *CloudRunServiceDetailsView) UpdateRegions(offsetX, offsetY int) {
	v.regionMgr.Clear()

	if !v.ready || (v.detailsLoading && v.details == nil) {
		return
	}

	y := offsetY

	// Tab bar region
	tabHeight := 1
	v.regionMgr.Add(runRegionIDTabs, mouse.Rect{
		X:      offsetX,
		Y:      y,
		Width:  v.width,
		Height: tabHeight,
	}, nil)
	y += tabHeight + 1

	// Viewport region
	viewportHeight := v.height - (y - offsetY)
	if viewportHeight > 0 {
		v.regionMgr.Add(runRegionIDViewport, mouse.Rect{
			X:      offsetX,
			Y:      y,
			Width:  v.width,
			Height: viewportHeight,
		}, nil)
	}
}

// GetRegions returns the current clickable regions
func (v *CloudRunServiceDetailsView) GetRegions() []mouse.Region {
	return v.regionMgr.GetRegions()
}

// HandleRegionClick processes a click on a specific region
func (v *CloudRunServiceDetailsView) HandleRegionClick(regionID string) tea.Cmd {
	v.focusMgr.SetActive(regionID)
	return nil
}

// trafficSplitEntry represents one row in the traffic split dialog.
// Each entry maps to a traffic target (either a specific revision or "latest").
type trafficSplitEntry struct {
	Label string // Display name (revision short name or "(latest)")
	Tag   string // Preserved from original traffic config
	Type  string // "LATEST" or "REVISION"
	// For REVISION entries, RevisionName is the short name.
	// For LATEST entries, RevisionName is empty (API doesn't need it).
	RevisionName string
}

// trafficSplitDialog is a lightweight inline dialog for editing traffic split percentages
type trafficSplitDialog struct {
	entries   []trafficSplitEntry
	inputs    []textinput.Model
	focused   int
	err       string
	submitted bool
	canceled  bool
	width     int
}

func newTrafficSplitDialog(revisions []gcp.CloudRunRevision, currentTraffic []gcp.CloudRunTrafficTarget) *trafficSplitDialog {
	// Build entries: one per current traffic target, plus any revisions without traffic.
	// This preserves LATEST allocations, tags, and types through the round-trip.

	// Index existing traffic by revision name for lookup
	trafficByName := make(map[string]gcp.CloudRunTrafficTarget)
	var hasLatest bool
	var latestTarget gcp.CloudRunTrafficTarget
	for _, t := range currentTraffic {
		if t.Type == "LATEST" {
			hasLatest = true
			latestTarget = t
		} else {
			trafficByName[t.RevisionName] = t
		}
	}

	var entries []trafficSplitEntry
	var initialValues []int64

	// Add "(latest)" row first if one exists in current config
	if hasLatest {
		entries = append(entries, trafficSplitEntry{
			Label: "(latest)",
			Tag:   latestTarget.Tag,
			Type:  "LATEST",
		})
		initialValues = append(initialValues, latestTarget.Percent)
	}

	// Add one row per revision
	for _, rev := range revisions {
		entry := trafficSplitEntry{
			Label:        rev.ShortName,
			Type:         "REVISION",
			RevisionName: rev.ShortName,
		}
		var pct int64
		if t, ok := trafficByName[rev.ShortName]; ok {
			pct = t.Percent
			entry.Tag = t.Tag
		}
		entries = append(entries, entry)
		initialValues = append(initialValues, pct)
	}

	inputs := make([]textinput.Model, len(entries))
	for i := range entries {
		ti := textinput.New()
		ti.Placeholder = "0"
		ti.CharLimit = 3
		ti.Width = 5
		ti.SetValue(strconv.FormatInt(initialValues[i], 10))
		if i == 0 {
			ti.Focus()
		}
		inputs[i] = ti
	}

	return &trafficSplitDialog{
		entries: entries,
		inputs:  inputs,
		focused: 0,
		width:   50,
	}
}

// Update handles key events for the traffic dialog. Returns (cmd, done).
func (d *trafficSplitDialog) Update(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		d.canceled = true
		return nil, true

	case tea.KeyEnter:
		// Validate and submit
		if err := d.validate(); err != "" {
			d.err = err
			return nil, false
		}
		d.submitted = true
		return nil, true

	case tea.KeyTab, tea.KeyDown:
		d.inputs[d.focused].Blur()
		d.focused = (d.focused + 1) % len(d.inputs)
		d.inputs[d.focused].Focus()
		return nil, false

	case tea.KeyShiftTab, tea.KeyUp:
		d.inputs[d.focused].Blur()
		d.focused = (d.focused - 1 + len(d.inputs)) % len(d.inputs)
		d.inputs[d.focused].Focus()
		return nil, false
	}

	// Pass to focused input
	var cmd tea.Cmd
	d.inputs[d.focused], cmd = d.inputs[d.focused].Update(msg)
	d.err = "" // Clear error on input change
	return cmd, false
}

func (d *trafficSplitDialog) validate() string {
	var total int64
	for i := range d.inputs {
		val := strings.TrimSpace(d.inputs[i].Value())
		if val == "" {
			val = "0"
		}
		pct, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Sprintf("Invalid percentage for %s", d.entries[i].Label)
		}
		if pct < 0 || pct > 100 {
			return fmt.Sprintf("Percentage must be 0-100 for %s", d.entries[i].Label)
		}
		total += pct
	}
	if total != 100 {
		return fmt.Sprintf("Percentages must sum to 100 (currently %d)", total)
	}
	return ""
}

// Result returns the traffic targets if submitted, or nil if canceled.
// Preserves Type, Tag, and RevisionName from the original traffic config.
func (d *trafficSplitDialog) Result() []gcp.CloudRunTrafficTarget {
	if !d.submitted {
		return nil
	}

	var targets []gcp.CloudRunTrafficTarget
	for i := range d.inputs {
		val := strings.TrimSpace(d.inputs[i].Value())
		if val == "" {
			val = "0"
		}
		pct, parseErr := strconv.ParseInt(val, 10, 64)
		if parseErr != nil {
			continue // validated before submit, skip on unexpected error
		}
		if pct == 0 && d.entries[i].Tag == "" {
			continue // Skip untagged zero-traffic entries; tagged 0% routes provide a URL
		}
		targets = append(targets, gcp.CloudRunTrafficTarget{
			RevisionName: d.entries[i].RevisionName,
			Percent:      pct,
			Tag:          d.entries[i].Tag,
			Type:         d.entries[i].Type,
		})
	}
	return targets
}

// View renders the traffic split dialog
func (d *trafficSplitDialog) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Width(30)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4285F4")).
		Padding(1, 2).
		Width(d.width)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Edit Traffic Split"))
	b.WriteString("\n\n")

	for i, entry := range d.entries {
		name := entry.Label
		if len(name) > 28 {
			name = name[:28] + "…"
		}
		b.WriteString(labelStyle.Render(name) + " " + d.inputs[i].View() + " %\n")
	}

	if d.err != "" {
		b.WriteString("\n" + errorStyle.Render(d.err))
	}

	b.WriteString("\n" + helpStyle.Render("Tab/↓: next • Shift+Tab/↑: prev • Enter: submit • Esc: cancel"))

	return borderStyle.Render(b.String())
}
