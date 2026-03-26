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
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/links"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/focus"
	"github.com/slayer/gcon/internal/ui/mouse"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// Tab IDs for network details view
const (
	networkTabIDDetails = "details"
	networkTabIDSubnets = "subnets"
)

// Focus region IDs for network details view
const (
	networkRegionIDTabs     = "tabs"
	networkRegionIDLinks    = "links"
	networkRegionIDViewport = "viewport"
)

// Lines reserved for tab bar + separator + help text
const networkDetailsViewportReservedLines = 5

// Internal messages for async data loading
type networkDetailsLoadedMsg struct {
	details *gcp.NetworkDetails
}

type networkDetailsErrorMsg struct {
	err error
}

type networkSubnetsLoadedMsg struct {
	subnets []gcp.Subnet
}

type networkSubnetsErrorMsg struct {
	err error
}

// NetworkDetailsView displays comprehensive network information with tabs
type NetworkDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	networkName   string
	ctx           *context.ProgramContext

	// Data
	details *gcp.NetworkDetails
	subnets []gcp.Subnet

	// UI state
	spinner        spinner.Model
	loading        bool
	subnetsLoading bool
	err            error
	subnetsErr     error
	width          int
	height         int
	ready          bool

	// Tab navigation
	tabs         *tabs.Tabs
	tabViewports []viewport.Model // Separate viewport per tab to preserve scroll

	// Navigable subnet links in Subnets tab
	subnetLinks *links.Links

	// Focus management
	focusMgr  *focus.Manager
	regionMgr *mouse.RegionManager

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	keys networkDetailsKeyMap
}

type networkDetailsKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Refresh    key.Binding
	ActionMenu key.Binding
}

func defaultNetworkDetailsKeyMap() networkDetailsKeyMap {
	return networkDetailsKeyMap{
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
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
	}
}

// NewNetworkDetailsView creates a new network details view
func NewNetworkDetailsView(projectID, networkName string, computeClient *gcp.ComputeClient) *NetworkDetailsView {
	s := components.NewGCPSpinner()

	tabsComponent := tabs.New([]tabs.Tab{
		{ID: networkTabIDDetails, Label: "Details"},
		{ID: networkTabIDSubnets, Label: "Subnets"},
	})

	// Links region starts disabled until subnets load
	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewRegion(networkRegionIDTabs, focus.RegionTabs, "Tabs"),
		focus.NewDisabledRegion(networkRegionIDLinks, focus.RegionLinks, "Subnets"),
		focus.NewRegion(networkRegionIDViewport, focus.RegionViewport, "Content"),
	})

	return &NetworkDetailsView{
		computeClient:  computeClient,
		projectID:      projectID,
		networkName:    networkName,
		spinner:        s,
		loading:        true,
		subnetsLoading: true,
		keys:           defaultNetworkDetailsKeyMap(),
		tabs:           tabsComponent,
		tabViewports:   make([]viewport.Model, 2),
		subnetLinks:    links.New(),
		focusMgr:       fm,
		regionMgr:      mouse.NewRegionManager(),
	}
}

// Init starts loading network details and subnets in parallel
func (v *NetworkDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
		v.loadSubnets(),
	)
}

func (v *NetworkDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return networkDetailsErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		details, err := v.computeClient.GetNetworkDetails(gocontext.Background(), v.projectID, v.networkName)
		if err != nil {
			return networkDetailsErrorMsg{err: err}
		}
		return networkDetailsLoadedMsg{details: details}
	}
}

func (v *NetworkDetailsView) loadSubnets() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return networkSubnetsErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		subnets, err := v.computeClient.ListSubnetsByNetwork(gocontext.Background(), v.projectID, v.networkName)
		if err != nil {
			return networkSubnetsErrorMsg{err: err}
		}
		return networkSubnetsLoadedMsg{subnets: subnets}
	}
}

