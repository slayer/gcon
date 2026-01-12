// Package symbols provides centralized UI symbols with ASCII fallback support.
// Use SetASCIIMode(true) to replace emojis with plain ASCII characters.
package symbols

import "github.com/charmbracelet/lipgloss"

var asciiMode bool

// Colors for status indicators (GCP-inspired palette)
var (
	colorGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	colorRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	colorYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))
	colorGray   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
)

// SetASCIIMode enables or disables ASCII-only mode.
// When enabled, all symbols return ASCII alternatives instead of Unicode/emojis.
func SetASCIIMode(enabled bool) {
	asciiMode = enabled
}

// IsASCIIMode returns whether ASCII mode is enabled.
func IsASCIIMode() bool {
	return asciiMode
}

// Header symbols
func Cloud() string {
	if asciiMode {
		return "#"
	}
	// "☁" Cloud - 1-char wide
	// return "◈" // Diamond with dot - 1-char wide
	return "#"
}

// Sidebar symbols
func Hamburger() string {
	if asciiMode {
		return "="
	}
	return "☰"
}

func Back() string {
	if asciiMode {
		return "<"
	}
	return "◀"
}

func Expand() string {
	if asciiMode {
		return ">"
	}
	return "▸"
}

func Cursor() string {
	if asciiMode {
		return ">"
	}
	return "▶"
}

func Active() string {
	if asciiMode {
		return "*"
	}
	return "●"
}

// Status symbols for instances
// Using 1-char wide Unicode circles with colors instead of 2-wide emoji circles
func StatusRunning() string {
	if asciiMode {
		return colorGreen.Render("[OK]")
	}
	// return "🟢"
	return colorGreen.Render("●")
}

func StatusStopped() string {
	if asciiMode {
		return colorRed.Render("[--]")
	}
	return colorRed.Render("●")
}

func StatusTransitioning() string {
	if asciiMode {
		return colorYellow.Render("[..]")
	}
	return colorYellow.Render("○")
}

func StatusUnknown() string {
	if asciiMode {
		return colorGray.Render("[??]")
	}
	return colorGray.Render("◌")
}

// GetStatusSymbol returns the appropriate status symbol for an instance status.
func GetStatusSymbol(status string) string {
	switch status {
	case "RUNNING":
		return StatusRunning()
	case "TERMINATED", "STOPPED":
		return StatusStopped()
	case "STAGING", "PROVISIONING", "STOPPING", "SUSPENDING":
		return StatusTransitioning()
	default:
		return StatusUnknown()
	}
}

// File/folder symbols
func Folder() string {
	if asciiMode {
		return "[D]"
	}
	return "📁"
}

func File() string {
	if asciiMode {
		return "[F]"
	}
	return "📄"
}

// Divider character for sidebar
func Divider() string {
	if asciiMode {
		return "-"
	}
	return "─"
}
