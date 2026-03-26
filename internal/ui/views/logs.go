package views

import (
	gocontext "context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/logviewer"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/overlay"
)

// logsState tracks the overall data-loading lifecycle.
type logsState int

const (
	logsStateIdle    logsState = iota
	logsStateLoading           // initial or full-query load
	logsStateError
)

// logsFocusArea determines which UI section owns keyboard input.
type logsFocusArea int

const (
	logsFocusEntries   logsFocusArea = iota
	logsFocusFilters                 // tab-navigable filter pills
	logsFocusQuery                   // LQL query text input
	logsFocusTimeRange               // time range selector
)

// filterDropdownType identifies which filter dropdown is open.
type filterDropdownType int

const (
	filterDropdownNone       filterDropdownType = iota
	filterDropdownResources                     // R key
	filterDropdownLogNames                      // L key
	filterDropdownSeverities                    // V key
)

// allSeverities defines the complete set of GCP log severity levels.
var allSeverities = []string{
	"DEBUG", "INFO", "NOTICE", "WARNING", "ERROR", "CRITICAL", "ALERT", "EMERGENCY",
}

// logsKeyMap defines key bindings for the Logs Explorer.
type logsKeyMap struct {
	Up          key.Binding
	Down        key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	Enter       key.Binding
	Left        key.Binding
	Right       key.Binding
	ExpandAll   key.Binding
	CollapseAl  key.Binding
	FocusQuery  key.Binding
	Refresh     key.Binding
	TailMode    key.Binding
	ToggleWrap  key.Binding
	ToggleColor key.Binding
	FilterRes   key.Binding
	FilterLog   key.Binding
	FilterSev   key.Binding
	OpenPager   key.Binding
	Escape      key.Binding
}

func defaultLogsKeyMap() logsKeyMap {
	return logsKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("PgUp", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("PgDn", "page down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "expand/filter"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "collapse"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "expand"),
		),
		ExpandAll: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "expand all"),
		),
		CollapseAl: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "collapse all"),
		),
		FocusQuery: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "query"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		TailMode: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "tail"),
		),
		ToggleWrap: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "wrap"),
		),
		ToggleColor: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "colors"),
		),
		FilterRes: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "resources"),
		),
		FilterLog: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "log names"),
		),
		FilterSev: key.NewBinding(
			key.WithKeys("V"),
			key.WithHelp("V", "severities"),
		),
		OpenPager: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pager"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}

// LogsView is the main Logs Explorer view.
type LogsView struct {
	projectID string
	gcpClient *gcp.Client

	state logsState
	err   error

	// Query input
	query        string
	queryInput   textinput.Model
	queryFocused bool

	// Selected filter values
	selectedResources  []string
	selectedLogNames   []string
	selectedSeverities []string

	// Available options loaded from GCP (async)
	availableResources []string
	availableLogNames  []string
	resourcesLoaded    bool
	logNamesLoaded     bool

	// Filter pill focus (tab-navigable: 0=Resources, 1=LogNames, 2=Severities)
	filterPillCursor int
	// Time range cursor (0-4 maps to PredefinedTimeRanges)
	timeRangeCursor int

	// Filter dropdown overlay state
	activeFilter    filterDropdownType
	filterOptions   []string // all options
	filterVisible   []string // options matching filterSearch
	filterSelected  map[string]bool
	filterCursor    int
	filterSearch    string // live search text
	filterSearching bool   // true when typing in search

	// Data
	entries       []gcp.LogEntry
	nextPageToken string
	totalCount    int64
	histogramData []gcp.DataPoint
	loadingMore   bool

	// Time range
	timeRange time.Duration

	// Tail mode: polls for new entries every 15s
	tailMode   bool
	tailTicker *time.Ticker
	tailDone   chan struct{}

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool
	exportMsg  string // transient success/error message after export

	// UI components
	logViewer *logviewer.Model
	spinner   spinner.Model
	focus     logsFocusArea
	keys      logsKeyMap
	width     int
	height    int
	ctx       *context.ProgramContext
}

// NewLogsView creates a new Logs Explorer view.
func NewLogsView(projectID string, gcpClient *gcp.Client) *LogsView {
	ti := textinput.New()
	ti.Placeholder = "Enter LQL filter or search text..."
	ti.CharLimit = 1024

	return &LogsView{
		projectID:          projectID,
		gcpClient:          gcpClient,
		state:              logsStateIdle,
		queryInput:         ti,
		timeRange:          time.Hour,
		selectedSeverities: []string{}, // empty = all severities
		logViewer:          logviewer.New(),
		spinner:            components.NewGCPSpinner(),
		focus:              logsFocusEntries,
		keys:               defaultLogsKeyMap(),
		filterSelected:     make(map[string]bool),
	}
}

// WithFilters pre-populates the Logs Explorer with filters from another view
// (e.g., Cloud Run observability). Call before Init().
func (v *LogsView) WithFilters(query string, severities []string, timeRange time.Duration) {
	if query != "" {
		v.query = query
		v.queryInput.SetValue(query)
	}
	if len(severities) > 0 {
		// Normalize to allSeverities order and de-dup so that slicesEqual
		// comparisons in applyFilterDropdown() are deterministic regardless
		// of the order in which the caller built the slice (e.g. from a map).
		sevSet := make(map[string]bool, len(severities))
		for _, s := range severities {
			sevSet[s] = true
		}
		normalized := make([]string, 0, len(severities))
		for _, s := range allSeverities {
			if sevSet[s] {
				normalized = append(normalized, s)
			}
		}
		v.selectedSeverities = normalized
	}
	if timeRange > 0 {
		v.timeRange = timeRange
	}
}

// Init starts data loading. Must be idempotent — may be called more than once.
// Resource types and log names are loaded lazily on first dropdown open to reduce
// API calls (GCP logging read quota is 120 req/min).
func (v *LogsView) Init() tea.Cmd {
	v.state = logsStateLoading
	v.err = nil
	return tea.Batch(
		v.spinner.Tick,
		v.executeQuery(),
		v.loadHistogram(),
	)
}

