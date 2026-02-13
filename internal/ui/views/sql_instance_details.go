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
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/focus"
	"github.com/slayer/gcon/internal/ui/mouse"
	"github.com/slayer/gcon/internal/ui/overlay"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// Tab IDs for SQL instance details view
const (
	sqlTabIDDetails   = "details"
	sqlTabIDDatabases = "databases"
	sqlTabIDBackups   = "backups"
)

// Focus region IDs for SQL instance details view
const (
	sqlRegionIDTabs     = "tabs"
	sqlRegionIDViewport = "viewport"
)

// Lines reserved for tab bar + separator + help text
const sqlDetailsViewportReservedLines = 5

// Internal messages for async data loading
type sqlInstanceDetailsLoadedMsg struct {
	details *gcp.SQLInstanceDetails
}

type sqlInstanceDetailsErrorMsg struct {
	err error
}

type sqlDatabasesLoadedMsg struct {
	databases []gcp.SQLDatabase
}

type sqlDatabasesErrorMsg struct {
	err error
}

type sqlBackupsLoadedMsg struct {
	backups []gcp.SQLBackupRun
}

type sqlBackupsErrorMsg struct {
	err error
}

// SQLInstanceDetailsView displays comprehensive SQL instance information with tabs
type SQLInstanceDetailsView struct {
	sqlClient    *gcp.SQLClient
	projectID    string
	instanceName string
	ctx          *context.ProgramContext

	// Data — each dataset loads independently
	details   *gcp.SQLInstanceDetails
	databases []gcp.SQLDatabase
	backups   []gcp.SQLBackupRun

	// Separate loading/error state per dataset so partial loads don't block the UI
	detailsLoading   bool
	databasesLoading bool
	backupsLoading   bool
	detailsErr       error
	databasesErr     error
	backupsErr       error

	// UI state
	spinner spinner.Model
	width   int
	height  int
	ready   bool

	// Tab navigation (Details / Databases / Backups)
	tabs         *tabs.Tabs
	tabViewports []viewport.Model // Separate viewport per tab to preserve scroll

	// Focus management
	focusMgr  *focus.Manager
	regionMgr *mouse.RegionManager

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool

	keys sqlInstanceDetailsKeyMap
}

type sqlInstanceDetailsKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Refresh      key.Binding
	Start        key.Binding
	Stop         key.Binding
	Restart      key.Binding
	Delete       key.Binding
	CreateBackup key.Binding
	ActionMenu   key.Binding
}

func defaultSQLInstanceDetailsKeyMap() sqlInstanceDetailsKeyMap {
	return sqlInstanceDetailsKeyMap{
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
		Start: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "start"),
		),
		Stop: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "stop"),
		),
		Restart: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "restart"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
		CreateBackup: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "create backup"),
		),
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
	}
}

// NewSQLInstanceDetailsView creates a new SQL instance details view
func NewSQLInstanceDetailsView(projectID, instanceName string, sqlClient *gcp.SQLClient) *SQLInstanceDetailsView {
	s := components.NewGCPSpinner()

	tabsComponent := tabs.New([]tabs.Tab{
		{ID: sqlTabIDDetails, Label: "Details"},
		{ID: sqlTabIDDatabases, Label: "Databases"},
		{ID: sqlTabIDBackups, Label: "Backups"},
	})

	// Only tabs and viewport — no navigable links needed for SQL details
	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewRegion(sqlRegionIDTabs, focus.RegionTabs, "Tabs"),
		focus.NewRegion(sqlRegionIDViewport, focus.RegionViewport, "Content"),
	})

	return &SQLInstanceDetailsView{
		sqlClient:        sqlClient,
		projectID:        projectID,
		instanceName:     instanceName,
		spinner:          s,
		detailsLoading:   true,
		databasesLoading: true,
		backupsLoading:   true,
		keys:             defaultSQLInstanceDetailsKeyMap(),
		tabs:             tabsComponent,
		tabViewports:     make([]viewport.Model, 3),
		focusMgr:         fm,
		regionMgr:        mouse.NewRegionManager(),
	}
}

