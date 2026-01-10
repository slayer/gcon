package components

import (
	"github.com/charmbracelet/lipgloss"
)

// StatusBar displays contextual information at the bottom of the screen
type StatusBar struct {
	width   int
	style   lipgloss.Style
	left    string
	center  string
	right   string
}

// NewStatusBar creates a new status bar
func NewStatusBar() StatusBar {
	return StatusBar{
		style: lipgloss.NewStyle().
			Background(lipgloss.Color("#303134")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1),
	}
}

// SetWidth sets the status bar width
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// SetLeft sets the left-aligned content
func (s *StatusBar) SetLeft(content string) {
	s.left = content
}

// SetCenter sets the center-aligned content
func (s *StatusBar) SetCenter(content string) {
	s.center = content
}

// SetRight sets the right-aligned content
func (s *StatusBar) SetRight(content string) {
	s.right = content
}

// View renders the status bar
func (s StatusBar) View() string {
	if s.width == 0 {
		return ""
	}

	leftStyle := lipgloss.NewStyle().Width(s.width / 3).Align(lipgloss.Left)
	centerStyle := lipgloss.NewStyle().Width(s.width / 3).Align(lipgloss.Center)
	rightStyle := lipgloss.NewStyle().Width(s.width / 3).Align(lipgloss.Right)

	bar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftStyle.Render(s.left),
		centerStyle.Render(s.center),
		rightStyle.Render(s.right),
	)

	return s.style.Width(s.width).Render(bar)
}
