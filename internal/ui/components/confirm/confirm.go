package confirm

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Colors matching GCP theme
var (
	borderColor   = lipgloss.Color("#EA4335") // Red for destructive action
	titleColor    = lipgloss.Color("#EA4335")
	textColor     = lipgloss.Color("#FFFFFF")
	mutedColor    = lipgloss.Color("#9AA0A6")
	buttonFocusBg = lipgloss.Color("#EA4335")
	buttonTextDim = lipgloss.Color("#9AA0A6")
)

// ConfirmMsg is sent when user confirms the action
type ConfirmMsg struct{}

// CancelMsg is sent when user cancels the action
type CancelMsg struct{}

// ConfirmDialog represents a modal confirmation dialog
type ConfirmDialog struct {
	title      string
	message    string
	details    []string // Additional lines (e.g., file list preview)
	focusedYes bool     // true = Yes focused, false = No focused
	width      int
	height     int
	keys       confirmKeyMap
}

// confirmKeyMap defines key bindings for the confirmation dialog
type confirmKeyMap struct {
	Left    key.Binding
	Right   key.Binding
	Tab     key.Binding
	Confirm key.Binding
	Yes     key.Binding
	No      key.Binding
	Cancel  key.Binding
}

func defaultConfirmKeyMap() confirmKeyMap {
	return confirmKeyMap{
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "select yes"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "select no"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm selection"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "yes"),
		),
		No: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "no"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "q"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// New creates a new confirmation dialog
func New(title, message string, details []string) *ConfirmDialog {
	return &ConfirmDialog{
		title:      title,
		message:    message,
		details:    details,
		focusedYes: false, // Default to "No" for safety on destructive actions
		width:      50,
		height:     10,
		keys:       defaultConfirmKeyMap(),
	}
}

// SetSize updates the dialog dimensions
func (c *ConfirmDialog) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// Init initializes the dialog (no-op for this component)
func (c *ConfirmDialog) Init() tea.Cmd {
	return nil
}

// Update handles input for the confirmation dialog
func (c *ConfirmDialog) Update(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, c.keys.Left):
			c.focusedYes = true
			return nil

		case key.Matches(keyMsg, c.keys.Right):
			c.focusedYes = false
			return nil

		case key.Matches(keyMsg, c.keys.Tab):
			c.focusedYes = !c.focusedYes
			return nil

		case key.Matches(keyMsg, c.keys.Confirm):
			if c.focusedYes {
				return func() tea.Msg { return ConfirmMsg{} }
			}
			return func() tea.Msg { return CancelMsg{} }

		case key.Matches(keyMsg, c.keys.Yes):
			return func() tea.Msg { return ConfirmMsg{} }

		case key.Matches(keyMsg, c.keys.No), key.Matches(keyMsg, c.keys.Cancel):
			return func() tea.Msg { return CancelMsg{} }
		}
	}
	return nil
}

// View renders the confirmation dialog
func (c *ConfirmDialog) View() string {
	// Styles
	containerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(c.width)

	titleStyle := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(textColor)

	detailStyle := lipgloss.NewStyle().
		Foreground(mutedColor)

	// Button styles
	buttonFocused := lipgloss.NewStyle().
		Foreground(textColor).
		Background(buttonFocusBg).
		Padding(0, 2).
		Bold(true)

	buttonNormal := lipgloss.NewStyle().
		Foreground(buttonTextDim).
		Background(lipgloss.Color("#303134")).
		Padding(0, 2)

	// Build content
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(c.title))
	b.WriteString("\n\n")

	// Message
	b.WriteString(messageStyle.Render(c.message))
	b.WriteString("\n")

	// Details (if any)
	if len(c.details) > 0 {
		b.WriteString("\n")
		for _, detail := range c.details {
			b.WriteString(detailStyle.Render("  • " + detail))
			b.WriteString("\n")
		}
	}

	// Buttons
	b.WriteString("\n")

	var yesBtn, noBtn string
	if c.focusedYes {
		yesBtn = buttonFocused.Render("  Yes  ")
		noBtn = buttonNormal.Render("  No  ")
	} else {
		yesBtn = buttonNormal.Render("  Yes  ")
		noBtn = buttonFocused.Render("  No  ")
	}

	// Center buttons
	buttons := yesBtn + "  " + noBtn
	buttonsWidth := lipgloss.Width(buttons)
	leftPad := (c.width - buttonsWidth - 4) / 2 // -4 for container padding
	if leftPad < 0 {
		leftPad = 0
	}
	b.WriteString(strings.Repeat(" ", leftPad))
	b.WriteString(buttons)

	// Help text
	b.WriteString("\n\n")
	helpStyle := lipgloss.NewStyle().Foreground(mutedColor)
	helpText := "←/→: select • enter: confirm • y/n: quick select • esc: cancel"
	b.WriteString(helpStyle.Render(helpText))

	return containerStyle.Render(b.String())
}

// IsFocusedYes returns true if the Yes button is currently focused
func (c *ConfirmDialog) IsFocusedYes() bool {
	return c.focusedYes
}