// SetContext updates the view with shared program context.
func (v *LogsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	// renderWithSidebar applies MaxWidth(ContentWidth - sidebarEmojis - contentEmojis).
	// EmojiWidthBudget has the sidebar portion. We add 1 for our own ▸/▾ indicator
	// which is a wide emoji (2-wide in terminals, 1-wide in lipgloss).
	emojiCorrection := ctx.EmojiWidthBudget + 1
	v.width = ctx.ContentWidth - emojiCorrection
	if v.width < 20 {
		v.width = 20
	}
	v.height = ctx.ContentHeight
	// logViewer size is set in View() where renderWidth accounts for menu overlay
}

// HasTextInputFocused returns true when the query input or dropdown search is active.
func (v *LogsView) HasTextInputFocused() bool {
	return v.queryFocused || v.filterSearching
}

// IsMenuOpen returns true when the action menu is open.
// Used by the app to route Esc to the view instead of navigating back.
func (v *LogsView) IsMenuOpen() bool {
	return v.menuOpen || v.activeFilter != filterDropdownNone
}

// Close stops the tail ticker and cleans up resources.
func (v *LogsView) Close() {
	v.stopTailMode()
}

// Update handles all messages for the Logs Explorer view.
//
//nolint:gocognit,cyclop // View update dispatch with many message types and key bindings
func (v *LogsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		v.spinner, cmd = v.spinner.Update(msg)
		return cmd

	// --- Data load results ---
	case logsEntriesLoadedMsg:
		v.state = logsStateIdle
		v.err = nil
		v.entries = msg.entries
		v.nextPageToken = msg.nextToken
		v.totalCount = int64(len(msg.entries))
		v.logViewer.SetEntries(msg.entries)
		v.logViewer.SetHasMore(msg.nextToken != "")
		return nil

	case logsEntriesErrorMsg:
		v.state = logsStateError
		v.err = msg.err
		return nil

	case logsAppendEntriesMsg:
		v.loadingMore = false
		v.entries = append(v.entries, msg.entries...)
		v.nextPageToken = msg.nextToken
		v.totalCount += int64(len(msg.entries))
		v.logViewer.AppendEntries(msg.entries)
		v.logViewer.SetHasMore(msg.nextToken != "")
		return nil

	case logsAppendErrorMsg:
		v.loadingMore = false
		// Non-fatal: keep existing entries, just log the error
		return nil

	case logsHistogramLoadedMsg:
		v.histogramData = msg.data
		return nil

	case logsHistogramErrorMsg:
		// Non-fatal: sparkline simply won't render
		return nil

	case logsResourceTypesLoadedMsg:
		v.availableResources = msg.types
		v.resourcesLoaded = true
		if v.activeFilter == filterDropdownResources {
			v.filterOptions = msg.types
			v.updateFilterVisible()
		}
		return nil

	case logsResourceTypesErrorMsg:
		v.resourcesLoaded = true // mark loaded even on error so we don't retry
		return nil

	case logsLogNamesLoadedMsg:
		v.availableLogNames = msg.names
		v.logNamesLoaded = true
		if v.activeFilter == filterDropdownLogNames {
			v.filterOptions = msg.names
			v.updateFilterVisible()
		}
		return nil

	case logsLogNamesErrorMsg:
		v.logNamesLoaded = true
		return nil

	// --- Tail mode tick ---
	case logsTailTickMsg:
		if !v.tailMode {
			return nil
		}
		return tea.Batch(v.executeTailQuery(), v.tickTailMode())

	case logsTailEntriesMsg:
		if len(msg.entries) > 0 {
			// Prepend new entries at the top (newest first),
			// preserving user's cursor and scroll position
			v.entries = append(msg.entries, v.entries...)
			v.totalCount += int64(len(msg.entries))
			v.logViewer.PrependEntries(msg.entries)
		}
		return nil

	case logsExportDoneMsg:
		v.exportMsg = msg.message
		return nil

	// --- Filter field from logviewer ---
	case logviewer.FilterFieldMsg:
		v.appendFilterToQuery(msg.Key, msg.Value)
		return v.runQuery()

	// --- Action menu ---
	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		return v.executeMenuAction(msg.Key)

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}

	// Forward to text input when focused
	if v.queryFocused {
		var cmd tea.Cmd
		v.queryInput, cmd = v.queryInput.Update(msg)
		return cmd
	}

	return nil
}

