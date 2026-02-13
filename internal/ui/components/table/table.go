// Package table provides a reusable table component with GCP styling and filtering support.
// Enhanced with column caching and flexible column definitions inspired by gh-dash.
package table

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/components/sortmenu"
	"github.com/slayer/gcon/internal/ui/mouse"
	"github.com/slayer/gcon/internal/ui/overlay"
)

// Row represents a table row with filterable content
type Row struct {
	Data        []string // Column values
	FilterValue string   // String used for filtering
	ID          string   // Unique identifier for the row
}

// RowDoubleClickedMsg is emitted when a row is double-clicked.
// Views should handle this to perform the default action (e.g., open details).
type RowDoubleClickedMsg struct {
	RowID string // ID of the double-clicked row
	Index int    // Index of the double-clicked row
}

// Column defines a table column with enhanced properties.
// Inspired by gh-dash's column management pattern.
type Column struct {
	Title         string   // Column header text
	Width         int      // Base width (minimum if Grow is true)
	Hidden        bool     // If true, column is not displayed
	Grow          bool     // If true, column expands to fill available space
	ComputedWidth int      // Cached width after layout calculation (internal)
	FilterKeys    []string // Keys for field:value filtering (auto-derived from Title if nil)
	Sortable      bool     // Whether column appears in sort menu
}

// deriveFilterKeys generates filter keys from a column title.
// "Machine Type" -> ["machine_type", "machinetype", "type"]
// "Zone" -> ["zone"]
func deriveFilterKeys(title string) []string {
	lower := strings.ToLower(title)

	// Split on word boundaries (spaces, underscores)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	keys := make([]string, 0, 3)

	// Full snake_case version
	snakeCase := strings.Join(words, "_")
	keys = append(keys, snakeCase)

	// Concatenated version (no separator)
	if len(words) > 1 {
		keys = append(keys, strings.Join(words, ""))
	}

	// Last word as shorthand (e.g., "type" from "Machine Type")
	if len(words) > 1 {
		lastWord := words[len(words)-1]
		// Only add if it doesn't duplicate an existing key
		if lastWord != snakeCase {
			keys = append(keys, lastWord)
		}
	}

	return keys
}

// KeyMap defines the key bindings for the table
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Filter  key.Binding
	Escape  key.Binding
	Refresh key.Binding
	Sort    key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back/cancel"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Sort: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "sort"),
		),
	}
}

// Model represents a filterable table with GCP styling
type Model struct {
	table     table.Model
	styles    table.Styles // Store styles for updating Selected width
	columns   []table.Column
	colDefs   []Column // Enhanced column definitions with caching
	allRows   []Row    // All rows (unfiltered)
	rows      []Row    // Currently visible rows (filtered)
	filter    textinput.Model
	filtering bool
	focused   bool
	width     int
	height    int
	title     string
	keys      KeyMap

	// Caching to prevent recalculation on every render
	lastWidth       int  // Last width used for column calculation
	columnsComputed bool // True if ComputedWidth values are valid

	// Loading and empty states
	loading      bool
	loadingText  string
	emptyText    string
	statusSuffix string // Extra text appended to status bar (e.g. "all loaded")

	// Sorting state
	sortColumn    int  // Visible column index being sorted (-1 = no sort)
	sortAscending bool // Sort direction
	sortMenu      *sortmenu.SortMenu
	sortMenuOpen  bool

	// Near-bottom detection for infinite scroll
	nearBottomThreshold int  // Rows from bottom to trigger NearBottomMsg (0 = disabled)
	nearBottomEmitted   bool // Prevents duplicate events until reset

	// Mouse support
	hoverIndex    int                  // Index of row being hovered (-1 if none)
	lastClickTime int64                // Unix timestamp of last click for double-click detection
	lastClickRow  int                  // Row index of last click
	regionMgr     *mouse.RegionManager // Manages clickable regions for mouse events
}

