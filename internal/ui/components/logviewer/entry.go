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
// colorize enables logfmt syntax highlighting in the message.
func RenderCompactEntry(entry gcp.LogEntry, expanded bool, width int, bg string, colorize bool) string {
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

	var styledMsg string
	if colorize {
		styledMsg = colorizeMessage(message, bg)
	} else {
		styledMsg = plainStyle.Render(message)
	}
	line := header + styledMsg

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
// colorize enables logfmt syntax highlighting.
// Returns the rendered string and the number of visual lines it occupies.
func RenderWrappedEntry(entry gcp.LogEntry, expanded bool, width int, bg string, colorize bool) (rendered string, lineCount int) {
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

	// Use visual width for the fits-on-one-line check (ANSI codes are zero-width)
	visWidth := lipgloss.Width(message)
	if visWidth <= msgWidth {
		// Fits on one line
		if colorize {
			return prefix + colorizeMessage(message, bg), 1
		}
		return prefix + plainStyle.Render(message), 1
	}

	// Multi-line wrap: use ANSI-aware chunking if message has escape codes
	hasANSI := strings.Contains(message, "\x1b[")
	var b strings.Builder
	indent := strings.Repeat(" ", prefixWidth)
	lineCount = 0

	if hasANSI {
		// ANSI-aware wrapping: chunk by visible width
		chunks := wrapANSI(message, msgWidth)
		for ci, chunk := range chunks {
			if ci == 0 {
				b.WriteString(prefix + chunk)
			} else {
				b.WriteString(indent + chunk)
			}
			lineCount++
			if ci < len(chunks)-1 {
				b.WriteString("\n")
			}
		}
	} else {
		msgRunes := []rune(message)
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

// logStyles holds pre-built styles for log message colorization.
type logStyles struct {
	key     lipgloss.Style
	str     lipgloss.Style
	num     lipgloss.Style
	bool    lipgloss.Style
	bracket lipgloss.Style
	plain   lipgloss.Style
}

func newLogStyles(bg string) logStyles {
	make := func(fg string) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(lipgloss.Color(fg))
		if bg != "" {
			s = s.Background(lipgloss.Color(bg))
		}
		return s
	}
	return logStyles{
		key:     make("#8AB4F8"), // blue — keys
		str:     make("#34A853"), // green — quoted strings
		num:     make("#8AB4F8"), // blue — numbers
		bool:    make("#FBBC04"), // yellow — true/false/null
		bracket: make("#9AA0A6"), // muted gray — [bracketed]
		plain:   make("#E8EAED"), // light gray — default
	}
}

// colorizeMessage applies syntax highlighting to logfmt-style key=value pairs.
// Highlights: keys (blue), quoted strings (green), numbers (blue),
// booleans (yellow), [brackets] (muted gray).
// If the message already contains ANSI escape sequences, they are preserved as-is.
func colorizeMessage(s string, bg string) string {
	if len(s) == 0 {
		return s
	}

	// If message already has ANSI colors, pass through — don't double-style
	if strings.Contains(s, "\x1b[") {
		return s
	}

	st := newLogStyles(bg)
	var b strings.Builder
	runes := []rune(s)
	i := 0

	for i < len(runes) {
		// Try to match key=value
		if newPos := writeKeyValue(&b, runes, i, st); newPos > i {
			i = newPos
		} else if runes[i] == '[' {
			// [bracketed text]
			i = writeBracket(&b, runes, i, st)
		} else {
			// Plain text until next space
			start := i
			for i < len(runes) && (i == start || runes[i] != ' ') {
				i++
			}
			b.WriteString(st.plain.Render(string(runes[start:i])))
		}
		// Spaces between tokens
		for i < len(runes) && runes[i] == ' ' {
			b.WriteString(st.plain.Render(" "))
			i++
		}
	}

	return b.String()
}

// writeKeyValue tries to parse key=value at runes[pos]. Returns new position if matched, or pos if not.
func writeKeyValue(b *strings.Builder, runes []rune, pos int, st logStyles) int {
	i := pos
	for i < len(runes) && isKeyRune(runes[i]) {
		i++
	}
	if i == pos || i >= len(runes) || runes[i] != '=' {
		return pos // not a key=value
	}

	// Write key=
	b.WriteString(st.key.Render(string(runes[pos:i])))
	b.WriteString(st.plain.Render("="))
	i++ // skip '='

	// Parse value
	if i < len(runes) && runes[i] == '"' {
		i = writeQuotedString(b, runes, i, st)
	} else {
		valStart := i
		for i < len(runes) && runes[i] != ' ' {
			i++
		}
		val := string(runes[valStart:i])
		b.WriteString(colorizeValue(val, st.num, st.bool, st.plain))
	}
	return i
}

// writeQuotedString parses a "quoted string" starting at runes[pos] and writes it styled.
func writeQuotedString(b *strings.Builder, runes []rune, pos int, st logStyles) int {
	i := pos + 1 // skip opening "
	for i < len(runes) && runes[i] != '"' {
		if runes[i] == '\\' && i+1 < len(runes) {
			i++ // skip escaped char
		}
		i++
	}
	if i < len(runes) {
		i++ // skip closing "
	}
	b.WriteString(st.str.Render(string(runes[pos:i])))
	return i
}

// writeBracket parses [bracketed text] starting at runes[pos] and writes it styled.
func writeBracket(b *strings.Builder, runes []rune, pos int, st logStyles) int {
	i := pos + 1 // skip '['
	for i < len(runes) && runes[i] != ']' {
		i++
	}
	if i < len(runes) {
		i++ // skip ']'
	}
	b.WriteString(st.bracket.Render(string(runes[pos:i])))
	return i
}

// wrapANSI splits a string with ANSI codes into chunks of maxVisible visible characters.
func wrapANSI(s string, maxVisible int) []string {
	var chunks []string
	var chunk strings.Builder
	runes := []rune(s)
	visible := 0
	i := 0

	for i < len(runes) {
		if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			// Copy entire ANSI escape sequence without counting
			start := i
			i += 2
			for i < len(runes) && runes[i] < 0x40 {
				i++
			}
			if i < len(runes) {
				i++
			}
			chunk.WriteString(string(runes[start:i]))
		} else {
			if visible >= maxVisible {
				// End current chunk with reset, start new one
				chunk.WriteString("\x1b[0m")
				chunks = append(chunks, chunk.String())
				chunk.Reset()
				visible = 0
			}
			chunk.WriteRune(runes[i])
			visible++
			i++
		}
	}

	if chunk.Len() > 0 {
		chunks = append(chunks, chunk.String())
	}

	return chunks
}

func isKeyRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-'
}

