package components

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

var (
	errorTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EA4335")).
			Bold(true)

	errorContextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#DADCE0"))

	// HintStyle is exported for consistent hint styling across views
	HintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9AA0A6")).
			Italic(true)
)

// RenderError formats an error for display in the TUI with user-friendly messages
func RenderError(err error) string {
	if err == nil {
		return ""
	}

	// Check if it's our wrapped GCP error with rich context
	// Use errors.As to support wrapped error chains
	var gcpErr *gcp.GCPError
	if !errors.As(err, &gcpErr) {
		// Fallback for non-GCP errors
		return errorTitleStyle.Render("  Error: "+err.Error()) +
			"\n\n" + HintStyle.Render("  Press 'r' to retry")
	}

	var result string

	// Error icon and message
	icon := errorIcon(gcpErr.Code)
	result += errorTitleStyle.Render(fmt.Sprintf("  %s %s", icon, gcpErr.Message))
	result += "\n"

	// Show underlying error details when available and different from message
	if gcpErr.Underlying != nil {
		detail := gcpErr.Underlying.Error()
		if detail != gcpErr.Message {
			result += errorContextStyle.Render("  " + detail)
			result += "\n"
		}
	}

	// Show operation context if available
	if gcpErr.Operation != "" {
		ctx := fmt.Sprintf("  Operation: %s", gcpErr.Operation)
		if gcpErr.Resource != "" {
			ctx += fmt.Sprintf(" (%s)", gcpErr.Resource)
		}
		result += "\n" + errorContextStyle.Render(ctx)
	}

	// Actionable hint - strip retry instruction if present since we add it below
	hint := gcpErr.Hint
	if idx := strings.Index(strings.ToLower(hint), "(press 'r'"); idx != -1 {
		hint = strings.TrimSpace(hint[:idx])
	}
	result += "\n\n" + HintStyle.Render("  "+hint)
	result += "\n" + HintStyle.Render("  Press 'r' to retry")

	return result
}

// RenderInlineError formats an error as a simple styled message for form/creation views.
// Unlike RenderError, it omits retry hints since forms have their own navigation.
func RenderInlineError(err error) string {
	if err == nil {
		return ""
	}
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))

	// For GCP errors, show the underlying details instead of just the title
	var gcpErr *gcp.GCPError
	if errors.As(err, &gcpErr) && gcpErr.Underlying != nil {
		detail := gcpErr.Underlying.Error()
		if detail != gcpErr.Message {
			return "\n\n" + errorStyle.Render("Error: "+gcpErr.Message) +
				"\n" + errorContextStyle.Render("  "+detail)
		}
	}

	return "\n\n" + errorStyle.Render("Error: "+err.Error())
}

// errorIcon returns an appropriate icon for each error type
func errorIcon(code gcp.ErrorCode) string {
	switch code {
	case gcp.ErrorUnauthenticated:
		return "🔑"
	case gcp.ErrorPermissionDenied:
		return "🚫"
	case gcp.ErrorAPINotEnabled:
		return "⚙️"
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
