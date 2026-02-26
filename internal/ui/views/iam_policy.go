package views

import (
	gocontext "context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/inputdialog"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/focus"
	"github.com/slayer/gcon/internal/ui/overlay"
)

// Tab IDs — "By Member" is now the primary tab
const (
	iamPolicyTabByMember = "by-member"
	iamPolicyTabByRole   = "by-role"
)

const (
	iamPolicyRegionTabs  = "tabs"
	iamPolicyRegionTable = "table"
)

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

// memberRole pairs a role with its optional condition title for display in the "By Member" overlay
type memberRole struct {
	role           string
	conditionTitle string
}

// memberEntry holds a member's roles for the "By Member" table
type memberEntry struct {
	fullMember string       // e.g. "user:alice@example.com"
	typeName   string       // e.g. "user", "sa", "group"
	identity   string       // e.g. "alice@example.com"
	roles      []memberRole // sorted by (role, conditionTitle)
}

// IAMPolicyView displays project IAM bindings as tables with editing
type IAMPolicyView struct {
	TableClickDelegate
	iamClient *gcp.IAMClient
	projectID string
	ctx       *context.ProgramContext

	policy  *gcp.IAMPolicy
	loading bool
	err     error

	// Derived data for lookups
	memberEntries []memberEntry // sorted by fullMember

	// UI components
	spinner     spinner.Model
	tabsComp    *tabs.Tabs
	memberTable table.Model
	roleTable   table.Model
	focusMgr    *focus.Manager

	// Overlay state — shows member's roles or role's members
	showOverlay     bool
	overlayTitle    string
	overlayItems    []string // display strings for the overlay list
	overlayBindKeys []string // parallel to overlayItems — BindingKey for each role entry (By Member overlay)
	overlayCursor   int
	overlayCtx      struct {
		role           string // set when showing role's members (By Role tab)
		conditionTitle string // condition title for the role being shown (By Role tab)
		member         string // set when showing member's roles (By Member tab)
	}

	// Dialogs
	inputDialog   *inputdialog.InputDialog
	showInput     bool
	confirmDialog *confirm.ConfirmDialog
	showConfirm   bool

	pendingRole           string
	pendingMember         string
	pendingConditionTitle string

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	width  int
	height int

	keys iamPolicyKeyMap
}

type iamPolicyKeyMap struct {
	Select     key.Binding
	Refresh    key.Binding
	Add        key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
}

func defaultIAMPolicyKeyMap() iamPolicyKeyMap {
	return iamPolicyKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "remove"),
		),
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
	}
}

func memberColumns() []table.Column {
	return []table.Column{
		{Title: "Type", Width: 12, Sortable: true},
		{Title: "Member", Width: 30, Sortable: true},
		{Title: "Roles", Width: 40, Grow: true, Sortable: true},
	}
}

func roleColumns() []table.Column {
	return []table.Column{
		{Title: "Role", Width: 40, Grow: true, Sortable: true},
		{Title: "Members", Width: 12, Sortable: true},
		{Title: "Preview", Width: 40, Sortable: false},
	}
}

// NewIAMPolicyView creates a new IAM policy view
func NewIAMPolicyView(projectID string) *IAMPolicyView {
	s := components.NewGCPSpinner()

	tabsComponent := tabs.New([]tabs.Tab{
		{ID: iamPolicyTabByMember, Label: "By Member"},
		{ID: iamPolicyTabByRole, Label: "By Role"},
	})

	mt := table.NewWithColumns(memberColumns(), fmt.Sprintf("IAM Policy - %s", projectID))
	rt := table.NewWithColumns(roleColumns(), fmt.Sprintf("IAM Policy - %s", projectID))

	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewRegion(iamPolicyRegionTabs, focus.RegionTabs, "Tabs"),
		focus.NewRegion(iamPolicyRegionTable, focus.RegionViewport, "Table"),
	})

	v := &IAMPolicyView{
		projectID:   projectID,
		spinner:     s,
		loading:     true,
		tabsComp:    tabsComponent,
		memberTable: mt,
		roleTable:   rt,
		focusMgr:    fm,
		keys:        defaultIAMPolicyKeyMap(),
	}
	// Default: "By Member" tab is active → delegate to memberTable
	v.Table = &v.memberTable
	return v
}