// Init starts loading all datasets in parallel
func (v *SQLInstanceDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
		v.loadDatabases(),
		v.loadBackups(),
	)
}

func (v *SQLInstanceDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		if v.sqlClient == nil {
			return sqlInstanceDetailsErrorMsg{err: uierrors.ErrSQLClientNotInitialized}
		}
		details, err := v.sqlClient.GetInstance(gocontext.Background(), v.projectID, v.instanceName)
		if err != nil {
			return sqlInstanceDetailsErrorMsg{err: err}
		}
		return sqlInstanceDetailsLoadedMsg{details: details}
	}
}

func (v *SQLInstanceDetailsView) loadDatabases() tea.Cmd {
	return func() tea.Msg {
		if v.sqlClient == nil {
			return sqlDatabasesErrorMsg{err: uierrors.ErrSQLClientNotInitialized}
		}
		databases, err := v.sqlClient.ListDatabases(gocontext.Background(), v.projectID, v.instanceName)
		if err != nil {
			return sqlDatabasesErrorMsg{err: err}
		}
		return sqlDatabasesLoadedMsg{databases: databases}
	}
}

func (v *SQLInstanceDetailsView) loadBackups() tea.Cmd {
	return func() tea.Msg {
		if v.sqlClient == nil {
			return sqlBackupsErrorMsg{err: uierrors.ErrSQLClientNotInitialized}
		}
		backups, err := v.sqlClient.ListBackupRuns(gocontext.Background(), v.projectID, v.instanceName)
		if err != nil {
			return sqlBackupsErrorMsg{err: err}
		}
		return sqlBackupsLoadedMsg{backups: backups}
	}
}

// Update handles messages for the SQL instance details view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern - complexity expected
func (v *SQLInstanceDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case sqlInstanceDetailsLoadedMsg:
		v.detailsLoading = false
		v.details = msg.details
		v.updateViewportContent()
		return nil

	case sqlInstanceDetailsErrorMsg:
		v.detailsLoading = false
		v.detailsErr = msg.err
		v.updateViewportContent()
		return nil

	case sqlDatabasesLoadedMsg:
		v.databasesLoading = false
		v.databases = msg.databases
		v.updateViewportContent()
		return nil

	case sqlDatabasesErrorMsg:
		v.databasesLoading = false
		v.databasesErr = msg.err
		v.updateViewportContent()
		return nil

	case sqlBackupsLoadedMsg:
		v.backupsLoading = false
		v.backups = msg.backups
		v.updateViewportContent()
		return nil

	case sqlBackupsErrorMsg:
		v.backupsLoading = false
		v.backupsErr = msg.err
		v.updateViewportContent()
		return nil

	case spinner.TickMsg:
		if v.detailsLoading || v.databasesLoading || v.backupsLoading {
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
				return DeleteSQLInstanceConfirmedMsg{
					InstanceName: v.details.Name,
				}
			}
		}
		return nil

	case confirm.TypeCancelMsg:
		v.showDeleteConfirm = false
		return nil

	case tabs.TabChangedMsg:
		v.updateViewportContent()
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

		// Route keys based on currently focused region
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

		// View-specific action keys (work regardless of focus)
		switch {
		case key.Matches(msg, v.keys.ActionMenu):
			if v.details != nil {
				v.actionMenu = actionmenu.New("SQL Instance Actions", v.buildActions())
				v.menuOpen = true
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			return v.refresh()

		case key.Matches(msg, v.keys.Start):
			return v.startInstance()

		case key.Matches(msg, v.keys.Stop):
			return v.stopInstance()

		case key.Matches(msg, v.keys.Restart):
			return v.restartInstance()

		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()

		case key.Matches(msg, v.keys.CreateBackup):
			// Only allow backup creation from the Backups tab
			if v.tabs.ActiveTab().ID == sqlTabIDBackups && v.details != nil {
				return v.createBackup()
			}
			return nil
		}
	}

	return nil
}