// New creates a new table model (backward compatible)
func New(columns []table.Column, title string) Model {
	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	styles := DefaultTableStyles()
	t.SetStyles(styles)

	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 100
	ti.Width = 30

	return Model{
		table:      t,
		styles:     styles,
		columns:    columns,
		allRows:    []Row{},
		rows:       []Row{},
		filter:     ti,
		focused:    true,
		title:      title,
		keys:       DefaultKeyMap(),
		emptyText:  "No items",
		sortColumn: -1,
		hoverIndex: -1,
		regionMgr:  mouse.NewRegionManager(),
	}
}

// NewWithColumns creates a table with enhanced column definitions.
// This enables column hiding, flexible growth, and width caching.
func NewWithColumns(cols []Column, title string) Model {
	// Auto-derive filter keys for columns that don't specify them
	for i := range cols {
		if len(cols[i].FilterKeys) == 0 {
			cols[i].FilterKeys = deriveFilterKeys(cols[i].Title)
		}
	}

	// Remove ambiguous filter keys that appear in multiple columns
	keyCounts := make(map[string]int)
	for _, col := range cols {
		for _, fk := range col.FilterKeys {
			keyCounts[fk]++
		}
	}
	for i := range cols {
		unique := cols[i].FilterKeys[:0]
		for _, fk := range cols[i].FilterKeys {
			if keyCounts[fk] == 1 {
				unique = append(unique, fk)
			}
		}
		cols[i].FilterKeys = unique
	}

	// Convert to bubbles/table columns initially
	tableCols := make([]table.Column, 0, len(cols))
	for _, c := range cols {
		if !c.Hidden {
			tableCols = append(tableCols, table.Column{
				Title: c.Title,
				Width: c.Width,
			})
		}
	}

	t := table.New(
		table.WithColumns(tableCols),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	styles := DefaultTableStyles()
	t.SetStyles(styles)

	ti := textinput.New()
	ti.Placeholder = "Type to filter (field:value)..."
	ti.CharLimit = 100
	ti.Width = 30

	return Model{
		table:      t,
		styles:     styles,
		columns:    tableCols,
		colDefs:    cols,
		allRows:    []Row{},
		rows:       []Row{},
		filter:     ti,
		focused:    true,
		title:      title,
		keys:       DefaultKeyMap(),
		emptyText:  "No items",
		sortColumn: -1,
		hoverIndex: -1,
		regionMgr:  mouse.NewRegionManager(),
	}
}

// SetLoading sets the loading state with optional custom text
func (m *Model) SetLoading(loading bool, text string) {
	m.loading = loading
	if text != "" {
		m.loadingText = text
	} else {
		m.loadingText = "Loading..."
	}
}

// SetTitle updates the table title displayed above the header row.
func (m *Model) SetTitle(title string) {
	m.title = title
}

// SetEmptyText sets the text shown when there are no rows
func (m *Model) SetEmptyText(text string) {
	m.emptyText = text
}

// SetStatusSuffix sets extra text appended to the status bar (e.g. "(all loaded)").
// Pass empty string to clear.
func (m *Model) SetStatusSuffix(suffix string) {
	m.statusSuffix = suffix
}

// IsLoading returns true if the table is in loading state
func (m *Model) IsLoading() bool {
	return m.loading
}

// SetRows updates the table rows. Resets sort since data has changed.
func (m *Model) SetRows(rows []Row) {
	m.allRows = rows
	m.sortColumn = -1
	m.applyFilter()
	// Recalculate to remove sort indicator from header
	m.recalcColumns()
}

// filterSpec represents a parsed filter with field-specific and free-text parts.
type filterSpec struct {
	fieldFilters map[int]string // visible column index -> match substring (lowercase)
	freeText     string         // remaining text for FilterValue match (lowercase)
}

// parseFilter splits filter input into field:value pairs and free text.
// Tokens like "status:running" match column FilterKeys; unmatched tokens become free text.
func (m *Model) parseFilter(input string) filterSpec {
	spec := filterSpec{fieldFilters: make(map[int]string)}

	if len(m.colDefs) == 0 {
		// No enhanced columns — treat entire input as free text
		spec.freeText = strings.ToLower(input)
		return spec
	}

	// Build lookup: filter key -> visible column index
	keyToVisibleIdx := make(map[string]int)
	visibleIdx := 0
	for _, col := range m.colDefs {
		if col.Hidden {
			continue
		}
		for _, fk := range col.FilterKeys {
			keyToVisibleIdx[fk] = visibleIdx
		}
		visibleIdx++
	}

	tokens := strings.Fields(input)
	var freeTokens []string

	for _, token := range tokens {
		colonIdx := strings.IndexByte(token, ':')
		if colonIdx > 0 && colonIdx < len(token)-1 {
			key := strings.ToLower(token[:colonIdx])
			value := strings.ToLower(token[colonIdx+1:])
			if idx, ok := keyToVisibleIdx[key]; ok {
				spec.fieldFilters[idx] = value
				continue
			}
		}
		freeTokens = append(freeTokens, token)
	}

	spec.freeText = strings.ToLower(strings.Join(freeTokens, " "))
	return spec
}

// matchesFilterSpec checks whether a row matches the given filter spec.
// All field filters must match (AND logic), and free text must match FilterValue.
func matchesFilterSpec(row Row, spec filterSpec) bool {
	// Check field-specific filters against row data columns
	for colIdx, value := range spec.fieldFilters {
		if colIdx >= len(row.Data) {
			return false
		}
		if !strings.Contains(strings.ToLower(row.Data[colIdx]), value) {
			return false
		}
	}

	// Check free text against FilterValue
	if spec.freeText != "" {
		if !strings.Contains(strings.ToLower(row.FilterValue), spec.freeText) {
			return false
		}
	}

	return true
}

// applyFilter filters rows based on current filter value.
// When enhanced column definitions exist, supports field:value syntax.
func (m *Model) applyFilter() {
	filterText := m.filter.Value()

	switch {
	case filterText == "":
		m.rows = m.allRows
	case len(m.colDefs) > 0:
		// Enhanced filtering with field:value support
		spec := m.parseFilter(filterText)
		filtered := make([]Row, 0)
		for _, row := range m.allRows {
			if matchesFilterSpec(row, spec) {
				filtered = append(filtered, row)
			}
		}
		m.rows = filtered
	default:
		// Legacy: simple substring match on FilterValue
		lowerFilter := strings.ToLower(filterText)
		filtered := make([]Row, 0)
		for _, row := range m.allRows {
			if strings.Contains(strings.ToLower(row.FilterValue), lowerFilter) {
				filtered = append(filtered, row)
			}
		}
		m.rows = filtered
	}

	// Re-apply sort if active
	if m.sortColumn >= 0 {
		m.sortRows()
	}

	// Convert to table.Row format
	tableRows := make([]table.Row, len(m.rows))
	for i, row := range m.rows {
		tableRows[i] = row.Data
	}
	m.table.SetRows(tableRows)
}

// SortBy sorts the table by a visible column index. Reapplies to current filter.
func (m *Model) SortBy(colIndex int, ascending bool) {
	m.sortColumn = colIndex
	m.sortAscending = ascending
	m.sortRows()

	// Rebuild table rows
	tableRows := make([]table.Row, len(m.rows))
	for i, row := range m.rows {
		tableRows[i] = row.Data
	}
	m.table.SetRows(tableRows)

	// Recalculate columns immediately so header shows sort indicator
	m.recalcColumns()
}

// ClearSort removes the current sort.
func (m *Model) ClearSort() {
	m.sortColumn = -1
	m.applyFilter()
	m.recalcColumns()
}

// recalcColumns forces column width recalculation (e.g. after sort indicator changes).
func (m *Model) recalcColumns() {
	if m.lastWidth > 0 {
		m.columnsComputed = false
		m.adjustColumnWidths(m.lastWidth)
		m.columnsComputed = true
	}
}

// IsSortMenuOpen returns true if the sort menu overlay is active.
// Views should check this to defer key handling.
func (m *Model) IsSortMenuOpen() bool {
	return m.sortMenuOpen
}

// HasTextInputFocused returns true if the filter text input or sort menu is focused.
// Used to prevent global hotkeys (like 'q' for quit) from triggering while typing.
func (m *Model) HasTextInputFocused() bool {
	return m.filtering || m.sortMenuOpen
}

// sortRows sorts m.rows in place by the active sort column.
func (m *Model) sortRows() {
	if m.sortColumn < 0 || m.sortColumn >= len(m.columns) {
		return
	}

	col := m.sortColumn
	asc := m.sortAscending

	sort.SliceStable(m.rows, func(i, j int) bool {
		if col >= len(m.rows[i].Data) || col >= len(m.rows[j].Data) {
			return false
		}
		a := m.rows[i].Data[col]
		b := m.rows[j].Data[col]

		cmp := compareValues(a, b)
		if asc {
			return cmp < 0
		}
		return cmp > 0
	})
}

// compareValues compares two cell values. Tries numeric first, then string.
// Strips leading status symbols (non-letter/digit) before comparing.
func compareValues(a, b string) int {
	cleanA := stripLeadingSymbols(a)
	cleanB := stripLeadingSymbols(b)

	// Try numeric comparison
	numA, okA := parseNumericValue(cleanA)
	numB, okB := parseNumericValue(cleanB)
	if okA && okB {
		switch {
		case numA < numB:
			return -1
		case numA > numB:
			return 1
		default:
			return 0
		}
	}

	// Fall back to case-insensitive string comparison
	return strings.Compare(strings.ToLower(cleanA), strings.ToLower(cleanB))
}

// stripLeadingSymbols removes leading non-alphanumeric characters (status icons like ● ○).
func stripLeadingSymbols(s string) string {
	for i, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return s[i:]
		}
	}
	return s
}

