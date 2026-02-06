package ui

import (
	"github.com/slayer/gcon/internal/debug"
)

// debugLog writes a formatted message to the debug log.
// This is a wrapper around debug.Log for backward compatibility within the UI package.
func debugLog(format string, args ...interface{}) {
	debug.Log(format, args...)
}

// debugLogView logs information about a rendered view for debugging layout issues.
// This is a wrapper around debug.LogView for backward compatibility within the UI package.
func debugLogView(name string, content string) {
	debug.LogView(name, content)
}
