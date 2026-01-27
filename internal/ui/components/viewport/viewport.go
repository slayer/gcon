// Package viewport provides a scrollable content container with GCP styling.
//
//nolint:gocritic // Model size is from embedded viewport
package viewport

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GCP color palette
var (
	colorPrimary = lipgloss.Color("#4285F4")
	colorMuted   = lipgloss.Color("#9AA0A6")
	colorBorder  = lipgloss.Color("#5F6368")
)

// KeyMap defines key bindings for the viewport
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),
	}
}

// Model wraps bubbles/viewport with GCP styling and optional header
type Model struct {
	viewport    viewport.Model
	title       string
	showBorder  bool
	width       int
	height      int
	keys        KeyMap
	ready       bool
	titleStyle  lipgloss.Style
	borderStyle lipgloss.Style
	infoStyle   lipgloss.Style
}

// New creates a new viewport model
func New(width, height int) Model {
	vp := viewport.New(width, height)
	vp.MouseWheelEnabled = true

	return Model{
		viewport:   vp,
		width:      width,
		height:     height,
		keys:       DefaultKeyMap(),
		showBorder: false,
		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary),
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder),
		infoStyle: lipgloss.NewStyle().
			Foreground(colorMuted),
	}
}

// WithTitle sets the viewport title
func (m Model) WithTitle(title string) Model {
	m.title = title
	return m
}

// WithBorder enables or disables the border
func (m Model) WithBorder(show bool) Model {
	m.showBorder = show
	return m
}

// SetContent sets the viewport content
func (m *Model) SetContent(content string) {
	m.viewport.SetContent(content)
	m.ready = true
}

// SetSize updates the viewport dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Reserve space for title and info line if present
	viewportHeight := height
	if m.title != "" {
		viewportHeight -= 2 // title + margin
	}
	viewportHeight-- // info line

	if m.showBorder {
		// Border takes 2 chars on each side
		width -= 4
		viewportHeight -= 2
	}

	if viewportHeight < 1 {
		viewportHeight = 1
	}
	if width < 1 {
		width = 1
	}

	m.viewport.Width = width
	m.viewport.Height = viewportHeight
}

// GotoTop scrolls to the top
func (m *Model) GotoTop() {
	m.viewport.GotoTop()
}

// GotoBottom scrolls to the bottom
func (m *Model) GotoBottom() {
	m.viewport.GotoBottom()
}

// ScrollPercent returns the current scroll position as a percentage
func (m Model) ScrollPercent() float64 {
	return m.viewport.ScrollPercent()
}

// AtTop returns true if scrolled to the top
func (m Model) AtTop() bool {
	return m.viewport.AtTop()
}

// AtBottom returns true if scrolled to the bottom
func (m Model) AtBottom() bool {
	return m.viewport.AtBottom()
}

// TotalLineCount returns the total number of lines in content
func (m Model) TotalLineCount() int {
	return m.viewport.TotalLineCount()
}

// VisibleLineCount returns the number of visible lines
func (m Model) VisibleLineCount() int {
	return m.viewport.VisibleLineCount()
}

// Init initializes the viewport
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Top):
			m.viewport.GotoTop()
		case key.Matches(msg, m.keys.Bottom):
			m.viewport.GotoBottom()
		default:
			m.viewport, cmd = m.viewport.Update(msg)
		}
	case tea.MouseMsg:
		m.viewport, cmd = m.viewport.Update(msg)
	}

	return m, cmd
}

// View renders the viewport
func (m Model) View() string {
	if !m.ready {
		return ""
	}

	var b strings.Builder

	// Title
	if m.title != "" {
		b.WriteString(m.titleStyle.Render(m.title))
		b.WriteString("\n\n")
	}

	// Content
	content := m.viewport.View()
	if m.showBorder {
		content = m.borderStyle.Render(content)
	}
	b.WriteString(content)

	// Scroll position info
	b.WriteString("\n")
	info := m.renderInfo()
	b.WriteString(m.infoStyle.Render(info))

	return b.String()
}

// renderInfo returns the scroll position indicator
func (m Model) renderInfo() string {
	total := m.TotalLineCount()
	if total <= m.VisibleLineCount() {
		return ""
	}

	percent := int(m.ScrollPercent() * 100)
	return lipgloss.NewStyle().
		Foreground(colorMuted).
		Render(strings.Repeat(" ", m.width-15) + formatPercent(percent))
}

func formatPercent(p int) string {
	if p <= 0 {
		return "Top"
	}
	if p >= 100 {
		return "Bottom"
	}
	return strings.TrimSpace(lipgloss.NewStyle().Render(string(rune('0'+p/100)) + string(rune('0'+(p/10)%10)) + string(rune('0'+p%10)) + "%"))
}

// KeyBindings returns the key bindings for help display
func (m Model) KeyBindings() []key.Binding {
	return []key.Binding{
		m.keys.Up,
		m.keys.Down,
		m.keys.PageUp,
		m.keys.PageDown,
		m.keys.Top,
		m.keys.Bottom,
	}
}
