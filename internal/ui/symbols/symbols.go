// Package symbols provides centralized UI symbols with multiple display modes.
// - Emoji mode (default): colorful emojis like 🟢🔴🟡⚪
// - Unicode mode (--no-emojis): colored Unicode symbols like ●○◌
// - ASCII mode (--ascii): plain ASCII characters
package symbols

import "github.com/charmbracelet/lipgloss"

// Mode represents the symbol display mode
type Mode int

const (
	ModeEmoji   Mode = iota // Full emojis (default)
	ModeUnicode             // Unicode symbols with colors (--no-emojis)
	ModeASCII               // ASCII only (--ascii)
)

var activeMode = ModeEmoji

// Colors for status indicators (GCP-inspired palette)
var (
	colorGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	colorRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	colorYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))
	colorGray   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	colorBlue   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	colorOrange = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6D00"))
)

// SymbolSet contains all symbols for a display mode
type SymbolSet struct {
	Cloud            string
	Hamburger        string
	Back             string
	Expand           string
	Cursor           string
	Active           string
	StatusRunning    string
	StatusStopped    string
	StatusSuspended  string
	StatusTransition string
	StatusUnknown    string
	Folder           string
	File             string
	Divider          string
	IdentityUser     string
	IdentityService  string
	HeaderSepRight   string // Powerline thin right arrow for header breadcrumbs
	HeaderSepLeft    string // Powerline thin left arrow for header breadcrumbs
}

// Symbol sets for each mode
var (
	emojiSet = SymbolSet{
		Cloud:            "☁",
		Hamburger:        "☰",
		Back:             "◀",
		Expand:           "▸",
		Cursor:           "▶",
		Active:           "●",
		StatusRunning:    "🟢",
		StatusStopped:    "🔴",
		StatusSuspended:  "🟠",
		StatusTransition: "🟡",
		StatusUnknown:    "⚪",
		Folder:           "📁",
		File:             "📄",
		Divider:          "─",
		IdentityUser:     "👤",
		IdentityService:  "🔧",
		HeaderSepRight:   "\ue0b0", // Solid right arrow (fat)
		HeaderSepLeft:    "\ue0b2", // Solid left arrow (fat)
	}

	unicodeSet = SymbolSet{
		Cloud:            "☁",
		Hamburger:        "☰",
		Back:             "◀",
		Expand:           "▸",
		Cursor:           "▶",
		Active:           "●",
		StatusRunning:    colorGreen.Render("●"),
		StatusStopped:    colorRed.Render("●"),
		StatusSuspended:  colorOrange.Render("●"),
		StatusTransition: colorYellow.Render("○"),
		StatusUnknown:    colorGray.Render("◌"),
		Folder:           "▪",
		File:             "·",
		Divider:          "─",
		IdentityUser:     colorBlue.Render("◉"),
		IdentityService:  colorOrange.Render("⚙"),
		HeaderSepRight:   "\ue0b0", // Solid right arrow (fat)
		HeaderSepLeft:    "\ue0b2", // Solid left arrow (fat)
	}

	asciiSet = SymbolSet{
		Cloud:            "#",
		Hamburger:        "=",
		Back:             "<",
		Expand:           ">",
		Cursor:           ">",
		Active:           "*",
		StatusRunning:    colorGreen.Render("[OK]"),
		StatusStopped:    colorRed.Render("[--]"),
		StatusSuspended:  colorOrange.Render("[ZZ]"),
		StatusTransition: colorYellow.Render("[..]"),
		StatusUnknown:    colorGray.Render("[??]"),
		Folder:           "[D]",
		File:             "[F]",
		Divider:          "-",
		IdentityUser:     colorBlue.Render("[U]"),
		IdentityService:  colorOrange.Render("[SA]"),
		HeaderSepRight:   ">", // ASCII fallback
		HeaderSepLeft:    "<", // ASCII fallback
	}
)

// activeSet points to the current symbol set
var activeSet = &emojiSet

// SetMode sets the symbol display mode
func SetMode(mode Mode) {
	activeMode = mode
	switch mode {
	case ModeEmoji:
		activeSet = &emojiSet
	case ModeUnicode:
		activeSet = &unicodeSet
	case ModeASCII:
		activeSet = &asciiSet
	}
}

// GetMode returns the current display mode
func GetMode() Mode {
	return activeMode
}

// SetASCIIMode is a convenience function for backward compatibility
func SetASCIIMode(enabled bool) {
	if enabled {
		SetMode(ModeASCII)
	} else {
		SetMode(ModeEmoji)
	}
}

// IsASCIIMode returns whether ASCII mode is enabled
func IsASCIIMode() bool {
	return activeMode == ModeASCII
}

// IsEmojiMode returns whether full emoji mode is enabled
func IsEmojiMode() bool {
	return activeMode == ModeEmoji
}

// Header symbols
func Cloud() string { return activeSet.Cloud }

// Sidebar symbols
func Hamburger() string { return activeSet.Hamburger }
func Back() string      { return activeSet.Back }
func Expand() string    { return activeSet.Expand }
func Cursor() string    { return activeSet.Cursor }
func Active() string    { return activeSet.Active }

// Status symbols for instances
func StatusRunning() string       { return activeSet.StatusRunning }
func StatusStopped() string       { return activeSet.StatusStopped }
func StatusSuspended() string     { return activeSet.StatusSuspended }
func StatusTransitioning() string { return activeSet.StatusTransition }
func StatusUnknown() string       { return activeSet.StatusUnknown }

// GetStatusSymbol returns the appropriate status symbol for an instance status
func GetStatusSymbol(status string) string {
	switch status {
	case "RUNNING":
		return StatusRunning()
	case "TERMINATED", "STOPPED":
		return StatusStopped()
	case "SUSPENDED":
		return StatusSuspended()
	case "STAGING", "PROVISIONING", "STOPPING", "SUSPENDING", "REPAIRING":
		return StatusTransitioning()
	default:
		return StatusUnknown()
	}
}

// File/folder symbols
func Folder() string { return activeSet.Folder }
func File() string   { return activeSet.File }

// Divider character for sidebar
func Divider() string { return activeSet.Divider }

// Identity type symbols
func IdentityUser() string    { return activeSet.IdentityUser }
func IdentityService() string { return activeSet.IdentityService }

// StatusSymbolWidth returns the display width of status symbols in current mode
// Emoji: 2 (🟢), Unicode: 1 (●), ASCII: 4 ([OK])
func StatusSymbolWidth() int {
	switch activeMode {
	case ModeASCII:
		return 4
	case ModeEmoji:
		return 2
	default: // ModeUnicode
		return 1
	}
}

// Header breadcrumb separators
func HeaderSepRight() string { return activeSet.HeaderSepRight }
func HeaderSepLeft() string  { return activeSet.HeaderSepLeft }
