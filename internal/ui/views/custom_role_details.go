package views

import (
	gocontext "context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/focus"
	"github.com/slayer/gcon/internal/ui/mouse"
)

// Tab IDs for custom role details view
const (
	crTabIDDetails     = "details"
	crTabIDPermissions = "permissions"
)

const (
	crRegionIDTabs     = "tabs"
	crRegionIDViewport = "viewport"
)

const crDetailsViewportReservedLines = 5

// Internal messages
type customRoleDetailsLoadedMsg struct {
	role *gcp.CustomRole
}
type customRoleDetailsErrorMsg struct {
	err error
}

// CustomRoleDetailsView displays custom role information with tabs
type CustomRoleDetailsView struct {
	iamClient *gcp.IAMClient
	projectID string
	roleID    string
	ctx       *context.ProgramContext

	role    *gcp.CustomRole
	loading bool
	err     error

	// UI components
	spinner      spinner.Model
	tabs         *tabs.Tabs
	tabViewports []viewport.Model
	focusMgr     *focus.Manager
	regionMgr    *mouse.RegionManager

	width  int
	height int
	ready  bool

	keys crDetailsKeyMap
}

type crDetailsKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Refresh key.Binding
}

func defaultCRDetailsKeyMap() crDetailsKeyMap {
	return crDetailsKeyMap{
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
	}
}

// NewCustomRoleDetailsView creates a new custom role details view
func NewCustomRoleDetailsView(projectID, roleID string, iamClient *gcp.IAMClient) *CustomRoleDetailsView {
	s := components.NewGCPSpinner()

	tabsComponent := tabs.New([]tabs.Tab{
		{ID: crTabIDDetails, Label: "Details"},
		{ID: crTabIDPermissions, Label: "Permissions"},
	})

	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewRegion(crRegionIDTabs, focus.RegionTabs, "Tabs"),
		focus.NewRegion(crRegionIDViewport, focus.RegionViewport, "Content"),
	})

	return &CustomRoleDetailsView{
		iamClient:    iamClient,
		projectID:    projectID,
		roleID:       roleID,
		spinner:      s,
		loading:      true,
		tabs:         tabsComponent,
		tabViewports: make([]viewport.Model, 2),
		focusMgr:     fm,
		regionMgr:    mouse.NewRegionManager(),
		keys:         defaultCRDetailsKeyMap(),
	}
}

// Init starts loading the role details
func (v *CustomRoleDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadRole(),
	)
}

func (v *CustomRoleDetailsView) loadRole() tea.Cmd {
	return func() tea.Msg {
		if v.iamClient == nil {
			return customRoleDetailsErrorMsg{err: uierrors.ErrIAMClientNotInitialized}
		}
		role, err := v.iamClient.GetCustomRole(gocontext.Background(), v.projectID, v.roleID)
		if err != nil {
			return customRoleDetailsErrorMsg{err: err}
		}
		return customRoleDetailsLoadedMsg{role: role}
	}
}

// Update handles messages
//
//nolint:gocognit // Bubble Tea Update pattern
func (v *CustomRoleDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case customRoleDetailsLoadedMsg:
		v.loading = false
		v.role = msg.role
		v.updateViewportContent()
		return nil

	case customRoleDetailsErrorMsg:
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

	case tabs.TabChangedMsg:
		v.updateViewportContent()
		return nil

	case focus.FocusChangedMsg:
		v.updateViewportContent()
		return nil

	case tea.KeyMsg:
		// Handle Tab/Shift+Tab for cycling between focus regions
		if focusMsg := v.focusMgr.HandleKey(msg); focusMsg != nil {
			v.updateViewportContent()
			return func() tea.Msg { return focusMsg }
		}

		if key.Matches(msg, v.keys.Refresh) {
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadRole())
		}

		// Route based on focused region
		switch v.focusMgr.ActiveType() {
		case focus.RegionTabs:
			if tabs.HandleKey(msg) {
				return v.tabs.Update(msg)
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

// View renders the custom role details view
func (v *CustomRoleDetailsView) View() string {
	if v.loading && v.role == nil {
		return renderLoading(v.spinner, "Loading custom role details...")
	}

	if v.err != nil && v.role == nil {
		return "\n" + components.RenderError(v.err)
	}

	if v.role == nil {
		return renderLoading(v.spinner, "No custom role details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return renderLoading(v.spinner, "Initializing view...")
	}

	tabBar := focus.RenderAccent("  "+v.tabs.View(), v.focusMgr.IsActive(crRegionIDTabs))

	activeIdx := v.tabs.ActiveIndex()
	var viewportContent string
	var scrollPercent float64
	if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
		viewportContent = focus.RenderAccent(v.tabViewports[activeIdx].View(), v.focusMgr.IsActive(crRegionIDViewport))
		scrollPercent = v.tabViewports[activeIdx].ScrollPercent()
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", scrollPercent*100))
	help := helpStyle.Render("\n  r: refresh • Tab: focus • h/l: switch tabs • esc: back") + " " + scrollInfo

	return tabBar + "\n" + viewportContent + help
}

// SetContext updates the view with shared program context
func (v *CustomRoleDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// GetRoleID returns the role ID for breadcrumbs
func (v *CustomRoleDetailsView) GetRoleID() string {
	return v.roleID
}

// GetIAMClient returns the IAM client for reuse
func (v *CustomRoleDetailsView) GetIAMClient() *gcp.IAMClient {
	return v.iamClient
}

func (v *CustomRoleDetailsView) applySize(width, height int) {
	viewportHeight := height - crDetailsViewportReservedLines
	if viewportHeight < 1 {
		viewportHeight = 1
	}

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

	if v.role != nil {
		v.updateViewportContent()
	}
}

func (v *CustomRoleDetailsView) updateViewportContent() {
	if v.role != nil {
		v.tabViewports[0].SetContent(v.renderDetailsTab())
		v.tabViewports[1].SetContent(v.renderPermissionsTab())
	}
}

func (v *CustomRoleDetailsView) renderDetailsTab() string {
	if v.role == nil {
		return ""
	}

	var b strings.Builder
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(16)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED"))

	field := func(label, value string) {
		if value == "" {
			value = "—"
		}
		b.WriteString(labelStyle.Render(label))
		b.WriteString(valueStyle.Render(value))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	field("Title", v.role.Title)
	field("Role ID", v.role.RoleID)
	field("Name", v.role.Name)
	field("Stage", v.role.Stage)
	field("Description", v.role.Description)
	field("Permissions", fmt.Sprintf("%d", len(v.role.Permissions)))

	if v.role.Deleted {
		b.WriteString("\n")
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		b.WriteString(errorStyle.Render("  This role has been deleted."))
		b.WriteString("\n")
	}

	return b.String()
}

func (v *CustomRoleDetailsView) renderPermissionsTab() string {
	if v.role == nil || len(v.role.Permissions) == 0 {
		return "\n  No permissions defined for this role."
	}

	var b strings.Builder
	permStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED"))
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	b.WriteString("\n")
	b.WriteString(countStyle.Render(fmt.Sprintf("  %d permissions\n\n", len(v.role.Permissions))))

	for _, perm := range v.role.Permissions {
		b.WriteString(permStyle.Render("  " + perm))
		b.WriteString("\n")
	}

	return b.String()
}
