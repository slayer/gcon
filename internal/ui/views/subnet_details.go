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
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// Lines reserved for help text + padding below viewport
const subnetDetailsViewportReservedLines = 3

// Internal messages for async data loading
type subnetDetailsLoadedMsg struct {
	details *gcp.SubnetDetails
}

type subnetDetailsErrorMsg struct {
	err error
}

// SubnetDetailsView displays comprehensive subnet information in a scrollable viewport
type SubnetDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	region        string
	subnetName    string
	ctx           *context.ProgramContext

	details *gcp.SubnetDetails

	spinner  spinner.Model
	loading  bool
	err      error
	width    int
	height   int
	ready    bool
	viewport viewport.Model

	// Network link (navigable to network details)
	networkLink *links.Links
	linkFocused bool

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool

	keys subnetDetailsKeyMap
}

type subnetDetailsKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Refresh    key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
	Tab        key.Binding
}

func defaultSubnetDetailsKeyMap() subnetDetailsKeyMap {
	return subnetDetailsKeyMap{
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
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "focus network"),
		),
	}
}

// NewSubnetDetailsView creates a new subnet details view
func NewSubnetDetailsView(projectID, region, subnetName string, computeClient *gcp.ComputeClient) *SubnetDetailsView {
	return &SubnetDetailsView{
		computeClient: computeClient,
		projectID:     projectID,
		region:        region,
		subnetName:    subnetName,
		spinner:       components.NewGCPSpinner(),
		loading:       true,
		networkLink:   links.New(),
		keys:          defaultSubnetDetailsKeyMap(),
	}
}

// Init starts loading subnet details
func (v *SubnetDetailsView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
	)
}

func (v *SubnetDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return subnetDetailsErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		details, err := v.computeClient.GetSubnetDetails(gocontext.Background(), v.projectID, v.region, v.subnetName)
		if err != nil {
			return subnetDetailsErrorMsg{err: err}
		}
		return subnetDetailsLoadedMsg{details: details}
	}
}

// Update handles messages for the subnet details view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern - complexity expected
func (v *SubnetDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case subnetDetailsLoadedMsg:
		v.loading = false
		v.details = msg.details
		v.populateNetworkLink()
		v.updateViewportContent()
		return nil

	case subnetDetailsErrorMsg:
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

	case confirm.TypeConfirmMsg:
		v.showDeleteConfirm = false
		if v.details != nil {
			return func() tea.Msg {
				return DeleteSubnetConfirmedMsg{
					SubnetName: v.details.Name,
					Region:     v.details.Region,
				}
			}
		}
		return nil

	case confirm.TypeCancelMsg:
		v.showDeleteConfirm = false
		return nil

	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		return v.executeAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
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

	case tea.KeyMsg:
		// Route to delete confirm when shown
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		// Skip key handling while loading
		if v.loading {
			return nil
		}

		// Route to action menu when open
		if v.menuOpen {
			return v.actionMenu.Update(msg)
		}

		switch {
		case key.Matches(msg, v.keys.Tab):
			// Toggle focus between network link and viewport
			if v.networkLink != nil && v.networkLink.HasItems() {
				v.linkFocused = !v.linkFocused
				v.networkLink.SetRegionFocused(v.linkFocused)
				v.updateViewportContent()
			}
			return nil

		case key.Matches(msg, v.keys.ActionMenu):
			if v.details != nil {
				v.actionMenu = actionmenu.New("Subnet Actions", v.buildActions())
				v.menuOpen = true
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadDetails())

		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()
		}

		// Delegate to link navigation when focused
		if v.linkFocused && v.networkLink != nil && v.networkLink.HasItems() {
			if links.HandleKey(msg) {
				cmd := v.networkLink.Update(msg)
				v.updateViewportContent()
				return cmd
			}
		}

		// Default: scroll viewport
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return cmd
	}

	return nil
}

func (v *SubnetDetailsView) buildActions() []actionmenu.Action {
	return []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
	}
}

func (v *SubnetDetailsView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'r':
		v.loading = true
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.loadDetails())
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

