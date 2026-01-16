// Package links provides a component for navigable links within detail views.
package links

import "github.com/charmbracelet/lipgloss"

// Colors - reuse GCP palette
var (
	colorPrimary = lipgloss.Color("#4285F4") // Google Blue
	colorWhite   = lipgloss.Color("#FFFFFF")
	colorMuted   = lipgloss.Color("#9AA0A6") // Gray
)

// Styles holds link list styles
type Styles struct {
	Normal  lipgloss.Style // Normal link row
	Focused lipgloss.Style // Focused/selected link row
	Cursor  lipgloss.Style // Cursor indicator style
	Header  lipgloss.Style // Table header style
	Divider lipgloss.Style // Divider line style
}

// DefaultStyles returns the default link list styles
func DefaultStyles() Styles {
	return Styles{
		Normal: lipgloss.NewStyle().
			Foreground(colorWhite),

		Focused: lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(colorPrimary).
			Bold(true),

		Cursor: lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true),

		Header: lipgloss.NewStyle().
			Foreground(colorMuted),

		Divider: lipgloss.NewStyle().
			Foreground(colorMuted),
	}
}