// Update handles messages for the network details view
//
//nolint:gocognit // Bubble Tea Update pattern - complexity expected
func (v *NetworkDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case networkDetailsLoadedMsg:
		v.loading = false
		v.details = msg.details
		v.updateViewportContent()
		return nil

	case networkDetailsErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case networkSubnetsLoadedMsg:
		v.subnetsLoading = false
		v.subnets = msg.subnets
		v.populateSubnetLinks()
		v.updateViewportContent()
		return nil

	case networkSubnetsErrorMsg:
		v.subnetsLoading = false
		v.subnetsErr = msg.err
		v.updateViewportContent()
		return nil

	case spinner.TickMsg:
		if v.loading || v.subnetsLoading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		return v.executeAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case tabs.TabChangedMsg:
		v.updateViewportContent()
		// Toggle links region based on active tab
		if v.tabs.ActiveTab().ID == networkTabIDSubnets && len(v.subnets) > 0 {
			v.focusMgr.EnableRegion(networkRegionIDLinks)
		} else {
			v.focusMgr.DisableRegion(networkRegionIDLinks)
		}
		return nil

	case links.LinkSelectedMsg:
		// Navigate to subnet details when a subnet link is selected
		if msg.Link.Type == "subnet" {
			if subnet, ok := msg.Link.Data.(gcp.Subnet); ok {
				return func() tea.Msg {
					return SubnetSelectedMsg{
						SubnetName: subnet.Name,
						Region:     subnet.Region,
					}
				}
			}
		}
		return nil

	case focus.FocusChangedMsg:
		v.updateViewportContent()
		return nil

	case tea.KeyMsg:
		// Route to action menu when open
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
			if tabs.HandleKey(msg) {
				return v.tabs.Update(msg)
			}

		case focus.RegionLinks:
			// Subnet links only in Subnets tab
			if v.tabs.ActiveTab().ID == networkTabIDSubnets && v.subnetLinks.HasItems() {
				if links.HandleKey(msg) {
					cmd := v.subnetLinks.Update(msg)
					v.updateViewportContent()
					return cmd
				}
			}

		case focus.RegionViewport:
			activeIdx := v.tabs.ActiveIndex()
			if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
				var cmd tea.Cmd
				v.tabViewports[activeIdx], cmd = v.tabViewports[activeIdx].Update(msg)
				return cmd
			}
		}

		// View-specific action keys (work regardless of focus)
		switch {
		case key.Matches(msg, v.keys.ActionMenu):
			if v.details != nil {
				v.actionMenu = actionmenu.New("Network Actions", v.buildActions())
				v.menuOpen = true
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.subnetsLoading = true
			v.err = nil
			v.subnetsErr = nil
			return tea.Batch(v.spinner.Tick, v.loadDetails(), v.loadSubnets())
		}
	}

	return nil
}

func (v *NetworkDetailsView) buildActions() []actionmenu.Action {
	return []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
	}
}

func (v *NetworkDetailsView) executeAction(actionKey rune) tea.Cmd {
	if actionKey == 'r' {
		v.loading = true
		v.subnetsLoading = true
		v.err = nil
		v.subnetsErr = nil
		return tea.Batch(v.spinner.Tick, v.loadDetails(), v.loadSubnets())
	}
	return nil
}

// View renders the network details view
func (v *NetworkDetailsView) View() string {
	if v.loading && v.details == nil {
		return renderLoading(v.spinner, "Loading network details...")
	}

	if v.err != nil && v.details == nil {
		return renderLoading(v.spinner, fmt.Sprintf("Error: %v\n  Press 'esc' to go back", v.err))
	}

	if v.details == nil {
		return renderLoading(v.spinner, "No network details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return renderLoading(v.spinner, "Initializing view...")
	}

	// Render tab bar with focus accent
	tabBar := focus.RenderAccent("  "+v.tabs.View(), v.focusMgr.IsActive(networkRegionIDTabs))

	// Get active tab viewport with focus accent
	activeIdx := v.tabs.ActiveIndex()
	var viewportContent string
	var scrollPercent float64
	if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
		viewportContent = focus.RenderAccent(v.tabViewports[activeIdx].View(), v.focusMgr.IsActive(networkRegionIDViewport))
		scrollPercent = v.tabViewports[activeIdx].ScrollPercent()
	}

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", scrollPercent*100))

	helpText := v.buildHelpText()
	help := helpStyle.Render(helpText) + " " + scrollInfo

	mainContent := tabBar + "\n" + viewportContent + help

	// Overlay action menu if open
	if v.menuOpen && v.actionMenu != nil {
		return v.renderWithOverlay(mainContent, v.actionMenu.View())
	}

	return mainContent
}

