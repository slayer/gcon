package logviewer

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

// NeedMoreLogsMsg is emitted when the viewer needs more log entries (infinite scroll).
type NeedMoreLogsMsg struct{}

// FilterFieldMsg is emitted when the user presses Enter on an expanded field.
type FilterFieldMsg struct {
	Key   string
	Value string
}

// Model is the log entry viewer component.
type Model struct {
	entries    []gcp.LogEntry
	expanded   map[int]bool // entry index -> expanded
	cursor     int          // selected entry index
	fieldCur   int          // selected field within expanded entry (-1 = none)
	hasMore    bool         // more pages available
	wrapLines  bool         // show full message wrapped vs truncated
	colorize   bool         // colorize logfmt key=value pairs
	width      int
	height     int
	offset     int // scroll offset for virtual scrolling
}

// New creates a new LogViewer model.
func New() *Model {
	return &Model{
		expanded: make(map[int]bool),
		fieldCur: -1,
		colorize: true, // on by default
	}
}

// SetSize sets the available rendering dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetEntries replaces the entry list (e.g., on new query).
func (m *Model) SetEntries(entries []gcp.LogEntry) {
	m.entries = entries
	m.expanded = make(map[int]bool)
	m.cursor = 0
	m.fieldCur = -1
	m.offset = 0
}

// AppendEntries adds entries to the end (for infinite scroll).
func (m *Model) AppendEntries(entries []gcp.LogEntry) {
	m.entries = append(m.entries, entries...)
}

// PrependEntries adds entries at the beginning (for tail mode).
// Adjusts cursor and scroll offset so the user's position is preserved.
func (m *Model) PrependEntries(entries []gcp.LogEntry) {
	count := len(entries)
	if count == 0 {
		return
	}
	m.entries = append(entries, m.entries...)

	// Shift cursor and offset down so user stays on the same entry
	m.cursor += count
	m.offset += count

	// Shift expanded entries map
	newExpanded := make(map[int]bool, len(m.expanded))
	for idx, v := range m.expanded {
		newExpanded[idx+count] = v
	}
	m.expanded = newExpanded
}

// SetHasMore sets whether there are more pages available.
func (m *Model) SetHasMore(hasMore bool) {
	m.hasMore = hasMore
}

// ToggleWrap toggles between wrapped (full message) and truncated display.
func (m *Model) ToggleWrap() {
	m.wrapLines = !m.wrapLines
	m.ensureVisible()
}

// WrapEnabled returns true when line wrapping is on.
func (m *Model) WrapEnabled() bool {
	return m.wrapLines
}

// ToggleColorize toggles logfmt syntax colorization.
func (m *Model) ToggleColorize() {
	m.colorize = !m.colorize
}

// ColorizeEnabled returns true when logfmt colorization is on.
func (m *Model) ColorizeEnabled() bool {
	return m.colorize
}

// EntryCount returns the number of entries.
func (m *Model) EntryCount() int {
	return len(m.entries)
}

// Cursor returns the current cursor position.
func (m *Model) Cursor() int {
	return m.cursor
}

// FieldCursor returns the field cursor position (-1 = not in field nav).
func (m *Model) FieldCursor() int {
	return m.fieldCur
}

// IsExpanded returns whether an entry at index is expanded.
func (m *Model) IsExpanded(idx int) bool {
	return m.expanded[idx]
}

// ToggleExpand toggles the expand state of an entry.
func (m *Model) ToggleExpand(idx int) {
	if m.expanded[idx] {
		delete(m.expanded, idx)
		m.fieldCur = -1
	} else {
		m.expanded[idx] = true
	}
}

// ExpandAll expands all visible entries.
func (m *Model) ExpandAll() {
	for i := range m.entries {
		m.expanded[i] = true
	}
}

// CollapseAll collapses all entries.
func (m *Model) CollapseAll() {
	m.expanded = make(map[int]bool)
	m.fieldCur = -1
}

// MoveUp moves the cursor up one entry (or exits field nav).
func (m *Model) MoveUp() {
	if m.fieldCur >= 0 {
		m.FieldUp()
		return
	}
	if m.cursor > 0 {
		m.cursor--
		m.fieldCur = -1
		m.ensureVisible()
	}
}

// MoveDown moves the cursor down one entry (or advances field nav).
func (m *Model) MoveDown() {
	if m.fieldCur >= 0 {
		m.FieldDown()
		return
	}
	if m.cursor < len(m.entries)-1 {
		m.cursor++
		m.fieldCur = -1
		m.ensureVisible()
	}
}

// PageUp moves the cursor up by roughly one page of entries.
func (m *Model) PageUp() {
	if len(m.entries) == 0 {
		return
	}
	m.fieldCur = -1
	m.cursor -= m.height
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureVisible()
}

