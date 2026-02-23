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

// Tab IDs for service account details view
const (
	saTabIDDetails = "details"
	saTabIDKeys    = "keys"
)

// Focus region IDs
const (
	saRegionIDTabs     = "tabs"
	saRegionIDViewport = "viewport"
)

// Lines reserved for tab bar + separator + help text
const saDetailsViewportReservedLines = 5

// Internal messages for async data loading
type saDetailsLoadedMsg struct {
	details *gcp.ServiceAccountDetails
}

type saDetailsErrorMsg struct {
	err error
}

type saKeysLoadedMsg struct {
	keys []gcp.ServiceAccountKey
}

type saKeysErrorMsg struct {
	err error
}

// ServiceAccountDetailsView displays comprehensive service account information with tabs
type ServiceAccountDetailsView struct {
	iamClient *gcp.IAMClient
	projectID string
	email     string
	ctx       *context.ProgramContext

	// Data — each dataset loads independently
	details *gcp.ServiceAccountDetails
	saKeys  []gcp.ServiceAccountKey

	// Separate loading/error state per dataset
	detailsLoading bool
	keysLoading    bool
	detailsErr     error
	keysErr        error

	// UI state
	spinner spinner.Model
	width   int
	height  int
	ready   bool

	// Tab navigation (Details / Keys)
	tabs         *tabs.Tabs
	tabViewports []viewport.Model

	// Focus management
	focusMgr  *focus.Manager
	regionMgr *mouse.RegionManager

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation (service account)
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool

	// Key selection cursor (Keys tab)
	keyCursor int

	// Key deletion confirmation
	deleteKeyConfirm     *confirm.TypeConfirmDialog
	showDeleteKeyConfirm bool
	pendingDeleteKeyName string

	// Pending key download — held after creation until user explicitly saves
	pendingKeyJSON []byte
	pendingKeyID   string

	keys_ saDetailsKeyMap
}

type saDetailsKeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Refresh     key.Binding
	Toggle      key.Binding
	Delete      key.Binding
	DeleteKey   key.Binding
	CreateKey   key.Binding
	DownloadKey key.Binding
	ActionMenu  key.Binding
}

func defaultSADetailsKeyMap() saDetailsKeyMap {
	return saDetailsKeyMap{
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
		DeleteKey: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete key"),
		),
		CreateKey: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "create key"),
		),
		DownloadKey: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "download key"),
		),
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
	}
}

// NewServiceAccountDetailsView creates a new service account details view
func NewServiceAccountDetailsView(projectID, email string, iamClient *gcp.IAMClient) *ServiceAccountDetailsView {
	s := components.NewGCPSpinner()

	tabsComponent := tabs.New([]tabs.Tab{
		{ID: saTabIDDetails, Label: "Details"},
		{ID: saTabIDKeys, Label: "Keys"},
	})

	fm := focus.NewManager()
	fm.SetRegions([]focus.Region{
		focus.NewRegion(saRegionIDTabs, focus.RegionTabs, "Tabs"),
		focus.NewRegion(saRegionIDViewport, focus.RegionViewport, "Content"),
	})

	return &ServiceAccountDetailsView{
		iamClient:      iamClient,
		projectID:      projectID,
		email:          email,
		spinner:        s,
		detailsLoading: true,
		keysLoading:    true,
		keys_:          defaultSADetailsKeyMap(),
		tabs:           tabsComponent,
		tabViewports:   make([]viewport.Model, 2),
		focusMgr:       fm,
		regionMgr:      mouse.NewRegionManager(),
	}
}

// Init starts loading all datasets in parallel
func (v *ServiceAccountDetailsView) Init() tea.Cmd {
	v.detailsLoading = true
	v.keysLoading = true
	v.detailsErr = nil
	v.keysErr = nil
	return tea.Batch(
		v.spinner.Tick,
		v.loadDetails(),
		v.loadKeys(),
	)
}