// unitMultipliers maps size suffixes to byte multipliers for correct magnitude comparison.
var unitMultipliers = map[string]float64{
	" B":   1,
	" KB":  1e3,
	" KiB": 1024,
	" MB":  1e6,
	" MiB": 1024 * 1024,
	" GB":  1e9,
	" GiB": 1024 * 1024 * 1024,
	" TB":  1e12,
	" TiB": 1024 * 1024 * 1024 * 1024,
	" PB":  1e15,
}

// parseNumericValue extracts a float64 from a string, applying unit multipliers.
// Handles: "100", "100 GB", "50.5%", "1,024", etc.
// Values with size units (KB/MB/GB/TB/PB) are normalized to bytes for correct comparison.
func parseNumericValue(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0, false
	}

	// Check for unit suffixes and apply multiplier
	multiplier := 1.0
	suffixes := []string{" GiB", " MiB", " KiB", " TiB", " GB", " MB", " KB", " TB", " PB", " B", "%"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			s = strings.TrimSuffix(s, suffix)
			if m, ok := unitMultipliers[suffix]; ok {
				multiplier = m
			}
			break
		}
	}

	// Remove commas from number formatting
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return val * multiplier, true
}

// openSortMenu builds and shows the sort menu from sortable columns.
func (m *Model) openSortMenu() {
	var cols []sortmenu.SortColumn
	visibleIdx := 0
	for _, cd := range m.colDefs {
		if cd.Hidden {
			continue
		}
		if cd.Sortable {
			cols = append(cols, sortmenu.SortColumn{
				Label:    cd.Title,
				ColIndex: visibleIdx,
			})
		}
		visibleIdx++
	}

	if len(cols) == 0 {
		return // No sortable columns
	}

	m.sortMenu = sortmenu.New(cols, m.sortColumn, m.sortAscending)
	m.sortMenuOpen = true
}

