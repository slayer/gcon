package views

import (
	gocontext "context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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

// Tab IDs for IAM policy view
const (
	iamPolicyTabByRole   = "by-role"
	iamPolicyTabByMember = "by-member"
)

const (
	iamPolicyRegionTabs     = "tabs"
	iamPolicyRegionViewport = "viewport"
)

const iamPolicyViewportReservedLines = 5

// Internal messages for async data loading
type iamPolicyLoadedMsg struct {
	policy *gcp.IAMPolicy
}
type iamPolicyErrorMsg struct {
	err error
}

// Internal message for IAM client init
type iamPolicyClientReadyMsg struct {
	client *gcp.IAMClient
}

// IAMPolicyView displays project IAM bindings in two tabs
type IAMPolicyView struct {
	iamClient *gcp.IAMClient
	projectID string
	ctx       *context.ProgramContext

	policy  *gcp.IAMPolicy
	loading bool
	err     error

	// UI components
	spinner      spinner.Model
	tabs         *tabs.Tabs
	tabViewports []viewport.Model
	focusMgr     *focus.Manager
	regionMgr    *mouse.RegionManager

	// Filter
	filterInput  textinput.Model
	filterActive bool
	filterText   string

	width  int
	height int
	ready  bool

	keys iamPolicyKeyMap
}

type iamPolicyKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Refresh key.Binding
	Filter  key.Binding
}

func defaultIAMPolicyKeyMap() iamPolicyKeyMap {
	return iamPolicyKeyMap{
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
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
	}
}

// NewIAMPolicyView creates a new IAM policy view
func NewIAMPolicyView(projectID string) *IAMPolicyView {
	s := components.NewGCPSpinner()

	tabsComponent := tabs.New([]tabs.Tab{
		{ID: iamPolicyTabByRole, Label: "By Role"},
		{ID: iamPolicyTabByMember, Label: "By Member"},
	})

	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewRegion(iamPolicyRegionTabs, focus.RegionTabs, "Tabs"),
		focus.NewRegion(iamPolicyRegionViewport, focus.RegionViewport, "Content"),
	})

	fi := textinput.New()
	fi.Placeholder = "Filter roles or members..."
	fi.CharLimit = 128

	return &IAMPolicyView{
		projectID:    projectID,
		spinner:      s,
		loading:      true,
		tabs:         tabsComponent,
		tabViewports: make([]viewport.Model, 2),
		focusMgr:     fm,
		regionMgr:    mouse.NewRegionManager(),
		filterInput:  fi,
		keys:         defaultIAMPolicyKeyMap(),
	}
}

// Init starts loading the IAM policy
func (v *IAMPolicyView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initIAMClient(),
	)
}

func (v *IAMPolicyView) initIAMClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewIAMClient(gocontext.Background())
		if err != nil {
			return iamPolicyErrorMsg{err: err}
		}
		return iamPolicyClientReadyMsg{client: client}
	}
}

func (v *IAMPolicyView) loadPolicy() tea.Cmd {
	return func() tea.Msg {
		if v.iamClient == nil {
			return iamPolicyErrorMsg{err: uierrors.ErrIAMClientNotInitialized}
		}
		policy, err := v.iamClient.GetProjectIAMPolicy(gocontext.Background(), v.projectID)
		if err != nil {
			return iamPolicyErrorMsg{err: err}
		}
		return iamPolicyLoadedMsg{policy: policy}
	}
}

