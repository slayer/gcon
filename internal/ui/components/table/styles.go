package table

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// GCP-inspired color palette
var (
	ColorPrimary = lipgloss.Color("#4285F4") // Google Blue
	ColorMuted   = lipgloss.Color("#9AA0A6") // Gray
	ColorWhite   = lipgloss.Color("#FFFFFF")
	ColorBgLight = lipgloss.Color("#303134") // Lighter background
)

// DefaultTableStyles returns table styles matching the GCP theme
func DefaultTableStyles() table.Styles {
	s := table.DefaultStyles()

	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorMuted).
		BorderBottom(true).
		Bold(true).
		Foreground(ColorWhite)

	s.Selected = s.Selected.
		Foreground(ColorWhite).
		Background(ColorPrimary).
		Bold(false)

	s.Cell = s.Cell.
		Foreground(ColorWhite)

	return s
}

// HeaderStyle returns the style for table headers
func HeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWhite).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(ColorMuted)
}

// CellStyle returns the default style for table cells
func CellStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorWhite).
		Padding(0, 1)
}

// SelectedStyle returns the style for selected rows
func SelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorWhite).
		Background(ColorPrimary)
}

// StatusStyle returns styles for different status values
type StatusStyle struct {
	Running lipgloss.Style
	Stopped lipgloss.Style
	Pending lipgloss.Style
	Unknown lipgloss.Style
}

// DefaultStatusStyles returns status-specific styles
func DefaultStatusStyles() StatusStyle {
	return StatusStyle{
		Running: lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")), // Green
		Stopped: lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")), // Red
		Pending: lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC05")), // Yellow
		Unknown: lipgloss.NewStyle().Foreground(ColorMuted),
	}
}