// NearBottomMsg is emitted when the cursor approaches the bottom of the table.
// Views can use this to trigger loading more data (infinite scroll).
type NearBottomMsg struct{}

// SetNearBottomThreshold enables near-bottom detection.
// The NearBottomMsg fires when the cursor is within n rows of the last row.
// Set to 0 to disable. Negative values are treated as 0.
func (m *Model) SetNearBottomThreshold(n int) {
	if n < 0 {
		n = 0
	}
	m.nearBottomThreshold = n
}

// ResetNearBottom allows the next NearBottomMsg to fire.
// Call this after new data has been appended.
func (m *Model) ResetNearBottom() {
	m.nearBottomEmitted = false
}

// AppendRows adds rows to the table without resetting cursor or sort.
// Used for infinite scroll to seamlessly add more data.
func (m *Model) AppendRows(newRows []Row) {
	// Remember selected row by ID so we can restore after re-sort
	var selectedID string
	cursor := m.table.Cursor()
	if cursor >= 0 && cursor < len(m.rows) {
		selectedID = m.rows[cursor].ID
	}

	m.allRows = append(m.allRows, newRows...)
	m.nearBottomEmitted = false

	// Re-apply filter (and sort if active)
	m.applyFilter()

	// Restore cursor: find by ID first (handles reordering from sort), fall back to index
	if selectedID != "" {
		for i, row := range m.rows {
			if row.ID == selectedID {
				m.table.SetCursor(i)
				return
			}
		}
	}
	if cursor >= 0 && cursor < len(m.rows) {
		m.table.SetCursor(cursor)
	}
}