// Update handles messages for the IAM policy view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern
func (v *IAMPolicyView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case iamPolicyClientReadyMsg:
		v.iamClient = msg.client
		v.registerTask("load-iam-policy", "Loading IAM policy...")
		return v.loadPolicy()

	case iamPolicyLoadedMsg:
		v.loading = false
		v.policy = msg.policy
		v.clearTask("load-iam-policy")
		v.updateViewportContent()
		return nil

	case iamPolicyErrorMsg:
		v.loading = false
		v.err = msg.err
		return v.failTask("load-iam-policy", msg.err)

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
		// When filter is active, route keys to the text input
		if v.filterActive {
			switch msg.Type {
			case tea.KeyEscape:
				v.filterActive = false
				v.filterInput.Blur()
				// Clear filter on Esc if empty, otherwise just deactivate
				if v.filterInput.Value() == "" {
					v.filterText = ""
				}
				v.updateViewportContent()
				return nil
			case tea.KeyEnter:
				// Apply filter and deactivate input
				v.filterText = v.filterInput.Value()
				v.filterActive = false
				v.filterInput.Blur()
				v.updateViewportContent()
				return nil
			default:
				var cmd tea.Cmd
				v.filterInput, cmd = v.filterInput.Update(msg)
				// Live-filter as user types
				v.filterText = v.filterInput.Value()
				v.updateViewportContent()
				return cmd
			}
		}

		// Clear filter on Esc before navigating back
		if msg.Type == tea.KeyEscape && v.filterText != "" {
			v.filterText = ""
			v.filterInput.SetValue("")
			v.updateViewportContent()
			return nil
		}

		// Handle Tab/Shift+Tab for cycling between focus regions
		if focusMsg := v.focusMgr.HandleKey(msg); focusMsg != nil {
			v.updateViewportContent()
			return func() tea.Msg { return focusMsg }
		}

		// Activate filter
		if key.Matches(msg, v.keys.Filter) && !v.loading {
			v.filterActive = true
			v.filterInput.Focus()
			return nil
		}

		if key.Matches(msg, v.keys.Refresh) {
			v.loading = true
			v.err = nil
			v.registerTask("load-iam-policy", "Refreshing...")
			if v.iamClient == nil {
				return tea.Batch(v.spinner.Tick, v.initIAMClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadPolicy())
		}

		// Route remaining keys based on focused region
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

// View renders the IAM policy view
func (v *IAMPolicyView) View() string {
	if v.loading && v.iamClient == nil {
		return renderLoading(v.spinner, "Initializing IAM client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading IAM policy...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if v.policy == nil || len(v.policy.Bindings) == 0 {
		return "\n  No IAM bindings found for this project.\n  Press 'esc' to go back."
	}

	if !v.ready {
		return renderLoading(v.spinner, "Initializing view...")
	}

	// Render tab bar with focus accent
	tabBar := focus.RenderAccent("  "+v.tabs.View(), v.focusMgr.IsActive(iamPolicyRegionTabs))

	// Show filter bar when active or when filter text is set
	var filterBar string
	if v.filterActive {
		filterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
		filterBar = "\n  " + filterStyle.Render("/") + " " + v.filterInput.View()
	} else if v.filterText != "" {
		filterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		filterBar = "\n  " + filterStyle.Render(fmt.Sprintf("Filter: %s (/ to edit, Esc to clear)", v.filterText))
	}

	// Get active tab viewport with focus accent
	activeIdx := v.tabs.ActiveIndex()
	var viewportContent string
	var scrollPercent float64
	if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
		viewportContent = focus.RenderAccent(v.tabViewports[activeIdx].View(), v.focusMgr.IsActive(iamPolicyRegionViewport))
		scrollPercent = v.tabViewports[activeIdx].ScrollPercent()
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", scrollPercent*100))
	help := helpStyle.Render("\n  /: filter • r: refresh • Tab: focus • h/l: switch tabs • esc: back") + " " + scrollInfo

	return tabBar + filterBar + "\n" + viewportContent + help
}

// SetContext updates the view with shared program context
func (v *IAMPolicyView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// GetIAMClient returns the IAM client for reuse
func (v *IAMPolicyView) GetIAMClient() *gcp.IAMClient {
	return v.iamClient
}

func (v *IAMPolicyView) applySize(width, height int) {
	viewportHeight := height - iamPolicyViewportReservedLines
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

	if v.policy != nil {
		v.updateViewportContent()
	}
}

func (v *IAMPolicyView) updateViewportContent() {
	if v.policy == nil {
		return
	}
	v.tabViewports[0].SetContent(v.renderByRoleTab())
	v.tabViewports[1].SetContent(v.renderByMemberTab())
}

func (v *IAMPolicyView) renderByRoleTab() string {
	if v.policy == nil {
		return ""
	}

	filter := strings.ToLower(v.filterText)

	var b strings.Builder
	roleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true)
	memberStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED"))
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	rendered := 0
	for _, binding := range v.policy.Bindings {
		// Filter: match role name or any member
		if filter != "" && !v.bindingMatchesFilter(binding, filter) {
			continue
		}

		if rendered > 0 {
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(roleStyle.Render("  " + binding.Role))
		b.WriteString(countStyle.Render(fmt.Sprintf(" (%d members)", len(binding.Members))))
		b.WriteString("\n")

		for _, member := range binding.Members {
			b.WriteString(memberStyle.Render("    " + member))
			b.WriteString("\n")
		}
		rendered++
	}

	if rendered == 0 && filter != "" {
		noMatchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		b.WriteString("\n")
		b.WriteString(noMatchStyle.Render("  No bindings match the filter."))
		b.WriteString("\n")
	}

	return b.String()
}

func (v *IAMPolicyView) renderByMemberTab() string {
	if v.policy == nil {
		return ""
	}

	filter := strings.ToLower(v.filterText)

	// Invert: member → []role
	memberRoles := make(map[string][]string)
	for _, binding := range v.policy.Bindings {
		for _, member := range binding.Members {
			memberRoles[member] = append(memberRoles[member], binding.Role)
		}
	}

	// Sort members for deterministic output
	members := make([]string, 0, len(memberRoles))
	for m := range memberRoles {
		members = append(members, m)
	}
	sort.Strings(members)

	var b strings.Builder
	memberStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true)
	roleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED"))
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	rendered := 0
	for _, member := range members {
		roles := memberRoles[member]
		sort.Strings(roles)

		// Filter: match member name or any of their roles
		if filter != "" && !v.memberMatchesFilter(member, roles, filter) {
			continue
		}

		if rendered > 0 {
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(memberStyle.Render("  " + member))
		b.WriteString(countStyle.Render(fmt.Sprintf(" (%d roles)", len(roles))))
		b.WriteString("\n")

		for _, role := range roles {
			b.WriteString(roleStyle.Render("    " + role))
			b.WriteString("\n")
		}
		rendered++
	}

	if rendered == 0 && filter != "" {
		noMatchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		b.WriteString("\n")
		b.WriteString(noMatchStyle.Render("  No members match the filter."))
		b.WriteString("\n")
	}

	return b.String()
}

// HasTextInputFocused returns true when the filter input is active
func (v *IAMPolicyView) HasTextInputFocused() bool {
	return v.filterActive
}

// Filter matching helpers

func (v *IAMPolicyView) bindingMatchesFilter(binding gcp.IAMBinding, filter string) bool {
	if strings.Contains(strings.ToLower(binding.Role), filter) {
		return true
	}
	for _, member := range binding.Members {
		if strings.Contains(strings.ToLower(member), filter) {
			return true
		}
	}
	return false
}

func (v *IAMPolicyView) memberMatchesFilter(member string, roles []string, filter string) bool {
	if strings.Contains(strings.ToLower(member), filter) {
		return true
	}
	for _, role := range roles {
		if strings.Contains(strings.ToLower(role), filter) {
			return true
		}
	}
	return false
}

// Task registration helpers
func (v *IAMPolicyView) registerTask(id, description string) {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskRunning,
			StartTime:   time.Now(),
		}
	}
}

func (v *IAMPolicyView) clearTask(id string) {
	if v.ctx != nil {
		delete(v.ctx.Tasks, id)
	}
}

func (v *IAMPolicyView) failTask(id string, err error) tea.Cmd {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: err.Error(),
			State:       context.TaskError,
			Error:       err,
		}
	}
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return context.TaskClearMsg{TaskID: id}
	})
}