func (v *SubnetDetailsView) initiateDelete() tea.Cmd {
	if v.details == nil {
		return nil
	}

	detailLines := []string{
		fmt.Sprintf("Network: %s", v.details.Network),
		fmt.Sprintf("Region: %s", v.details.Region),
		fmt.Sprintf("CIDR: %s", v.details.IPCidrRange),
	}

	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Subnet", v.details.Name, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

func (v *SubnetDetailsView) populateNetworkLink() {
	if v.details == nil || v.details.Network == "" {
		v.networkLink.SetItems(nil)
		return
	}
	v.networkLink.SetItems([]links.Link{
		{ID: v.details.Network, Label: v.details.Network, Type: "network"},
	})
}

// View renders the subnet details view
func (v *SubnetDetailsView) View() string {
	if v.loading && v.details == nil {
		return renderLoading(v.spinner, "Loading subnet details...")
	}

	if v.err != nil && v.details == nil {
		return components.RenderError(v.err)
	}

	if v.details == nil {
		return renderLoading(v.spinner, "No subnet details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return renderLoading(v.spinner, "Initializing view...")
	}

	// Help and scroll info
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", v.viewport.ScrollPercent()*100))

	helpText := "↑/↓: scroll • tab: focus network • .: actions • D: delete • r: refresh • esc: back"
	help := helpStyle.Render(helpText) + " " + scrollInfo

	mainContent := v.viewport.View() + "\n" + help

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

func (v *SubnetDetailsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// SetContext updates the view with shared program context
func (v *SubnetDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// IsMenuOpen returns true if the action menu or delete confirm is open
func (v *SubnetDetailsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetSubnetName returns the subnet name for breadcrumbs
func (v *SubnetDetailsView) GetSubnetName() string {
	return v.subnetName
}

// GetComputeClient returns the compute client for reuse
func (v *SubnetDetailsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// HasTextInputFocused returns true if the delete confirm input is active.
// Prevents global hotkeys (like 'q' for quit) from triggering while typing.
func (v *SubnetDetailsView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return false
}

func (v *SubnetDetailsView) applySize(width, height int) {
	viewportHeight := height - subnetDetailsViewportReservedLines
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Reserve 1 char for focus bar space
	viewportWidth := width - 1
	if viewportWidth < 1 {
		viewportWidth = 1
	}

	if !v.ready {
		v.viewport = viewport.New(viewportWidth, viewportHeight)
		v.viewport.Style = lipgloss.NewStyle().Padding(0, 2)
		v.ready = true
	} else {
		v.viewport.Width = viewportWidth
		v.viewport.Height = viewportHeight
	}

	if v.details != nil {
		v.updateViewportContent()
	}
}

func (v *SubnetDetailsView) updateViewportContent() {
	if !v.ready {
		return
	}

	// Update link focus state before rendering
	v.networkLink.SetRegionFocused(v.linkFocused)
	v.viewport.SetContent(v.renderContent())
}

// renderContent generates the full details content for the viewport
func (v *SubnetDetailsView) renderContent() string {
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
	b.WriteString(titleStyle.Render(fmt.Sprintf("Subnet: %s", d.Name)))
	b.WriteString("\n")
	repeatWidth := max(0, min(v.width-4, 60))
	b.WriteString(strings.Repeat("─", repeatWidth))
	b.WriteString("\n\n")

	// Basic Information
	b.WriteString(sectionStyle.Render("Basic Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "ID", strconv.FormatUint(d.ID, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Status", defaultIfEmpty(d.Status, "READY")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Region", d.Region))

	// Network as navigable link
	if v.networkLink != nil && v.networkLink.HasItems() {
		label := labelStyle.Render("Network:")
		linkRendered := v.networkLink.RenderRow(0, d.Network)
		b.WriteString(label + " " + linkRendered + "\n")
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Network", d.Network))
	}

	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "CIDR Range", d.IPCidrRange))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Gateway", defaultIfEmpty(d.GatewayAddress, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	b.WriteString("\n")

	// Configuration
	b.WriteString(sectionStyle.Render("Configuration"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Purpose", formatSubnetPurpose(d.Purpose)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Stack Type", defaultIfEmpty(d.StackType, "—")))

	if d.IPv6AccessType != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "IPv6 Access Type", d.IPv6AccessType))
	}
	if d.IPv6CidrRange != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "IPv6 CIDR Range", d.IPv6CidrRange))
	}

	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Private Google Access", formatYesNo(d.PrivateIPGoogleAccess)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Flow Logs", formatYesNo(d.EnableFlowLogs)))
	b.WriteString("\n")

	// Flow Logs Configuration (only if enabled)
	if d.EnableFlowLogs {
		b.WriteString(sectionStyle.Render("Flow Logs Configuration"))
		b.WriteString("\n")
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Aggregation Interval", defaultIfEmpty(d.FlowLogConfig.AggregationInterval, "—")))
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Flow Sampling", formatFlowLogSampling(d.FlowLogConfig.FlowSampling)))
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Metadata", defaultIfEmpty(d.FlowLogConfig.Metadata, "—")))
		if d.FlowLogConfig.FilterExpr != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Filter", d.FlowLogConfig.FilterExpr))
		}
		b.WriteString("\n")
	}

	// Secondary IP Ranges
	b.WriteString(sectionStyle.Render("Secondary IP Ranges"))
	b.WriteString("\n")
	if len(d.SecondaryIPRanges) == 0 {
		b.WriteString(mutedStyle.Render("  No secondary IP ranges configured"))
		b.WriteString("\n")
	} else {
		// Table header
		header := fmt.Sprintf("  %-30s %s", "Name", "CIDR Range")
		b.WriteString(header)
		b.WriteString("\n")
		b.WriteString("  " + strings.Repeat("─", 50))
		b.WriteString("\n")
		for _, r := range d.SecondaryIPRanges {
			b.WriteString(fmt.Sprintf("  %-30s %s\n", r.Name, r.CidrRange))
		}
	}
	b.WriteString("\n")

	return b.String()
}

// formatFlowLogSampling converts a flow sampling rate (0.0-1.0) to a percentage string
func formatFlowLogSampling(rate float64) string {
	return fmt.Sprintf("%.0f%%", rate*100)
}
