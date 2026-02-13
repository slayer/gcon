package sortmenu

import "github.com/charmbracelet/lipgloss"

// GCP color palette
var (
	colorPrimary = lipgloss.Color("#4285F4") // Google Blue
	colorMuted   = lipgloss.Color("#9AA0A6") // Gray
	colorWhite   = lipgloss.Color("#FFFFFF")
	colorAccent  = lipgloss.Color("#34A853") // Green for active sort indicator
)

// Styles holds sort menu styles.
type Styles struct {
	Container     lipgloss.Style
	Title         lipgloss.Style
	Key           lipgloss.Style
	KeySelected   lipgloss.Style
	Label         lipgloss.Style
	LabelSelected lipgloss.Style
	Indicator     lipgloss.Style
	Divider       lipgloss.Style
	Help          lipgloss.Style
}

// DefaultStyles returns the default sort menu styles.
func DefaultStyles() Styles {
	return Styles{
		Container: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1),

		Key: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary),

		KeySelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary),

		Label: lipgloss.NewStyle().
			Foreground(colorWhite),

		LabelSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary),

		Indicator: lipgloss.NewStyle().
			Foreground(colorAccent),

		Divider: lipgloss.NewStyle().
			Foreground(colorMuted),

		Help: lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true),
	}
}
