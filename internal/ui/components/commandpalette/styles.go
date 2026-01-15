package commandpalette

import "github.com/charmbracelet/lipgloss"

// Colors matching GCP theme
var (
	colorPrimary  = lipgloss.Color("#4285F4") // Google Blue
	colorMuted    = lipgloss.Color("#9AA0A6") // Gray
	colorText     = lipgloss.Color("#FFFFFF") // White text
	colorBorder   = lipgloss.Color("#5F6368") // Border gray
	colorSelected = lipgloss.Color("#4285F4") // Selected item background
)

// Styles holds all command palette styles
type Styles struct {
	Container     lipgloss.Style // Outer container with border
	Input         lipgloss.Style // Input line style
	InputPrefix   lipgloss.Style // The ":" prefix style
	Separator     lipgloss.Style // Line between input and results
	List          lipgloss.Style // Results list container
	Item          lipgloss.Style // Normal item
	ItemSelected  lipgloss.Style // Selected item
	ItemDisabled  lipgloss.Style // Disabled item (no project selected)
	Icon          lipgloss.Style // Icon style
	IconDisabled  lipgloss.Style // Disabled icon
	Cursor        lipgloss.Style // Selection cursor
	NoResults     lipgloss.Style // "No results" message
	CategoryLabel lipgloss.Style // Category label for grouping (future)
}

// DefaultStyles returns the default command palette styles
func DefaultStyles() Styles {
	return Styles{
		// Container with border, no background (transparent)
		Container: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1),

		Input: lipgloss.NewStyle().
			Foreground(colorText),

		InputPrefix: lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true),

		Separator: lipgloss.NewStyle().
			Foreground(colorBorder),

		List: lipgloss.NewStyle(),

		Item: lipgloss.NewStyle().
			Foreground(colorText).
			Padding(0, 1),

		ItemSelected: lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorSelected).
			Bold(true).
			Padding(0, 1),

		ItemDisabled: lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1),

		Icon: lipgloss.NewStyle().
			Foreground(colorText).
			MarginRight(1),

		IconDisabled: lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginRight(1),

		Cursor: lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true),

		NoResults: lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true).
			Padding(1, 1),

		CategoryLabel: lipgloss.NewStyle().
			Foreground(colorMuted).
			Bold(true).
			Padding(0, 1).
			MarginTop(1),
	}
}