// PageDown moves the cursor down by roughly one page of entries.
func (m *Model) PageDown() {
	if len(m.entries) == 0 {
		return
	}
	m.fieldCur = -1
	m.cursor += m.height
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	m.ensureVisible()
}

// EnterFieldNav enters field navigation mode for the current expanded entry.
func (m *Model) EnterFieldNav() {
	if !m.expanded[m.cursor] {
		return
	}
	fields := m.entries[m.cursor].FlattenFields()
	if len(fields) > 0 {
		m.fieldCur = 0
	}
}

// ExitFieldNav exits field navigation mode.
func (m *Model) ExitFieldNav() {
	m.fieldCur = -1
}

// FieldUp moves the field cursor up.
func (m *Model) FieldUp() {
	if m.fieldCur > 0 {
		m.fieldCur--
	} else {
		// Exit field nav and go back to entry navigation
		m.fieldCur = -1
	}
}

// FieldDown moves the field cursor down.
func (m *Model) FieldDown() {
	if !m.expanded[m.cursor] {
		return
	}
	fields := m.entries[m.cursor].FlattenFields()
	if m.fieldCur < len(fields)-1 {
		m.fieldCur++
	}
}

// SelectedField returns the currently selected field, or nil.
func (m *Model) SelectedField() *gcp.FlattenedField {
	if m.fieldCur < 0 || !m.expanded[m.cursor] {
		return nil
	}
	fields := m.entries[m.cursor].FlattenFields()
	if m.fieldCur >= len(fields) {
		return nil
	}
	return &fields[m.fieldCur]
}

// NeedsMore returns true if the cursor is near the bottom and more pages are available.
func (m *Model) NeedsMore() bool {
	if !m.hasMore || len(m.entries) == 0 {
		return false
	}
	return m.cursor >= len(m.entries)-10
}

// entryLines returns how many visual lines an entry occupies.
func (m *Model) entryLines(idx int) int {
	lines := 1 // header line
	if m.wrapLines && !m.expanded[idx] {
		_, lines = RenderWrappedEntry(m.entries[idx], false, m.width, "", m.colorize)
	}
	if m.expanded[idx] {
		// Expanded: 1 header line + wrapped field lines
		_, fieldLines := RenderExpandedFields(m.entries[idx], -1, m.width)
		lines += fieldLines
	}
	return lines
}

// ensureVisible adjusts scroll offset so the cursor is visible.
func (m *Model) ensureVisible() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}

	// Count lines consumed by entries from offset to cursor
	linesUsed := 0
	for i := m.offset; i <= m.cursor && i < len(m.entries); i++ {
		linesUsed += m.entryLines(i)
	}

	for linesUsed > m.height && m.offset < m.cursor {
		linesUsed -= m.entryLines(m.offset)
		m.offset++
	}
}

// View renders the log viewer.
func (m *Model) View() string {
	if len(m.entries) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		return mutedStyle.Render("  No log entries")
	}

	var b strings.Builder
	linesRendered := 0
	const selectedBg = "#3C4043"

	for i := m.offset; i < len(m.entries) && linesRendered < m.height; i++ {
		entry := m.entries[i]
		isCursorEntry := i == m.cursor
		isExpanded := m.expanded[i]

		// Highlight the header when the cursor is on this entry and NOT
		// navigating fields — field nav highlights individual fields instead.
		highlightHeader := isCursorEntry && m.fieldCur < 0
		bg := ""
		if highlightHeader {
			bg = selectedBg
		}

		// Render header line:
		// - expanded entries show only indicator+severity+timestamp+resource (no message)
		// - collapsed entries use wrapped or compact based on global wrapLines toggle
		if m.wrapLines && !isExpanded {
			line, lineCount := RenderWrappedEntry(entry, false, m.width, bg, m.colorize)
			b.WriteString(line)
			b.WriteString("\n")
			linesRendered += lineCount
		} else {
			line := RenderCompactEntry(entry, isExpanded, m.width, bg, m.colorize)
			b.WriteString(line)
			b.WriteString("\n")
			linesRendered++
		}

		// Render expanded fields — pass fieldCur for the cursor entry
		// regardless of whether the header is highlighted, so field-level
		// navigation cursor is always visible.
		if isExpanded && linesRendered < m.height {
			fieldCurIdx := -1
			if isCursorEntry {
				fieldCurIdx = m.fieldCur
			}
			fieldContent, fieldCount := RenderExpandedFields(entry, fieldCurIdx, m.width)
			b.WriteString(fieldContent)
			linesRendered += fieldCount
		}
	}

	// Loading more indicator
	if m.hasMore && linesRendered < m.height {
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		b.WriteString(mutedStyle.Render("  Loading more..."))
		b.WriteString("\n")
	}

	return b.String()
}
