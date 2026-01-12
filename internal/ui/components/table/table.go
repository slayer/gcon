// Package table provides a reusable table component with GCP styling and filtering support.
package table

import (
	"strings"

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
	columns   []table.Column
	allRows   []Row // All rows (unfiltered)
	rows      []Row // Currently visible rows (filtered)
	filter    textinput.Model
	filtering bool
	focused   bool
	width     int
	height    int
	title     string
	keys      KeyMap
}

// New creates a new table model
func New(columns []table.Column, title string) Model {
	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(DefaultTableStyles())

	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 100
	ti.Width = 30

	return Model{
		table:   t,
		columns: columns,
		allRows: []Row{},
		rows:    []Row{},
		filter:  ti,
		focused: true,
		title:   title,
		keys:    DefaultKeyMap(),
	}
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

	// Adjust column widths proportionally
	m.adjustColumnWidths(width)
}

// adjustColumnWidths distributes width among columns
func (m *Model) adjustColumnWidths(totalWidth int) {
	if len(m.columns) == 0 {
		return
	}

	// Calculate total requested width
	requestedWidth := 0
	for _, col := range m.columns {
		requestedWidth += col.Width
	}

	// If we have more space, expand columns proportionally
	if totalWidth > requestedWidth {
		extraSpace := totalWidth - requestedWidth - 4 // Leave some padding
		extraPerColumn := extraSpace / len(m.columns)
		for i := range m.columns {
			m.columns[i].Width += extraPerColumn
		}
	}

	m.table.SetColumns(m.columns)
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

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.filtering {
			// Handle filter input
			switch keyMsg.String() {
			case "esc", "enter":
				m.filtering = false
				m.filter.Blur()
				m.table.Focus()
				m.applyFilter()
				return m, nil
			default:
				var cmd tea.Cmd
				m.filter, cmd = m.filter.Update(keyMsg)
				m.applyFilter()
				return m, cmd
			}
		}

		// Table navigation - enter filter mode on '/'
		if key.Matches(keyMsg, m.keys.Filter) {
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
		return lipgloss.NewStyle().Render(
			strings.Join([]string{
				string(rune('0' + visible)),
				"/",
				string(rune('0' + total)),
				" items",
			}, ""),
		)
	}

	if total == 1 {
		return "1 item"
	}
	return lipgloss.NewStyle().Render(
		strings.Join([]string{
			intToStr(total),
			" items",
		}, ""),
	)
}

// intToStr converts int to string without fmt import overhead
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToStr(-n)
	}

	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