func (v *ServiceAccountDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		if v.iamClient == nil {
			return saDetailsErrorMsg{err: uierrors.ErrIAMClientNotInitialized}
		}
		details, err := v.iamClient.GetServiceAccount(gocontext.Background(), v.email)
		if err != nil {
			return saDetailsErrorMsg{err: err}
		}
		return saDetailsLoadedMsg{details: details}
	}
}

func (v *ServiceAccountDetailsView) loadKeys() tea.Cmd {
	return func() tea.Msg {
		if v.iamClient == nil {
			return saKeysErrorMsg{err: uierrors.ErrIAMClientNotInitialized}
		}
		keys, err := v.iamClient.ListServiceAccountKeys(gocontext.Background(), v.email)
		if err != nil {
			return saKeysErrorMsg{err: err}
		}
		return saKeysLoadedMsg{keys: keys}
	}
}

// Update handles messages for the service account details view
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern - complexity expected
func (v *ServiceAccountDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case saDetailsLoadedMsg:
		v.detailsLoading = false
		v.details = msg.details
		v.updateViewportContent()
		return nil

	case saDetailsErrorMsg:
		v.detailsLoading = false
		v.detailsErr = msg.err
		v.updateViewportContent()
		return nil

	case saKeysLoadedMsg:
		v.keysLoading = false
		v.saKeys = msg.keys
		v.updateViewportContent()
		return nil

	case saKeysErrorMsg:
		v.keysLoading = false
		v.keysErr = msg.err
		v.updateViewportContent()
		return nil

	case spinner.TickMsg:
		if v.detailsLoading || v.keysLoading {
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
		// Could be SA deletion or key deletion
		if v.showDeleteKeyConfirm {
			v.showDeleteKeyConfirm = false
			keyName := v.pendingDeleteKeyName
			email := v.email
			v.pendingDeleteKeyName = ""
			return func() tea.Msg {
				return DeleteServiceAccountKeyMsg{KeyName: keyName, Email: email}
			}
		}
		v.showDeleteConfirm = false
		if v.details != nil {
			return func() tea.Msg {
				return DeleteServiceAccountConfirmedMsg{Email: v.details.Email}
			}
		}
		return nil

	case confirm.TypeCancelMsg:
		v.showDeleteConfirm = false
		v.showDeleteKeyConfirm = false
		v.pendingDeleteKeyName = ""
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

		// Route to delete confirm when shown (SA or key)
		if v.showDeleteKeyConfirm && v.deleteKeyConfirm != nil {
			return v.deleteKeyConfirm.Update(msg)
		}
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		// Handle Tab/Shift+Tab for cycling between focus regions
		if focusMsg := v.focusMgr.HandleKey(msg); focusMsg != nil {
			v.updateViewportContent()
			return func() tea.Msg { return focusMsg }
		}

		// Action keys
		switch {
		case key.Matches(msg, v.keys_.ActionMenu):
			if v.details != nil {
				v.actionMenu = actionmenu.New("Service Account Actions", v.buildActions())
				v.menuOpen = true
			}
			return nil

		case key.Matches(msg, v.keys_.Refresh):
			return v.refresh()

		case key.Matches(msg, v.keys_.Toggle):
			return v.toggleAccount()

		case key.Matches(msg, v.keys_.Delete):
			return v.initiateDelete()

		case key.Matches(msg, v.keys_.CreateKey):
			// Only allow key creation from the Keys tab
			if v.tabs.ActiveTab().ID == saTabIDKeys && v.details != nil {
				return func() tea.Msg {
					return CreateServiceAccountKeyMsg{Email: v.details.Email}
				}
			}
			return nil

		case key.Matches(msg, v.keys_.DeleteKey):
			return v.initiateDeleteKey()

		case key.Matches(msg, v.keys_.DownloadKey):
			return v.downloadPendingKey()
		}

		// Route remaining keys based on focused region
		switch v.focusMgr.ActiveType() {
		case focus.RegionTabs:
			if tabs.HandleKey(msg) {
				return v.tabs.Update(msg)
			}

		case focus.RegionViewport:
			// Keys tab: up/down moves key cursor instead of scrolling
			if v.tabs.ActiveTab().ID == saTabIDKeys && len(v.saKeys) > 0 {
				if key.Matches(msg, v.keys_.Up) {
					if v.keyCursor > 0 {
						v.keyCursor--
						v.updateViewportContent()
					}
					return nil
				}
				if key.Matches(msg, v.keys_.Down) {
					if v.keyCursor < len(v.saKeys)-1 {
						v.keyCursor++
						v.updateViewportContent()
					}
					return nil
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

func (v *ServiceAccountDetailsView) buildActions() []actionmenu.Action {
	actions := []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
	}

	if v.details != nil {
		if v.details.Disabled {
			actions = append(actions, actionmenu.Action{Key: 't', Label: "Enable", Enabled: true})
		} else {
			actions = append(actions, actionmenu.Action{Key: 't', Label: "Disable", Enabled: true})
		}
		if v.tabs.ActiveTab().ID == saTabIDKeys {
			actions = append(actions, actionmenu.Action{Key: 'c', Label: "Create Key", Enabled: true})
			if v.HasPendingKey() {
				actions = append(actions, actionmenu.Action{Key: 'w', Label: "Download Key (JSON)", Enabled: true})
			}
			if v.selectedKeyIsUserManaged() {
				actions = append(actions, actionmenu.Action{Key: 'd', Label: "Delete Selected Key", Enabled: true, Dangerous: true})
			}
		}
		actions = append(actions, actionmenu.Action{Key: 'D', Label: "Delete SA", Enabled: true, Dangerous: true})
	}

	return actions
}

func (v *ServiceAccountDetailsView) executeAction(actionKey rune) tea.Cmd {
	switch actionKey {
	case 'r':
		return v.refresh()
	case 't':
		return v.toggleAccount()
	case 'c':
		if v.tabs.ActiveTab().ID == saTabIDKeys && v.details != nil {
			return func() tea.Msg {
				return CreateServiceAccountKeyMsg{Email: v.details.Email}
			}
		}
		return nil
	case 'w':
		return v.downloadPendingKey()
	case 'd':
		return v.initiateDeleteKey()
	case 'D':
		return v.initiateDelete()
	}
	return nil
}

func (v *ServiceAccountDetailsView) refresh() tea.Cmd {
	v.detailsLoading = true
	v.keysLoading = true
	v.detailsErr = nil
	v.keysErr = nil
	return tea.Batch(v.spinner.Tick, v.loadDetails(), v.loadKeys())
}

func (v *ServiceAccountDetailsView) toggleAccount() tea.Cmd {
	if v.details == nil {
		return nil
	}
	return func() tea.Msg {
		return ToggleServiceAccountMsg{Email: v.details.Email, Disable: !v.details.Disabled}
	}
}

func (v *ServiceAccountDetailsView) initiateDelete() tea.Cmd {
	if v.details == nil {
		return nil
	}

	detailLines := []string{
		fmt.Sprintf("Display Name: %s", v.details.DisplayName),
		fmt.Sprintf("Unique ID: %s", v.details.UniqueID),
	}

	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Service Account", v.details.Email, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

func (v *ServiceAccountDetailsView) initiateDeleteKey() tea.Cmd {
	if v.tabs.ActiveTab().ID != saTabIDKeys {
		return nil
	}
	if v.keyCursor < 0 || v.keyCursor >= len(v.saKeys) {
		return nil
	}

	k := v.saKeys[v.keyCursor]
	// Only user-managed keys can be deleted
	if k.KeyType != "USER_MANAGED" {
		return nil
	}

	v.pendingDeleteKeyName = k.Name
	v.deleteKeyConfirm = confirm.NewTypeConfirmDialog(
		"Delete Service Account Key",
		k.KeyID,
		[]string{
			fmt.Sprintf("Algorithm: %s", k.KeyAlgorithm),
			fmt.Sprintf("Created: %s", timeutil.FormatTimestamp(k.ValidAfterTime)),
		},
	)
	v.showDeleteKeyConfirm = true
	return v.deleteKeyConfirm.Init()
}

func (v *ServiceAccountDetailsView) downloadPendingKey() tea.Cmd {
	if len(v.pendingKeyJSON) == 0 {
		return nil
	}
	keyJSON := v.pendingKeyJSON
	keyID := v.pendingKeyID
	return func() tea.Msg {
		return DownloadServiceAccountKeyMsg{KeyJSON: keyJSON, KeyID: keyID}
	}
}

func (v *ServiceAccountDetailsView) selectedKeyIsUserManaged() bool {
	if v.keyCursor < 0 || v.keyCursor >= len(v.saKeys) {
		return false
	}
	return v.saKeys[v.keyCursor].KeyType == "USER_MANAGED"
}

// View renders the service account details view
func (v *ServiceAccountDetailsView) View() string {
	if v.detailsLoading && v.details == nil {
		return renderLoading(v.spinner, "Loading service account details...")
	}

	if v.detailsErr != nil && v.details == nil {
		return renderLoading(v.spinner, fmt.Sprintf("Error: %v\n  Press 'esc' to go back", v.detailsErr))
	}

	if v.details == nil {
		return renderLoading(v.spinner, "No service account details available.\n  Press 'esc' to go back.")
	}

	if !v.ready {
		return renderLoading(v.spinner, "Initializing view...")
	}

	// Render tab bar with focus accent
	tabBar := focus.RenderAccent("  "+v.tabs.View(), v.focusMgr.IsActive(saRegionIDTabs))

	// Get active tab viewport with focus accent
	activeIdx := v.tabs.ActiveIndex()
	var viewportContent string
	var scrollPercent float64
	if activeIdx >= 0 && activeIdx < len(v.tabViewports) {
		viewportContent = focus.RenderAccent(v.tabViewports[activeIdx].View(), v.focusMgr.IsActive(saRegionIDViewport))
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

	// Overlay delete confirmation if shown (key or SA)
	if v.showDeleteKeyConfirm && v.deleteKeyConfirm != nil {
		return v.renderWithOverlay(mainContent, v.deleteKeyConfirm.View())
	}
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.renderWithOverlay(mainContent, v.deleteConfirm.View())
	}

	return mainContent
}

func (v *ServiceAccountDetailsView) buildHelpText() string {
	activeTab := v.tabs.ActiveTab().ID
	if activeTab == saTabIDKeys {
		base := "\n  .: actions • c: create key"
		if v.HasPendingKey() {
			base += " • w: download key"
		}
		base += " • d: delete key • t: toggle • D: delete SA • r: refresh • Tab: focus • esc: back"
		return base
	}
	return "\n  .: actions • t: toggle • D: delete SA • r: refresh • Tab: focus • esc: back"
}

func (v *ServiceAccountDetailsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}

// SetContext updates the view with shared program context
func (v *ServiceAccountDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// IsMenuOpen returns true if the action menu or a confirm dialog is open
func (v *ServiceAccountDetailsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm || v.showDeleteKeyConfirm
}

// GetEmail returns the email for breadcrumbs
func (v *ServiceAccountDetailsView) GetEmail() string {
	return v.email
}

// GetIAMClient returns the IAM client for reuse
func (v *ServiceAccountDetailsView) GetIAMClient() *gcp.IAMClient {
	return v.iamClient
}

// SetPendingKey stores key data after creation — user must explicitly download
func (v *ServiceAccountDetailsView) SetPendingKey(keyJSON []byte, keyID string) {
	v.pendingKeyJSON = keyJSON
	v.pendingKeyID = keyID
	// Switch to Keys tab to show the download indicator
	v.tabs.SetActiveByID(saTabIDKeys)
	v.updateViewportContent()
}

// ClearPendingKey removes the pending key data after download
func (v *ServiceAccountDetailsView) ClearPendingKey() {
	v.pendingKeyJSON = nil
	v.pendingKeyID = ""
	v.updateViewportContent()
}

// HasPendingKey returns true if a key is waiting for download
func (v *ServiceAccountDetailsView) HasPendingKey() bool {
	return len(v.pendingKeyJSON) > 0
}

// HasTextInputFocused returns true if a confirm dialog input is active
func (v *ServiceAccountDetailsView) HasTextInputFocused() bool {
	if v.showDeleteKeyConfirm && v.deleteKeyConfirm != nil {
		return v.deleteKeyConfirm.HasTextInputFocused()
	}
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.deleteConfirm.HasTextInputFocused()
	}
	return false
}

func (v *ServiceAccountDetailsView) applySize(width, height int) {
	viewportHeight := height - saDetailsViewportReservedLines
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

	if v.details != nil {
		v.updateViewportContent()
	}
}

func (v *ServiceAccountDetailsView) updateViewportContent() {
	if v.details != nil {
		v.tabViewports[0].SetContent(v.renderDetailsTab())
	}
	v.tabViewports[1].SetContent(v.renderKeysTab())
}

func (v *ServiceAccountDetailsView) renderDetailsTab() string {
	if v.details == nil {
		return ""
	}

	var b strings.Builder
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(20)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED"))

	field := func(label, value string) {
		if value == "" {
			value = "—"
		}
		b.WriteString(labelStyle.Render(label))
		b.WriteString(valueStyle.Render(value))
		b.WriteString("\n")
	}

	// Status
	var statusStr string
	if v.details.Disabled {
		statusStr = symbols.StatusStopped() + " Disabled"
	} else {
		statusStr = symbols.StatusRunning() + " Active"
	}

	b.WriteString("\n")
	field("Email", v.details.Email)
	field("Display Name", v.details.DisplayName)
	field("Description", v.details.Description)
	field("Status", statusStr)
	b.WriteString("\n")
	field("Unique ID", v.details.UniqueID)
	field("OAuth2 Client ID", v.details.OAuth2ClientID)
	field("Project", v.details.ProjectID)

	return b.String()
}

func (v *ServiceAccountDetailsView) renderKeysTab() string {
	if v.keysLoading {
		return "\n  " + v.spinner.View() + " Loading keys..."
	}

	if v.keysErr != nil {
		return fmt.Sprintf("\n  Error loading keys: %v", v.keysErr)
	}

	if len(v.saKeys) == 0 {
		return "\n  No keys found for this service account."
	}

	// Clamp cursor after reload
	if v.keyCursor >= len(v.saKeys) {
		v.keyCursor = len(v.saKeys) - 1
	}

	var b strings.Builder
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true)
	selectedHeaderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED")).Bold(true).Background(lipgloss.Color("#4285F4"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(16)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED"))

	// Show pending key download banner
	if v.HasPendingKey() {
		bannerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")).Bold(true)
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		b.WriteString("\n")
		b.WriteString(bannerStyle.Render("  Key created successfully!"))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("  Press 'w' to download the JSON key file. This is the only time the private key is available."))
		b.WriteString("\n")
	}

	isViewportFocused := v.focusMgr.IsActive(saRegionIDViewport)

	for i, k := range v.saKeys {
		selected := i == v.keyCursor && isViewportFocused
		b.WriteString("\n")

		// Show cursor indicator and highlight for selected key
		prefix := "  "
		hStyle := headerStyle
		if selected {
			prefix = "▸ "
			hStyle = selectedHeaderStyle
		}
		b.WriteString(hStyle.Render(fmt.Sprintf("%sKey %d", prefix, i+1)))
		b.WriteString("\n")

		field := func(label, value string) {
			if value == "" {
				value = "—"
			}
			b.WriteString("  ")
			b.WriteString(labelStyle.Render(label))
			b.WriteString(valueStyle.Render(value))
			b.WriteString("\n")
		}

		field("Key ID", k.KeyID)
		field("Type", k.KeyType)
		field("Algorithm", k.KeyAlgorithm)
		field("Origin", k.KeyOrigin)

		if k.ValidAfterTime != "" {
			field("Created", timeutil.FormatTimestamp(k.ValidAfterTime))
		}
		if k.ValidBeforeTime != "" {
			field("Expires", timeutil.FormatTimestamp(k.ValidBeforeTime))
		}

		if k.Disabled {
			field("Status", symbols.StatusStopped()+" Disabled")
		} else {
			field("Status", symbols.StatusRunning()+" Active")
		}
	}

	return b.String()
}
