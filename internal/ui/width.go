package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// TerminalWidth calculates the actual terminal width of a string,
// accounting for emoji characters that lipgloss.Width() miscounts.
//
// lipgloss.Width() counts most emojis as 1 character, but native terminals
// (Kitty, Alacritty, macOS Terminal) render them as 2 characters wide.
// This function adds 1 for each emoji to get the true terminal width.
func TerminalWidth(s string) int {
	base := lipgloss.Width(s)
	extra := countWideEmojis(s)
	return base + extra
}

// SafeWidth returns the maximum width that can be used for content
// to avoid line wrapping due to emoji width miscalculation.
// Pass the terminal width and the string that will be rendered.
//
// For multi-line content, this finds the line with the most miscounted emojis
// and reduces width by that amount. This ensures all lines will fit when
// lipgloss.Place() pads each line to this width.
func SafeWidth(terminalWidth int, content string) int {
	maxExtra := maxLineEmojiCount(content)
	safe := terminalWidth - maxExtra
	if safe < 10 {
		return 10
	}
	return safe
}

// maxLineEmojiCount finds the maximum emoji count on any single line.
// This is used to calculate safe width for multi-line content.
func maxLineEmojiCount(content string) int {
	maxCount := 0
	start := 0
	for i, r := range content {
		if r == '\n' {
			line := content[start:i]
			count := countWideEmojis(line)
			if count > maxCount {
				maxCount = count
			}
			start = i + 1
		}
	}
	// Check last line
	if start < len(content) {
		line := content[start:]
		count := countWideEmojis(line)
		if count > maxCount {
			maxCount = count
		}
	}
	return maxCount
}

// SafeWidthForEmojis returns the terminal width reduced by the expected
// number of emojis that will appear on a line. Use this when you know
// how many emojis will be rendered but don't have the final string yet.
func SafeWidthForEmojis(terminalWidth int, emojiCount int) int {
	safe := terminalWidth - emojiCount
	if safe < 10 {
		return 10
	}
	return safe
}

// countWideEmojis counts emojis that terminals render as 2-wide
// but lipgloss counts as 1-wide.
func countWideEmojis(s string) int {
	count := 0
	for _, r := range s {
		if isWideEmoji(r) {
			count++
		}
	}
	return count
}

// isWideEmoji returns true if the rune is rendered as 2 characters wide
// in native terminals but lipgloss.Width() counts as 1.
//
// Note: lipgloss already counts colored circle emojis (🟢, 🔴, 🟡, ⚪) as 2-wide,
// so we only need to handle symbols it miscounts.
//
// When ASCII mode is enabled, no emojis are used so this always returns false.
func isWideEmoji(r rune) bool {
	// In ASCII mode, all emojis are replaced with ASCII chars, no width adjustment needed
	if symbols.IsASCIIMode() {
		return false
	}

	// Symbols that lipgloss counts as 1 but terminals render as 2
	switch r {
	case '☁', // Cloud (header) - U+2601
		'☰',           // Hamburger menu (sidebar) - U+2630
		'◀', '▶', '▸', // Arrows (sidebar navigation) - U+25C0, U+25B6, U+25B8
		'●': // Active indicator (sidebar) - U+25CF
		return true
	}

	// Note: Colored circle emojis (🟢 🔴 🟡 ⚪) are already counted as 2-wide by lipgloss
	// so we don't need to add them here.

	return false
}

// MaxLineWidth finds the maximum terminal width among all lines in content.
func MaxLineWidth(content string) int {
	max := 0
	start := 0
	for i, r := range content {
		if r == '\n' {
			line := content[start:i]
			w := TerminalWidth(line)
			if w > max {
				max = w
			}
			start = i + 1
		}
	}
	// Check last line (if no trailing newline)
	if start < len(content) {
		line := content[start:]
		w := TerminalWidth(line)
		if w > max {
			max = w
		}
	}
	return max
}
