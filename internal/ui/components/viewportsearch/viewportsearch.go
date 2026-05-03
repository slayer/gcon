// Package viewportsearch provides a search component for viewport content.
// It supports text searching with highlighting and navigation between matches.
package viewportsearch

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the viewport search component
type Model struct {
	input         textinput.Model
	active        bool
	query         string
	currentMatch  int    // 0-based index of current match
	totalMatches  int    // total number of matches
	caseSensitive bool   // case-sensitive search
	width         int    // available width for rendering
}

// New creates a new viewport search model
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 100
	ti.Width = 30

	return Model{
		input:         ti,
		active:        false,
		currentMatch:  -1,
		caseSensitive: false,
	}
}

// Activate activates the search input
func (m *Model) Activate() tea.Cmd {
	m.active = true
	m.input.Focus()
	return textinput.Blink
}

// Deactivate deactivates the search input
func (m *Model) Deactivate() {
	m.active = false
	m.input.Blur()
	m.query = ""
	m.currentMatch = -1
	m.totalMatches = 0
}

// IsActive returns true if search is active
func (m *Model) IsActive() bool {
	return m.active
}

// SetWidth sets the available width for the search bar
func (m *Model) SetWidth(width int) {
	m.width = width
	switch {
	case width > 40:
		m.input.Width = 40
	case width > 20:
		m.input.Width = width - 10
	default:
		m.input.Width = max(10, width-5)
	}
}

// Update handles messages for the search component
func (m *Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.active {
		return *m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.query = m.input.Value()

	return *m, cmd
}

// NextMatch moves to the next match
func (m *Model) NextMatch() {
	if m.totalMatches == 0 {
		return
	}
	m.currentMatch = (m.currentMatch + 1) % m.totalMatches
}

// PrevMatch moves to the previous match
func (m *Model) PrevMatch() {
	if m.totalMatches == 0 {
		return
	}
	m.currentMatch--
	if m.currentMatch < 0 {
		m.currentMatch = m.totalMatches - 1
	}
}

// GetQuery returns the current search query
func (m *Model) GetQuery() string {
	return m.query
}

// GetCurrentMatch returns the current match index (0-based)
func (m *Model) GetCurrentMatch() int {
	return m.currentMatch
}

// SetMatchInfo updates the match count and resets position if needed
func (m *Model) SetMatchInfo(total int) {
	m.totalMatches = total
	if m.currentMatch >= total {
		m.currentMatch = 0
	}
	if total == 0 {
		m.currentMatch = -1
	}
}

// View renders the search bar
func (m *Model) View() string {
	if !m.active {
		return ""
	}

	searchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	matchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	label := searchStyle.Render("Search: ")
	inputView := m.input.View()

	var matchInfo string
	if m.query != "" {
		if m.totalMatches == 0 {
			matchInfo = labelStyle.Render(" (no matches)")
		} else {
			matchInfo = matchStyle.Render(" (" + formatMatchInfo(m.currentMatch+1, m.totalMatches) + ")")
		}
	}

	return label + inputView + matchInfo
}

// HighlightMatches returns content with search matches highlighted
// It returns the modified content and updates match count
func (m *Model) HighlightMatches(content string) string {
	if m.query == "" || !m.active {
		m.totalMatches = 0
		m.currentMatch = -1
		return content
	}

	query := m.query
	searchIn := content
	if !m.caseSensitive {
		query = strings.ToLower(query)
		searchIn = strings.ToLower(searchIn)
	}

	// Count matches and find positions
	var positions []int
	pos := 0
	for {
		idx := strings.Index(searchIn[pos:], query)
		if idx == -1 {
			break
		}
		actualPos := pos + idx
		positions = append(positions, actualPos)
		pos = actualPos + 1
	}

	m.totalMatches = len(positions)
	if m.totalMatches == 0 {
		m.currentMatch = -1
		return content
	}

	// Ensure currentMatch is valid
	if m.currentMatch < 0 || m.currentMatch >= m.totalMatches {
		m.currentMatch = 0
	}

	// Apply highlighting
	highlightStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#FBBC04")).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	currentHighlightStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#34A853")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	// Build result by inserting highlights
	var result strings.Builder
	lastPos := 0

	for i, matchPos := range positions {
		// Add text before match
		result.WriteString(content[lastPos:matchPos])

		// Add highlighted match
		matchText := content[matchPos : matchPos+len(m.query)]
		if i == m.currentMatch {
			result.WriteString(currentHighlightStyle.Render(matchText))
		} else {
			result.WriteString(highlightStyle.Render(matchText))
		}

		lastPos = matchPos + len(m.query)
	}

	// Add remaining text
	result.WriteString(content[lastPos:])

	return result.String()
}

func formatMatchInfo(current, total int) string {
	if total == 0 {
		return "0/0"
	}
	return lipgloss.NewStyle().Render(
		lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")).Render(formatInt(current)) +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Render("/") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(formatInt(total)))
}

func formatInt(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return lipgloss.NewStyle().Render(string(rune('0'+n/10)) + string(rune('0'+n%10)))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
