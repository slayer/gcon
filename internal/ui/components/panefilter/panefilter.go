// Package panefilter provides an in-pane text search controller for long
// rendered details views.
//
// The owning view typically:
//   - Calls Open() to activate (returns the textinput Init cmd).
//   - Calls Close() to dismiss.
//   - Routes Update(msg) to the model while Visible().
//   - Calls Apply(content) inside its render path to obtain content with
//     match spans wrapped in highlight SGR.
//   - Reports IsFocused() from its HasTextInputFocused() so the global 'q'
//     quit key is suppressed while the search input is focused.
//   - Reports Visible() from its IsMenuOpen()-equivalent so 'esc' closes the
//     search before falling through to navigation.
package panefilter

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Match describes a single match span in the visible (ANSI-stripped) content.
type Match struct {
	Line     int // 0-based line number where the match begins
	VisStart int // byte offset in ANSI-stripped content where match begins
	VisEnd   int // byte offset in ANSI-stripped content where match ends (exclusive)
}

// KeyMap holds key bindings owned by the pane filter.
type KeyMap struct {
	Open  key.Binding
	Next  key.Binding
	Prev  key.Binding
	Close key.Binding
}

// DefaultKeyMap returns the standard pane-filter bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Open:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Next:  key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next match")),
		Prev:  key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev match")),
		Close: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	}
}

// Model is the in-pane search controller. It is stateful but lightweight; it
// is safe to keep on the view across renders and refreshes.
type Model struct {
	input   textinput.Model
	keys    KeyMap
	visible bool
	matches []Match
	cursor  int // -1 when no matches
}

// New constructs a pane filter with default styling.
func New() *Model {
	ti := textinput.New()
	ti.Placeholder = "search..."
	ti.Prompt = "/"
	ti.CharLimit = 200
	return &Model{input: ti, keys: DefaultKeyMap(), cursor: -1}
}

// KeyMap returns the model's key bindings (typically used by the view to
// decide whether a key event should be routed here).
func (m *Model) KeyMap() KeyMap { return m.keys }

// Open activates the search bar and focuses the input.
func (m *Model) Open() tea.Cmd {
	m.visible = true
	return m.input.Focus()
}

// Close hides the search bar and resets state.
func (m *Model) Close() {
	m.visible = false
	m.input.Reset()
	m.input.Blur()
	m.matches = nil
	m.cursor = -1
}

// Submit blurs the input but keeps the bar visible (so existing matches stay
// highlighted). After Submit, the host view typically allows n/N navigation.
func (m *Model) Submit() {
	m.input.Blur()
}

// Visible reports whether the search bar is currently shown.
func (m *Model) Visible() bool { return m.visible }

// IsFocused reports whether the input has keyboard focus.
func (m *Model) IsFocused() bool { return m.visible && m.input.Focused() }

// Query returns the current query string (empty when not visible).
func (m *Model) Query() string {
	if !m.visible {
		return ""
	}
	return m.input.Value()
}

// Update routes textinput events. Returns a cmd suitable for tea.Batch.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if !m.visible {
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

// MatchCount returns the total number of matches.
func (m *Model) MatchCount() int { return len(m.matches) }

// CurrentMatchIndex returns the 1-based cursor position (0 when no matches).
func (m *Model) CurrentMatchIndex() int {
	if m.cursor < 0 || len(m.matches) == 0 {
		return 0
	}
	return m.cursor + 1
}

// MatchLine returns the line number of the current match, or -1 when none.
func (m *Model) MatchLine() int {
	if m.cursor < 0 || m.cursor >= len(m.matches) {
		return -1
	}
	return m.matches[m.cursor].Line
}

// Next advances the cursor to the next match (wraps).
func (m *Model) Next() {
	if len(m.matches) == 0 {
		return
	}
	m.cursor = (m.cursor + 1) % len(m.matches)
}

// Prev moves the cursor to the previous match (wraps).
func (m *Model) Prev() {
	if len(m.matches) == 0 {
		return
	}
	m.cursor--
	if m.cursor < 0 {
		m.cursor = len(m.matches) - 1
	}
}

// Apply finds matches of Query() in content and returns the content with each
// match span wrapped in highlight SGR. The current match (selected by Next/Prev)
// gets a brighter highlight than the others. Updates internal state.
//
// When the query is empty or there are no matches, content is returned unchanged.
func (m *Model) Apply(content string) string {
	query := m.Query()
	if query == "" {
		m.matches = nil
		m.cursor = -1
		return content
	}
	m.matches = findMatches(content, query)
	if len(m.matches) == 0 {
		m.cursor = -1
		return content
	}
	if m.cursor < 0 || m.cursor >= len(m.matches) {
		m.cursor = 0
	}
	return highlightMatches(content, m.matches, m.cursor)
}

