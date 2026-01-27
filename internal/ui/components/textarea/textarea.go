// Package textarea provides a multi-line text editor with GCP styling.
package textarea

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GCP color palette
var (
	colorPrimary   = lipgloss.Color("#4285F4")
	colorMuted     = lipgloss.Color("#9AA0A6")
	colorBorder    = lipgloss.Color("#5F6368")
	colorCursor    = lipgloss.Color("#4285F4")
	colorLineNum   = lipgloss.Color("#5F6368")
	colorSelection = lipgloss.Color("#3C4043")
)

// KeyMap defines key bindings for the textarea
type KeyMap struct {
	Submit key.Binding
	Cancel key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Submit: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "submit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// Model wraps bubbles/textarea with GCP styling
type Model struct {
	textarea    textarea.Model
	title       string
	readOnly    bool
	width       int
	height      int
	keys        KeyMap
	focused     bool
	titleStyle  lipgloss.Style
	borderStyle lipgloss.Style
	infoStyle   lipgloss.Style
}

// New creates a new textarea model
func New() Model {
	ta := textarea.New()
	ta.Placeholder = "Enter text..."
	ta.ShowLineNumbers = true
	ta.CharLimit = 0 // No limit by default

	// Apply GCP styling to the underlying textarea
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary)
	ta.BlurredStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().
		Background(colorSelection)
	ta.FocusedStyle.LineNumber = lipgloss.NewStyle().
		Foreground(colorLineNum)
	ta.BlurredStyle.LineNumber = lipgloss.NewStyle().
		Foreground(colorLineNum)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().
		Foreground(colorMuted)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().
		Foreground(colorMuted)
	ta.Cursor.Style = lipgloss.NewStyle().
		Foreground(colorCursor)

	return Model{
		textarea: ta,
		keys:     DefaultKeyMap(),
		focused:  false,
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

// WithTitle sets the textarea title
//nolint:gocritic // hugeParam: Model size from embedded textarea
func (m Model) WithTitle(title string) Model {
	m.title = title
	return m
}

// WithPlaceholder sets the placeholder text
//nolint:gocritic // hugeParam: Model size from embedded textarea
func (m Model) WithPlaceholder(placeholder string) Model {
	m.textarea.Placeholder = placeholder
	return m
}

// WithCharLimit sets the character limit (0 for no limit)
//nolint:gocritic // hugeParam: Model size from embedded textarea
func (m Model) WithCharLimit(limit int) Model {
	m.textarea.CharLimit = limit
	return m
}

// WithLineNumbers enables or disables line numbers
func (m Model) WithLineNumbers(show bool) Model {
	m.textarea.ShowLineNumbers = show
	return m
}

// ReadOnly sets read-only mode
func (m Model) ReadOnly(readOnly bool) Model {
	m.readOnly = readOnly
	return m
}

// SetValue sets the textarea content
func (m *Model) SetValue(value string) {
	m.textarea.SetValue(value)
}

// Value returns the current content
func (m Model) Value() string {
	return m.textarea.Value()
}

// SetSize updates the textarea dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Reserve space for title if present
	textareaHeight := height
	if m.title != "" {
		textareaHeight -= 2 // title + margin
	}
	textareaHeight -= 1 // info line

	if textareaHeight < 3 {
		textareaHeight = 3
	}
	if width < 10 {
		width = 10
	}

	m.textarea.SetWidth(width - 4) // Account for borders
	m.textarea.SetHeight(textareaHeight - 2)
}

// Focus focuses the textarea
func (m *Model) Focus() tea.Cmd {
	m.focused = true
	return m.textarea.Focus()
}

// Blur removes focus from the textarea
func (m *Model) Blur() {
	m.focused = false
	m.textarea.Blur()
}

// Focused returns true if the textarea has focus
func (m Model) Focused() bool {
	return m.focused
}

// Length returns the number of characters in the content
func (m Model) Length() int {
	return m.textarea.Length()
}

// LineCount returns the number of lines
func (m Model) LineCount() int {
	return m.textarea.LineCount()
}

// Line returns the content split by lines at the given index
func (m Model) Line(i int) string {
	lines := strings.Split(m.Value(), "\n")
	if i >= 0 && i < len(lines) {
		return lines[i]
	}
	return ""
}

// CurrentLine returns the current line position (0-indexed)
func (m Model) CurrentLine() int {
	return m.textarea.Line()
}

// CursorPosition returns the current cursor row and column
func (m Model) CursorPosition() (row, col int) {
	return m.textarea.LineInfo().RowOffset, m.textarea.LineInfo().ColumnOffset
}

// Init initializes the textarea
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	// In read-only mode, ignore input
	if m.readOnly {
		return m, nil
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// View renders the textarea
func (m Model) View() string {
	var b strings.Builder

	// Title
	if m.title != "" {
		b.WriteString(m.titleStyle.Render(m.title))
		b.WriteString("\n\n")
	}

	// Textarea content
	b.WriteString(m.textarea.View())

	// Info line
	b.WriteString("\n")
	info := m.renderInfo()
	b.WriteString(m.infoStyle.Render(info))

	return b.String()
}

// renderInfo returns the status line showing cursor position and character count
func (m Model) renderInfo() string {
	row, col := m.CursorPosition()
	lines := m.LineCount()
	chars := m.Length()

	var parts []string
	parts = append(parts, lipgloss.NewStyle().Render("Ln "+itoa(row+1)+", Col "+itoa(col+1)))
	parts = append(parts, lipgloss.NewStyle().Render(itoa(lines)+" lines"))
	//nolint:gocritic // appendCombine: appends are intentionally separate for clarity
	parts = append(parts, lipgloss.NewStyle().Render(itoa(chars)+" chars"))

	if m.readOnly {
		parts = append(parts, lipgloss.NewStyle().Bold(true).Render("[READ-ONLY]"))
	}

	return strings.Join(parts, "  ")
}

// itoa converts int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// KeyBindings returns the key bindings for help display
func (m Model) KeyBindings() []key.Binding {
	return []key.Binding{
		m.keys.Submit,
		m.keys.Cancel,
	}
}