// checkNearBottom returns a NearBottomMsg cmd if cursor is near the bottom.
func (m *Model) checkNearBottom() tea.Cmd {
	if m.nearBottomThreshold <= 0 || m.nearBottomEmitted {
		return nil
	}

	cursor := m.table.Cursor()
	rowCount := len(m.rows)

	if rowCount > 0 && cursor >= rowCount-m.nearBottomThreshold {
		m.nearBottomEmitted = true
		return func() tea.Msg { return NearBottomMsg{} }
	}
	return nil
}

// ScrollPercent returns 0.0–1.0 based on cursor position within all rows.
// First item = 0%, last item = 100%. Returns 0 when all rows fit on screen.
func (m *Model) ScrollPercent() float64 {
	total := len(m.rows)
	if total <= 1 || total <= m.table.Height() {
		return 0
	}
	return float64(m.table.Cursor()) / float64(total-1)
}

// IsScrollable returns true when total rows exceed visible height.
func (m *Model) IsScrollable() bool {
	return len(m.rows) > m.table.Height()
}

// SelectedRow returns the currently selected row, or nil if none
func (m *Model) SelectedRow() *Row {
	cursor := m.table.Cursor()
	if cursor >= 0 && cursor < len(m.rows) {
		return &m.rows[cursor]
	}
	return nil
}

// SelectedIndex returns the cursor position
func (m *Model) SelectedIndex() int {
	return m.table.Cursor()
}

// SetSize updates the table dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Reserve space for title (2 lines with margin) and status bar (1 line)
	tableHeight := height - 3
	if m.filtering {
		tableHeight -= 2
	}
	if tableHeight < 3 {
		tableHeight = 3
	}

	m.table.SetWidth(width)
	m.table.SetHeight(tableHeight)

	// Only recalculate column widths if width changed (caching optimization)
	if width != m.lastWidth || !m.columnsComputed {
		m.adjustColumnWidths(width)
		m.lastWidth = width
		m.columnsComputed = true
	}

	// Update Selected style to span full row width with padding to fill background
	m.styles.Selected = m.styles.Selected.
		Width(width).
		MaxWidth(width)
	m.table.SetStyles(m.styles)
}

// adjustColumnWidths distributes width among columns.
// Uses enhanced column definitions if available, with support for Grow flag.
func (m *Model) adjustColumnWidths(totalWidth int) {
	if len(m.columns) == 0 {
		return
	}

	// Account for table overhead: borders and cell separators
	// bubbles/table adds spacing between columns (2 chars each) plus border chars
	tableOverhead := 4 + len(m.columns)*2 // base padding + 2 chars per column gap
	availableWidth := totalWidth - tableOverhead

	// If we have enhanced column definitions, use them
	if len(m.colDefs) > 0 {
		m.adjustColumnsWithDefs(availableWidth)
		return
	}

	// Legacy behavior: expand wider columns proportionally
	requestedWidth := 0
	expandableCount := 0
	for _, col := range m.columns {
		requestedWidth += col.Width
		// Narrow columns (<=5 chars) are kept fixed (e.g., status icons)
		if col.Width > 5 {
			expandableCount++
		}
	}

	// If we have more space, expand only wider columns proportionally
	if availableWidth > requestedWidth && expandableCount > 0 {
		extraSpace := availableWidth - requestedWidth
		extraPerColumn := extraSpace / expandableCount
		for i := range m.columns {
			if m.columns[i].Width > 5 {
				m.columns[i].Width += extraPerColumn
			}
		}
	}

	m.table.SetColumns(m.columns)
}

