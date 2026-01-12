// Package symbols provides centralized UI symbols with ASCII fallback support.
// Use SetASCIIMode(true) to replace emojis with plain ASCII characters.
package symbols

var asciiMode bool

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
	return "☁"
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
func StatusRunning() string {
	if asciiMode {
		return "[OK]"
	}
	return "🟢"
}

func StatusStopped() string {
	if asciiMode {
		return "[--]"
	}
	return "🔴"
}

func StatusTransitioning() string {
	if asciiMode {
		return "[..]"
	}
	return "🟡"
}

func StatusUnknown() string {
	if asciiMode {
		return "[??]"
	}
	return "⚪"
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
