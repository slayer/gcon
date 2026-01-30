// Package confirm provides confirmation dialogs for destructive actions.
// TypeConfirmDialog requires users to type a resource name to confirm deletion,
// providing an extra safety layer for destructive operations.
package confirm

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TypeConfirmMsg is sent when user successfully confirms by typing the name
type TypeConfirmMsg struct{}

// TypeCancelMsg is sent when user cancels the dialog
type TypeCancelMsg struct{}

// TypeConfirmDialog requires typing a resource name to confirm deletion
type TypeConfirmDialog struct {
	title         string
	resourceName  string   // The name user must type to confirm
	details       []string // Additional info lines (zone, type, etc.)
	warningMsg    string   // Optional warning (e.g., deletion protection)
	textInput     textinput.Model
	width         int
	height        int
	keys          typeConfirmKeyMap
	cannotConfirm bool // True if action is blocked (e.g., deletion protection)
}

// typeConfirmKeyMap defines key bindings for the type confirmation dialog
type typeConfirmKeyMap struct {
	Confirm key.Binding
	Cancel  key.Binding
}

func defaultTypeConfirmKeyMap() typeConfirmKeyMap {
	return typeConfirmKeyMap{
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// NewTypeConfirmDialog creates a new type-to-confirm dialog
func NewTypeConfirmDialog(title, resourceName string, details []string) *TypeConfirmDialog {
	ti := textinput.New()
	ti.Placeholder = "Type resource name to confirm"
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 45

	// Style the text input
	ti.TextStyle = lipgloss.NewStyle().Foreground(textColor)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(mutedColor)
	ti.PromptStyle = lipgloss.NewStyle().Foreground(textColor)

	return &TypeConfirmDialog{
		title:        title,
		resourceName: resourceName,
		details:      details,
		textInput:    ti,
		width:        55,
		height:       15,
		keys:         defaultTypeConfirmKeyMap(),
	}
}

// SetWarning sets a warning message that blocks confirmation
func (d *TypeConfirmDialog) SetWarning(warning string) *TypeConfirmDialog {
	d.warningMsg = warning
	d.cannotConfirm = warning != ""
	return d
}

// SetCannotConfirm sets whether confirmation is blocked (e.g., deletion protection)
func (d *TypeConfirmDialog) SetCannotConfirm(blocked bool) *TypeConfirmDialog {
	d.cannotConfirm = blocked
	return d
}

// SetSize updates the dialog dimensions
func (d *TypeConfirmDialog) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.textInput.Width = width - 12
}

// Init initializes the dialog
func (d *TypeConfirmDialog) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles input for the dialog
func (d *TypeConfirmDialog) Update(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, d.keys.Confirm):
			// Only confirm if typed name matches exactly and not blocked
			if !d.cannotConfirm && d.isNameMatching() {
				return func() tea.Msg { return TypeConfirmMsg{} }
			}
			return nil

		case key.Matches(keyMsg, d.keys.Cancel):
			return func() tea.Msg { return TypeCancelMsg{} }
		}
	}

	// Update text input
	var cmd tea.Cmd
	d.textInput, cmd = d.textInput.Update(msg)
	return cmd
}

// isNameMatching returns true if typed name matches the resource name
func (d *TypeConfirmDialog) isNameMatching() bool {
	return strings.TrimSpace(d.textInput.Value()) == d.resourceName
}

// View renders the type confirmation dialog
func (d *TypeConfirmDialog) View() string {
	// Styles
	containerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(d.width)

	titleStyle := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(textColor)

	detailStyle := lipgloss.NewStyle().
		Foreground(mutedColor)

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FBBC04")). // Yellow/orange for warning
		Bold(true)

	resourceStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4285F4")). // Blue for emphasis
		Bold(true)

	matchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#34A853")). // Green when matching
		Bold(true)

	noMatchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EA4335")). // Red when not matching
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(mutedColor)

	// Build content
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(d.title))
	b.WriteString("\n\n")

	// Warning message (if blocked)
	if d.warningMsg != "" {
		b.WriteString(warningStyle.Render("! " + d.warningMsg))
		b.WriteString("\n\n")
	}

	// Message with resource name
	b.WriteString(messageStyle.Render("This will permanently delete:"))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(resourceStyle.Render(d.resourceName))
	b.WriteString("\n")

	// Details (if any)
	if len(d.details) > 0 {
		b.WriteString("\n")
		for _, detail := range d.details {
			b.WriteString(detailStyle.Render("  " + detail))
			b.WriteString("\n")
		}
	}

	// Warning about irreversibility
	b.WriteString("\n")
	b.WriteString(warningStyle.Render("This action cannot be undone."))
	b.WriteString("\n")

	// Text input section (only show if not blocked)
	if !d.cannotConfirm {
		b.WriteString("\n")
		b.WriteString(messageStyle.Render("Type the name to confirm:"))
		b.WriteString("\n\n")
		b.WriteString(d.textInput.View())
		b.WriteString("\n")

		// Match status indicator
		if d.textInput.Value() != "" {
			if d.isNameMatching() {
				b.WriteString(matchStyle.Render("  Name matches - press Enter to delete"))
			} else {
				b.WriteString(noMatchStyle.Render("  Name does not match"))
			}
		}
	}

	// Help text
	b.WriteString("\n\n")
	if d.cannotConfirm {
		b.WriteString(helpStyle.Render("esc: close"))
	} else {
		b.WriteString(helpStyle.Render("enter: confirm (when name matches) | esc: cancel"))
	}

	return containerStyle.Render(b.String())
}

// HasTextInputFocused returns true since this dialog has a text input
func (d *TypeConfirmDialog) HasTextInputFocused() bool {
	return !d.cannotConfirm // Only has focused input when confirmation is allowed
}
