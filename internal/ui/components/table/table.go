// Package table provides a reusable table component with GCP styling and filtering support.
// Enhanced with column caching and flexible column definitions inspired by gh-dash.
package table

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Row represents a table row with filterable content
type Row struct {
	Data        []string // Column values
	FilterValue string   // String used for filtering
	ID          string   // Unique identifier for the row
}

// Column defines a table column with enhanced properties.
// Inspired by gh-dash's column management pattern.
type Column struct {
	Title         string // Column header text
	Width         int    // Base width (minimum if Grow is true)
	Hidden        bool   // If true, column is not displayed
	Grow          bool   // If true, column expands to fill available space
	ComputedWidth int    // Cached width after layout calculation (internal)
}

// KeyMap defines the key bindings for the table
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Filter  key.Binding
	Escape  key.Binding
	Refresh key.Binding
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
	loading     bool
	loadingText string
	emptyText   string

	// Mouse support
	hoverIndex    int   // Index of row being hovered (-1 if none)
	lastClickTime int64 // Unix timestamp of last click for double-click detection
	lastClickRow  int   // Row index of last click
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
		hoverIndex: -1,
	}
}

// NewWithColumns creates a table with enhanced column definitions.
// This enables column hiding, flexible growth, and width caching.
func NewWithColumns(cols []Column, title string) Model {
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
	ti.Placeholder = "Type to filter..."
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
		hoverIndex: -1,
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

// SetEmptyText sets the text shown when there are no rows
func (m *Model) SetEmptyText(text string) {
	m.emptyText = text
}

// IsLoading returns true if the table is in loading state
func (m *Model) IsLoading() bool {
	return m.loading
}

// SetRows updates the table rows
func (m *Model) SetRows(rows []Row) {
	m.allRows = rows
	m.applyFilter()
}

// applyFilter filters rows based on current filter value
func (m *Model) applyFilter() {
	filterText := strings.ToLower(m.filter.Value())

	if filterText == "" {
		m.rows = m.allRows
	} else {
		filtered := make([]Row, 0)
		for _, row := range m.allRows {
			if strings.Contains(strings.ToLower(row.FilterValue), filterText) {
				filtered = append(filtered, row)
			}
		}
		m.rows = filtered
	}

	// Convert to table.Row format
	tableRows := make([]table.Row, len(m.rows))
	for i, row := range m.rows {
		tableRows[i] = row.Data
	}
	m.table.SetRows(tableRows)
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

	// Reserve space for title, filter, and help
	tableHeight := height - 4
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

	// Calculate extra space to distribute among growable columns
	extraSpace := availableWidth - fixedWidth
	extraPerGrow := 0
	if growCount > 0 && extraSpace > 0 {
		extraPerGrow = extraSpace / growCount
	}

	// Build table columns with computed widths
	tableCols := make([]table.Column, 0, visibleIdx)
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

		tableCols = append(tableCols, table.Column{
			Title: m.colDefs[i].Title,
			Width: width,
		})
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
			m.columnsComputed = false // Force recalculation
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
func (m Model) handleMouseEvent(msg tea.MouseMsg) (Model, tea.Cmd) {
	// Calculate the Y offset to account for title and filter bar
	// Title: 2 lines (text + newline)
	// Filter: 2 lines if active or 1 line if showing filter value
	yOffset := 2 // Title
	if m.filtering || m.filter.Value() != "" {
		yOffset += 2 // Filter bar
	}
	// Add 2 more for table header
	yOffset += 2

	// Check if click is within the table rows area
	if msg.Y < yOffset {
		return m, nil
	}

	// Calculate which row was clicked (relative to first visible row)
	rowY := msg.Y - yOffset
	if rowY < 0 || rowY >= len(m.rows) {
		m.hoverIndex = -1
		return m, nil
	}

	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonLeft {
			// Check for double-click (within 500ms)
			now := time.Now().UnixMilli()
			doubleClick := false
			if m.lastClickRow == rowY && now-m.lastClickTime < 500 {
				doubleClick = true
			}
			m.lastClickTime = now
			m.lastClickRow = rowY

			// Update cursor to clicked row
			m.table.SetCursor(rowY)

			// If double-click, trigger selection (simulated Enter key)
			if doubleClick {
				var cmd tea.Cmd
				m.table, cmd = m.table.Update(tea.KeyMsg{Type: tea.KeyEnter})
				return m, cmd
			}
		}

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
		m.hoverIndex = rowY
	}

	return m, nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m.handleMouseEvent(msg)

	case tea.KeyMsg:
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
	b.WriteString(m.table.View())

	// Status bar
	statusStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		MarginTop(1)

	var status string
	if m.filter.Value() != "" {
		status = lipgloss.NewStyle().Render(
			statusStyle.Render(
				strings.Join([]string{
					m.statusText(),
					"(filtered)",
				}, " "),
			),
		)
	} else {
		status = statusStyle.Render(m.statusText())
	}
	b.WriteString("\n")
	b.WriteString(status)

	return b.String()
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