// adjustColumnsWithDefs uses enhanced column definitions for width calculation.
// Columns with Grow=true share extra space proportionally.
func (m *Model) adjustColumnsWithDefs(availableWidth int) {
	// Calculate fixed width and count growable columns
	fixedWidth := 0
	growCount := 0
	visibleIdx := 0

	// Reserve space for sort indicator on the sorted column so it doesn't overflow
	sortIndicatorWidth := 0
	sortVisibleIdx := -1
	if m.sortColumn >= 0 {
		vi := 0
		for i := range m.colDefs {
			if m.colDefs[i].Hidden {
				continue
			}
			if vi == m.sortColumn {
				sortIndicatorWidth = lipgloss.Width(" ▲")
				sortVisibleIdx = vi
				break
			}
			vi++
		}
	}

	for i := range m.colDefs {
		if m.colDefs[i].Hidden {
			continue
		}
		fixedWidth += m.colDefs[i].Width
		if m.colDefs[i].Grow {
			growCount++
		}
		visibleIdx++
	}

	// Deduct sort indicator from available space so total stays within budget
	extraSpace := availableWidth - fixedWidth - sortIndicatorWidth
	extraPerGrow := 0
	if growCount > 0 && extraSpace > 0 {
		extraPerGrow = extraSpace / growCount
	}

	// Build table columns with computed widths and sort indicators
	tableCols := make([]table.Column, 0, visibleIdx)
	visibleColIdx := 0
	for i := range m.colDefs {
		if m.colDefs[i].Hidden {
			continue
		}

		width := m.colDefs[i].Width
		if m.colDefs[i].Grow && extraPerGrow > 0 {
			width += extraPerGrow
		}

		// Cache the computed width
		m.colDefs[i].ComputedWidth = width

		// Add sort indicator to the sorted column's title
		title := m.colDefs[i].Title
		if visibleColIdx == sortVisibleIdx {
			var indicator string
			if m.sortAscending {
				indicator = " ▲"
			} else {
				indicator = " ▼"
			}
			title += indicator
			width += sortIndicatorWidth
		}

		tableCols = append(tableCols, table.Column{
			Title: title,
			Width: width,
		})
		visibleColIdx++
	}

	m.columns = tableCols
	m.table.SetColumns(tableCols)
}

// SetColumnHidden changes the visibility of a column by title.
// Triggers a recalculation of column widths.
func (m *Model) SetColumnHidden(title string, hidden bool) {
	for i := range m.colDefs {
		if m.colDefs[i].Title == title {
			m.colDefs[i].Hidden = hidden
			m.recalcColumns()
			break
		}
	}
}

// GetVisibleColumnCount returns the number of visible columns
func (m *Model) GetVisibleColumnCount() int {
	if len(m.colDefs) == 0 {
		return len(m.columns)
	}

	count := 0
	for _, col := range m.colDefs {
		if !col.Hidden {
			count++
		}
	}
	return count
}

// Focus sets the focus state
func (m *Model) Focus() {
	m.focused = true
	m.table.Focus()
}

// Blur removes focus
func (m *Model) Blur() {
	m.focused = false
	m.table.Blur()
}

// IsFiltering returns true if filter input is active
func (m *Model) IsFiltering() bool {
	return m.filtering
}

// FilterValue returns the current filter text
func (m *Model) FilterValue() string {
	return m.filter.Value()
}

// RowCount returns the number of visible rows
func (m *Model) RowCount() int {
	return len(m.rows)
}

