package overlay

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// Center places the overlay content centered on top of the background,
// preserving the background content on both sides of the overlay.
// This is ANSI-safe and handles color escape sequences correctly.
func Center(background, overlayContent string, bgWidth, bgHeight int) string {
	bgLines := strings.Split(background, "\n")
	overlayLines := strings.Split(overlayContent, "\n")

	// Ensure we have enough background lines
	for len(bgLines) < bgHeight {
		bgLines = append(bgLines, "")
	}

	// Find max overlay width for consistent positioning
	maxOverlayWidth := 0
	for _, line := range overlayLines {
		w := lipgloss.Width(line)
		if w > maxOverlayWidth {
			maxOverlayWidth = w
		}
	}

	// Calculate center position
	leftPad := (bgWidth - maxOverlayWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	topPad := (bgHeight - len(overlayLines)) / 2
	if topPad < 0 {
		topPad = 0
	}

	// Build result preserving background on sides
	result := make([]string, len(bgLines))
	copy(result, bgLines)

	rightStart := leftPad + maxOverlayWidth

	for i, overlayLine := range overlayLines {
		bgIndex := topPad + i
		if bgIndex >= len(result) {
			break
		}

		bgLine := result[bgIndex]

		var newLine strings.Builder

		// Left part: truncate background to leftPad characters
		if leftPad > 0 {
			leftPart := truncateRightOverlay(bgLine, leftPad)
			newLine.WriteString(leftPart)
			// Pad if background was shorter than leftPad
			leftWidth := lipgloss.Width(leftPart)
			if leftWidth < leftPad {
				newLine.WriteString(strings.Repeat(" ", leftPad-leftWidth))
			}
		}

		// Middle: the overlay line, padded to consistent width
		newLine.WriteString(overlayLine)
		overlayLineWidth := lipgloss.Width(overlayLine)
		if overlayLineWidth < maxOverlayWidth {
			newLine.WriteString(strings.Repeat(" ", maxOverlayWidth-overlayLineWidth))
		}

		// Right part: skip first rightStart characters of background
		bgWidth := lipgloss.Width(bgLine)
		if bgWidth > rightStart {
			rightPart := truncateLeftOverlay(bgLine, rightStart)
			newLine.WriteString(rightPart)
		}

		result[bgIndex] = newLine.String()
	}

	return strings.Join(result, "\n")
}

// truncateRightOverlay keeps only the first n visible columns of an ANSI string.
func truncateRightOverlay(s string, n int) string {
	if n <= 0 {
		return ""
	}

	var result strings.Builder
	var visibleWidth int
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}
		if inEscape {
			result.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}

		rw := runewidth.RuneWidth(r)
		if visibleWidth+rw > n {
			break
		}
		result.WriteRune(r)
		visibleWidth += rw
	}

	return result.String()
}

// truncateLeftOverlay removes the first n visible columns from an ANSI string.
func truncateLeftOverlay(s string, n int) string {
	width := lipgloss.Width(s)
	if n >= width {
		return ""
	}

	var result strings.Builder
	var visibleWidth int
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			if visibleWidth >= n {
				result.WriteRune(r)
			}
			continue
		}
		if inEscape {
			if visibleWidth >= n {
				result.WriteRune(r)
			}
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}

		rw := runewidth.RuneWidth(r)
		if visibleWidth >= n {
			result.WriteRune(r)
		}
		visibleWidth += rw
	}

	return result.String()
}
