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
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/links"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/focus"
	"github.com/slayer/gcon/internal/ui/mouse"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// Tab IDs for firewall details view
const (
	firewallTabIDDetails = "details"
	firewallTabIDRules   = "rules"
)

// Focus region IDs for firewall details view
const (
	firewallRegionIDTabs     = "tabs"
	firewallRegionIDLinks    = "links"
	firewallRegionIDViewport = "viewport"
)

// Lines reserved for tab bar + separator + help text
const firewallDetailsViewportReservedLines = 5

// Internal messages for async data loading
type firewallDetailsLoadedMsg struct {
	details *gcp.FirewallRuleDetails
}

type firewallDetailsErrorMsg struct {
	err error
}

// FirewallDetailsView displays comprehensive firewall rule information with tabs
type FirewallDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	ruleName      string
	ctx           *context.ProgramContext

	// Data
	details *gcp.FirewallRuleDetails

	// UI state
	spinner spinner.Model
	loading bool
	err     error
	width   int
	height  int
	ready   bool

	// Tab navigation (Details / Rules)
	tabs         *tabs.Tabs
	tabViewports []viewport.Model // Separate viewport per tab to preserve scroll

	// Navigable link for network name
	networkLink *links.Links

	// Focus management
	focusMgr  *focus.Manager
	regionMgr *mouse.RegionManager

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool

	keys firewallDetailsKeyMap
}

type firewallDetailsKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Refresh    key.Binding
	Toggle     key.Binding // e - enable/disable
	Delete     key.Binding // D - delete
	ActionMenu key.Binding // .
}

func defaultFirewallDetailsKeyMap() firewallDetailsKeyMap {
	return firewallDetailsKeyMap{
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

// NewFirewallDetailsView creates a new firewall rule details view
func NewFirewallDetailsView(projectID, ruleName string, computeClient *gcp.ComputeClient) *FirewallDetailsView {
	s := components.NewGCPSpinner()

	tabsComponent := tabs.New([]tabs.Tab{
		{ID: firewallTabIDDetails, Label: "Details"},
		{ID: firewallTabIDRules, Label: "Rules"},
	})

	// Links region starts disabled until details load (network link)
	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewRegion(firewallRegionIDTabs, focus.RegionTabs, "Tabs"),
		focus.NewDisabledRegion(firewallRegionIDLinks, focus.RegionLinks, "Network"),
		focus.NewRegion(firewallRegionIDViewport, focus.RegionViewport, "Content"),
	})

	return &FirewallDetailsView{
		computeClient: computeClient,
		projectID:     projectID,
		ruleName:      ruleName,
		spinner:       s,
		loading:       true,
		keys:          defaultFirewallDetailsKeyMap(),
		tabs:          tabsComponent,
		tabViewports:  make([]viewport.Model, 2),
		networkLink:   links.New(),
		focusMgr:      fm,
		regionMgr:     mouse.NewRegionManager(),
	}
}

// Init starts loading firewall rule details
func (v *FirewallDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
	)
}

func (v *FirewallDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return firewallDetailsErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		details, err := v.computeClient.GetFirewallRuleDetails(gocontext.Background(), v.projectID, v.ruleName)
		if err != nil {
			return firewallDetailsErrorMsg{err: err}
		}
		return firewallDetailsLoadedMsg{details: details}
	}
}

