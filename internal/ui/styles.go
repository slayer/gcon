package ui

import "github.com/charmbracelet/lipgloss"

// Color palette - GCP inspired colors
var (
	ColorPrimary   = lipgloss.Color("#4285F4") // Google Blue
	ColorSecondary = lipgloss.Color("#34A853") // Google Green
	ColorWarning   = lipgloss.Color("#FBBC05") // Google Yellow
	ColorError     = lipgloss.Color("#EA4335") // Google Red
	ColorMuted     = lipgloss.Color("#9AA0A6") // Gray
	ColorBg        = lipgloss.Color("#202124") // Dark background
	ColorBgLight   = lipgloss.Color("#303134") // Lighter background
)

// Styles holds all application styles
type Styles struct {
	App          lipgloss.Style
	Title        lipgloss.Style
	Subtitle     lipgloss.Style
	Selected     lipgloss.Style
	Normal       lipgloss.Style
	Muted        lipgloss.Style
	Error        lipgloss.Style
	Success      lipgloss.Style
	Warning      lipgloss.Style
	Help         lipgloss.Style
	StatusBar    lipgloss.Style
	ListItem     lipgloss.Style
	ActiveBorder lipgloss.Style
}

// DefaultStyles returns the default application styles
func DefaultStyles() Styles {
	return Styles{
		App: lipgloss.NewStyle().
			Padding(1, 2),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginBottom(1),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorPrimary).
			Padding(0, 1),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")),

		Muted: lipgloss.NewStyle().
			Foreground(ColorMuted),

		Error: lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(ColorSecondary),

		Warning: lipgloss.NewStyle().
			Foreground(ColorWarning),

		Help: lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(1),

		StatusBar: lipgloss.NewStyle().
			Background(ColorBgLight).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1),

		ListItem: lipgloss.NewStyle().
			PaddingLeft(2),

		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1),
	}
}