// handleKey processes keyboard input based on current focus area and dropdown state.
//
//nolint:gocognit,cyclop // Key dispatch for multiple focus states and filter dropdowns
func (v *LogsView) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Action menu is open — route all keys to it
	if v.menuOpen && v.actionMenu != nil {
		return v.actionMenu.Update(msg)
	}

	// Clear transient export message on any key press
	v.exportMsg = ""

	// Filter dropdown is open — handle its keys first
	if v.activeFilter != filterDropdownNone {
		return v.handleFilterDropdownKey(msg)
	}

	// Query input is focused
	if v.queryFocused {
		return v.handleQueryKey(msg)
	}

	// Filter pills are focused — tab cycles, enter opens dropdown
	if v.focus == logsFocusFilters {
		return v.handleFilterPillKey(msg)
	}

	// Time range selector is focused
	if v.focus == logsFocusTimeRange {
		return v.handleTimeRangeKey(msg)
	}

	// Time range hotkeys (1-5) work in entry focus
	switch msg.String() {
	case "1":
		return v.setTimeRange(components.PredefinedTimeRanges[0].Duration)
	case "2":
		return v.setTimeRange(components.PredefinedTimeRanges[1].Duration)
	case "3":
		return v.setTimeRange(components.PredefinedTimeRanges[2].Duration)
	case "4":
		return v.setTimeRange(components.PredefinedTimeRanges[3].Duration)
	case "5":
		return v.setTimeRange(components.PredefinedTimeRanges[4].Duration)
	}

	// Entry navigation keys
	switch {
	case msg.String() == "tab":
		// Tab from entries → filters
		v.focus = logsFocusFilters
		v.filterPillCursor = 0
		return nil

	case msg.String() == "shift+tab":
		// Shift+Tab from entries → time range
		v.focus = logsFocusTimeRange
		v.timeRangeCursor = v.currentTimeRangeIndex()
		return nil

	case key.Matches(msg, v.keys.FocusQuery):
		v.queryFocused = true
		v.focus = logsFocusQuery
		v.queryInput.Focus()
		return v.queryInput.Cursor.BlinkCmd()

	case key.Matches(msg, v.keys.Up):
		v.logViewer.MoveUp()
		return nil

	case key.Matches(msg, v.keys.Down):
		v.logViewer.MoveDown()
		return v.checkInfiniteScroll()

	case key.Matches(msg, v.keys.PageUp):
		v.logViewer.PageUp()
		return nil

	case key.Matches(msg, v.keys.PageDown):
		v.logViewer.PageDown()
		return v.checkInfiniteScroll()

	case key.Matches(msg, v.keys.Enter):
		return v.handleEntryEnter()

	case key.Matches(msg, v.keys.Right):
		if v.logViewer.FieldCursor() < 0 && v.logViewer.IsExpanded(v.logViewer.Cursor()) {
			v.logViewer.EnterFieldNav()
		} else if !v.logViewer.IsExpanded(v.logViewer.Cursor()) {
			v.logViewer.ToggleExpand(v.logViewer.Cursor())
		}
		return nil

	case key.Matches(msg, v.keys.Left):
		if v.logViewer.FieldCursor() >= 0 {
			v.logViewer.ExitFieldNav()
		} else if v.logViewer.IsExpanded(v.logViewer.Cursor()) {
			v.logViewer.ToggleExpand(v.logViewer.Cursor())
		}
		return nil

	case key.Matches(msg, v.keys.ExpandAll):
		v.logViewer.ExpandAll()
		return nil

	case key.Matches(msg, v.keys.CollapseAl):
		v.logViewer.CollapseAll()
		return nil

	case key.Matches(msg, v.keys.ToggleWrap):
		v.logViewer.ToggleWrap()
		return nil

	case key.Matches(msg, v.keys.ToggleColor):
		v.logViewer.ToggleColorize()
		return nil

	case key.Matches(msg, v.keys.OpenPager):
		return v.openInPager()

	case key.Matches(msg, v.keys.TailMode):
		return v.toggleTailMode()

	case key.Matches(msg, v.keys.Refresh):
		return v.runQuery()

	case key.Matches(msg, v.keys.FilterRes):
		return v.openFilterDropdown(filterDropdownResources)

	case key.Matches(msg, v.keys.FilterLog):
		return v.openFilterDropdown(filterDropdownLogNames)

	case key.Matches(msg, v.keys.FilterSev):
		return v.openFilterDropdown(filterDropdownSeverities)
	}

	// Action menu (. key) — check after switch to avoid conflicting with key bindings
	if msg.String() == "." {
		v.actionMenu = actionmenu.New("Logs Actions", v.buildActions())
		v.menuOpen = true
		return nil
	}

	return nil
}

// handleQueryKey handles keys while the query input is focused.
func (v *LogsView) handleQueryKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case msg.String() == "tab":
		// Tab from query → time range
		v.query = v.queryInput.Value()
		v.queryFocused = false
		v.focus = logsFocusTimeRange
		v.timeRangeCursor = v.currentTimeRangeIndex()
		v.queryInput.Blur()
		return nil

	case msg.String() == "shift+tab":
		// Shift+Tab from query → filters
		v.query = v.queryInput.Value()
		v.queryFocused = false
		v.focus = logsFocusFilters
		v.filterPillCursor = 2 // last pill
		v.queryInput.Blur()
		return nil

	case key.Matches(msg, v.keys.Enter):
		v.query = v.queryInput.Value()
		v.queryFocused = false
		v.focus = logsFocusEntries
		v.queryInput.Blur()
		// Changing query disables tail mode to avoid confusion
		v.stopTailMode()
		return v.runQuery()

	case key.Matches(msg, v.keys.Escape):
		v.queryFocused = false
		v.focus = logsFocusEntries
		v.queryInput.Blur()
		return nil
	}

	// Forward to textinput
	var cmd tea.Cmd
	v.queryInput, cmd = v.queryInput.Update(msg)
	return cmd
}

// filterPillTypes maps pill cursor position to dropdown type.
var filterPillTypes = [3]filterDropdownType{
	filterDropdownResources,
	filterDropdownLogNames,
	filterDropdownSeverities,
}

// currentTimeRangeIndex returns the index of the currently active time range.
func (v *LogsView) currentTimeRangeIndex() int {
	for i, tr := range components.PredefinedTimeRanges {
		if tr.Duration == v.timeRange {
			return i
		}
	}
	return 0
}

// handleTimeRangeKey handles keys while the time range selector is focused.
func (v *LogsView) handleTimeRangeKey(msg tea.KeyMsg) tea.Cmd {
	rangeCount := len(components.PredefinedTimeRanges)
	switch msg.String() {
	case "right", "l":
		v.timeRangeCursor = (v.timeRangeCursor + 1) % rangeCount
		return nil
	case "left", "h":
		v.timeRangeCursor = (v.timeRangeCursor + rangeCount - 1) % rangeCount
		return nil
	case "enter", " ":
		return v.setTimeRange(components.PredefinedTimeRanges[v.timeRangeCursor].Duration)
	case "tab":
		// Tab from time range → entries
		v.focus = logsFocusEntries
		return nil
	case "shift+tab":
		// Shift+Tab from time range → query
		v.focus = logsFocusQuery
		v.queryFocused = true
		v.queryInput.Focus()
		return v.queryInput.Cursor.BlinkCmd()
	case "esc":
		v.focus = logsFocusEntries
		return nil
	}
	return nil
}

// handleFilterPillKey handles keys while filter pills are focused.
func (v *LogsView) handleFilterPillKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "right", "l":
		v.filterPillCursor = (v.filterPillCursor + 1) % 3
		return nil
	case "left", "h":
		v.filterPillCursor = (v.filterPillCursor + 2) % 3 // +2 wraps backward
		return nil
	case "tab":
		// Tab from filters → query
		v.focus = logsFocusQuery
		v.queryFocused = true
		v.queryInput.Focus()
		return v.queryInput.Cursor.BlinkCmd()
	case "shift+tab":
		// Shift+Tab from filters → entries
		v.focus = logsFocusEntries
		return nil
	case "enter", " ":
		return v.openFilterDropdown(filterPillTypes[v.filterPillCursor])
	case "esc":
		v.focus = logsFocusEntries
		return nil
	}
	return nil
}

