package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var debugFile *os.File
var debugEnabled bool

func init() {
	// Enable debug logging if GCON_DEBUG env is set
	if os.Getenv("GCON_DEBUG") != "" {
		debugEnabled = true
		var err error
		debugFile, err = os.Create("/tmp/gcon-debug.log")
		if err != nil {
			// Silently disable debug if we can't create the file
			debugEnabled = false
		}
	}
}

func debugLog(format string, args ...interface{}) {
	if !debugEnabled || debugFile == nil {
		return
	}
	_, _ = fmt.Fprintf(debugFile, format+"\n", args...)
	_ = debugFile.Sync()
}

func debugLogView(name string, content string) {
	if !debugEnabled {
		return
	}
	lines := strings.Split(content, "\n")
	debugLog("%s: height=%d (lipgloss=%d), lines=%d, width=%d",
		name, len(lines), lipgloss.Height(content), len(lines), lipgloss.Width(content))
	if len(lines) > 0 {
		debugLog("  first (w=%d): %q", lipgloss.Width(lines[0]), truncStr(lines[0], 50))
	}
	if len(lines) > 1 {
		debugLog("  last (w=%d): %q", lipgloss.Width(lines[len(lines)-1]), truncStr(lines[len(lines)-1], 50))
	}
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
