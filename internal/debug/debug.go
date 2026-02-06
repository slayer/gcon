package debug

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	debugFile    *os.File
	debugEnabled bool
)

// EnableDebug enables debug logging to ./gcon-debug.log in the current directory.
// Warning: This significantly impacts performance due to disk sync on every log.
func EnableDebug() error {
	debugEnabled = true
	var err error
	debugFile, err = os.Create("gcon-debug.log")
	if err != nil {
		debugEnabled = false
		return fmt.Errorf("failed to create debug log file: %w", err)
	}
	return nil
}

// Close closes the debug log file.
func Close() error {
	if debugFile != nil {
		err := debugFile.Close()
		debugFile = nil
		debugEnabled = false
		return err
	}
	return nil
}

// IsEnabled returns true if debug logging is enabled.
func IsEnabled() bool {
	return debugEnabled
}

// Log writes a formatted message to the debug log.
func Log(format string, args ...interface{}) {
	if !debugEnabled || debugFile == nil {
		return
	}
	_, _ = fmt.Fprintf(debugFile, format+"\n", args...) //nolint:errcheck // Debug logging
	_ = debugFile.Sync()                                //nolint:errcheck // Debug logging
}

// LogView logs information about a rendered view for debugging layout issues.
func LogView(name string, content string) {
	if !debugEnabled {
		return
	}
	lines := strings.Split(content, "\n")
	Log("%s: height=%d (lipgloss=%d), lines=%d, width=%d",
		name, len(lines), lipgloss.Height(content), len(lines), lipgloss.Width(content))
	if len(lines) > 0 {
		Log("  first (w=%d): %q", lipgloss.Width(lines[0]), truncStr(lines[0], 50))
	}
	if len(lines) > 1 {
		Log("  last (w=%d): %q", lipgloss.Width(lines[len(lines)-1]), truncStr(lines[len(lines)-1], 50))
	}
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
