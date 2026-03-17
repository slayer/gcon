package logviewer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

// Severity color mapping
var severityColors = map[string]string{
	"DEBUG":     "#9AA0A6",
	"DEFAULT":   "#9AA0A6",
	"INFO":      "#4285F4",
	"NOTICE":    "#4285F4",
	"WARNING":   "#FBBC04",
	"ERROR":     "#EA4335",
	"CRITICAL":  "#EA4335",
	"ALERT":     "#EA4335",
	"EMERGENCY": "#EA4335",
}

// SeverityAbbrev returns a single-character abbreviation for a severity level.
func SeverityAbbrev(severity string) string {
	switch severity {
	case "INFO":
		return "I"
	case "WARNING":
		return "W"
	case "ERROR":
		return "E"
	case "CRITICAL":
		return "C"
	case "DEBUG", "DEFAULT":
		return "D"
	case "NOTICE":
		return "N"
	case "ALERT":
		return "A"
	case "EMERGENCY":
		return "!"
	default:
		return "?"
	}
}

// severityStyle returns a lipgloss style for the given severity.
func severityStyle(severity string) lipgloss.Style {
	color, ok := severityColors[severity]
	if !ok {
		color = "#9AA0A6"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)
}

// RenderCompactEntry renders a single log entry in compact (one-line) format.
// The output is guaranteed to fit within width visual characters.
func RenderCompactEntry(entry gcp.LogEntry, expanded bool, width int) string {
	indicator := "▸"
	if expanded {
		indicator = "▾"
	}

	sevStyle := severityStyle(entry.Severity)
	abbrev := SeverityAbbrev(entry.Severity)
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")

	resourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8"))
	resource := entry.ResourceType
	if resource == "" {
		resource = "unknown"
	}
	if len(resource) > 20 {
		resource = resource[:17] + "..."
	}

	// Build the fixed prefix: "  ▸ I  2006-01-02 15:04:05  resource_type  "
	prefix := fmt.Sprintf("  %s %s  %s  %s  ",
		indicator,
		sevStyle.Render(abbrev),
		timestamp,
		resourceStyle.Render(resource),
	)
	prefixWidth := lipgloss.Width(prefix)

	msgWidth := width - prefixWidth
	if msgWidth < 10 {
		msgWidth = 10
	}
	message := truncateEntry(entry.Message, msgWidth)

	return prefix + message
}

// RenderWrappedEntry renders a log entry with the full message soft-wrapped to width.
// Returns the rendered string and the number of visual lines it occupies.
func RenderWrappedEntry(entry gcp.LogEntry, expanded bool, width int) (rendered string, lineCount int) {
	indicator := "▸"
	if expanded {
		indicator = "▾"
	}

	sevStyle := severityStyle(entry.Severity)
	abbrev := SeverityAbbrev(entry.Severity)
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")

	resourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8"))
	resource := entry.ResourceType
	if resource == "" {
		resource = "unknown"
	}
	if len(resource) > 20 {
		resource = resource[:17] + "..."
	}

	prefix := fmt.Sprintf("  %s %s  %s  %s  ",
		indicator,
		sevStyle.Render(abbrev),
		timestamp,
		resourceStyle.Render(resource),
	)
	prefixWidth := lipgloss.Width(prefix)

	// Replace newlines in message with spaces for consistent wrapping
	message := strings.ReplaceAll(entry.Message, "\n", " ")

	msgWidth := width - prefixWidth
	if msgWidth < 10 {
		msgWidth = 10
	}

	// Wrap message into lines of msgWidth runes each
	msgRunes := []rune(message)
	if len(msgRunes) <= msgWidth {
		// Fits on one line
		return prefix + message, 1
	}

	var b strings.Builder
	indent := strings.Repeat(" ", prefixWidth)
	lineCount = 0

	for start := 0; start < len(msgRunes); start += msgWidth {
		end := start + msgWidth
		if end > len(msgRunes) {
			end = len(msgRunes)
		}
		chunk := string(msgRunes[start:end])

		if start == 0 {
			b.WriteString(prefix + chunk)
		} else {
			b.WriteString(indent + chunk)
		}
		lineCount++
		if end < len(msgRunes) {
			b.WriteString("\n")
		}
	}

	return b.String(), lineCount
}

// RenderExpandedFields renders the expanded field view for a log entry.
// cursorIdx is the 0-based index of the field the cursor is on (-1 for no cursor).
func RenderExpandedFields(entry gcp.LogEntry, cursorIdx int, width int) string {
	fields := entry.FlattenFields()
	if len(fields) == 0 {
		return ""
	}

	var b strings.Builder
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8"))
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED"))
	cursorStyle := lipgloss.NewStyle().Background(lipgloss.Color("#3C4043"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368")).Faint(true)

	for i, field := range fields {
		prefix := "      " // 6 spaces indent
		// Truncate value to fit within width (prefix + key + ": " + value + optional hint)
		keyRendered := keyStyle.Render(field.Key)
		keyWidth := lipgloss.Width(keyRendered)
		hintWidth := 0
		if i == cursorIdx {
			hintWidth = 5 // " [+f]"
		}
		maxValWidth := width - len(prefix) - keyWidth - 2 - hintWidth // 2 for ": "
		if maxValWidth < 10 {
			maxValWidth = 10
		}
		truncatedVal := truncateEntry(field.Value, maxValWidth)
		line := fmt.Sprintf("%s: %s", keyRendered, valStyle.Render(truncatedVal))

		if i == cursorIdx {
			hint := hintStyle.Render(" [+f]")
			b.WriteString(prefix + cursorStyle.Render(line) + hint)
		} else {
			b.WriteString(prefix + line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// truncateEntry truncates a string for compact display, replacing newlines with spaces.
func truncateEntry(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