// handleEntryEnter handles Enter key on log entries or expanded fields.
func (v *LogsView) handleEntryEnter() tea.Cmd {
	// In field navigation, pressing Enter adds field to query
	if field := v.logViewer.SelectedField(); field != nil {
		v.appendFilterToQuery(field.Key, field.Value)
		return v.runQuery()
	}

	// Toggle expand/collapse on the entry
	v.logViewer.ToggleExpand(v.logViewer.Cursor())
	return nil
}

// --- Filter dropdown ---

// openFilterDropdown opens a multi-select dropdown for the given filter type.
// Triggers lazy loading of options from GCP on first open.
func (v *LogsView) openFilterDropdown(ft filterDropdownType) tea.Cmd {
	v.activeFilter = ft
	v.filterCursor = 0
	v.filterSearch = ""
	v.filterSearching = false

	// Build option list and pre-select current selections
	v.filterSelected = make(map[string]bool)
	var lazyLoad tea.Cmd

	switch ft {
	case filterDropdownResources:
		v.filterOptions = v.availableResources
		for _, r := range v.selectedResources {
			v.filterSelected[r] = true
		}
		if !v.resourcesLoaded {
			lazyLoad = v.loadResourceTypes()
		}
	case filterDropdownLogNames:
		v.filterOptions = v.availableLogNames
		for _, n := range v.selectedLogNames {
			v.filterSelected[n] = true
		}
		if !v.logNamesLoaded {
			lazyLoad = v.loadLogNames()
		}
	case filterDropdownSeverities:
		v.filterOptions = allSeverities
		for _, s := range v.selectedSeverities {
			v.filterSelected[s] = true
		}
	default:
		v.activeFilter = filterDropdownNone
	}

	v.updateFilterVisible()
	return lazyLoad
}

// updateFilterVisible rebuilds the visible options list based on the search query.
func (v *LogsView) updateFilterVisible() {
	if v.filterSearch == "" {
		v.filterVisible = v.filterOptions
		return
	}
	needle := strings.ToLower(v.filterSearch)
	v.filterVisible = nil
	for _, opt := range v.filterOptions {
		if strings.Contains(strings.ToLower(opt), needle) {
			v.filterVisible = append(v.filterVisible, opt)
		}
	}
	// Clamp cursor
	if v.filterCursor >= len(v.filterVisible) {
		v.filterCursor = max(0, len(v.filterVisible)-1)
	}
}

// handleFilterDropdownKey handles keys while a filter dropdown is open.
func (v *LogsView) handleFilterDropdownKey(msg tea.KeyMsg) tea.Cmd {
	// When in search mode, route typing to the search input
	if v.filterSearching {
		return v.handleFilterSearchKey(msg)
	}

	switch msg.String() {
	case "up", "k":
		if v.filterCursor > 0 {
			v.filterCursor--
		}
		return nil

	case "down", "j":
		if v.filterCursor < len(v.filterVisible)-1 {
			v.filterCursor++
		}
		return nil

	case "pgup":
		v.filterCursor -= 15
		if v.filterCursor < 0 {
			v.filterCursor = 0
		}
		return nil

	case "pgdown":
		v.filterCursor += 15
		if v.filterCursor >= len(v.filterVisible) {
			v.filterCursor = max(0, len(v.filterVisible)-1)
		}
		return nil

	case " ", "tab", "enter":
		// Toggle selection on the current visible option
		if v.filterCursor < len(v.filterVisible) {
			opt := v.filterVisible[v.filterCursor]
			v.filterSelected[opt] = !v.filterSelected[opt]
		}
		return nil

	case "/":
		// Activate search mode
		v.filterSearching = true
		return nil

	case "esc":
		// Apply selections and close the dropdown
		changed := v.applyFilterDropdown()
		v.activeFilter = filterDropdownNone
		v.filterSearch = ""
		v.filterSearching = false
		v.focus = logsFocusEntries
		if changed {
			return v.runQuery()
		}
		return nil
	}

	return nil
}

// handleFilterSearchKey handles keys while typing in the dropdown search.
func (v *LogsView) handleFilterSearchKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		// Exit search mode, keep the filter text
		v.filterSearching = false
		return nil
	case tea.KeyBackspace:
		if len(v.filterSearch) > 0 {
			v.filterSearch = v.filterSearch[:len(v.filterSearch)-1]
			v.updateFilterVisible()
		}
		return nil
	case tea.KeyRunes:
		v.filterSearch += string(msg.Runes)
		v.updateFilterVisible()
		return nil
	}
	return nil
}