func (v *NetworkDetailsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// SetContext updates the view with shared program context
func (v *NetworkDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// IsMenuOpen returns true if the action menu is open
func (v *NetworkDetailsView) IsMenuOpen() bool {
	return v.menuOpen
}

// GetNetworkName returns the network name for breadcrumbs
func (v *NetworkDetailsView) GetNetworkName() string {
	return v.networkName
}

// GetComputeClient returns the compute client for reuse
func (v *NetworkDetailsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

func (v *NetworkDetailsView) applySize(width, height int) {
	viewportHeight := height - networkDetailsViewportReservedLines
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Reserve 1 char for focus accent bar
	viewportWidth := width - 1
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

	if v.details != nil {
		v.updateViewportContent()
	}
}

func (v *NetworkDetailsView) updateViewportContent() {
	if !v.ready {
		return
	}

	activeIdx := v.tabs.ActiveIndex()
	if activeIdx < 0 || activeIdx >= len(v.tabViewports) {
		return
	}

	// Update links focus state
	v.subnetLinks.SetRegionFocused(v.focusMgr.IsActive(networkRegionIDLinks))

	var content string
	switch v.tabs.ActiveTab().ID {
	case networkTabIDDetails:
		v.focusMgr.DisableRegion(networkRegionIDLinks)
		content = v.renderDetailsTab()
	case networkTabIDSubnets:
		content = v.renderSubnetsTab()
	default:
		content = v.renderDetailsTab()
	}

	v.tabViewports[activeIdx].SetContent(content)
}

func (v *NetworkDetailsView) populateSubnetLinks() {
	if len(v.subnets) == 0 {
		v.subnetLinks.SetItems(nil)
		v.focusMgr.DisableRegion(networkRegionIDLinks)
		return
	}

	items := make([]links.Link, len(v.subnets))
	for i, subnet := range v.subnets {
		items[i] = links.Link{
			ID:    subnet.Name,
			Label: subnet.Name,
			Type:  "subnet",
			Data:  subnet,
		}
	}
	v.subnetLinks.SetItems(items)

	// Enable links region when on Subnets tab
	if v.tabs.ActiveTab().ID == networkTabIDSubnets {
		v.focusMgr.EnableRegion(networkRegionIDLinks)
	}
}

// renderDetailsTab generates the Details tab content
func (v *NetworkDetailsView) renderDetailsTab() string {
	if v.details == nil {
		return ""
	}

	d := v.details
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Network: %s", d.Name)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Basic Information
	b.WriteString(sectionStyle.Render("Basic Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Network ID", strconv.FormatUint(d.ID, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Description", defaultIfEmpty(d.Description, "None")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	b.WriteString("\n")

	// Configuration
	b.WriteString(sectionStyle.Render("Configuration"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Subnet Mode", formatSubnetMode(d.AutoCreateSubnetworks)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Routing Mode", defaultIfEmpty(d.RoutingMode, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "MTU", formatMTU(d.Mtu)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Gateway IPv4", defaultIfEmpty(d.GatewayIPv4, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Subnets", strconv.Itoa(d.SubnetworkCount)))
	b.WriteString("\n")

	// IPv6 section (only if enabled)
	if d.EnableUlaInternalIpv6 {
		b.WriteString(sectionStyle.Render("IPv6"))
		b.WriteString("\n")
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "ULA Internal IPv6", "Enabled"))
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Internal IPv6 Range", defaultIfEmpty(d.InternalIpv6Range, "—")))
		b.WriteString("\n")
	}

	// Peerings section (only if present)
	if len(d.Peerings) > 0 {
		b.WriteString(sectionStyle.Render(fmt.Sprintf("Peerings (%d)", len(d.Peerings))))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  %-30s %-30s %-10s\n", "Name", "Peer Network", "State"))
		b.WriteString("  " + strings.Repeat("─", 72) + "\n")
		for _, p := range d.Peerings {
			b.WriteString(fmt.Sprintf("  %-30s %-30s %-10s\n",
				truncate(p.Name, 30),
				truncate(p.Network, 30),
				p.State))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderSubnetsTab generates the Subnets tab content
func (v *NetworkDetailsView) renderSubnetsTab() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))

	// Header
	if v.details != nil {
		b.WriteString(titleStyle.Render(fmt.Sprintf("Network: %s", v.details.Name)))
		b.WriteString("\n")
		b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
		b.WriteString("\n\n")
	}

	// Loading state
	if v.subnetsLoading {
		b.WriteString(fmt.Sprintf("  %s Loading subnets...\n", v.spinner.View()))
		return b.String()
	}

	// Error state
	if v.subnetsErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error loading subnets: %s", v.subnetsErr.Error())))
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("  Press 'r' to retry"))
		b.WriteString("\n")
		return b.String()
	}

	// Empty state
	if len(v.subnets) == 0 {
		b.WriteString(mutedStyle.Render("  No subnets found"))
		b.WriteString("\n")
		return b.String()
	}

	// Subnet table header
	header := fmt.Sprintf("%-25s %-18s %-18s %-12s %-8s %-8s",
		"Name", "Region", "CIDR Range", "Purpose", "Google", "Logs")
	b.WriteString(v.subnetLinks.RenderHeader(header))
	b.WriteString("\n")
	b.WriteString(v.subnetLinks.RenderDivider(91))
	b.WriteString("\n")

	// Render each subnet row with link highlighting
	for i, subnet := range v.subnets {
		purpose := formatSubnetPurpose(subnet.Purpose)
		googleAccess := formatYesNo(subnet.PrivateIPGoogleAccess)
		flowLogs := formatYesNo(subnet.EnableFlowLogs)

		row := fmt.Sprintf("%-25s %-18s %-18s %-12s %-8s %-8s",
			truncate(subnet.Name, 25),
			truncate(subnet.Region, 18),
			subnet.IPCidrRange,
			truncate(purpose, 12),
			googleAccess,
			flowLogs)
		b.WriteString(v.subnetLinks.RenderRow(i, row))
		b.WriteString("\n")
	}

	return b.String()
}

// buildHelpText generates context-sensitive help text
func (v *NetworkDetailsView) buildHelpText() string {
	bindings := focus.HelpForRegion(v.focusMgr.ActiveType(), v.getRegionLabel())
	helpStr := focus.FormatHelp(bindings)
	badge := focus.FormatRegionBadge(v.focusMgr.Active())
	if badge != "" {
		return "\n  " + badge + " • " + helpStr + " • .: actions"
	}
	return "\n  " + helpStr + " • .: actions"
}

func (v *NetworkDetailsView) getRegionLabel() string {
	if v.focusMgr.ActiveType() == focus.RegionLinks {
		return "subnet"
	}
	return ""
}

// UpdateRegions calculates clickable regions for tabs, links, and viewport
func (v *NetworkDetailsView) UpdateRegions(offsetX, offsetY int) {
	v.regionMgr.Clear()

	if !v.ready || v.loading {
		return
	}

	y := offsetY

	// Tab bar region
	tabHeight := 1
	v.regionMgr.Add(networkRegionIDTabs, mouse.Rect{
		X:      offsetX,
		Y:      y,
		Width:  v.width,
		Height: tabHeight,
	}, nil)
	y += tabHeight + 1

	// Links region (only in Subnets tab, if subnets exist)
	if v.tabs.ActiveTab().ID == networkTabIDSubnets && v.subnetLinks != nil && v.subnetLinks.Count() > 0 {
		linksHeight := v.subnetLinks.Count()
		v.regionMgr.Add(networkRegionIDLinks, mouse.Rect{
			X:      offsetX,
			Y:      y,
			Width:  v.width,
			Height: linksHeight,
		}, nil)
		y += linksHeight
	}

	// Viewport region
	viewportHeight := v.height - (y - offsetY)
	if viewportHeight > 0 {
		v.regionMgr.Add(networkRegionIDViewport, mouse.Rect{
			X:      offsetX,
			Y:      y,
			Width:  v.width,
			Height: viewportHeight,
		}, nil)
	}
}

// GetRegions returns the current clickable regions
func (v *NetworkDetailsView) GetRegions() []mouse.Region {
	return v.regionMgr.GetRegions()
}

// HandleRegionClick processes a click on a specific region
func (v *NetworkDetailsView) HandleRegionClick(regionID string) tea.Cmd {
	v.focusMgr.SetActive(regionID)
	return nil
}

// Helper functions

func formatSubnetMode(autoCreate bool) string {
	if autoCreate {
		return "Auto"
	}
	return "Custom"
}

func formatMTU(mtu int64) string {
	if mtu == 0 {
		return "Default (1460)"
	}
	return strconv.FormatInt(mtu, 10)
}

func formatSubnetPurpose(purpose string) string {
	switch purpose {
	case "PRIVATE":
		return "Private"
	case "REGIONAL_MANAGED_PROXY":
		return "Managed Proxy"
	case "GLOBAL_MANAGED_PROXY":
		return "Global Proxy"
	case "PRIVATE_SERVICE_CONNECT":
		return "PSC"
	case "INTERNAL_HTTPS_LOAD_BALANCER":
		return "Internal LB"
	default:
		return defaultIfEmpty(purpose, "—")
	}
}

func formatYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
