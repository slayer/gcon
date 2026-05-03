// Package viewportsearch provides an inline search bar for viewport content.
// It finds text matches within viewport content and scrolls to each match,
// supporting next/prev navigation with n/N keys.
package viewportsearch

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// GCP color palette
var (
	colorPrimary  = lipgloss.Color("#4285F4")
	colorMuted    = lipgloss.Color("#9AA0A6")
	colorError    = lipgloss.Color("#EA4335")
	colorSuccess  = lipgloss.Color("#34A853")
	colorBg       = lipgloss.Color("#1E2124")
	colorBgActive = lipgloss.Color("#2D3142")
)

// keyMap defines key bindings for the search bar.
type keyMap struct {
	Next   key.Binding
	Prev   key.Binding
	Close  key.Binding
	Submit key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Next: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next match"),
		),
		Prev: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev match"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close search"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "next match"),
		),
	}
}

// Model is the viewport search component.
// It maintains the search query, matched line numbers, and current position.
// The host view is responsible for scrolling the viewport to CurrentMatchLine().
type Model struct {
	input   textinput.Model
	active  bool
	lines   []string // plain-text lines for matching (ANSI-stripped)
	matches []int    // line indices that contain the query
	current int      // index into matches slice (-1 = no current match)
	width   int
	keys    keyMap
}

// New creates a new viewportsearch Model.
func New() *Model {
	ti := textinput.New()
	ti.Placeholder = "search..."
	ti.CharLimit = 100
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorPrimary)
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted)

	return &Model{
		input:   ti,
		current: -1,
		keys:    defaultKeyMap(),
	}
}

// SetSize updates the available width for rendering.
func (m *Model) SetSize(width int) {
	m.width = width
	// Reserve space for prefix "/" + status portion
	inputWidth := width - 30
	if inputWidth < 10 {
		inputWidth = 10
	}
	m.input.Width = inputWidth
}

// Open activates the search bar, clears any previous query, and focuses the input.
func (m *Model) Open() {
	m.active = true
	m.input.SetValue("")
	m.input.Focus()
	m.matches = nil
	m.current = -1
}

// Close deactivates the search bar and clears state.
func (m *Model) Close() {
	m.active = false
	m.input.Blur()
	m.input.SetValue("")
	m.matches = nil
	m.current = -1
}

// IsActive returns true when the search bar is open.
func (m *Model) IsActive() bool {
	return m.active
}

// HasTextInputFocused returns true when search is active (for key routing).
// Implements the TextInputFocusable interface check used by the app.
func (m *Model) HasTextInputFocused() bool {
	return m.active
}

// SetContent provides the content to search against.
// ANSI escape codes are stripped before indexing.
// Call this whenever the viewport content changes.
func (m *Model) SetContent(content string) {
	plain := ansi.Strip(content)
	m.lines = strings.Split(plain, "\n")
	m.rebuildMatches()
}

// CurrentMatchLine returns the 0-based line number of the active match,
// or -1 when there are no matches or no query.
func (m *Model) CurrentMatchLine() int {
	if len(m.matches) == 0 || m.current < 0 {
		return -1
	}
	return m.matches[m.current]
}

// MatchCount returns the total number of lines containing the query.
func (m *Model) MatchCount() int {
	return len(m.matches)
}

// CurrentMatchIndex returns the 1-based index of the active match (for display).
func (m *Model) CurrentMatchIndex() int {
	if len(m.matches) == 0 {
		return 0
	}
	return m.current + 1
}

// NextMatch advances the active match forward (wraps around).
func (m *Model) NextMatch() {
	if len(m.matches) == 0 {
		return
	}
	m.current = (m.current + 1) % len(m.matches)
}

// PrevMatch moves the active match backward (wraps around).
func (m *Model) PrevMatch() {
	if len(m.matches) == 0 {
		return
	}
	if m.current <= 0 {
		m.current = len(m.matches) - 1
	} else {
		m.current--
	}
}

// Update processes a key message when the search bar is active.
// Returns the updated model and a tea.Cmd (for textinput blink).
// The host view should call CurrentMatchLine() after Update to scroll the viewport.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if !m.active {
		return nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Forward non-key messages to the text input (e.g., blink tick)
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return cmd
	}

	switch {
	case key.Matches(keyMsg, m.keys.Close):
		m.Close()
		return nil

	case key.Matches(keyMsg, m.keys.Submit):
		// Enter = next match
		m.NextMatch()
		return nil

	case key.Matches(keyMsg, m.keys.Next):
		m.NextMatch()
		return nil

	case key.Matches(keyMsg, m.keys.Prev):
		m.PrevMatch()
		return nil

	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(keyMsg)
		m.rebuildMatches()
		return cmd
	}
}

// View renders the search bar as a single line.
// Returns an empty string when not active.
func (m *Model) View() string {
	if !m.active {
		return ""
	}

	prefixStyle := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Background(colorBgActive).
		Bold(true)

	barStyle := lipgloss.NewStyle().
		Background(colorBgActive).
		Padding(0, 1)

	query := m.input.Value()

	var statusStr string
	switch {
	case query == "":
		statusStr = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(colorBgActive).
			Render("  type to search")
	case len(m.matches) == 0:
		statusStr = lipgloss.NewStyle().
			Foreground(colorError).
			Background(colorBgActive).
			Render("  no matches")
	default:
		countStr := fmt.Sprintf("  %d/%d", m.CurrentMatchIndex(), m.MatchCount())
		navHint := "  n:next  N:prev  esc:close"
		statusStr = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Background(colorBgActive).
			Render(countStr) +
			lipgloss.NewStyle().
				Foreground(colorMuted).
				Background(colorBgActive).
				Render(navHint)
	}

	prefix := prefixStyle.Render(" /")
	inputRendered := lipgloss.NewStyle().Background(colorBg).Render(m.input.View())

	content := prefix + " " + inputRendered + statusStr
	return barStyle.Render(content)
}

// rebuildMatches recomputes the match list based on the current query.
// Resets current to 0 on new matches, or -1 when there are none.
func (m *Model) rebuildMatches() {
	query := strings.ToLower(m.input.Value())
	m.matches = nil

	if query == "" {
		m.current = -1
		return
	}

	for i, line := range m.lines {
		if strings.Contains(strings.ToLower(line), query) {
			m.matches = append(m.matches, i)
		}
	}

	if len(m.matches) == 0 {
		m.current = -1
	} else if m.current < 0 || m.current >= len(m.matches) {
		// Keep the current match index valid, defaulting to first match
		m.current = 0
	}
}
