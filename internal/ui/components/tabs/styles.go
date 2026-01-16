// Package tabs provides a reusable tab bar component for detail views.
package tabs

import "github.com/charmbracelet/lipgloss"

// Colors - reuse GCP palette
var (
	colorPrimary = lipgloss.Color("#4285F4") // Google Blue
	colorMuted   = lipgloss.Color("#9AA0A6") // Gray
)

// Styles holds tab bar styles
type Styles struct {
	Active   lipgloss.Style // Active tab with brackets
	Inactive lipgloss.Style // Inactive tab plain text
	Bar      lipgloss.Style // Container for the tab bar
}

// DefaultStyles returns the default tab bar styles
func DefaultStyles() Styles {
	return Styles{
		Active: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary),

		Inactive: lipgloss.NewStyle().
			Foreground(colorMuted),

		Bar: lipgloss.NewStyle().
			MarginBottom(1),
	}
}