// Init starts loading the IAM policy
func (v *IAMPolicyView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
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

// activeTable returns a pointer to the currently active table based on tab
func (v *IAMPolicyView) activeTable() *table.Model {
	if v.tabsComp.ActiveTab().ID == iamPolicyTabByRole {
		return &v.roleTable
	}
	return &v.memberTable
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
		v.rebuildTables()
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
		// Switch TableClickDelegate to the new active table
		v.Table = v.activeTable()
		return nil

	case focus.FocusChangedMsg:
		return nil

	case table.RowDoubleClickedMsg:
		return v.handleSelect()

	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		return v.executeAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case inputdialog.InputConfirmMsg:
		v.showInput = false
		return v.handleInputConfirm(msg.Value)

	case inputdialog.InputCancelMsg:
		v.showInput = false
		return nil

	case confirm.ConfirmMsg:
		v.showConfirm = false
		return v.handleConfirm()

	case confirm.CancelMsg:
		v.showConfirm = false
		return nil

	case tea.KeyMsg:
		return v.handleKeyMsg(msg)
	}

	// Update active table for navigation
	tbl := v.activeTable()
	var cmd tea.Cmd
	*tbl, cmd = tbl.Update(msg)
	return cmd
}

//nolint:gocognit,cyclop // Key handling requires many branches
func (v *IAMPolicyView) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	// Route to input dialog when shown
	if v.showInput && v.inputDialog != nil {
		return v.inputDialog.Update(msg)
	}

	// Route to confirm dialog when shown
	if v.showConfirm && v.confirmDialog != nil {
		return v.confirmDialog.Update(msg)
	}

	// Handle overlay navigation
	if v.showOverlay {
		return v.handleOverlayKey(msg)
	}

	if v.loading {
		return nil
	}

	// Route to action menu when open
	if v.menuOpen {
		return v.actionMenu.Update(msg)
	}

	tbl := v.activeTable()

	// Delegate to table when sort menu is open
	if tbl.IsSortMenuOpen() {
		var cmd tea.Cmd
		*tbl, cmd = tbl.Update(msg)
		return cmd
	}

	// Let table handle filtering mode
	if tbl.IsFiltering() {
		var cmd tea.Cmd
		*tbl, cmd = tbl.Update(msg)
		return cmd
	}

	// Handle Tab/Shift+Tab for cycling between focus regions
	if focusMsg := v.focusMgr.HandleKey(msg); focusMsg != nil {
		return func() tea.Msg { return focusMsg }
	}

	// Route based on focused region
	switch v.focusMgr.ActiveType() {
	case focus.RegionTabs:
		if tabs.HandleKey(msg) {
			return v.tabsComp.Update(msg)
		}

	case focus.RegionViewport:
		switch {
		case key.Matches(msg, v.keys.Select):
			return v.handleSelect()

		case key.Matches(msg, v.keys.Add):
			return v.handleAdd()

		case key.Matches(msg, v.keys.ActionMenu):
			v.openActionMenu()
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			v.registerTask("load-iam-policy", "Refreshing...")
			if v.iamClient == nil {
				return tea.Batch(v.spinner.Tick, v.initIAMClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadPolicy())
		}
	}

	// Default: update active table
	var cmd tea.Cmd
	*tbl, cmd = tbl.Update(msg)
	return cmd
}

// handleSelect opens an overlay showing the selected row's details
func (v *IAMPolicyView) handleSelect() tea.Cmd {
	tbl := v.activeTable()
	row := tbl.SelectedRow()
	if row == nil {
		return nil
	}

	if v.tabsComp.ActiveTab().ID == iamPolicyTabByMember {
		// Show member's roles — each overlay item is a role with optional condition suffix
		entry := v.findMemberEntry(row.ID)
		if entry == nil {
			return nil
		}
		v.showOverlay = true
		v.overlayTitle = entry.fullMember
		v.overlayItems = make([]string, len(entry.roles))
		v.overlayBindKeys = make([]string, len(entry.roles))
		for i, mr := range entry.roles {
			v.overlayItems[i] = formatRoleWithCondition(mr.role, mr.conditionTitle)
			b := gcp.IAMBinding{Role: mr.role, ConditionTitle: mr.conditionTitle}
			v.overlayBindKeys[i] = b.BindingKey()
		}
		v.overlayCursor = 0
		v.overlayCtx.member = entry.fullMember
		v.overlayCtx.role = ""
		v.overlayCtx.conditionTitle = ""
	} else {
		// Show role's members — row.ID is now BindingKey
		binding := v.findBinding(row.ID)
		if binding == nil {
			return nil
		}
		v.showOverlay = true
		v.overlayTitle = formatRoleWithCondition(binding.Role, binding.ConditionTitle)
		v.overlayItems = make([]string, len(binding.Members))
		v.overlayBindKeys = nil
		copy(v.overlayItems, binding.Members)
		v.overlayCursor = 0
		v.overlayCtx.role = binding.Role
		v.overlayCtx.conditionTitle = binding.ConditionTitle
		v.overlayCtx.member = ""
	}
	return nil
}

