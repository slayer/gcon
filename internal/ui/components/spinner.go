package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LoadingSpinner wraps the bubbles spinner with custom styling
type LoadingSpinner struct {
	spinner spinner.Model
	message string
	style   lipgloss.Style
}

// NewLoadingSpinner creates a new loading spinner with a message
func NewLoadingSpinner(message string) LoadingSpinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return LoadingSpinner{
		spinner: s,
		message: message,
		style:   lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")),
	}
}

// Init starts the spinner animation
func (l LoadingSpinner) Init() tea.Cmd {
	return l.spinner.Tick
}

// Update handles spinner tick messages
func (l LoadingSpinner) Update(msg tea.Msg) (LoadingSpinner, tea.Cmd) {
	var cmd tea.Cmd
	l.spinner, cmd = l.spinner.Update(msg)
	return l, cmd
}

// View renders the spinner with message
func (l LoadingSpinner) View() string {
	return l.spinner.View() + " " + l.style.Render(l.message)
}

// SetMessage updates the loading message
func (l *LoadingSpinner) SetMessage(message string) {
	l.message = message
}

// Tick returns a command to continue spinner animation
func (l LoadingSpinner) Tick() tea.Cmd {
	return l.spinner.Tick
}
