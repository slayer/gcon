package views

import (
	gocontext "context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
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
	logsFocusEntries logsFocusArea = iota
	logsFocusQuery
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
	Up         key.Binding
	Down       key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Enter      key.Binding
	Left       key.Binding
	Right      key.Binding
	ExpandAll  key.Binding
	CollapseAl key.Binding
	FocusQuery key.Binding
	Refresh    key.Binding
	TailMode   key.Binding
	ToggleWrap key.Binding
	FilterRes  key.Binding
	FilterLog  key.Binding
	FilterSev  key.Binding
	Escape     key.Binding
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

	// Filter dropdown overlay state
	activeFilter   filterDropdownType
	filterOptions  []string
	filterSelected map[string]bool
	filterCursor   int

	// Data
	entries       []gcp.LogEntry
	nextPageToken string
	totalCount    int64
	histogramData []gcp.DataPoint
	loadingMore   bool

	// Time range
	timeRange time.Duration

	// Tail mode: polls for new entries every 5s
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

// Init starts data loading. Must be idempotent — may be called more than once.
func (v *LogsView) Init() tea.Cmd {
	v.state = logsStateLoading
	v.err = nil
	return tea.Batch(
		v.spinner.Tick,
		v.executeQuery(),
		v.loadHistogram(),
		v.loadResourceTypes(),
		v.loadLogNames(),
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
	v.logViewer.SetSize(v.width, v.height-12) // reserve space for header/sparkline/hints
}

// HasTextInputFocused returns true when the query input or a filter dropdown is active.
func (v *LogsView) HasTextInputFocused() bool {
	return v.queryFocused
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
		v.totalCount = msg.total
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
		return nil

	case logsResourceTypesErrorMsg:
		v.resourcesLoaded = true // mark loaded even on error so we don't retry
		return nil

	case logsLogNamesLoadedMsg:
		v.availableLogNames = msg.names
		v.logNamesLoaded = true
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
			// Prepend new entries at the top (newest first)
			v.entries = append(msg.entries, v.entries...)
			v.logViewer.SetEntries(v.entries)
			v.logViewer.SetHasMore(v.nextPageToken != "")
		}
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

	case key.Matches(msg, v.keys.TailMode):
		return v.toggleTailMode()

	case key.Matches(msg, v.keys.Refresh):
		return v.runQuery()

	case key.Matches(msg, v.keys.FilterRes):
		v.openFilterDropdown(filterDropdownResources)
		return nil

	case key.Matches(msg, v.keys.FilterLog):
		v.openFilterDropdown(filterDropdownLogNames)
		return nil

	case key.Matches(msg, v.keys.FilterSev):
		v.openFilterDropdown(filterDropdownSeverities)
		return nil
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
func (v *LogsView) openFilterDropdown(ft filterDropdownType) {
	v.activeFilter = ft
	v.filterCursor = 0

	// Build option list and pre-select current selections
	v.filterSelected = make(map[string]bool)
	switch ft {
	case filterDropdownResources:
		v.filterOptions = v.availableResources
		for _, r := range v.selectedResources {
			v.filterSelected[r] = true
		}
	case filterDropdownLogNames:
		v.filterOptions = v.availableLogNames
		for _, n := range v.selectedLogNames {
			v.filterSelected[n] = true
		}
	case filterDropdownSeverities:
		v.filterOptions = allSeverities
		for _, s := range v.selectedSeverities {
			v.filterSelected[s] = true
		}
	default:
		v.activeFilter = filterDropdownNone
	}
}

// handleFilterDropdownKey handles keys while a filter dropdown is open.
func (v *LogsView) handleFilterDropdownKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if v.filterCursor > 0 {
			v.filterCursor--
		}
		return nil

	case "down", "j":
		if v.filterCursor < len(v.filterOptions)-1 {
			v.filterCursor++
		}
		return nil

	case " ", "enter":
		// Toggle selection on the current option
		if v.filterCursor < len(v.filterOptions) {
			opt := v.filterOptions[v.filterCursor]
			v.filterSelected[opt] = !v.filterSelected[opt]
		}
		return nil

	case "esc":
		// Apply selections and close the dropdown
		v.applyFilterDropdown()
		v.activeFilter = filterDropdownNone
		return v.runQuery()
	}

	return nil
}