// applyFilterDropdown saves the dropdown selections to the corresponding filter slice.
// Returns true if selections changed (query needs re-run).
func (v *LogsView) applyFilterDropdown() bool {
	var selected []string
	for _, opt := range v.filterOptions {
		if v.filterSelected[opt] {
			selected = append(selected, opt)
		}
	}

	var prev *[]string
	switch v.activeFilter {
	case filterDropdownResources:
		prev = &v.selectedResources
	case filterDropdownLogNames:
		prev = &v.selectedLogNames
	case filterDropdownSeverities:
		prev = &v.selectedSeverities
	default:
		return false
	}

	// Check if selections actually changed
	if slicesEqual(*prev, selected) {
		return false
	}
	*prev = selected
	return true
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- Query building ---

// buildEffectiveQuery combines time range, resource filter, log name filter,
// severity filter, and user query into a single LQL filter string.
func (v *LogsView) buildEffectiveQuery() string {
	var parts []string

	// Time range — GCP LQL requires double-quoted timestamps
	cutoff := time.Now().Add(-v.timeRange).UTC().Format(time.RFC3339)
	parts = append(parts, fmt.Sprintf(`timestamp >= "%s"`, cutoff)) //nolint:gocritic // GCP LQL filter syntax requires double quotes

	// Resource type filter
	if len(v.selectedResources) > 0 {
		parts = append(parts, buildORClause("resource.type", v.selectedResources))
	}

	// Log name filter
	if len(v.selectedLogNames) > 0 {
		parts = append(parts, buildORClause("logName", v.selectedLogNames))
	}

	// Severity filter
	if len(v.selectedSeverities) > 0 {
		parts = append(parts, buildORClause("severity", v.selectedSeverities))
	}

	// User query (appended as-is so users can write arbitrary LQL)
	if v.query != "" {
		parts = append(parts, v.query)
	}

	return strings.Join(parts, "\n")
}

// buildORClause creates an LQL filter clause with OR for multiple values.
//
//nolint:gocritic // GCP LQL filter syntax requires double-quoted values, not Go %q
func buildORClause(field string, values []string) string {
	if len(values) == 1 {
		return fmt.Sprintf(`%s = "%s"`, field, escapeLQL(values[0]))
	}
	clauses := make([]string, len(values))
	for i, v := range values {
		clauses[i] = fmt.Sprintf(`%s = "%s"`, field, escapeLQL(v))
	}
	return "(" + strings.Join(clauses, " OR ") + ")"
}

// escapeLQL escapes double-quotes in values for safe LQL filter embedding.
func escapeLQL(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

// appendFilterToQuery adds a key=value clause from a log entry field.
func (v *LogsView) appendFilterToQuery(fieldKey, value string) {
	clause := fmt.Sprintf(`%s = "%s"`, fieldKey, escapeLQL(value)) //nolint:gocritic // GCP LQL filter syntax
	if v.query != "" {
		v.query += "\n" + clause
	} else {
		v.query = clause
	}
	v.queryInput.SetValue(v.query)
}

// --- Data loading ---

// runQuery resets state and executes the current query.
// Histogram is not re-fetched here — it only changes with time range.
func (v *LogsView) runQuery() tea.Cmd {
	v.state = logsStateLoading
	v.err = nil
	v.nextPageToken = ""
	return v.executeQuery()
}

// executeQuery fetches the first page of log entries.
func (v *LogsView) executeQuery() tea.Cmd {
	client := v.gcpClient
	projectID := v.projectID
	filter := v.buildEffectiveQuery()

	return func() tea.Msg {
		if client == nil {
			return logsEntriesErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return logsEntriesErrorMsg{err: fmt.Errorf("logging client: %w", err)}
		}

		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 30*time.Second)
		defer cancel()

		entries, nextToken, err := logClient.ListLogEntries(ctx, filter, 200, "")
		if err != nil {
			return logsEntriesErrorMsg{err: err}
		}

		return logsEntriesLoadedMsg{
			entries:   entries,
			nextToken: nextToken,
		}
	}
}

// loadMore fetches the next page of log entries.
func (v *LogsView) loadMore() tea.Cmd {
	if v.loadingMore || v.nextPageToken == "" {
		return nil
	}
	v.loadingMore = true

	client := v.gcpClient
	projectID := v.projectID
	filter := v.buildEffectiveQuery()
	token := v.nextPageToken

	return func() tea.Msg {
		if client == nil {
			return logsAppendErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return logsAppendErrorMsg{err: fmt.Errorf("logging client: %w", err)}
		}

		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 30*time.Second)
		defer cancel()

		entries, nextToken, err := logClient.ListLogEntries(ctx, filter, 200, token)
		if err != nil {
			return logsAppendErrorMsg{err: err}
		}

		return logsAppendEntriesMsg{
			entries:   entries,
			nextToken: nextToken,
		}
	}
}

// loadHistogram fetches the log entry count metric for the sparkline.
func (v *LogsView) loadHistogram() tea.Cmd {
	client := v.gcpClient
	projectID := v.projectID
	timeRange := v.timeRange

	return func() tea.Msg {
		if client == nil {
			return logsHistogramErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		monClient, err := client.GetMonitoringClient(projectID)
		if err != nil {
			return logsHistogramErrorMsg{err: fmt.Errorf("monitoring client: %w", err)}
		}

		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 15*time.Second)
		defer cancel()

		data, err := monClient.GetLogEntryCount(ctx, timeRange)
		if err != nil {
			return logsHistogramErrorMsg{err: err}
		}

		return logsHistogramLoadedMsg{data: data}
	}
}

// loadResourceTypes fetches available resource types for the filter dropdown.
func (v *LogsView) loadResourceTypes() tea.Cmd {
	client := v.gcpClient
	projectID := v.projectID

	return func() tea.Msg {
		if client == nil {
			return logsResourceTypesErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return logsResourceTypesErrorMsg{err: fmt.Errorf("logging client: %w", err)}
		}

		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 15*time.Second)
		defer cancel()

		types, err := logClient.ListResourceTypes(ctx)
		if err != nil {
			return logsResourceTypesErrorMsg{err: err}
		}

		return logsResourceTypesLoadedMsg{types: types}
	}
}

// loadLogNames fetches available log names for the filter dropdown.
func (v *LogsView) loadLogNames() tea.Cmd {
	client := v.gcpClient
	projectID := v.projectID

	return func() tea.Msg {
		if client == nil {
			return logsLogNamesErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return logsLogNamesErrorMsg{err: fmt.Errorf("logging client: %w", err)}
		}

		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 15*time.Second)
		defer cancel()

		names, err := logClient.ListLogNames(ctx)
		if err != nil {
			return logsLogNamesErrorMsg{err: err}
		}

		return logsLogNamesLoadedMsg{names: names}
	}
}

// --- Time range ---

func (v *LogsView) setTimeRange(d time.Duration) tea.Cmd {
	if v.timeRange == d {
		return nil
	}
	v.timeRange = d
	// Disable tail when time range changes
	v.stopTailMode()
	// Time range change affects both query and histogram
	v.state = logsStateLoading
	v.err = nil
	v.nextPageToken = ""
	return tea.Batch(v.executeQuery(), v.loadHistogram())
}

// --- Tail mode ---

func (v *LogsView) toggleTailMode() tea.Cmd {
	if v.tailMode {
		v.stopTailMode()
		return nil
	}
	return v.startTailMode()
}

func (v *LogsView) startTailMode() tea.Cmd {
	v.stopTailMode() // clean up any previous ticker
	v.tailMode = true
	v.tailTicker = time.NewTicker(15 * time.Second)
	v.tailDone = make(chan struct{})
	return v.tickTailMode()
}

