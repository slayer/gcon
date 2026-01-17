// Package links provides a component for navigable links within detail views.
package links

import "github.com/charmbracelet/lipgloss"

// Colors - reuse GCP palette
var (
	colorLink    = lipgloss.Color("#5A95F5") // Lighter blue for links
	colorPrimary = lipgloss.Color("#4285F4") // Google Blue
	colorWhite   = lipgloss.Color("#FFFFFF")
	colorMuted   = lipgloss.Color("#9AA0A6") // Gray
)

// Styles holds link list styles
type Styles struct {
	Normal  lipgloss.Style // Normal link row (unfocused region)
	Link    lipgloss.Style // Link style - colored and underlined to indicate clickable
	Focused lipgloss.Style // Focused/selected link row (when region is focused)
	Cursor  lipgloss.Style // Cursor indicator style
	Header  lipgloss.Style // Table header style
	Divider lipgloss.Style // Divider line style
}

// DefaultStyles returns the default link list styles
func DefaultStyles() Styles {
	return Styles{
		// Normal style when region is not focused - still shows link color
		Normal: lipgloss.NewStyle().
			Foreground(colorLink),

		// Link style - blue to indicate navigable
		Link: lipgloss.NewStyle().
			Foreground(colorLink),

		// Focused style when cursor is on this row
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
