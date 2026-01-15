package actionmenu

import "github.com/charmbracelet/lipgloss"

// Colors - reuse GCP palette
var (
	colorPrimary = lipgloss.Color("#4285F4") // Google Blue
	colorDanger  = lipgloss.Color("#EA4335") // Google Red
	colorMuted   = lipgloss.Color("#9AA0A6") // Gray
	colorWhite   = lipgloss.Color("#FFFFFF")
)

// Styles holds action menu styles
type Styles struct {
	Container      lipgloss.Style // Outer container with border
	Title          lipgloss.Style // Menu title
	Key            lipgloss.Style // Hotkey character
	KeySelected    lipgloss.Style // Hotkey when row is selected
	KeyDisabled    lipgloss.Style // Hotkey when action is disabled
	KeyDangerous   lipgloss.Style // Hotkey for dangerous actions
	Label          lipgloss.Style // Action label
	LabelSelected  lipgloss.Style // Label when row is selected
	LabelDisabled  lipgloss.Style // Label when action is disabled
	LabelDangerous lipgloss.Style // Label for dangerous actions
	Divider        lipgloss.Style // Horizontal divider
	Help           lipgloss.Style // Help text at bottom
}

// DefaultStyles returns the default action menu styles
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

		KeyDisabled: lipgloss.NewStyle().
			Foreground(colorMuted),

		KeyDangerous: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorDanger),

		Label: lipgloss.NewStyle().
			Foreground(colorWhite),

		LabelSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary),

		LabelDisabled: lipgloss.NewStyle().
			Foreground(colorMuted),

		LabelDangerous: lipgloss.NewStyle().
			Foreground(colorDanger),

		Divider: lipgloss.NewStyle().
			Foreground(colorMuted),

		Help: lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true),
	}
}