// Update handles messages for the firewall details view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern - complexity expected
func (v *FirewallDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case firewallDetailsLoadedMsg:
		v.loading = false
		v.details = msg.details
		v.populateNetworkLink()
		v.updateViewportContent()
		return nil

	case firewallDetailsErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.loading {
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

	case confirm.TypeConfirmMsg:
		v.showDeleteConfirm = false
		if v.details != nil {
			return func() tea.Msg {
				return DeleteFirewallConfirmedMsg{
					RuleName: v.details.Name,
				}
			}
		}
		return nil

	case confirm.TypeCancelMsg:
		v.showDeleteConfirm = false
		return nil

	case tabs.TabChangedMsg:
		v.updateViewportContent()
		// Toggle links region based on active tab — links only on Details tab
		if v.tabs.ActiveTab().ID == firewallTabIDDetails && v.details != nil && v.details.Network != "" {
			v.focusMgr.EnableRegion(firewallRegionIDLinks)
		} else {
			v.focusMgr.DisableRegion(firewallRegionIDLinks)
		}
		return nil

	case links.LinkSelectedMsg:
		// Navigate to the network when link is clicked
		if v.details != nil {
			return func() tea.Msg {
				return NetworkSelectedMsg{
					Network: gcp.Network{Name: v.details.Network},
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

		// Route to delete confirm when shown
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		// Handle Tab/Shift+Tab for cycling between focus regions
		if focusMsg := v.focusMgr.HandleKey(msg); focusMsg != nil {
			v.updateViewportContent()
			return func() tea.Msg { return focusMsg }
		}

		// Action keys checked first so they work regardless of focused region
		switch {
		case key.Matches(msg, v.keys.ActionMenu):
			if v.details != nil {
				v.actionMenu = actionmenu.New("Firewall Actions", v.buildActions())
				v.menuOpen = true
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadDetails())

		case key.Matches(msg, v.keys.Toggle):
			return v.toggleFirewall()

		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()
		}

		// Route remaining keys based on currently focused region
		switch v.focusMgr.ActiveType() {
		case focus.RegionTabs:
			if tabs.HandleKey(msg) {
				return v.tabs.Update(msg)
			}

		case focus.RegionLinks:
			// Network link only on Details tab
			if v.tabs.ActiveTab().ID == firewallTabIDDetails && v.networkLink.HasItems() {
				if links.HandleKey(msg) {
					cmd := v.networkLink.Update(msg)
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
	}

	return nil
}

func (v *FirewallDetailsView) buildActions() []actionmenu.Action {
	actions := []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
	}

	if v.details != nil {
		if v.details.Disabled {
			actions = append(actions, actionmenu.Action{Key: 't', Label: "Enable", Enabled: true})
		} else {
			actions = append(actions, actionmenu.Action{Key: 't', Label: "Disable", Enabled: true})
		}
		actions = append(actions, actionmenu.Action{Key: 'D', Label: "Delete", Enabled: true})
	}

	return actions
}

func (v *FirewallDetailsView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'r':
		v.loading = true
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.loadDetails())
	case 't':
		return v.toggleFirewall()
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

func (v *FirewallDetailsView) initiateDelete() tea.Cmd {
	if v.details == nil {
		return nil
	}

	detailLines := []string{
		fmt.Sprintf("Direction: %s", v.details.Direction),
		fmt.Sprintf("Priority: %d", v.details.Priority),
		fmt.Sprintf("Action: %s", v.details.Action),
	}

	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Firewall Rule", v.details.Name, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

func (v *FirewallDetailsView) toggleFirewall() tea.Cmd {
	if v.details == nil {
		return nil
	}
	return func() tea.Msg {
		return ToggleFirewallMsg{RuleName: v.details.Name, Disable: !v.details.Disabled}
	}
}

// View renders the firewall details view
func (v *FirewallDetailsView) View() string {
	if v.loading && v.details == nil {
		return renderLoading(v.spinner, "Loading firewall rule details...")
	}

	if v.err != nil && v.details == nil {
		return renderLoading(v.spinner, fmt.Sprintf("Error: %v\n  Press 'esc' to go back", v.err))
	}

	if v.details == nil {
		return renderLoading(v.spinner, "No firewall rule details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return renderLoading(v.spinner, "Initializing view...")
	}

	// Render tab bar with focus accent
	tabBar := focus.RenderAccent("  "+v.tabs.View(), v.focusMgr.IsActive(firewallRegionIDTabs))

	// Get active tab viewport with focus accent
	activeIdx := v.tabs.ActiveIndex()
	var viewportContent string
	var scrollPercent float64
	if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
		viewportContent = focus.RenderAccent(v.tabViewports[activeIdx].View(), v.focusMgr.IsActive(firewallRegionIDViewport))
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

	// Overlay delete confirmation if shown
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.renderWithOverlay(mainContent, v.deleteConfirm.View())
	}

	return mainContent
}

func (v *FirewallDetailsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// SetContext updates the view with shared program context
func (v *FirewallDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// IsMenuOpen returns true if the action menu or delete confirm is open
func (v *FirewallDetailsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetRuleName returns the rule name for breadcrumbs
func (v *FirewallDetailsView) GetRuleName() string {
	return v.ruleName
}

// GetComputeClient returns the compute client for reuse
func (v *FirewallDetailsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// HasTextInputFocused returns true if the delete confirm input is active.
// Prevents global hotkeys (like 'q' for quit) from triggering while typing.
func (v *FirewallDetailsView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return false
}

func (v *FirewallDetailsView) applySize(width, height int) {
	viewportHeight := height - firewallDetailsViewportReservedLines
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

func (v *FirewallDetailsView) updateViewportContent() {
	if !v.ready {
		return
	}

	activeIdx := v.tabs.ActiveIndex()
	if activeIdx < 0 || activeIdx >= len(v.tabViewports) {
		return
	}

	// Update links focus state
	v.networkLink.SetRegionFocused(v.focusMgr.IsActive(firewallRegionIDLinks))

	var content string
	switch v.tabs.ActiveTab().ID {
	case firewallTabIDDetails:
		content = v.renderDetailsTab()
	case firewallTabIDRules:
		// Disable links on Rules tab — only available on Details tab
		v.focusMgr.DisableRegion(firewallRegionIDLinks)
		content = v.renderRulesTab()
	default:
		content = v.renderDetailsTab()
	}

	v.tabViewports[activeIdx].SetContent(content)
}

func (v *FirewallDetailsView) populateNetworkLink() {
	if v.details == nil || v.details.Network == "" {
		v.networkLink.SetItems(nil)
		v.focusMgr.DisableRegion(firewallRegionIDLinks)
		return
	}

	v.networkLink.SetItems([]links.Link{
		{ID: v.details.Network, Label: v.details.Network, Type: "network"},
	})

	// Enable links only on Details tab
	if v.tabs.ActiveTab().ID == firewallTabIDDetails {
		v.focusMgr.EnableRegion(firewallRegionIDLinks)
	}
}

// renderDetailsTab generates the Details tab content
func (v *FirewallDetailsView) renderDetailsTab() string {
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
	enabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	disabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Firewall Rule: %s", d.Name)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Basic Information
	b.WriteString(sectionStyle.Render("Basic Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Rule ID", strconv.FormatUint(d.ID, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Description", defaultIfEmpty(d.Description, "None")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	b.WriteString("\n")

	// Configuration
	b.WriteString(sectionStyle.Render("Configuration"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Direction", d.Direction))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Priority", strconv.FormatInt(d.Priority, 10)))

	// Status with color
	var statusStr string
	if d.Disabled {
		statusStr = disabledStyle.Render("Disabled")
	} else {
		statusStr = enabledStyle.Render("Enabled")
	}
	b.WriteString(labelStyle.Render("Status:") + " " + statusStr + "\n")

	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Action", d.Action))
	b.WriteString("\n")

	// Network (navigable link)
	b.WriteString(sectionStyle.Render("Network"))
	b.WriteString("\n")
	b.WriteString(v.networkLink.RenderRow(0, labelStyle.Render("Network:")+" "+d.Network))
	b.WriteString("\n\n")

	// Source Filters (only show sections that have data)
	if len(d.SourceRanges) > 0 || len(d.SourceTags) > 0 || len(d.SourceServiceAccounts) > 0 {
		b.WriteString(sectionStyle.Render("Source Filters"))
		b.WriteString("\n")
		if len(d.SourceRanges) > 0 {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Ranges", strings.Join(d.SourceRanges, ", ")))
		}
		if len(d.SourceTags) > 0 {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Tags", strings.Join(d.SourceTags, ", ")))
		}
		if len(d.SourceServiceAccounts) > 0 {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Source Service Accounts", strings.Join(d.SourceServiceAccounts, ", ")))
		}
		b.WriteString("\n")
	}

	// Target Filters (only show sections that have data)
	if len(d.TargetTags) > 0 || len(d.TargetServiceAccounts) > 0 || len(d.DestinationRanges) > 0 {
		b.WriteString(sectionStyle.Render("Target Filters"))
		b.WriteString("\n")
		if len(d.TargetTags) > 0 {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Target Tags", strings.Join(d.TargetTags, ", ")))
		}
		if len(d.TargetServiceAccounts) > 0 {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Target Service Accounts", strings.Join(d.TargetServiceAccounts, ", ")))
		}
		if len(d.DestinationRanges) > 0 {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Destination Ranges", strings.Join(d.DestinationRanges, ", ")))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderRulesTab generates the Rules tab content showing allowed/denied entries
func (v *FirewallDetailsView) renderRulesTab() string {
	if v.details == nil {
		return ""
	}

	d := v.details
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Firewall Rule: %s", d.Name)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Allowed rules
	if len(d.Allowed) > 0 {
		b.WriteString(sectionStyle.Render("Allowed Rules"))
		b.WriteString("\n")
		b.WriteString(v.renderRuleEntries(d.Allowed))
		b.WriteString("\n")
	}

	// Denied rules
	if len(d.Denied) > 0 {
		b.WriteString(sectionStyle.Render("Denied Rules"))
		b.WriteString("\n")
		b.WriteString(v.renderRuleEntries(d.Denied))
		b.WriteString("\n")
	}

	// Empty state when neither allowed nor denied entries exist
	if len(d.Allowed) == 0 && len(d.Denied) == 0 {
		b.WriteString(mutedStyle.Render("  No protocol rules defined"))
		b.WriteString("\n\n")
	}

	// Log Configuration
	b.WriteString(sectionStyle.Render("Log Configuration"))
	b.WriteString("\n")
	logEnabled := "No"
	if d.LogEnabled {
		logEnabled = "Yes"
	}
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Log Enabled", logEnabled))
	if d.LogEnabled && d.LogMetadata != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Log Metadata Filter", d.LogMetadata))
	}
	b.WriteString("\n")

	return b.String()
}

// renderRuleEntries renders a protocol/ports table for allow or deny entries
func (v *FirewallDetailsView) renderRuleEntries(entries []gcp.FirewallRuleEntry) string {
	var b strings.Builder

	header := fmt.Sprintf("  %-15s %s", "Protocol", "Ports")
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString("  " + strings.Repeat("─", 40))
	b.WriteString("\n")

	for _, entry := range entries {
		ports := "(all)"
		if len(entry.Ports) > 0 {
			ports = strings.Join(entry.Ports, ", ")
		}
		b.WriteString(fmt.Sprintf("  %-15s %s\n", entry.Protocol, ports))
	}

	return b.String()
}

// buildHelpText generates context-sensitive help text
func (v *FirewallDetailsView) buildHelpText() string {
	bindings := focus.HelpForRegion(v.focusMgr.ActiveType(), v.getRegionLabel())
	helpStr := focus.FormatHelp(bindings)
	badge := focus.FormatRegionBadge(v.focusMgr.Active())
	if badge != "" {
		return "\n  " + badge + " • " + helpStr + " • .: actions • t: toggle • D: delete"
	}
	return "\n  " + helpStr + " • .: actions • t: toggle • D: delete"
}

func (v *FirewallDetailsView) getRegionLabel() string {
	if v.focusMgr.ActiveType() == focus.RegionLinks {
		return "network"
	}
	return ""
}

// UpdateRegions calculates clickable regions for tabs, links, and viewport
func (v *FirewallDetailsView) UpdateRegions(offsetX, offsetY int) {
	v.regionMgr.Clear()

	if !v.ready || v.loading {
		return
	}

	y := offsetY

	// Tab bar region
	tabHeight := 1
	v.regionMgr.Add(firewallRegionIDTabs, mouse.Rect{
		X:      offsetX,
		Y:      y,
		Width:  v.width,
		Height: tabHeight,
	}, nil)
	y += tabHeight + 1

	// Links region (only in Details tab, if network link exists)
	if v.tabs.ActiveTab().ID == firewallTabIDDetails && v.networkLink != nil && v.networkLink.Count() > 0 {
		linksHeight := v.networkLink.Count()
		v.regionMgr.Add(firewallRegionIDLinks, mouse.Rect{
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
		v.regionMgr.Add(firewallRegionIDViewport, mouse.Rect{
			X:      offsetX,
			Y:      y,
			Width:  v.width,
			Height: viewportHeight,
		}, nil)
	}
}

// GetRegions returns the current clickable regions
func (v *FirewallDetailsView) GetRegions() []mouse.Region {
	return v.regionMgr.GetRegions()
}

// HandleRegionClick processes a click on a specific region
func (v *FirewallDetailsView) HandleRegionClick(regionID string) tea.Cmd {
	v.focusMgr.SetActive(regionID)
	return nil
}
