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
// bg is an optional background color applied to the entire line (empty string = no background).
func RenderCompactEntry(entry gcp.LogEntry, expanded bool, width int, bg string) string {
	indicator := "▸"
	if expanded {
		indicator = "▾"
	}

	sevStyle := severityStyle(entry.Severity)
	resourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8"))
	// plainStyle covers indicator, timestamp, spaces — everything without a foreground color
	plainStyle := lipgloss.NewStyle()

	// When a background is set, propagate it to ALL styles so ANSI
	// resets don't punch holes in the highlight bar.
	if bg != "" {
		bgColor := lipgloss.Color(bg)
		sevStyle = sevStyle.Background(bgColor)
		resourceStyle = resourceStyle.Background(bgColor)
		plainStyle = plainStyle.Background(bgColor)
	}

	abbrev := SeverityAbbrev(entry.Severity)
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")

	resource := entry.ResourceType
	if resource == "" {
		resource = "unknown"
	}
	if len(resource) > 20 {
		resource = resource[:17] + "..."
	}

	// Build header: indicator + severity + timestamp + resource
	header := plainStyle.Render("  "+indicator+" ") +
		sevStyle.Render(abbrev) +
		plainStyle.Render("  "+timestamp+"  ") +
		resourceStyle.Render(resource)

	// When expanded, header is the full line — message is shown in expanded fields
	if expanded {
		line := header
		if bg != "" {
			lineWidth := lipgloss.Width(line)
			if lineWidth < width {
				pad := strings.Repeat(" ", width-lineWidth)
				line += lipgloss.NewStyle().Background(lipgloss.Color(bg)).Render(pad)
			}
		}
		return line
	}

	// Collapsed: append truncated message after the header
	header += plainStyle.Render("  ")
	prefixWidth := lipgloss.Width(header)

	msgWidth := width - prefixWidth
	if msgWidth < 10 {
		msgWidth = 10
	}
	message := truncateEntry(entry.Message, msgWidth)

	line := header + plainStyle.Render(message)

	// Pad to full width so the background spans the entire terminal line
	if bg != "" {
		lineWidth := lipgloss.Width(line)
		if lineWidth < width {
			pad := strings.Repeat(" ", width-lineWidth)
			line += lipgloss.NewStyle().Background(lipgloss.Color(bg)).Render(pad)
		}
	}

	return line
}

// RenderWrappedEntry renders a log entry with the full message soft-wrapped to width.
// bg is an optional background color (empty string = no background).
// Returns the rendered string and the number of visual lines it occupies.
func RenderWrappedEntry(entry gcp.LogEntry, expanded bool, width int, bg string) (rendered string, lineCount int) {
	indicator := "▸"
	if expanded {
		indicator = "▾"
	}

	sevStyle := severityStyle(entry.Severity)
	resourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8"))
	plainStyle := lipgloss.NewStyle()

	if bg != "" {
		bgColor := lipgloss.Color(bg)
		sevStyle = sevStyle.Background(bgColor)
		resourceStyle = resourceStyle.Background(bgColor)
		plainStyle = plainStyle.Background(bgColor)
	}

	abbrev := SeverityAbbrev(entry.Severity)
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")

	resource := entry.ResourceType
	if resource == "" {
		resource = "unknown"
	}
	if len(resource) > 20 {
		resource = resource[:17] + "..."
	}

	prefix := plainStyle.Render("  "+indicator+" ") +
		sevStyle.Render(abbrev) +
		plainStyle.Render("  "+timestamp+"  ") +
		resourceStyle.Render(resource) +
		plainStyle.Render("  ")
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
		return prefix + plainStyle.Render(message), 1
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
// Long values are soft-wrapped to fit within width.
// cursorIdx is the 0-based index of the field the cursor is on (-1 for no cursor).
// Returns the rendered string and the number of visual lines it occupies.
func RenderExpandedFields(entry gcp.LogEntry, cursorIdx int, width int) (rendered string, lineCount int) {
	fields := entry.FlattenFields()
	if len(fields) == 0 {
		return "", 0
	}

	var b strings.Builder
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8"))
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED"))
	cursorStyle := lipgloss.NewStyle().Background(lipgloss.Color("#3C4043"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368")).Faint(true)
	totalLines := 0

	for i, field := range fields {
		prefix := "      " // 6 spaces indent
		keyRendered := keyStyle.Render(field.Key)
		keyWidth := lipgloss.Width(keyRendered)
		hintWidth := 0
		if i == cursorIdx {
			hintWidth = 5 // " [+f]"
		}

		// Available width for value text after "prefix key: " and optional hint
		maxValWidth := width - len(prefix) - keyWidth - 2 - hintWidth // 2 for ": "
		if maxValWidth < 10 {
			maxValWidth = 10
		}

		// Clean newlines for consistent wrapping
		cleanVal := strings.ReplaceAll(field.Value, "\n", " ")
		valRunes := []rune(cleanVal)

		// First line: "      key: value..."
		firstChunkEnd := maxValWidth
		if firstChunkEnd > len(valRunes) {
			firstChunkEnd = len(valRunes)
		}
		firstChunk := string(valRunes[:firstChunkEnd])
		firstLine := fmt.Sprintf("%s: %s", keyRendered, valStyle.Render(firstChunk))

		if i == cursorIdx {
			hint := hintStyle.Render(" [+f]")
			b.WriteString(prefix + cursorStyle.Render(firstLine) + hint)
		} else {
			b.WriteString(prefix + firstLine)
		}
		b.WriteString("\n")
		totalLines++

		// Continuation lines: indented to align under value
		if firstChunkEnd < len(valRunes) {
			contIndent := strings.Repeat(" ", len(prefix)+keyWidth+2) // align under value
			contWidth := width - len(prefix) - keyWidth - 2
			if contWidth < 10 {
				contWidth = 10
			}
			for start := firstChunkEnd; start < len(valRunes); start += contWidth {
				end := start + contWidth
				if end > len(valRunes) {
					end = len(valRunes)
				}
				chunk := string(valRunes[start:end])
				b.WriteString(contIndent + valStyle.Render(chunk))
				b.WriteString("\n")
				totalLines++
			}
		}
	}

	return b.String(), totalLines
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