func (v *LogsView) stopTailMode() {
	v.tailMode = false
	if v.tailTicker != nil {
		v.tailTicker.Stop()
		v.tailTicker = nil
	}
	if v.tailDone != nil {
		close(v.tailDone)
		v.tailDone = nil
	}
}

// tickTailMode waits for the next ticker tick, using done channel to avoid goroutine leaks.
func (v *LogsView) tickTailMode() tea.Cmd {
	ticker := v.tailTicker
	done := v.tailDone
	if ticker == nil || done == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case <-ticker.C:
			return logsTailTickMsg{}
		case <-done:
			return nil
		}
	}
}

// executeTailQuery fetches entries newer than the most recent one we have.
func (v *LogsView) executeTailQuery() tea.Cmd {
	client := v.gcpClient
	projectID := v.projectID
	filter := v.buildEffectiveQuery()

	// Narrow to entries newer than our latest
	var newestTimestamp time.Time
	if len(v.entries) > 0 {
		newestTimestamp = v.entries[0].Timestamp
	}

	return func() tea.Msg {
		if client == nil {
			return nil // silently skip on nil client
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return nil
		}

		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 10*time.Second)
		defer cancel()

		// Add timestamp constraint for entries newer than what we have
		tailFilter := filter
		if !newestTimestamp.IsZero() {
			tailFilter += fmt.Sprintf("\ntimestamp > \"%s\"", newestTimestamp.UTC().Format(time.RFC3339Nano)) //nolint:gocritic // GCP LQL filter syntax requires double quotes
		}

		entries, _, err := logClient.ListLogEntries(ctx, tailFilter, 50, "")
		if err != nil {
			return nil // silently skip errors in tail mode
		}

		return logsTailEntriesMsg{entries: entries}
	}
}

// --- Infinite scroll ---

func (v *LogsView) checkInfiniteScroll() tea.Cmd {
	if v.logViewer.NeedsMore() && !v.loadingMore {
		return v.loadMore()
	}
	return nil
}

// --- View rendering ---

// View renders the Logs Explorer.
//
//nolint:gocognit // Multi-section rendering with filter dropdowns
func (v *LogsView) View() string {
	// Loading state
	if v.state == logsStateLoading && len(v.entries) == 0 {
		return renderLoading(v.spinner, "Loading logs...")
	}

	// When the action menu is open, the overlay stitches menu lines (containing
	// ▶ cursor) onto bg lines (containing ▸ indicator). The combined line has 2
	// wide emojis instead of 1, so renderWithSidebar reduces mainWidth by 1 more.
	// Shrink the log viewer to match so bg lines still fit after the reduction.
	renderWidth := v.width
	if v.menuOpen {
		renderWidth = v.width - 1
	}
	v.logViewer.SetSize(renderWidth, v.height-12)

	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Bold(true)

	// Time range selector — logs-specific rendering without misleading auto-refresh/last-updated
	b.WriteString("  ")
	b.WriteString(v.renderTimeRangeBar())
	b.WriteString("\n")

	// Filter pills (resource, log name, severity)
	b.WriteString("  ")
	b.WriteString(v.renderFilterPills())
	b.WriteString("\n")

	// Query input
	b.WriteString("  ")
	switch {
	case v.queryFocused:
		b.WriteString(titleStyle.Render("▸ "))
		b.WriteString(v.queryInput.View())
	case v.query != "":
		b.WriteString(mutedStyle.Render("▸ "))
		// Show first line of multi-line query
		firstLine := strings.SplitN(v.query, "\n", 2)[0]
		b.WriteString(mutedStyle.Render(truncate(firstLine, v.width-8)))
	default:
		b.WriteString(mutedStyle.Render("▸ / to search"))
	}
	b.WriteString("\n")

	// Sparkline
	b.WriteString(logviewer.RenderSparkline(v.histogramData, v.width-4, v.totalCount))
	b.WriteString("\n")
	b.WriteString("  " + strings.Repeat("─", max(0, v.width-6)))
	b.WriteString("\n")

	// Error state
	if v.state == logsStateError && v.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %s", v.err.Error())))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  Press 'r' to retry"))
		b.WriteString("\n")
		return b.String()
	}

	// Log entries
	b.WriteString(v.logViewer.View())

	// Status line
	b.WriteString("\n")
	statusParts := []string{}
	if v.logViewer.WrapEnabled() {
		wrapStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8")).Bold(true)
		statusParts = append(statusParts, wrapStyle.Render("WRAP"))
	}
	if v.logViewer.ColorizeEnabled() {
		colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")).Bold(true)
		statusParts = append(statusParts, colorStyle.Render("COLOR"))
	}
	if v.loadingMore {
		statusParts = append(statusParts, mutedStyle.Render(fmt.Sprintf("%s loading...", v.spinner.View())))
	}
	if v.state == logsStateLoading && len(v.entries) > 0 {
		statusParts = append(statusParts, mutedStyle.Render(fmt.Sprintf("%s refreshing...", v.spinner.View())))
	}
	statusParts = append(statusParts, mutedStyle.Render(fmt.Sprintf("%d entries", v.logViewer.EntryCount())))
	if v.exportMsg != "" {
		exportStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
		if strings.HasPrefix(v.exportMsg, "Export error") {
			exportStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		}
		statusParts = append(statusParts, exportStyle.Render(v.exportMsg))
	}
	b.WriteString("  " + strings.Join(statusParts, "  "))
	b.WriteString("\n")

	// Key hints
	b.WriteString(mutedStyle.Render("  /: query  .: actions  tab: filters  1-5: time  f: tail  w: wrap  c: colors  p: pager  r: refresh"))
	b.WriteString("\n")

	mainContent := b.String()

	// Overlay: action menu (higher priority than filter dropdown)
	if v.menuOpen && v.actionMenu != nil {
		return v.renderWithOverlay(mainContent, v.actionMenu.View())
	}

	// Overlay: filter dropdown
	if v.activeFilter != filterDropdownNone {
		return v.renderWithFilterDropdown(mainContent)
	}

	return mainContent
}