// handleAdd opens the appropriate input dialog for adding
func (v *IAMPolicyView) handleAdd() tea.Cmd {
	if v.showOverlay {
		return v.handleOverlayAdd()
	}

	tbl := v.activeTable()
	row := tbl.SelectedRow()
	if row == nil {
		return nil
	}

	if v.tabsComp.ActiveTab().ID == iamPolicyTabByRole {
		// Add member to selected role — row.ID is BindingKey
		role, condTitle := gcp.ParseBindingKey(row.ID)
		v.pendingRole = role
		v.pendingConditionTitle = condTitle
		v.pendingMember = ""
		v.inputDialog = inputdialog.New(
			"Add Member",
			fmt.Sprintf("Add member to %s", shortRoleName(role)),
			"user:email@example.com",
		).SetValidator(validateIAMMember)
		v.showInput = true
		return v.inputDialog.Init()
	}

	// By Member tab: add role to selected member — new bindings are always unconditioned
	v.pendingMember = row.ID
	v.pendingRole = ""
	v.pendingConditionTitle = ""
	v.inputDialog = inputdialog.New(
		"Add Role",
		fmt.Sprintf("Add role to %s", row.ID),
		"roles/viewer",
	).SetValidator(validateIAMRole)
	v.showInput = true
	return v.inputDialog.Init()
}

// handleOverlayAdd adds from within the overlay context
func (v *IAMPolicyView) handleOverlayAdd() tea.Cmd {
	if v.overlayCtx.role != "" {
		// Viewing role's members → add another member to same binding
		v.pendingRole = v.overlayCtx.role
		v.pendingConditionTitle = v.overlayCtx.conditionTitle
		v.pendingMember = ""
		v.inputDialog = inputdialog.New(
			"Add Member",
			fmt.Sprintf("Add member to %s", shortRoleName(v.overlayCtx.role)),
			"user:email@example.com",
		).SetValidator(validateIAMMember)
		v.showInput = true
		return v.inputDialog.Init()
	}

	// Viewing member's roles → add another role (new bindings are unconditioned)
	v.pendingMember = v.overlayCtx.member
	v.pendingRole = ""
	v.pendingConditionTitle = ""
	v.inputDialog = inputdialog.New(
		"Add Role",
		fmt.Sprintf("Add role to %s", v.overlayCtx.member),
		"roles/viewer",
	).SetValidator(validateIAMRole)
	v.showInput = true
	return v.inputDialog.Init()
}

// handleOverlayKey handles navigation inside the overlay
func (v *IAMPolicyView) handleOverlayKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case msg.Type == tea.KeyEscape:
		v.showOverlay = false
		return nil

	case msg.String() == "j" || msg.Type == tea.KeyDown:
		if v.overlayCursor < len(v.overlayItems)-1 {
			v.overlayCursor++
		}
		return nil

	case msg.String() == "k" || msg.Type == tea.KeyUp:
		if v.overlayCursor > 0 {
			v.overlayCursor--
		}
		return nil

	case key.Matches(msg, v.keys.Add):
		return v.handleOverlayAdd()

	case key.Matches(msg, v.keys.Delete):
		return v.handleOverlayRemove()
	}
	return nil
}