func colorizeValue(val string, numStyle, boolStyle, defStyle lipgloss.Style) string {
	// Boolean / null
	switch val {
	case "true", "false", "null", "nil":
		return boolStyle.Render(val)
	}

	// Number: integer or float (possibly negative)
	if isNumeric(val) {
		return numStyle.Render(val)
	}

	return defStyle.Render(val)
}

func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	start := 0
	if s[0] == '-' || s[0] == '+' {
		start = 1
	}
	if start >= len(s) {
		return false
	}
	hasDot := false
	for i := start; i < len(s); i++ {
		if s[i] == '.' {
			if hasDot {
				return false
			}
			hasDot = true
		} else if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// truncateEntry truncates a string for compact display, replacing newlines with spaces.
// ANSI-aware: uses visual width so escape sequences don't count toward the limit.
func truncateEntry(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	visWidth := lipgloss.Width(s)
	if visWidth <= maxLen {
		return s
	}

	// No ANSI — fast path using rune slicing
	if !strings.Contains(s, "\x1b[") {
		runes := []rune(s)
		if maxLen <= 3 {
			return string(runes[:maxLen])
		}
		return string(runes[:maxLen-3]) + "..."
	}

	// ANSI-aware truncation: walk runes, skip escape sequences,
	// count only visible characters toward the limit.
	if maxLen <= 3 {
		return truncateANSI(s, maxLen)
	}
	return truncateANSI(s, maxLen-3) + "..."
}

// truncateANSI truncates a string with ANSI codes to maxVisible visible characters.
// ANSI escape sequences are passed through without counting toward the limit.
func truncateANSI(s string, maxVisible int) string {
	var b strings.Builder
	runes := []rune(s)
	visible := 0
	i := 0

	for i < len(runes) && visible < maxVisible {
		if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			// Copy entire ANSI escape sequence: ESC [ ... final_byte
			start := i
			i += 2 // skip ESC [
			for i < len(runes) && runes[i] < 0x40 {
				i++ // skip parameter and intermediate bytes
			}
			if i < len(runes) {
				i++ // skip final byte
			}
			b.WriteString(string(runes[start:i]))
		} else {
			b.WriteRune(runes[i])
			visible++
			i++
		}
	}

	// Append any trailing reset sequence so colors don't bleed
	if strings.Contains(s, "\x1b[") {
		b.WriteString("\x1b[0m")
	}

	return b.String()
}