// renderTimeRangeBar renders the time range selector with optional tail indicator.
func (v *LogsView) renderTimeRangeBar() string {
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368"))
	focusedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E8EAED")).Background(lipgloss.Color("#3C4043"))
	tailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")).Bold(true)
	focused := v.focus == logsFocusTimeRange

	var parts []string
	for i, tr := range components.PredefinedTimeRanges {
		display := fmt.Sprintf("[%s]", tr.Label)
		switch {
		case focused && v.timeRangeCursor == i:
			parts = append(parts, focusedStyle.Render(display))
		case tr.Duration == v.timeRange:
			parts = append(parts, activeStyle.Render(display))
		default:
			parts = append(parts, inactiveStyle.Render(display))
		}
	}
	result := strings.Join(parts, " ")

	if v.tailMode {
		result += "  " + tailStyle.Render("TAIL (15s)")
	}
	return result
}

// renderFilterPills renders compact buttons for each active filter.
// When filter pills are focused, the active pill gets a highlight.
func (v *LogsView) renderFilterPills() string {
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368"))
	focusedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E8EAED")).Background(lipgloss.Color("#3C4043"))
	focused := v.focus == logsFocusFilters

	renderPill := func(idx int, label string) string {
		if focused && v.filterPillCursor == idx {
			return focusedStyle.Render(label)
		}
		return inactiveStyle.Render(label)
	}

	renderActivePill := func(idx int, label string) string {
		if focused && v.filterPillCursor == idx {
			return focusedStyle.Render(label)
		}
		return activeStyle.Render(label)
	}

	var pills []string

	// Resources
	if len(v.selectedResources) > 0 {
		pills = append(pills, renderActivePill(0, fmt.Sprintf("[Resource Types: %d]", len(v.selectedResources))))
	} else {
		pills = append(pills, renderPill(0, "[Resource Types: all]"))
	}

	// Log names
	if len(v.selectedLogNames) > 0 {
		pills = append(pills, renderActivePill(1, fmt.Sprintf("[Log Names: %d]", len(v.selectedLogNames))))
	} else {
		pills = append(pills, renderPill(1, "[Log Names: all]"))
	}

	// Severities
	if len(v.selectedSeverities) > 0 {
		pills = append(pills, renderActivePill(2, fmt.Sprintf("[Severities: %d]", len(v.selectedSeverities))))
	} else {
		pills = append(pills, renderPill(2, "[Severities: all]"))
	}

	return strings.Join(pills, " ")
}

// renderWithFilterDropdown renders the filter dropdown as a centered overlay.
func (v *LogsView) renderWithFilterDropdown(base string) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("#3C4043"))
	checkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5F6368")).
		Padding(0, 1)

	var title string
	switch v.activeFilter {
	case filterDropdownResources:
		title = "Resource Types"
	case filterDropdownLogNames:
		title = "Log Names"
	case filterDropdownSeverities:
		title = "Severities"
	}

	searchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED"))
	searchActiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED")).Background(lipgloss.Color("#3C4043"))

	var b strings.Builder
	b.WriteString(headerStyle.Render(title))
	b.WriteString("\n")

	// Search bar
	switch {
	case v.filterSearching:
		b.WriteString(searchActiveStyle.Render("/ " + v.filterSearch + "▏"))
	case v.filterSearch != "":
		b.WriteString(searchStyle.Render("/ " + v.filterSearch))
	default:
		b.WriteString(mutedStyle.Render("/ search"))
	}
	b.WriteString("\n")

	loading := (v.activeFilter == filterDropdownResources && !v.resourcesLoaded) ||
		(v.activeFilter == filterDropdownLogNames && !v.logNamesLoaded)

	switch {
	case len(v.filterOptions) == 0 && loading:
		b.WriteString(v.spinner.View() + " Loading...")
		b.WriteString("\n")
	case len(v.filterOptions) == 0:
		b.WriteString(mutedStyle.Render("(no options available)"))
		b.WriteString("\n")
	case len(v.filterVisible) == 0:
		b.WriteString(mutedStyle.Render("(no matches)"))
		b.WriteString("\n")
	default:
		maxVisible := 15
		start := 0
		if v.filterCursor >= maxVisible {
			start = v.filterCursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(v.filterVisible) {
			end = len(v.filterVisible)
		}

		if start > 0 {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("↑ %d more", start)))
			b.WriteString("\n")
		}

		for i := start; i < end; i++ {
			opt := v.filterVisible[i]
			check := "  "
			if v.filterSelected[opt] {
				check = checkStyle.Render("✓ ")
			}

			line := fmt.Sprintf("%s%s", check, opt)
			if i == v.filterCursor {
				line = selectedStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}

		if end < len(v.filterVisible) {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("↓ %d more", len(v.filterVisible)-end)))
			b.WriteString("\n")
		}
	}

	b.WriteString(mutedStyle.Render("/: search  enter: toggle  esc: apply & close"))

	dropdownContent := borderStyle.Render(b.String())
	contentHeight := lipgloss.Height(base)
	return overlay.Center(base, dropdownContent, v.width, contentHeight)
}

// --- Pager ---

// openInPager writes all loaded entries to a temp file and opens $PAGER.
// Respects the colorize toggle — includes ANSI codes when colors are on.
func (v *LogsView) openInPager() tea.Cmd {
	if len(v.entries) == 0 {
		return nil
	}

	colorize := v.logViewer.ColorizeEnabled()

	// Build content
	var b strings.Builder
	for i := range v.entries {
		e := &v.entries[i]
		ts := e.Timestamp.Format("2006-01-02 15:04:05.000")
		sev := e.Severity
		msg := strings.ReplaceAll(e.Message, "\n", " ")

		if colorize {
			sevStyle := logviewer.SeverityStyle(e.Severity)
			line := fmt.Sprintf("%s  %s  %s", ts, sevStyle.Render(sev), logviewer.ColorizeMessage(msg, ""))
			b.WriteString(line)
		} else {
			b.WriteString(fmt.Sprintf("%s  %-8s  %s", ts, sev, msg))
		}
		b.WriteString("\n")
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "gcon-logs-*.txt")
	if err != nil {
		v.exportMsg = fmt.Sprintf("Pager error: %s", err)
		return nil
	}
	if _, err := tmpFile.WriteString(b.String()); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpFile.Name()) //nolint:errcheck // best-effort cleanup
		v.exportMsg = fmt.Sprintf("Pager error: %s", err)
		return nil
	}
	tmpFile.Close()
	tmpName := tmpFile.Name()

	// Determine pager command; split on whitespace to support
	// PAGER="less -FRSX" style values with embedded arguments.
	pagerEnv := os.Getenv("PAGER")
	if pagerEnv == "" {
		pagerEnv = "less"
	}
	pagerParts := strings.Fields(pagerEnv)
	pagerCmd := pagerParts[0]
	pagerArgs := pagerParts[1:]

	// Build command with -R flag for ANSI color support
	if pagerCmd == "less" && colorize {
		pagerArgs = append(pagerArgs, "-R")
	}
	pagerArgs = append(pagerArgs, tmpName)

	//nolint:gosec // pager command is user-configured via $PAGER
	c := exec.Command(pagerCmd, pagerArgs...)
	return tea.ExecProcess(c, func(_ error) tea.Msg {
		_ = os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return nil
	})
}

