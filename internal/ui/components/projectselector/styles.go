package projectselector

import "github.com/charmbracelet/lipgloss"

// Colors - GCP palette
var (
	colorPrimary = lipgloss.Color("#4285F4") // Google Blue
	colorMuted   = lipgloss.Color("#9AA0A6") // Gray
	colorWhite   = lipgloss.Color("#FFFFFF")
	colorError   = lipgloss.Color("#EA4335") // Google Red
)

// Styles holds project selector styles
type Styles struct {
	Container           lipgloss.Style
	Title               lipgloss.Style
	ProjectName         lipgloss.Style
	ProjectNameSelected lipgloss.Style
	ProjectID           lipgloss.Style
	ProjectIDSelected   lipgloss.Style
	Divider             lipgloss.Style
	Help                lipgloss.Style
	Error               lipgloss.Style
	EmptyState          lipgloss.Style
}

// DefaultStyles returns default project selector styles
func DefaultStyles() Styles {
	return Styles{
		Container: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary),

		ProjectName: lipgloss.NewStyle().
			Foreground(colorWhite),

		ProjectNameSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary),

		ProjectID: lipgloss.NewStyle().
			Foreground(colorMuted),

		ProjectIDSelected: lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(colorPrimary),

		Divider: lipgloss.NewStyle().
			Foreground(colorMuted),

		Help: lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true),

		Error: lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true),

		EmptyState: lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true),
	}
}
