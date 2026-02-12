// Package sortmenu provides a popup sort column selector for table views.
// Follows the actionmenu pattern: renders as centered overlay with hotkey selection.
package sortmenu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// SortColumn represents a sortable column entry in the menu.
type SortColumn struct {
	Label    string // Display text (column title)
	ColIndex int    // Index into visible columns (for sorting Row.Data)
}

// SortSelectedMsg is emitted when a sort column is selected.
type SortSelectedMsg struct {
	ColIndex  int
	Ascending bool
}

// SortMenuClosedMsg is emitted when the menu is closed without selection.
type SortMenuClosedMsg struct{}

// SortMenu is the popup sort column selector.
type SortMenu struct {
	columns []SortColumn
	cursor  int
	width   int
	keys    keyMap
	styles  Styles

	// Current sort state (to show indicator and toggle direction)
	activeColIndex int  // -1 if no sort active
	activeAsc      bool // direction of current sort
}

type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Close  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "q", "S"),
			key.WithHelp("esc", "close"),
		),
	}
}

// New creates a new sort menu from a list of sortable columns.
// activeColIndex is the currently sorted column (-1 for none).
func New(columns []SortColumn, activeColIndex int, activeAsc bool) *SortMenu {
	// Calculate width based on longest label + hotkey + indicator
	maxWidth := len("Sort by Column") + 6
	for i, c := range columns {
		// Format: "  1  Label  ▲" = hotkey(1-2 chars) + spaces + label + indicator
		w := 7 + len(c.Label) + len(hotkey(i))
		if w > maxWidth {
			maxWidth = w
		}
	}
	maxWidth += 10 // padding for border

	// Find initial cursor: put on active column if any, else first
	cursor := 0
	if activeColIndex >= 0 {
		for i, c := range columns {
			if c.ColIndex == activeColIndex {
				cursor = i
				break
			}
		}
	}

	return &SortMenu{
		columns:        columns,
		cursor:         cursor,
		width:          maxWidth,
		keys:           defaultKeyMap(),
		styles:         DefaultStyles(),
		activeColIndex: activeColIndex,
		activeAsc:      activeAsc,
	}
}

// Update handles input for the sort menu.
func (m *SortMenu) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	// Handle navigation and control keys before hotkey matching
	// to prevent collisions when column count exceeds 9 (j/k would be hotkeys)
	switch {
	case key.Matches(keyMsg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return nil

	case key.Matches(keyMsg, m.keys.Down):
		if m.cursor < len(m.columns)-1 {
			m.cursor++
		}
		return nil

	case key.Matches(keyMsg, m.keys.Select):
		if m.cursor >= 0 && m.cursor < len(m.columns) {
			return m.selectColumn(m.cursor)
		}
		return nil

	case key.Matches(keyMsg, m.keys.Close):
		return func() tea.Msg { return SortMenuClosedMsg{} }
	}

	// Direct hotkey selection (number keys 1-9, then a-z)
	keyStr := keyMsg.String()
	if len(keyStr) == 1 {
		idx := hotkeyToIndex(rune(keyStr[0]))
		if idx >= 0 && idx < len(m.columns) {
			return m.selectColumn(idx)
		}
	}

	return nil
}

// selectColumn picks a column and determines sort direction.
// Re-selecting the same column toggles direction; new column defaults ascending.
func (m *SortMenu) selectColumn(idx int) tea.Cmd {
	col := m.columns[idx]
	ascending := true

	if col.ColIndex == m.activeColIndex {
		// Toggle direction
		ascending = !m.activeAsc
	}

	return func() tea.Msg {
		return SortSelectedMsg{
			ColIndex:  col.ColIndex,
			Ascending: ascending,
		}
	}
}

// View renders the sort menu.
func (m *SortMenu) View() string {
	var b strings.Builder

	// Title
	b.WriteString(m.styles.Title.Render("Sort by Column"))
	b.WriteString("\n")

	// Column entries
	for i, col := range m.columns {
		line := m.renderEntry(col, i, i == m.cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Divider
	dividerWidth := m.width - 4
	if dividerWidth < 1 {
		dividerWidth = 1
	}
	b.WriteString(m.styles.Divider.Render(strings.Repeat("─", dividerWidth)))
	b.WriteString("\n")

	// Help
	b.WriteString(m.styles.Help.Render("j/k:nav  enter:sort  esc:close"))

	return m.styles.Container.Width(m.width).Render(b.String())
}

// renderEntry renders a single column entry.
func (m *SortMenu) renderEntry(col SortColumn, idx int, selected bool) string {
	cursor := "  "
	if selected {
		cursor = symbols.Cursor() + " "
	}

	hk := hotkey(idx)

	// Sort direction indicator for active column
	indicator := "  "
	if col.ColIndex == m.activeColIndex {
		if m.activeAsc {
			indicator = " ▲"
		} else {
			indicator = " ▼"
		}
	}

	var keyStyle, labelStyle lipgloss.Style
	if selected {
		keyStyle = m.styles.KeySelected
		labelStyle = m.styles.LabelSelected
	} else {
		keyStyle = m.styles.Key
		labelStyle = m.styles.Label
	}

	return cursor + keyStyle.Render(hk) + "  " + labelStyle.Render(col.Label) + m.styles.Indicator.Render(indicator)
}

// hotkey returns the display string for a positional hotkey.
// 0-8 -> "1"-"9", 9+ -> "a", "b", etc.
func hotkey(idx int) string {
	if idx < 9 {
		return fmt.Sprintf("%d", idx+1)
	}
	return string(rune('a' + idx - 9))
}

// hotkeyToIndex converts a key rune to a column index.
func hotkeyToIndex(r rune) int {
	if r >= '1' && r <= '9' {
		return int(r - '1')
	}
	if r >= 'a' && r <= 'z' {
		return int(r-'a') + 9
	}
	return -1
}

// GetCursor returns the current cursor position (for testing).
func (m *SortMenu) GetCursor() int {
	return m.cursor
}

// GetColumns returns the columns (for testing).
func (m *SortMenu) GetColumns() []SortColumn {
	return m.columns
}