// TotalRowCount returns the total number of rows (before filtering)
func (m *Model) TotalRowCount() int {
	return len(m.allRows)
}

// handleMouseEvent processes mouse interactions with the table
// This is kept for backward compatibility and handles wheel scroll and hover.
// Click handling is now done via the Clickable interface (HandleRegionClick).
func (m Model) handleMouseEvent(msg tea.MouseMsg) (Model, tea.Cmd) {
	switch msg.Action {
	case tea.MouseActionRelease:
		// Handle wheel scroll
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.table.MoveUp(1)
		case tea.MouseButtonWheelDown:
			m.table.MoveDown(1)
		}

	case tea.MouseActionMotion:
		// Track hover state (for future hover styling)
		// Calculate Y offset for title and filter sections
		yOffset := 3
		if m.filtering || m.filter.Value() != "" {
			yOffset += 2
		}
		yOffset += 1 // Skip header row

		if msg.Y >= yOffset {
			rowY := msg.Y - yOffset
			if rowY >= 0 && rowY < len(m.rows) {
				m.hoverIndex = rowY
			} else {
				m.hoverIndex = -1
			}
		}
	}

	return m, nil
}

// UpdateRegions recalculates clickable regions based on current state.
// Implements the Clickable interface.
func (m *Model) UpdateRegions(offsetX, offsetY int) {
	m.regionMgr.Clear()

	// Calculate Y offset for title and filter sections
	// titleStyle has MarginBottom(1) which renders as Height=2 (title text + blank line)
	// Then we add b.WriteString("\n") which adds one more line
	// Total: 3 lines for title section
	yOffset := offsetY + 3

	// Filter bar if present (either active or showing filter value)
	if m.filtering || m.filter.Value() != "" {
		yOffset += 2 // Filter text + newline
	}

	// The bubbles/table renders: header line (position 0), then data rows (position 1+)
	// Skip the header line by adding 1
	yOffset += 1

	// Add region for each visible row
	visibleRows := m.height - 1 // Subtract header row
	if visibleRows > len(m.rows) {
		visibleRows = len(m.rows)
	}

	for i := range visibleRows {
		m.regionMgr.Add(
			fmt.Sprintf("row-%d", i),
			mouse.Rect{
				X:      offsetX,
				Y:      yOffset + i,
				Width:  m.width,
				Height: 1,
			},
			i, // Row index as metadata
		)
	}
}

// GetRegions returns current clickable regions.
// Implements the Clickable interface.
func (m *Model) GetRegions() []mouse.Region {
	return m.regionMgr.GetRegions()
}