// applyFilterDropdown saves the dropdown selections to the corresponding filter slice.
func (v *LogsView) applyFilterDropdown() {
	var selected []string
	for _, opt := range v.filterOptions {
		if v.filterSelected[opt] {
			selected = append(selected, opt)
		}
	}

	switch v.activeFilter {
	case filterDropdownResources:
		v.selectedResources = selected
	case filterDropdownLogNames:
		v.selectedLogNames = selected
	case filterDropdownSeverities:
		v.selectedSeverities = selected
	}
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
		return fmt.Sprintf(`%s = "%s"`, field, values[0])
	}
	clauses := make([]string, len(values))
	for i, v := range values {
		clauses[i] = fmt.Sprintf(`%s = "%s"`, field, v)
	}
	return "(" + strings.Join(clauses, " OR ") + ")"
}

// appendFilterToQuery adds a key=value clause from a log entry field.
func (v *LogsView) appendFilterToQuery(fieldKey, value string) {
	clause := fmt.Sprintf(`%s = "%s"`, fieldKey, value) //nolint:gocritic // GCP LQL filter syntax
	if v.query != "" {
		v.query += "\n" + clause
	} else {
		v.query = clause
	}
	v.queryInput.SetValue(v.query)
}

// --- Data loading ---

// runQuery resets state and executes the current query.
func (v *LogsView) runQuery() tea.Cmd {
	v.state = logsStateLoading
	v.err = nil
	v.nextPageToken = ""
	return tea.Batch(v.executeQuery(), v.loadHistogram())
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

		entries, nextToken, err := logClient.ListLogEntries(ctx, filter, 100, "")
		if err != nil {
			return logsEntriesErrorMsg{err: err}
		}

		return logsEntriesLoadedMsg{
			entries:   entries,
			nextToken: nextToken,
			total:     int64(len(entries)), // approximate; histogram gives better count
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

		entries, nextToken, err := logClient.ListLogEntries(ctx, filter, 100, token)
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
	return v.runQuery()
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
	v.tailTicker = time.NewTicker(5 * time.Second)
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

	// Time range selector
	b.WriteString("  ")
	b.WriteString(components.RenderTimeRangeSelector(v.timeRange, v.tailMode, time.Time{}))
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
	if v.tailMode {
		tailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")).Bold(true)
		statusParts = append(statusParts, tailStyle.Render("TAIL"))
	}
	if v.logViewer.WrapEnabled() {
		wrapStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8")).Bold(true)
		statusParts = append(statusParts, wrapStyle.Render("WRAP"))
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
	b.WriteString(mutedStyle.Render("  /: query  .: actions  1-5: time  f: tail  w: wrap  R/L/V: filters  E/C: expand  r: refresh"))
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

// renderFilterPills renders compact buttons for each active filter.
func (v *LogsView) renderFilterPills() string {
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368"))

	var pills []string

	// Resources
	if len(v.selectedResources) > 0 {
		pills = append(pills, activeStyle.Render(fmt.Sprintf("[R:%d]", len(v.selectedResources))))
	} else {
		pills = append(pills, inactiveStyle.Render("[R:all]"))
	}

	// Log names
	if len(v.selectedLogNames) > 0 {
		pills = append(pills, activeStyle.Render(fmt.Sprintf("[L:%d]", len(v.selectedLogNames))))
	} else {
		pills = append(pills, inactiveStyle.Render("[L:all]"))
	}

	// Severities
	if len(v.selectedSeverities) > 0 {
		pills = append(pills, activeStyle.Render(fmt.Sprintf("[V:%d]", len(v.selectedSeverities))))
	} else {
		pills = append(pills, inactiveStyle.Render("[V:all]"))
	}

	return strings.Join(pills, " ")
}

// renderWithFilterDropdown overlays the filter dropdown onto the main view.
func (v *LogsView) renderWithFilterDropdown(base string) string {
	var b strings.Builder
	b.WriteString(base)

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("#3C4043"))
	checkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	// Dropdown header
	var title string
	switch v.activeFilter {
	case filterDropdownResources:
		title = "Resource Types"
	case filterDropdownLogNames:
		title = "Log Names"
	case filterDropdownSeverities:
		title = "Severities"
	}

	b.WriteString("\n")
	b.WriteString("  " + headerStyle.Render(title))
	b.WriteString("\n")

	if len(v.filterOptions) == 0 {
		b.WriteString("  " + mutedStyle.Render("(no options available)"))
		b.WriteString("\n")
	} else {
		// Show up to 15 visible options with scroll
		maxVisible := 15
		start := 0
		if v.filterCursor >= maxVisible {
			start = v.filterCursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(v.filterOptions) {
			end = len(v.filterOptions)
		}

		if start > 0 {
			b.WriteString(fmt.Sprintf("  %s\n", mutedStyle.Render(fmt.Sprintf("↑ %d more", start))))
		}

		for i := start; i < end; i++ {
			opt := v.filterOptions[i]
			check := "  "
			if v.filterSelected[opt] {
				check = checkStyle.Render("✓ ")
			}

			line := fmt.Sprintf("  %s%s", check, opt)
			if i == v.filterCursor {
				line = selectedStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}

		if end < len(v.filterOptions) {
			b.WriteString(fmt.Sprintf("  %s\n", mutedStyle.Render(fmt.Sprintf("↓ %d more", len(v.filterOptions)-end))))
		}
	}

	b.WriteString("  " + mutedStyle.Render("space: toggle  esc: apply & close"))
	b.WriteString("\n")

	return b.String()
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
	var b strings.Builder
	for i := range v.entries {
		e := &v.entries[i]
		b.WriteString(fmt.Sprintf("%s  %s  %s  %s\n",
			e.Timestamp.Format("2006-01-02 15:04:05.000"),
			e.Severity,
			e.ResourceType,
			strings.ReplaceAll(e.Message, "\n", " "),
		))
	}
	if err := os.WriteFile(filename, []byte(b.String()), 0o644); err != nil {
		v.exportMsg = fmt.Sprintf("Export error: %s", err)
		return nil
	}
	v.exportMsg = fmt.Sprintf("Exported %d entries to %s", len(v.entries), filename)
	return nil
}

func (v *LogsView) exportCSV() tea.Cmd {
	filename := v.exportFilename("csv")
	f, err := os.Create(filename)
	if err != nil {
		v.exportMsg = fmt.Sprintf("Export error: %s", err)
		return nil
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"timestamp", "severity", "resource_type", "log_name", "insert_id", "message"}); err != nil {
		v.exportMsg = fmt.Sprintf("Export error: %s", err)
		return nil
	}
	for i := range v.entries {
		e := &v.entries[i]
		if err := w.Write([]string{
			e.Timestamp.Format(time.RFC3339Nano),
			e.Severity,
			e.ResourceType,
			e.LogName,
			e.InsertID,
			e.Message,
		}); err != nil {
			v.exportMsg = fmt.Sprintf("Export error: %s", err)
			return nil
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		v.exportMsg = fmt.Sprintf("Export error: %s", err)
		return nil
	}
	v.exportMsg = fmt.Sprintf("Exported %d entries to %s", len(v.entries), filename)
	return nil
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
	f, err := os.Create(filename)
	if err != nil {
		v.exportMsg = fmt.Sprintf("Export error: %s", err)
		return nil
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for i := range v.entries {
		e := &v.entries[i]
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
			v.exportMsg = fmt.Sprintf("Export error: %s", err)
			return nil
		}
	}
	v.exportMsg = fmt.Sprintf("Exported %d entries to %s", len(v.entries), filename)
	return nil
}

// renderWithOverlay centers an overlay dialog on top of the content.
func (v *LogsView) renderWithOverlay(content, overlayContent string) string {
	contentHeight := lipgloss.Height(content)
	return overlay.Center(content, overlayContent, v.width, contentHeight)
}
