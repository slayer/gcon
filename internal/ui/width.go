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
// lipgloss.Place() pads ALL lines to the target width. So if any line has
// emojis that terminals render wider than lipgloss measures, those lines
// will overflow. We must reduce width by the max emoji count on any line
// to ensure the worst-case line still fits.
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
// In ASCII mode, no special symbols are used so this always returns false.
// In Unicode mode (--no-emojis), we still use Unicode symbols that need adjustment.
func isWideEmoji(r rune) bool {
	// In ASCII mode, all symbols are replaced with ASCII chars, no width adjustment needed
	if symbols.IsASCIIMode() {
		return false
	}

	// Symbols that lipgloss counts as 1 but terminals render as 2.
	// These apply to both Emoji and Unicode modes since both use these symbols.
	switch r {
	case '☁', // Cloud (header) - U+2601

		// Arrows (sidebar navigation)
		'◀', '▶', '▸',

		// Geometric shapes used as icons
		'□', '○', '◇', // Category icons (hollow)
		'■', '●', '▪', '◆', '▲', '◌', // Status and leaf icons

		// Powerline symbols (Private Use Area)
		'\ue0b0', '\ue0b1', '\ue0b2', '\ue0b3': // Powerline arrows
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