// HandleRegionClick processes a click on a specific region.
// Implements the Clickable interface.
func (m *Model) HandleRegionClick(regionID string) tea.Cmd {
	// Parse region ID to get row index
	var rowIdx int
	if _, err := fmt.Sscanf(regionID, "row-%d", &rowIdx); err != nil {
		return nil
	}

	if rowIdx < 0 || rowIdx >= len(m.rows) {
		return nil
	}

	// Check for double-click (within 500ms)
	now := time.Now().UnixMilli()
	isDoubleClick := m.lastClickRow == rowIdx && now-m.lastClickTime < 500
	m.lastClickTime = now
	m.lastClickRow = rowIdx

	// Update cursor to clicked row
	m.table.SetCursor(rowIdx)

	// Emit double-click message for views to handle
	if isDoubleClick {
		row := m.rows[rowIdx]
		return func() tea.Msg {
			return RowDoubleClickedMsg{
				RowID: row.ID,
				Index: rowIdx,
			}
		}
	}

	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	// Sort menu messages
	case sortmenu.SortSelectedMsg:
		m.sortMenuOpen = false
		m.sortMenu = nil
		m.SortBy(msg.ColIndex, msg.Ascending)
		return m, nil

	case sortmenu.SortMenuClosedMsg:
		m.sortMenuOpen = false
		m.sortMenu = nil
		return m, nil

	case tea.MouseMsg:
		if m.sortMenuOpen {
			return m, nil // Block mouse events while sort menu is open
		}
		return m.handleMouseEvent(msg)

	case tea.KeyMsg:
		// Route keys to sort menu when open
		if m.sortMenuOpen && m.sortMenu != nil {
			cmd := m.sortMenu.Update(msg)
			return m, cmd
		}

		if m.filtering {
			// Handle filter input
			switch msg.String() {
			case "esc", "enter":
				m.filtering = false
				m.filter.Blur()
				m.table.Focus()
				m.applyFilter()
				return m, nil
			default:
				var cmd tea.Cmd
				m.filter, cmd = m.filter.Update(msg)
				m.applyFilter()
				return m, cmd
			}
		}

		// Open sort menu on 'S' (only if sortable columns exist)
		if key.Matches(msg, m.keys.Sort) && len(m.colDefs) > 0 {
			m.openSortMenu()
			return m, nil
		}

		// Table navigation - enter filter mode on '/'
		if key.Matches(msg, m.keys.Filter) {
			m.filtering = true
			m.filter.Focus()
			m.table.Blur()
			return m, m.filter.Focus()
		}
	}

	// Update table
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	// Check near-bottom after cursor movement for infinite scroll
	if nbCmd := m.checkNearBottom(); nbCmd != nil {
		cmds = append(cmds, nbCmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the table
func (m Model) View() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n")

	// Filter input if active
	if m.filtering {
		filterStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginBottom(1)
		b.WriteString(filterStyle.Render("Filter: "))
		b.WriteString(m.filter.View())
		b.WriteString("\n")
	} else if m.filter.Value() != "" {
		// Show active filter
		filterStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)
		b.WriteString(filterStyle.Render("Filtered: " + m.filter.Value()))
		b.WriteString("\n")
	}

	// Loading state
	if m.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Italic(true).
			Padding(2, 0)
		b.WriteString(loadingStyle.Render(m.loadingText))
		return b.String()
	}

	// Empty state
	if len(m.rows) == 0 && len(m.allRows) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true).
			Padding(2, 0)
		b.WriteString(emptyStyle.Render(m.emptyText))
		return b.String()
	}

	// Table
	tableView := m.table.View()
	b.WriteString(tableView)

	// Status bar: item count on left, scroll % right-aligned
	statusStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	leftText := m.statusText()
	if m.statusSuffix != "" {
		leftText += " " + m.statusSuffix
	}
	if m.filter.Value() != "" {
		leftText += " (filtered)"
	}

	b.WriteString("\n")
	if m.IsScrollable() && tableView != "" {
		scrollStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
		rightText := fmt.Sprintf("%.0f%%", m.ScrollPercent()*100)

		// Measure from rendered table width so status bar aligns with table edge
		firstLine := strings.SplitN(tableView, "\n", 2)[0]
		lineWidth := lipgloss.Width(firstLine)
		contentWidth := lipgloss.Width(leftText) + lipgloss.Width(rightText)

		if lineWidth > contentWidth {
			gap := lineWidth - contentWidth
			b.WriteString(statusStyle.Render(leftText))
			b.WriteString(strings.Repeat(" ", gap))
			b.WriteString(scrollStyle.Render(rightText))
		} else {
			// Not enough room for right-alignment, show inline
			b.WriteString(statusStyle.Render(leftText))
			b.WriteString(" ")
			b.WriteString(scrollStyle.Render(rightText))
		}
	} else {
		b.WriteString(statusStyle.Render(leftText))
	}

	content := b.String()

	// Overlay sort menu if open
	if m.sortMenuOpen && m.sortMenu != nil {
		contentHeight := lipgloss.Height(content)
		content = overlay.Center(content, m.sortMenu.View(), m.width, contentHeight)
	}

	return content
}

// statusText returns the status bar text
func (m Model) statusText() string {
	total := m.TotalRowCount()
	visible := m.RowCount()

	if total == 0 {
		return "No items"
	}

	if visible != total {
		return strconv.Itoa(visible) + "/" + strconv.Itoa(total) + " items"
	}

	if total == 1 {
		return "1 item"
	}
	return strconv.Itoa(total) + " items"
}
