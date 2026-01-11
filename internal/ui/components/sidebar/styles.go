package sidebar

import "github.com/charmbracelet/lipgloss"

// Colors - reuse GCP palette from main styles
var (
	colorPrimary   = lipgloss.Color("#4285F4") // Google Blue
	colorSecondary = lipgloss.Color("#34A853") // Google Green
	colorMuted     = lipgloss.Color("#9AA0A6") // Gray
	colorDimmed    = lipgloss.Color("#5F6368") // Dimmed gray (unfocused)
	colorWhite     = lipgloss.Color("#FFFFFF")
)

// Styles holds sidebar-specific styles
type Styles struct {
	Container    lipgloss.Style // Outer container with border
	Header       lipgloss.Style // Header with hamburger menu
	Divider      lipgloss.Style // Horizontal divider line
	Item         lipgloss.Style // Normal menu item
	ItemSelected lipgloss.Style // Cursor on item (focused)
	ItemActive   lipgloss.Style // Currently active view
	Category     lipgloss.Style // Category headers (when at root)
	BackItem     lipgloss.Style // "< Back" item
}

// DefaultStyles returns the default sidebar styles
func DefaultStyles() Styles {
	return Styles{
		Container: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorMuted).
			BorderRight(true).
			Padding(0, 1),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 0).
			MarginBottom(0),

		Divider: lipgloss.NewStyle().
			Foreground(colorMuted),

		Item: lipgloss.NewStyle().
			Foreground(colorWhite).
			Padding(0, 0),

		ItemSelected: lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(colorWhite).
			Bold(true).
			Padding(0, 0),

		ItemActive: lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true).
			Padding(0, 0),

		Category: lipgloss.NewStyle().
			Foreground(colorWhite).
			Bold(true).
			Padding(0, 0),

		BackItem: lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true).
			Padding(0, 0),
	}
}

// WithFocus returns styles adjusted for focused state
func (s Styles) WithFocus() Styles {
	// Highlight container border when focused
	s.Container = s.Container.
		BorderForeground(colorPrimary)

	return s
}

// Dimmed returns styles adjusted for unfocused state
func (s Styles) Dimmed() Styles {
	// Dim all colors when not focused
	s.Container = s.Container.
		BorderForeground(colorDimmed)

	s.Header = s.Header.
		Foreground(colorDimmed)

	s.Divider = s.Divider.
		Foreground(colorDimmed)

	s.Item = s.Item.
		Foreground(colorDimmed)

	s.Category = s.Category.
		Foreground(colorDimmed).
		Bold(false)

	s.BackItem = s.BackItem.
		Foreground(colorDimmed)

	// Keep active item visible but dimmed
	s.ItemActive = s.ItemActive.
		Foreground(colorMuted).
		Bold(false)

	return s
}
