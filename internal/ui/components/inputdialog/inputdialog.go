// Package inputdialog provides a simple modal input dialog for capturing text input.
// Used for naming resources like snapshots and images.
package inputdialog

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

// Colors matching GCP theme
var (
	borderColor   = lipgloss.Color("#4285F4")
	titleColor    = lipgloss.Color("#4285F4")
	textColor     = lipgloss.Color("#FFFFFF")
	mutedColor    = lipgloss.Color("#9AA0A6")
	errorColor    = lipgloss.Color("#EA4335")
	buttonFocusBg = lipgloss.Color("#4285F4")
	buttonTextDim = lipgloss.Color("#9AA0A6")
)

// InputConfirmMsg is sent when user confirms the input
type InputConfirmMsg struct {
	Value         string
	CheckboxValue bool // Value of optional checkbox if shown
}

// InputCancelMsg is sent when user cancels the dialog
type InputCancelMsg struct{}

// InputDialog represents a modal input dialog
type InputDialog struct {
	title       string
	prompt      string
	textInput   textinput.Model
	validator   forms.Validator
	err         error
	width       int
	height      int
	keys        inputKeyMap
	placeholder string

	// Optional warning message with checkbox
	warning       string // Warning message (shown in yellow/orange)
	checkboxLabel string // Checkbox label (empty = no checkbox)
	checkboxValue bool   // Current checkbox state
	showCheckbox  bool   // Whether to show the checkbox
}

// inputKeyMap defines key bindings for the input dialog
type inputKeyMap struct {
	Confirm        key.Binding
	Cancel         key.Binding
	ToggleCheckbox key.Binding
}

func defaultInputKeyMap() inputKeyMap {
	return inputKeyMap{
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		ToggleCheckbox: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle checkbox"),
		),
	}
}

// New creates a new input dialog
func New(title, prompt, placeholder string) *InputDialog {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 63 // GCP resource name limit
	ti.Width = 40

	// Style the text input
	ti.TextStyle = lipgloss.NewStyle().Foreground(textColor)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(mutedColor)
	ti.PromptStyle = lipgloss.NewStyle().Foreground(textColor)

	return &InputDialog{
		title:       title,
		prompt:      prompt,
		textInput:   ti,
		validator:   forms.ValidateGCPResourceName,
		width:       50,
		height:      10,
		keys:        defaultInputKeyMap(),
		placeholder: placeholder,
	}
}

// SetValidator sets a custom validator for the input
func (d *InputDialog) SetValidator(v forms.Validator) *InputDialog {
	d.validator = v
	return d
}

// SetWarning sets a warning message to display above the input
func (d *InputDialog) SetWarning(warning string) *InputDialog {
	d.warning = warning
	return d
}

// SetCheckbox adds an optional checkbox with the given label
func (d *InputDialog) SetCheckbox(label string, defaultValue bool) *InputDialog {
	d.checkboxLabel = label
	d.checkboxValue = defaultValue
	d.showCheckbox = true
	return d
}

// SetSize updates the dialog dimensions
func (d *InputDialog) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.textInput.Width = width - 8
}

// Init initializes the dialog
func (d *InputDialog) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles input for the dialog
func (d *InputDialog) Update(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, d.keys.Confirm):
			// Validate before confirming
			value := strings.TrimSpace(d.textInput.Value())

			// Check required
			if value == "" {
				d.err = forms.ValidateRequired(value)
				return nil
			}

			// Run custom validator
			if d.validator != nil {
				if err := d.validator(value); err != nil {
					d.err = err
					return nil
				}
			}

			checkboxVal := d.checkboxValue
			return func() tea.Msg {
				return InputConfirmMsg{Value: value, CheckboxValue: checkboxVal}
			}

		case key.Matches(keyMsg, d.keys.Cancel):
			return func() tea.Msg {
				return InputCancelMsg{}
			}

		case key.Matches(keyMsg, d.keys.ToggleCheckbox):
			if d.showCheckbox {
				d.checkboxValue = !d.checkboxValue
				return nil
			}
		}
	}

	// Update text input
	var cmd tea.Cmd
	d.textInput, cmd = d.textInput.Update(msg)

	// Clear error on input change
	d.err = nil

	return cmd
}

// View renders the input dialog
func (d *InputDialog) View() string {
	// Styles
	containerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(d.width)

	titleStyle := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(true)

	promptStyle := lipgloss.NewStyle().
		Foreground(textColor)

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FBBC04")). // Yellow/orange for warning
		Bold(true)

	errorStyle := lipgloss.NewStyle().
		Foreground(errorColor)

	helpStyle := lipgloss.NewStyle().
		Foreground(mutedColor)

	checkboxStyle := lipgloss.NewStyle().
		Foreground(textColor)

	// Build content
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(d.title))
	b.WriteString("\n\n")

	// Warning message (if any)
	if d.warning != "" {
		b.WriteString(warningStyle.Render("⚠ " + d.warning))
		b.WriteString("\n\n")
	}

	// Prompt
	b.WriteString(promptStyle.Render(d.prompt))
	b.WriteString("\n\n")

	// Text input
	b.WriteString(d.textInput.View())
	b.WriteString("\n")

	// Checkbox (if enabled)
	if d.showCheckbox {
		b.WriteString("\n")
		checkbox := "[ ]"
		if d.checkboxValue {
			checkbox = "[x]"
		}
		b.WriteString(checkboxStyle.Render(checkbox + " " + d.checkboxLabel))
	}

	// Error message
	if d.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  " + d.err.Error()))
	}

	// Help text
	b.WriteString("\n\n")
	helpText := "enter: confirm • esc: cancel"
	if d.showCheckbox {
		helpText = "tab: toggle checkbox • enter: confirm • esc: cancel"
	}
	b.WriteString(helpStyle.Render(helpText))

	return containerStyle.Render(b.String())
}

// Value returns the current input value
func (d *InputDialog) Value() string {
	return d.textInput.Value()
}

// HasTextInputFocused returns true since this dialog always has text input focused
func (d *InputDialog) HasTextInputFocused() bool {
	return true
}