// View renders the search bar (single line).
func (m *Model) View() string {
	if !m.visible {
		return ""
	}
	bar := m.input.View()
	var status string
	if m.input.Value() != "" {
		if m.MatchCount() > 0 {
			status = mutedStyle.Render(fmt.Sprintf(" %d/%d", m.CurrentMatchIndex(), m.MatchCount()))
		} else {
			status = errorStyle.Render(" no matches")
		}
	}
	return bar + status + helpStyle.Render("  n/N: next/prev  esc: close")
}

var (
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Faint(true)
)

// SGR codes used for in-place highlighting. Kept simple (8-color where
// possible) for terminal compatibility; current-match uses 256-color orange
// to distinguish from non-current matches.
const (
	sgrReset   = "\x1b[0m"
	sgrMatch   = "\x1b[30;43m"        // black on yellow
	sgrCurrent = "\x1b[30;48;5;208m"  // black on orange
)

// findMatches walks content, ignoring ANSI sequences, and returns positions of
// all case-insensitive matches of query in the visible (ANSI-stripped) string.
func findMatches(content, query string) []Match {
	if query == "" {
		return nil
	}
	visible := stripANSI(content)
	lowerVis := strings.ToLower(visible)
	lowerQ := strings.ToLower(query)
	if lowerQ == "" || len(lowerQ) > len(lowerVis) {
		return nil
	}

	var matches []Match
	start := 0
	for start <= len(lowerVis)-len(lowerQ) {
		idx := strings.Index(lowerVis[start:], lowerQ)
		if idx < 0 {
			break
		}
		absStart := start + idx
		absEnd := absStart + len(lowerQ)
		line := strings.Count(visible[:absStart], "\n")
		matches = append(matches, Match{Line: line, VisStart: absStart, VisEnd: absEnd})
		start = absEnd
	}
	return matches
}

// stripANSI returns content with CSI escape sequences removed.
func stripANSI(content string) string {
	var b strings.Builder
	b.Grow(len(content))
	for i := 0; i < len(content); {
		if content[i] == 0x1b && i+1 < len(content) && content[i+1] == '[' {
			j := i + 2
			for j < len(content) && content[j] < 0x40 {
				j++
			}
			if j < len(content) {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(content[i])
		i++
	}
	return b.String()
}

// highlightMatches walks the original content and wraps match spans (given in
// visible-byte offsets) in highlight SGR. The currentIdx match gets a brighter
// SGR than the others.
//
// After closing each match span, the most recent active SGR is re-emitted so
// the surrounding lipgloss styling is preserved.
func highlightMatches(content string, matches []Match, currentIdx int) string {
	if len(matches) == 0 {
		return content
	}
	var b strings.Builder
	b.Grow(len(content) + len(matches)*16)
	visiblePos := 0
	matchIdx := 0
	inMatch := false
	var activeSGR string

	for i := 0; i < len(content); {
		// ANSI CSI: copy through; track active SGR (sequences ending in 'm') so
		// it can be restored after a match span resets the style.
		if content[i] == 0x1b && i+1 < len(content) && content[i+1] == '[' {
			j := i + 2
			for j < len(content) && content[j] < 0x40 {
				j++
			}
			if j < len(content) {
				j++
			}
			seq := content[i:j]
			if !inMatch && len(seq) > 0 && seq[len(seq)-1] == 'm' {
				if seq == sgrReset {
					activeSGR = ""
				} else {
					activeSGR = seq
				}
			}
			b.WriteString(seq)
			i = j
			continue
		}

		if !inMatch && matchIdx < len(matches) && visiblePos == matches[matchIdx].VisStart {
			if matchIdx == currentIdx {
				b.WriteString(sgrCurrent)
			} else {
				b.WriteString(sgrMatch)
			}
			inMatch = true
		}

		b.WriteByte(content[i])
		visiblePos++
		i++

		if inMatch && visiblePos >= matches[matchIdx].VisEnd {
			b.WriteString(sgrReset)
			if activeSGR != "" {
				b.WriteString(activeSGR)
			}
			inMatch = false
			matchIdx++
		}
	}
	if inMatch {
		b.WriteString(sgrReset)
	}
	return b.String()
}
