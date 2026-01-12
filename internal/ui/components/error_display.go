package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

var (
	errorTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EA4335")).
			Bold(true)

	errorContextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#DADCE0"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9AA0A6")).
			Italic(true)
)

// RenderError formats an error for display in the TUI with user-friendly messages
func RenderError(err error) string {
	if err == nil {
		return ""
	}

	// Check if it's our wrapped GCP error with rich context
	gcpErr, ok := err.(*gcp.GCPError)
	if !ok {
		// Fallback for non-GCP errors
		return errorTitleStyle.Render("  Error: "+err.Error()) +
			"\n\n" + hintStyle.Render("  Press 'r' to retry")
	}

	var result string

	// Error icon and message
	icon := errorIcon(gcpErr.Code)
	result += errorTitleStyle.Render(fmt.Sprintf("  %s %s", icon, gcpErr.Message))
	result += "\n"

	// Show operation context if available
	if gcpErr.Operation != "" {
		ctx := fmt.Sprintf("  Operation: %s", gcpErr.Operation)
		if gcpErr.Resource != "" {
			ctx += fmt.Sprintf(" (%s)", gcpErr.Resource)
		}
		result += "\n" + errorContextStyle.Render(ctx)
	}

	// Actionable hint
	result += "\n\n" + hintStyle.Render("  "+gcpErr.Hint)
	result += "\n" + hintStyle.Render("  Press 'r' to retry")

	return result
}

// errorIcon returns an appropriate icon for each error type
func errorIcon(code gcp.ErrorCode) string {
	switch code {
	case gcp.ErrorUnauthenticated:
		return "🔑"
	case gcp.ErrorPermissionDenied:
		return "🚫"
	case gcp.ErrorNotFound:
		return "❓"
	case gcp.ErrorRateLimited, gcp.ErrorQuotaExceeded:
		return "⏳"
	case gcp.ErrorServiceUnavailable:
		return "🔧"
	case gcp.ErrorNetwork:
		return "🌐"
	default:
		return "❌"
	}
}
