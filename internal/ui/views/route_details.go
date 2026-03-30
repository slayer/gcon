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
const routeDetailsViewportReservedLines = 3

// Internal messages for async data loading
type routeDetailsLoadedMsg struct {
	details *gcp.RouteDetails
}

type routeDetailsErrorMsg struct {
	err error
}

// RouteDetailsView displays comprehensive route information in a scrollable viewport
type RouteDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	routeName     string
	ctx           *context.ProgramContext

	details *gcp.RouteDetails

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

	// Delete confirmation (only for static routes)
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool

	keys routeDetailsKeyMap
}

type routeDetailsKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Refresh    key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
	Tab        key.Binding
}

func defaultRouteDetailsKeyMap() routeDetailsKeyMap {
	return routeDetailsKeyMap{
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

// NewRouteDetailsView creates a new route details view
func NewRouteDetailsView(projectID, routeName string, computeClient *gcp.ComputeClient) *RouteDetailsView {
	return &RouteDetailsView{
		computeClient: computeClient,
		projectID:     projectID,
		routeName:     routeName,
		spinner:       components.NewGCPSpinner(),
		loading:       true,
		networkLink:   links.New(),
		keys:          defaultRouteDetailsKeyMap(),
	}
}

// Init starts loading route details
func (v *RouteDetailsView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
	)
}

func (v *RouteDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return routeDetailsErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		details, err := v.computeClient.GetRouteDetails(gocontext.Background(), v.projectID, v.routeName)
		if err != nil {
			return routeDetailsErrorMsg{err: err}
		}
		return routeDetailsLoadedMsg{details: details}
	}
}

// Update handles messages for the route details view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern - complexity expected
func (v *RouteDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case routeDetailsLoadedMsg:
		v.loading = false
		v.details = msg.details
		v.populateNetworkLink()
		v.updateViewportContent()
		return nil

	case routeDetailsErrorMsg:
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
				return RouteDeleteRequestMsg{
					Name: v.details.Name,
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
				v.actionMenu = actionmenu.New("Route Actions", v.buildActions())
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

func (v *RouteDetailsView) buildActions() []actionmenu.Action {
	// Only allow delete for static routes
	deleteEnabled := v.details != nil && v.details.RouteType == "Static"
	return []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: deleteEnabled, Dangerous: true},
	}
}

func (v *RouteDetailsView) executeAction(actionKey rune) tea.Cmd {
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

func (v *RouteDetailsView) initiateDelete() tea.Cmd {
	if v.details == nil {
		return nil
	}

	// Only static routes can be deleted
	if v.details.RouteType != "Static" {
		return nil
	}

	detailLines := []string{
		fmt.Sprintf("Network: %s", v.details.Network),
		fmt.Sprintf("Destination: %s", v.details.DestRange),
		fmt.Sprintf("Priority: %d", v.details.Priority),
	}

	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Route", v.details.Name, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

func (v *RouteDetailsView) populateNetworkLink() {
	if v.details == nil || v.details.Network == "" {
		v.networkLink.SetItems(nil)
		return
	}
	v.networkLink.SetItems([]links.Link{
		{ID: v.details.Network, Label: v.details.Network, Type: "network"},
	})
}

// View renders the route details view
func (v *RouteDetailsView) View() string {
	if v.loading && v.details == nil {
		return renderLoading(v.spinner, "Loading route details...")
	}

	if v.err != nil && v.details == nil {
		return components.RenderError(v.err)
	}

	if v.details == nil {
		return renderLoading(v.spinner, "No route details available.\n  Press 'esc' to go back.")
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

func (v *RouteDetailsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// SetContext updates the view with shared program context
func (v *RouteDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// IsMenuOpen returns true if the action menu or delete confirm is open
func (v *RouteDetailsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetRouteName returns the route name for breadcrumbs
func (v *RouteDetailsView) GetRouteName() string {
	return v.routeName
}

// GetComputeClient returns the compute client for reuse
func (v *RouteDetailsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// HasTextInputFocused returns true if the delete confirm input is active.
// Prevents global hotkeys (like 'q' for quit) from triggering while typing.
func (v *RouteDetailsView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return false
}

func (v *RouteDetailsView) applySize(width, height int) {
	viewportHeight := height - routeDetailsViewportReservedLines
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

func (v *RouteDetailsView) updateViewportContent() {
	if !v.ready {
		return
	}

	// Update link focus state before rendering
	v.networkLink.SetRegionFocused(v.linkFocused)
	v.viewport.SetContent(v.renderContent())
}

// renderContent generates the full details content for the viewport
func (v *RouteDetailsView) renderContent() string {
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
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Route: %s", d.Name)))
	b.WriteString("\n")
	repeatWidth := max(0, min(v.width-4, 60))
	b.WriteString(strings.Repeat("─", repeatWidth))
	b.WriteString("\n\n")

	// Basic Information
	b.WriteString(sectionStyle.Render("Basic Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "ID", strconv.FormatUint(d.ID, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Description", defaultIfEmpty(d.Description, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	b.WriteString("\n")

	// Routing
	b.WriteString(sectionStyle.Render("Routing"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Destination Range", d.DestRange))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Priority", strconv.FormatInt(d.Priority, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Next Hop Type", defaultIfEmpty(d.NextHopType, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Next Hop", defaultIfEmpty(d.NextHop, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Route Type", defaultIfEmpty(d.RouteType, "—")))

	// Tags — comma-joined or "None"
	tags := "None"
	if len(d.Tags) > 0 {
		tags = strings.Join(d.Tags, ", ")
	}
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Tags", tags))
	b.WriteString("\n")

	// Network (navigable link)
	b.WriteString(sectionStyle.Render("Network"))
	b.WriteString("\n")
	if v.networkLink != nil && v.networkLink.HasItems() {
		label := labelStyle.Render("Network:")
		linkRendered := v.networkLink.RenderRow(0, d.Network)
		b.WriteString(label + " " + linkRendered + "\n")
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Network", d.Network))
	}
	b.WriteString("\n")

	// Warnings (only if any)
	if len(d.Warnings) > 0 {
		b.WriteString(sectionStyle.Render("Warnings"))
		b.WriteString("\n")
		for _, w := range d.Warnings {
			b.WriteString("  " + warningStyle.Render("⚠ "+w) + "\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}