// --- Action menu and export ---

func (v *LogsView) buildActions() []actionmenu.Action {
	hasEntries := len(v.entries) > 0
	return []actionmenu.Action{
		{Key: 't', Label: "Export as TXT", Enabled: hasEntries},
		{Key: 'c', Label: "Export as CSV", Enabled: hasEntries},
		{Key: 'j', Label: "Export as JSONL", Enabled: hasEntries},
	}
}

func (v *LogsView) executeMenuAction(key rune) tea.Cmd {
	switch key {
	case 't':
		return v.exportTXT()
	case 'c':
		return v.exportCSV()
	case 'j':
		return v.exportJSONL()
	}
	return nil
}

func (v *LogsView) exportFilename(ext string) string {
	return fmt.Sprintf("logs-%s.%s", time.Now().Format("20060102-150405"), ext)
}

func (v *LogsView) exportTXT() tea.Cmd {
	filename := v.exportFilename("txt")
	// Snapshot entries for the goroutine
	entries := make([]gcp.LogEntry, len(v.entries))
	copy(entries, v.entries)

	return func() tea.Msg {
		var b strings.Builder
		for i := range entries {
			e := &entries[i]
			b.WriteString(fmt.Sprintf("%s  %s  %s  %s\n",
				e.Timestamp.Format("2006-01-02 15:04:05.000"),
				e.Severity,
				e.ResourceType,
				strings.ReplaceAll(e.Message, "\n", " "),
			))
		}
		if err := os.WriteFile(filename, []byte(b.String()), 0o644); err != nil {
			return logsExportDoneMsg{message: fmt.Sprintf("Export error: %s", err)}
		}
		return logsExportDoneMsg{message: fmt.Sprintf("Exported %d entries to %s", len(entries), filename)}
	}
}

func (v *LogsView) exportCSV() tea.Cmd {
	filename := v.exportFilename("csv")
	entries := make([]gcp.LogEntry, len(v.entries))
	copy(entries, v.entries)

	return func() tea.Msg {
		f, err := os.Create(filename)
		if err != nil {
			return logsExportDoneMsg{message: fmt.Sprintf("Export error: %s", err)}
		}
		defer f.Close()

		w := csv.NewWriter(f)
		if err := w.Write([]string{"timestamp", "severity", "resource_type", "log_name", "insert_id", "message"}); err != nil {
			return logsExportDoneMsg{message: fmt.Sprintf("Export error: %s", err)}
		}
		for i := range entries {
			e := &entries[i]
			if err := w.Write([]string{
				e.Timestamp.Format(time.RFC3339Nano),
				e.Severity,
				e.ResourceType,
				e.LogName,
				e.InsertID,
				e.Message,
			}); err != nil {
				return logsExportDoneMsg{message: fmt.Sprintf("Export error: %s", err)}
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return logsExportDoneMsg{message: fmt.Sprintf("Export error: %s", err)}
		}
		return logsExportDoneMsg{message: fmt.Sprintf("Exported %d entries to %s", len(entries), filename)}
	}
}

// jsonlEntry is the JSON structure for JSONL export.
type jsonlEntry struct {
	Timestamp      string            `json:"timestamp"`
	Severity       string            `json:"severity"`
	Message        string            `json:"message"`
	LogName        string            `json:"logName,omitempty"`
	ResourceType   string            `json:"resourceType,omitempty"`
	ResourceLabels map[string]string `json:"resourceLabels,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	JSONPayload    map[string]any    `json:"jsonPayload,omitempty"`
	TextPayload    string            `json:"textPayload,omitempty"`
	InsertID       string            `json:"insertId,omitempty"`
	TraceID        string            `json:"traceId,omitempty"`
	SpanID         string            `json:"spanId,omitempty"`
}

func (v *LogsView) exportJSONL() tea.Cmd {
	filename := v.exportFilename("jsonl")
	entries := make([]gcp.LogEntry, len(v.entries))
	copy(entries, v.entries)

	return func() tea.Msg {
		f, err := os.Create(filename)
		if err != nil {
			return logsExportDoneMsg{message: fmt.Sprintf("Export error: %s", err)}
		}
		defer f.Close()

		enc := json.NewEncoder(f)
		for i := range entries {
			e := &entries[i]
			je := jsonlEntry{
				Timestamp:      e.Timestamp.Format(time.RFC3339Nano),
				Severity:       e.Severity,
				Message:        e.Message,
				LogName:        e.LogName,
				ResourceType:   e.ResourceType,
				ResourceLabels: e.ResourceLabels,
				Labels:         e.Labels,
				JSONPayload:    e.JSONPayload,
				TextPayload:    e.TextPayload,
				InsertID:       e.InsertID,
				TraceID:        e.TraceID,
				SpanID:         e.SpanID,
			}
			if err := enc.Encode(je); err != nil {
				return logsExportDoneMsg{message: fmt.Sprintf("Export error: %s", err)}
			}
		}
		return logsExportDoneMsg{message: fmt.Sprintf("Exported %d entries to %s", len(entries), filename)}
	}
}

// renderWithOverlay centers an overlay dialog on top of the content.
func (v *LogsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}