// handleOverlayRemove initiates removal of the selected overlay item
func (v *IAMPolicyView) handleOverlayRemove() tea.Cmd {
	if v.overlayCursor < 0 || v.overlayCursor >= len(v.overlayItems) {
		return nil
	}
	selected := v.overlayItems[v.overlayCursor]

	if v.overlayCtx.role != "" {
		// Removing member from role — condition comes from overlay context
		v.pendingRole = v.overlayCtx.role
		v.pendingConditionTitle = v.overlayCtx.conditionTitle
		v.pendingMember = selected

		v.confirmDialog = confirm.New(
			"Remove Member",
			fmt.Sprintf("Remove %s from %s?", selected, shortRoleName(v.overlayCtx.role)),
			nil,
		)
		v.showConfirm = true
		return nil
	}

	// Removing role from member — look up condition from the parallel binding keys
	if v.overlayCursor >= len(v.overlayBindKeys) {
		return nil // defensive: overlayBindKeys should always be populated in parallel
	}
	role, condTitle := gcp.ParseBindingKey(v.overlayBindKeys[v.overlayCursor])
	v.pendingRole = role
	v.pendingConditionTitle = condTitle
	v.pendingMember = v.overlayCtx.member

	v.confirmDialog = confirm.New(
		"Remove Role",
		fmt.Sprintf("Remove %s from %s?", shortRoleName(role), v.overlayCtx.member),
		nil,
	)
	v.showConfirm = true
	return nil
}

// handleInputConfirm processes input dialog confirmation
func (v *IAMPolicyView) handleInputConfirm(value string) tea.Cmd {
	role := v.pendingRole
	member := v.pendingMember
	condTitle := v.pendingConditionTitle

	if role == "" {
		// Adding a role to a member — new binding is unconditioned
		role = value
		condTitle = ""
	} else {
		// Adding a member to a role
		member = value
	}

	v.showOverlay = false
	return func() tea.Msg {
		return AddIAMBindingMsg{
			ProjectID:      v.projectID,
			Role:           role,
			ConditionTitle: condTitle,
			Member:         member,
		}
	}
}

// handleConfirm processes confirm dialog confirmation
func (v *IAMPolicyView) handleConfirm() tea.Cmd {
	role := v.pendingRole
	member := v.pendingMember
	condTitle := v.pendingConditionTitle
	v.showOverlay = false

	return func() tea.Msg {
		return RemoveIAMBindingMsg{
			ProjectID:      v.projectID,
			Role:           role,
			ConditionTitle: condTitle,
			Member:         member,
		}
	}
}

// openActionMenu builds and shows the action menu
func (v *IAMPolicyView) openActionMenu() {
	actions := []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
	}

	tbl := v.activeTable()
	row := tbl.SelectedRow()
	if row != nil {
		if v.tabsComp.ActiveTab().ID == iamPolicyTabByRole {
			actions = append(actions, actionmenu.Action{
				Key: 'a', Label: "Add member to role", Enabled: true,
			})
		} else {
			actions = append(actions, actionmenu.Action{
				Key: 'a', Label: "Add role to member", Enabled: true,
			})
		}
	}

	v.actionMenu = actionmenu.New("IAM Policy Actions", actions)
	v.menuOpen = true
}

// executeAction performs the action selected from the menu
func (v *IAMPolicyView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'r':
		v.loading = true
		v.err = nil
		v.registerTask("load-iam-policy", "Refreshing...")
		if v.iamClient == nil {
			return tea.Batch(v.spinner.Tick, v.initIAMClient())
		}
		return tea.Batch(v.spinner.Tick, v.loadPolicy())
	case 'a':
		return v.handleAdd()
	}
	return nil
}

// UpdatePolicy replaces the policy and rebuilds tables (called by app on successful update)
func (v *IAMPolicyView) UpdatePolicy(policy *gcp.IAMPolicy) {
	v.policy = policy
	v.rebuildTables()
}