func (v *SQLInstanceDetailsView) buildActions() []actionmenu.Action {
	actions := []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
	}

	if v.details != nil {
		if v.details.State == "STOPPED" || v.details.State == "SUSPENDED" {
			actions = append(actions, actionmenu.Action{Key: 's', Label: "Start", Enabled: true})
		}
		if v.details.State == "RUNNABLE" {
			actions = append(actions, actionmenu.Action{Key: 'x', Label: "Stop", Enabled: true})
			actions = append(actions, actionmenu.Action{Key: 'R', Label: "Restart", Enabled: true})
		}
		actions = append(actions, actionmenu.Action{Key: 'b', Label: "Create Backup", Enabled: true})
		actions = append(actions, actionmenu.Action{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true})
	}

	return actions
}

func (v *SQLInstanceDetailsView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'r':
		return v.refresh()
	case 's':
		return v.startInstance()
	case 'x':
		return v.stopInstance()
	case 'R':
		return v.restartInstance()
	case 'b':
		return v.createBackup()
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

func (v *SQLInstanceDetailsView) refresh() tea.Cmd {
	v.detailsLoading = true
	v.databasesLoading = true
	v.backupsLoading = true
	v.detailsErr = nil
	v.databasesErr = nil
	v.backupsErr = nil
	return tea.Batch(v.spinner.Tick, v.loadDetails(), v.loadDatabases(), v.loadBackups())
}

func (v *SQLInstanceDetailsView) startInstance() tea.Cmd {
	if v.details == nil || (v.details.State != "STOPPED" && v.details.State != "SUSPENDED") {
		return nil
	}
	return func() tea.Msg {
		return SQLInstanceActionMsg{InstanceName: v.details.Name, Action: "start"}
	}
}

func (v *SQLInstanceDetailsView) stopInstance() tea.Cmd {
	if v.details == nil || v.details.State != "RUNNABLE" {
		return nil
	}
	return func() tea.Msg {
		return SQLInstanceActionMsg{InstanceName: v.details.Name, Action: "stop"}
	}
}

func (v *SQLInstanceDetailsView) restartInstance() tea.Cmd {
	if v.details == nil || v.details.State != "RUNNABLE" {
		return nil
	}
	return func() tea.Msg {
		return SQLInstanceActionMsg{InstanceName: v.details.Name, Action: "restart"}
	}
}

func (v *SQLInstanceDetailsView) createBackup() tea.Cmd {
	if v.details == nil {
		return nil
	}
	return func() tea.Msg {
		return CreateSQLBackupMsg{InstanceName: v.details.Name, Description: "On-demand backup"}
	}
}

func (v *SQLInstanceDetailsView) initiateDelete() tea.Cmd {
	if v.details == nil {
		return nil
	}

	detailLines := []string{
		fmt.Sprintf("Version: %s", gcp.FormatDatabaseVersion(v.details.DatabaseVersion)),
		fmt.Sprintf("Region: %s", v.details.Region),
		fmt.Sprintf("Tier: %s", v.details.Tier),
	}

	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete SQL Instance", v.details.Name, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

// View renders the SQL instance details view
func (v *SQLInstanceDetailsView) View() string {
	// Show loading until at least details are loaded
	if v.detailsLoading && v.details == nil {
		return renderLoading(v.spinner, "Loading SQL instance details...")
	}

	if v.detailsErr != nil && v.details == nil {
		return renderLoading(v.spinner, fmt.Sprintf("Error: %v\n  Press 'esc' to go back", v.detailsErr))
	}

	if v.details == nil {
		return renderLoading(v.spinner, "No SQL instance details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return renderLoading(v.spinner, "Initializing view...")
	}

	// Render tab bar with focus accent
	tabBar := focus.RenderAccent("  "+v.tabs.View(), v.focusMgr.IsActive(sqlRegionIDTabs))

	// Get active tab viewport with focus accent
	activeIdx := v.tabs.ActiveIndex()
	var viewportContent string
	var scrollPercent float64
	if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
		viewportContent = focus.RenderAccent(v.tabViewports[activeIdx].View(), v.focusMgr.IsActive(sqlRegionIDViewport))
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

func (v *SQLInstanceDetailsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// SetContext updates the view with shared program context
func (v *SQLInstanceDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// IsMenuOpen returns true if the action menu or delete confirm is open
func (v *SQLInstanceDetailsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetInstanceName returns the instance name for breadcrumbs
func (v *SQLInstanceDetailsView) GetInstanceName() string {
	return v.instanceName
}

// GetSQLClient returns the SQL client for reuse
func (v *SQLInstanceDetailsView) GetSQLClient() *gcp.SQLClient {
	return v.sqlClient
}

// HasTextInputFocused returns true if the delete confirm input is active.
// Prevents global hotkeys (like 'q' for quit) from triggering while typing.
func (v *SQLInstanceDetailsView) HasTextInputFocused() bool {
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return false
}

func (v *SQLInstanceDetailsView) applySize(width, height int) {
	viewportHeight := height - sqlDetailsViewportReservedLines
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

func (v *SQLInstanceDetailsView) updateViewportContent() {
	if !v.ready {
		return
	}

	activeIdx := v.tabs.ActiveIndex()
	if activeIdx < 0 || activeIdx >= len(v.tabViewports) {
		return
	}

	var content string
	switch v.tabs.ActiveTab().ID {
	case sqlTabIDDetails:
		content = v.renderDetailsTab()
	case sqlTabIDDatabases:
		content = v.renderDatabasesTab()
	case sqlTabIDBackups:
		content = v.renderBackupsTab()
	default:
		content = v.renderDetailsTab()
	}

	v.tabViewports[activeIdx].SetContent(content)
}

// renderDetailsTab generates the Details tab content
func (v *SQLInstanceDetailsView) renderDetailsTab() string {
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
	b.WriteString(titleStyle.Render(fmt.Sprintf("SQL Instance: %s", d.Name)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Instance Information
	b.WriteString(sectionStyle.Render("Instance Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Database Version", gcp.FormatDatabaseVersion(d.DatabaseVersion)))

	// State with color
	var stateStr string
	switch d.State {
	case "RUNNABLE":
		stateStr = enabledStyle.Render("RUNNABLE")
	case "STOPPED", "SUSPENDED":
		stateStr = disabledStyle.Render(d.State)
	default:
		stateStr = valueStyle.Render(d.State)
	}
	b.WriteString(labelStyle.Render("State:") + " " + stateStr + "\n")

	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Region", d.Region))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Connection Name", defaultIfEmpty(d.ConnectionName, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", timeutil.FormatTimestamp(d.CreatedAt)))
	b.WriteString("\n")

	// Configuration
	b.WriteString(sectionStyle.Render("Configuration"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Tier", d.Tier))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Disk Size / Disk Type",
		fmt.Sprintf("%d GB / %s", d.DiskSizeGB, defaultIfEmpty(d.DiskType, "—"))))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Availability Type", defaultIfEmpty(d.AvailabilityType, "—")))

	autoResizeStr := disabledStyle.Render("No")
	if d.StorageAutoResize {
		autoResizeStr = enabledStyle.Render("Yes")
	}
	b.WriteString(labelStyle.Render("Storage Auto-Resize:") + " " + autoResizeStr + "\n")
	b.WriteString("\n")

	// Backup Configuration
	b.WriteString(sectionStyle.Render("Backup Configuration"))
	b.WriteString("\n")
	b.WriteString(v.renderBoolRow(labelStyle, enabledStyle, disabledStyle, "Backup Enabled", d.BackupEnabled))
	b.WriteString(v.renderBoolRow(labelStyle, enabledStyle, disabledStyle, "Binary Log Enabled", d.BinaryLogEnabled))
	b.WriteString(v.renderBoolRow(labelStyle, enabledStyle, disabledStyle, "PITR Enabled", d.PITREnabled))
	b.WriteString("\n")

	// Maintenance
	b.WriteString(sectionStyle.Render("Maintenance"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Window", defaultIfEmpty(d.MaintenanceWindow, "—")))
	b.WriteString("\n")

	// Networking
	b.WriteString(sectionStyle.Render("Networking"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Public IP", defaultIfEmpty(d.PrimaryIP, "—")))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Private IP", defaultIfEmpty(d.PrivateIP, "—")))
	for _, ip := range d.IPs {
		// Skip primary/private since they're already shown above
		if ip.Type == "PRIMARY" || ip.Type == "PRIVATE" {
			continue
		}
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, ip.Type+" IP", ip.IPAddress))
	}
	b.WriteString("\n")

	// Replication — only show if there's replication data
	if d.MasterInstanceName != "" || len(d.ReplicaNames) > 0 {
		b.WriteString(sectionStyle.Render("Replication"))
		b.WriteString("\n")
		if d.MasterInstanceName != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Master Instance", d.MasterInstanceName))
		}
		if len(d.ReplicaNames) > 0 {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Replica Names", strings.Join(d.ReplicaNames, ", ")))
		}
		b.WriteString("\n")
	}

	// Database Flags — only show if flags exist, sorted alphabetically
	if len(d.DatabaseFlags) > 0 {
		b.WriteString(sectionStyle.Render("Database Flags"))
		b.WriteString("\n")

		keys := make([]string, 0, len(d.DatabaseFlags))
		for k := range d.DatabaseFlags {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, k, d.DatabaseFlags[k]))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderBoolRow renders a label + Yes/No value with green/red coloring
func (v *SQLInstanceDetailsView) renderBoolRow(labelStyle, enabledStyle, disabledStyle lipgloss.Style, label string, value bool) string { //nolint:gocritic // Style params acceptable
	str := disabledStyle.Render("No")
	if value {
		str = enabledStyle.Render("Yes")
	}
	return labelStyle.Render(label+":") + " " + str + "\n"
}

// renderDatabasesTab generates the Databases tab content
func (v *SQLInstanceDetailsView) renderDatabasesTab() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))

	if v.databasesLoading {
		b.WriteString(fmt.Sprintf("\n  %s Loading databases...\n", v.spinner.View()))
		return b.String()
	}

	if v.databasesErr != nil {
		b.WriteString("\n  " + errorStyle.Render(fmt.Sprintf("Error: %v", v.databasesErr)) + "\n")
		return b.String()
	}

	if len(v.databases) == 0 {
		b.WriteString("\n  " + mutedStyle.Render("No databases found") + "\n")
		return b.String()
	}

	// Column widths — compute dynamically based on data
	nameWidth := len("Name")
	charsetWidth := len("Charset")
	collationWidth := len("Collation")
	for _, db := range v.databases {
		if len(db.Name) > nameWidth {
			nameWidth = len(db.Name)
		}
		if len(db.Charset) > charsetWidth {
			charsetWidth = len(db.Charset)
		}
		if len(db.Collation) > collationWidth {
			collationWidth = len(db.Collation)
		}
	}

	// Header
	header := fmt.Sprintf("  %-*s  %-*s  %s", nameWidth, "Name", charsetWidth, "Charset", "Collation")
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString("  " + strings.Repeat("─", nameWidth+charsetWidth+collationWidth+4))
	b.WriteString("\n")

	// Rows
	for _, db := range v.databases {
		b.WriteString(fmt.Sprintf("  %-*s  %-*s  %s\n",
			nameWidth, db.Name,
			charsetWidth, defaultIfEmpty(db.Charset, "—"),
			defaultIfEmpty(db.Collation, "—")))
	}

	return b.String()
}

// renderBackupsTab generates the Backups tab content
func (v *SQLInstanceDetailsView) renderBackupsTab() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	enabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	disabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	if v.backupsLoading {
		b.WriteString(fmt.Sprintf("\n  %s Loading backups...\n", v.spinner.View()))
		return b.String()
	}

	if v.backupsErr != nil {
		b.WriteString("\n  " + errorStyle.Render(fmt.Sprintf("Error: %v", v.backupsErr)) + "\n")
		return b.String()
	}

	if len(v.backups) == 0 {
		b.WriteString("\n  " + mutedStyle.Render("No backup runs found") + "\n")
		b.WriteString("\n  " + helpStyle.Render("b: create on-demand backup") + "\n")
		return b.String()
	}

	// Table header
	header := fmt.Sprintf("  %-10s  %-12s  %-12s  %-20s  %s", "ID", "Status", "Type", "Start Time", "End Time")
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString("  " + strings.Repeat("─", 80))
	b.WriteString("\n")

	// Rows with colored status
	for _, backup := range v.backups {
		var statusStr string
		switch backup.Status {
		case "SUCCESSFUL":
			statusStr = enabledStyle.Render("SUCCESSFUL")
		case "FAILED", "SKIPPED":
			statusStr = disabledStyle.Render(backup.Status)
		default:
			statusStr = warningStyle.Render(backup.Status)
		}

		startTime := timeutil.FormatTimestamp(backup.StartTime)
		endTime := timeutil.FormatTimestamp(backup.EndTime)

		// Use ID as string for display
		idStr := strconv.FormatInt(backup.ID, 10)

		b.WriteString(fmt.Sprintf("  %-10s  %-12s  %-12s  %-20s  %s\n",
			idStr,
			statusStr,
			backup.Type,
			startTime,
			endTime))
	}

	// Help hint at bottom
	b.WriteString("\n  " + helpStyle.Render("b: create on-demand backup") + "\n")

	return b.String()
}

// buildHelpText generates context-sensitive help text
func (v *SQLInstanceDetailsView) buildHelpText() string {
	bindings := focus.HelpForRegion(v.focusMgr.ActiveType(), "")
	helpStr := focus.FormatHelp(bindings)
	badge := focus.FormatRegionBadge(v.focusMgr.Active())

	// Build action hints based on instance state
	actionHints := ".: actions"
	if v.details != nil {
		switch v.details.State {
		case "RUNNABLE":
			actionHints += " • x: stop • R: restart"
		case "STOPPED", "SUSPENDED":
			actionHints += " • s: start"
		}
	}
	actionHints += " • D: delete"

	if badge != "" {
		return "\n  " + badge + " • " + helpStr + " • " + actionHints
	}
	return "\n  " + helpStr + " • " + actionHints
}

// UpdateRegions calculates clickable regions for tabs and viewport
func (v *SQLInstanceDetailsView) UpdateRegions(offsetX, offsetY int) {
	v.regionMgr.Clear()

	if !v.ready || (v.detailsLoading && v.details == nil) {
		return
	}

	y := offsetY

	// Tab bar region
	tabHeight := 1
	v.regionMgr.Add(sqlRegionIDTabs, mouse.Rect{
		X:      offsetX,
		Y:      y,
		Width:  v.width,
		Height: tabHeight,
	}, nil)
	y += tabHeight + 1

	// Viewport region
	viewportHeight := v.height - (y - offsetY)
	if viewportHeight > 0 {
		v.regionMgr.Add(sqlRegionIDViewport, mouse.Rect{
			X:      offsetX,
			Y:      y,
			Width:  v.width,
			Height: viewportHeight,
		}, nil)
	}
}

// GetRegions returns the current clickable regions
func (v *SQLInstanceDetailsView) GetRegions() []mouse.Region {
	return v.regionMgr.GetRegions()
}

// HandleRegionClick processes a click on a specific region
func (v *SQLInstanceDetailsView) HandleRegionClick(regionID string) tea.Cmd {
	v.focusMgr.SetActive(regionID)
	return nil
}