// SetError resets error state (called by app handlers)
func (v *IAMPolicyView) SetError(err error) {
	v.loading = false
	v.showOverlay = false
	v.showInput = false
	v.showConfirm = false
	v.err = err
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

	// Tab bar with focus accent
	tabBar := focus.RenderAccent("  "+v.tabsComp.View(), v.focusMgr.IsActive(iamPolicyRegionTabs))

	// Active table
	tbl := v.activeTable()
	tableView := tbl.View()

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • a: add • .: actions • S: sort • /: filter • r: refresh • Tab: focus • h/l: tabs • esc: back")

	mainContent := tabBar + "\n" + tableView + help

	// Overlay dialogs on top
	if v.showOverlay {
		overlayContent := v.renderOverlay()
		contentHeight := lipgloss.Height(mainContent)
		return overlay.Center(mainContent, overlayContent, v.width, contentHeight)
	}

	if v.showInput && v.inputDialog != nil {
		contentHeight := lipgloss.Height(mainContent)
		return overlay.Center(mainContent, v.inputDialog.View(), v.width, contentHeight)
	}

	if v.showConfirm && v.confirmDialog != nil {
		contentHeight := lipgloss.Height(mainContent)
		return overlay.Center(mainContent, v.confirmDialog.View(), v.width, contentHeight)
	}

	if v.menuOpen && v.actionMenu != nil {
		contentHeight := lipgloss.Height(mainContent)
		return overlay.Center(mainContent, v.actionMenu.View(), v.width, contentHeight)
	}

	return mainContent
}

// renderOverlay renders the detail overlay for a member's roles or role's members
func (v *IAMPolicyView) renderOverlay() string {
	containerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4285F4")).
		Padding(1, 2).
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4285F4")).
		Bold(true)

	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E8EAED"))

	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4285F4")).
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9AA0A6"))

	var b strings.Builder
	b.WriteString(titleStyle.Render(v.overlayTitle))
	b.WriteString("\n\n")

	for i, item := range v.overlayItems {
		if i == v.overlayCursor {
			b.WriteString(cursorStyle.Render("▸ " + item))
		} else {
			b.WriteString(itemStyle.Render("  " + item))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k: navigate • a: add • d: remove • esc: close"))

	return containerStyle.Render(b.String())
}

// SetContext updates the view with shared program context
func (v *IAMPolicyView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.memberTable.SetSize(ctx.ContentWidth, ctx.ContentHeight-4)
	v.roleTable.SetSize(ctx.ContentWidth, ctx.ContentHeight-4)
}

// GetIAMClient returns the IAM client for reuse
func (v *IAMPolicyView) GetIAMClient() *gcp.IAMClient {
	return v.iamClient
}

// HasTextInputFocused returns true when any text input is active
func (v *IAMPolicyView) HasTextInputFocused() bool {
	if v.showInput && v.inputDialog != nil {
		return true
	}
	return v.activeTable().HasTextInputFocused()
}

// IsMenuOpen returns true if an overlay or menu is open
func (v *IAMPolicyView) IsMenuOpen() bool {
	return v.menuOpen || v.showOverlay || v.showInput || v.showConfirm
}

// --- Table building ---

func (v *IAMPolicyView) rebuildTables() {
	if v.policy == nil {
		return
	}
	v.buildMemberEntries()
	v.buildMemberTable()
	v.buildRoleTable()
}

// buildMemberEntries inverts the policy bindings to member→roles (with condition info)
func (v *IAMPolicyView) buildMemberEntries() {
	memberRolesMap := make(map[string][]memberRole)
	for _, b := range v.policy.Bindings {
		for _, m := range b.Members {
			memberRolesMap[m] = append(memberRolesMap[m], memberRole{
				role:           b.Role,
				conditionTitle: b.ConditionTitle,
			})
		}
	}

	members := make([]string, 0, len(memberRolesMap))
	for m := range memberRolesMap {
		members = append(members, m)
	}
	sort.Strings(members)

	v.memberEntries = make([]memberEntry, 0, len(members))
	for _, m := range members {
		roles := memberRolesMap[m]
		sort.Slice(roles, func(i, j int) bool {
			if roles[i].role != roles[j].role {
				return roles[i].role < roles[j].role
			}
			return roles[i].conditionTitle < roles[j].conditionTitle
		})
		typeName, identity := gcp.ParseMemberType(m)
		v.memberEntries = append(v.memberEntries, memberEntry{
			fullMember: m,
			typeName:   typeName,
			identity:   identity,
			roles:      roles,
		})
	}
}

func (v *IAMPolicyView) buildMemberTable() {
	rows := make([]table.Row, 0, len(v.memberEntries))
	for _, e := range v.memberEntries {
		rolesDisplay := formatRolesColumn(e.roles)
		// FilterValue includes type, identity, and all role names for matching
		filterParts := []string{e.typeName, e.identity}
		for _, mr := range e.roles {
			filterParts = append(filterParts, mr.role)
		}
		rows = append(rows, table.Row{
			Data:        []string{e.typeName, e.identity, rolesDisplay},
			FilterValue: strings.Join(filterParts, " "),
			ID:          e.fullMember,
		})
	}
	v.memberTable.SetRows(rows)
}

func (v *IAMPolicyView) buildRoleTable() {
	rows := make([]table.Row, 0, len(v.policy.Bindings))
	for _, b := range v.policy.Bindings {
		membersCount := fmt.Sprintf("%d members", len(b.Members))
		if len(b.Members) == 1 {
			membersCount = "1 member"
		}
		preview := truncatePreview(b.Members, 40)
		roleDisplay := formatRoleWithCondition(b.Role, b.ConditionTitle)

		// FilterValue includes role, condition title, and all member names
		filterParts := []string{b.Role}
		if b.ConditionTitle != "" {
			filterParts = append(filterParts, b.ConditionTitle)
		}
		filterParts = append(filterParts, b.Members...)
		rows = append(rows, table.Row{
			Data:        []string{roleDisplay, membersCount, preview},
			FilterValue: strings.Join(filterParts, " "),
			ID:          b.BindingKey(),
		})
	}
	v.roleTable.SetRows(rows)
}

// --- Lookup helpers ---

func (v *IAMPolicyView) findMemberEntry(fullMember string) *memberEntry {
	for i := range v.memberEntries {
		if v.memberEntries[i].fullMember == fullMember {
			return &v.memberEntries[i]
		}
	}
	return nil
}

func (v *IAMPolicyView) findBinding(bindingKey string) *gcp.IAMBinding {
	if v.policy == nil {
		return nil
	}
	for i := range v.policy.Bindings {
		if v.policy.Bindings[i].BindingKey() == bindingKey {
			return &v.policy.Bindings[i]
		}
	}
	return nil
}

// --- Display helpers ---

func formatRolesColumn(roles []memberRole) string {
	short := make([]string, len(roles))
	for i, mr := range roles {
		short[i] = formatRoleWithCondition(shortRoleName(mr.role), mr.conditionTitle)
	}
	return strings.Join(short, ", ")
}

// formatRoleWithCondition appends a parenthesized condition title when present
func formatRoleWithCondition(role, conditionTitle string) string {
	if conditionTitle == "" {
		return role
	}
	return role + " (" + conditionTitle + ")"
}

// shortRoleName strips the "roles/" prefix for brevity
func shortRoleName(role string) string {
	if after, ok := strings.CutPrefix(role, "roles/"); ok {
		return after
	}
	return role
}

func truncatePreview(members []string, maxLen int) string {
	preview := strings.Join(members, ", ")
	runes := []rune(preview)
	if len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return preview
}

// --- Validators ---

var (
	errExpectedString    = errors.New("expected string")
	errMemberIdentEmpty  = errors.New("member identity cannot be empty")
	errInvalidMemberType = errors.New("must start with user:, serviceAccount:, group:, domain:, or deleted: prefix")
	errInvalidRolePrefix = errors.New("must start with roles/, projects/, or organizations/")
)

func validateIAMMember(value any) error {
	s, ok := value.(string)
	if !ok {
		return errExpectedString
	}
	validPrefixes := []string{"user:", "serviceAccount:", "group:", "domain:", "deleted:"}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(s, prefix) {
			// Must have something after the prefix
			rest := s[len(prefix):]
			if rest == "" {
				return errMemberIdentEmpty
			}
			return nil
		}
	}
	return errInvalidMemberType
}

var errRoleIDEmpty = errors.New("role ID cannot be empty (e.g. use roles/viewer)")

func validateIAMRole(value any) error {
	s, ok := value.(string)
	if !ok {
		return errExpectedString
	}
	// Must have a valid prefix with a non-empty ID segment after it
	for _, prefix := range []string{"roles/", "projects/", "organizations/"} {
		if strings.HasPrefix(s, prefix) {
			if len(s) <= len(prefix) {
				return errRoleIDEmpty
			}
			return nil
		}
	}
	return errInvalidRolePrefix
}

// --- Task registration helpers ---

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
