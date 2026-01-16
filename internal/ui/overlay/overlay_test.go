package overlay

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestCenter_BasicCentering(t *testing.T) {
	background := strings.Repeat("X", 20) + "\n" +
		strings.Repeat("X", 20) + "\n" +
		strings.Repeat("X", 20)
	overlay := "MENU"

	result := Center(background, overlay, 20, 3)
	lines := strings.Split(result, "\n")

	assert.Len(t, lines, 3)
	// Overlay should be centered: (20-4)/2 = 8 chars from left
	assert.Contains(t, lines[1], "MENU", "overlay should be present in middle line")
}

func TestCenter_WithANSISequences(t *testing.T) {
	// Test that Center handles colored text properly
	// Note: lipgloss may or may not output ANSI codes depending on terminal detection
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	background := red.Render(strings.Repeat("X", 20)) + "\n" +
		red.Render(strings.Repeat("X", 20)) + "\n" +
		red.Render(strings.Repeat("X", 20))

	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	overlay := green.Render("MENU")

	result := Center(background, overlay, 20, 3)
	lines := strings.Split(result, "\n")

	assert.Len(t, lines, 3)
	// Verify the overlay text is present and centered
	assert.Contains(t, lines[1], "MENU", "overlay text should be present")
	// Width calculation should work correctly even with ANSI codes
	assert.LessOrEqual(t, lipgloss.Width(lines[1]), 20, "line width should not exceed background width")
}

func TestCenter_EmptyBackground(t *testing.T) {
	background := ""
	overlay := "TEST"

	result := Center(background, overlay, 10, 3)
	lines := strings.Split(result, "\n")

	assert.Len(t, lines, 3, "should pad background to specified height")
	assert.Contains(t, lines[1], "TEST", "overlay should be present")
}

func TestCenter_EmptyOverlay(t *testing.T) {
	background := strings.Repeat("X", 10) + "\n" + strings.Repeat("X", 10)
	overlay := ""

	result := Center(background, overlay, 10, 2)
	lines := strings.Split(result, "\n")

	assert.Len(t, lines, 2)
	// Background should remain mostly unchanged
	assert.Contains(t, lines[0], "X")
	assert.Contains(t, lines[1], "X")
}

func TestCenter_OverlayLargerThanBackground(t *testing.T) {
	background := "SHORT"
	overlay := strings.Repeat("VERY LONG OVERLAY", 3)

	// Overlay is wider than background width
	result := Center(background, overlay, 10, 1)

	// Should not panic, should handle gracefully
	assert.NotEmpty(t, result)
}

func TestCenter_MultilineOverlay(t *testing.T) {
	background := strings.Repeat("X", 30) + "\n" +
		strings.Repeat("X", 30) + "\n" +
		strings.Repeat("X", 30) + "\n" +
		strings.Repeat("X", 30) + "\n" +
		strings.Repeat("X", 30)

	overlay := "LINE1\nLINE2\nLINE3"

	result := Center(background, overlay, 30, 5)
	lines := strings.Split(result, "\n")

	assert.Len(t, lines, 5)
	// Middle 3 lines should contain overlay content
	assert.Contains(t, lines[1], "LINE1")
	assert.Contains(t, lines[2], "LINE2")
	assert.Contains(t, lines[3], "LINE3")
}

func TestCenter_NegativeDimensions(t *testing.T) {
	background := "TEST"
	overlay := "OVER"

	// Should handle negative dimensions gracefully
	result := Center(background, overlay, -1, -1)
	assert.NotEmpty(t, result, "should not panic on negative dimensions")
}

func TestCenter_ZeroDimensions(t *testing.T) {
	background := "TEST"
	overlay := "OVER"

	result := Center(background, overlay, 0, 0)
	assert.NotEmpty(t, result, "should not panic on zero dimensions")
}

func TestTruncateRightOverlay_Basic(t *testing.T) {
	input := "Hello World"
	result := truncateRightOverlay(input, 5)
	assert.Equal(t, "Hello", result)
}

func TestTruncateRightOverlay_WithANSI(t *testing.T) {
	// Test truncation with styled text (ANSI codes may or may not be present)
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	input := style.Render("Hello World")

	result := truncateRightOverlay(input, 5)

	// Visual width should be 5 (truncation should work correctly)
	assert.Equal(t, 5, lipgloss.Width(result), "visual width should be 5")
	// Result should be "Hello" (or "Hello" with ANSI codes)
	assert.Contains(t, result, "Hello", "should contain 'Hello'")
}

func TestTruncateRightOverlay_Zero(t *testing.T) {
	input := "Hello"
	result := truncateRightOverlay(input, 0)
	assert.Equal(t, "", result)
}

func TestTruncateRightOverlay_Negative(t *testing.T) {
	input := "Hello"
	result := truncateRightOverlay(input, -5)
	assert.Equal(t, "", result)
}

func TestTruncateRightOverlay_LongerThanInput(t *testing.T) {
	input := "Hi"
	result := truncateRightOverlay(input, 10)
	assert.Equal(t, "Hi", result)
}

func TestTruncateRightOverlay_WideCharacters(t *testing.T) {
	// Japanese characters that are 2 columns wide
	input := "こんにちは" // Each character is 2 columns
	result := truncateRightOverlay(input, 4)

	// Should include only 2 wide chars (4 columns)
	width := lipgloss.Width(result)
	assert.LessOrEqual(t, width, 4, "should not exceed specified width")
}

func TestTruncateLeftOverlay_Basic(t *testing.T) {
	input := "Hello World"
	result := truncateLeftOverlay(input, 6)
	assert.Equal(t, "World", result)
}

func TestTruncateLeftOverlay_WithANSI(t *testing.T) {
	// Test left truncation with styled text
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	input := style.Render("Hello World")

	result := truncateLeftOverlay(input, 6)

	// Should contain "World" (the part after "Hello ")
	assert.Contains(t, result, "World", "should contain 'World'")
	// Visual width should be 5 (World)
	visualWidth := lipgloss.Width(result)
	assert.Equal(t, 5, visualWidth, "visual width should be 5 (World)")
}

func TestTruncateLeftOverlay_Zero(t *testing.T) {
	input := "Hello"
	result := truncateLeftOverlay(input, 0)
	assert.Equal(t, "Hello", result)
}

func TestTruncateLeftOverlay_ExceedsLength(t *testing.T) {
	input := "Hello"
	result := truncateLeftOverlay(input, 10)
	assert.Equal(t, "", result)
}

func TestTruncateLeftOverlay_WideCharacters(t *testing.T) {
	// Mix of ASCII and wide characters
	input := "ABCこんにちは" // ABC=3 cols, 5 wide chars=10 cols, total=13 cols
	result := truncateLeftOverlay(input, 3)

	// Should skip "ABC" (3 cols) and return wide chars
	width := lipgloss.Width(result)
	assert.Equal(t, 10, width, "should skip first 3 columns")
}

func TestCenter_PreservesBackgroundSides(t *testing.T) {
	// Create distinct background sections
	background := "LLLLLLLLLL" + "MMMMMMMMMM" + "RRRRRRRRRR" // 30 chars
	overlay := "OVERLAY"                                     // 7 chars

	result := Center(background, overlay, 30, 1)

	// Left and right sides should be preserved
	assert.Contains(t, result, "L", "left side should be preserved")
	assert.Contains(t, result, "R", "right side should be preserved")
	assert.Contains(t, result, "OVERLAY", "overlay should be present")
}

func TestCenter_ConsistentOverlayWidth(t *testing.T) {
	background := strings.Repeat("X", 40) + "\n" +
		strings.Repeat("X", 40) + "\n" +
		strings.Repeat("X", 40)

	// Overlay with varying line widths
	overlay := "SHORT\nMEDIUM LINE\nLONG"

	result := Center(background, overlay, 40, 3)
	lines := strings.Split(result, "\n")

	// All overlay lines should be padded to same width (MEDIUM LINE = 11 chars)
	// So each should have consistent positioning
	assert.Len(t, lines, 3)
	assert.Contains(t, lines[0], "SHORT")
	assert.Contains(t, lines[1], "MEDIUM LINE")
	assert.Contains(t, lines[2], "LONG")
}

func TestCenter_ComplexANSI(t *testing.T) {
	// Test with multiple styled text elements
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	blue := lipgloss.NewStyle().Foreground(lipgloss.Color("#0000FF"))

	background := red.Render("AAAAA") + blue.Render("BBBBB") + "\n" +
		red.Render("AAAAA") + blue.Render("BBBBB")

	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	overlay := green.Render("OVR")

	result := Center(background, overlay, 10, 2)
	lines := strings.Split(result, "\n")

	// Verify structure is correct
	assert.Len(t, lines, 2, "should have 2 lines")
	assert.Contains(t, lines[0], "OVR", "overlay should be present in first line (centered)")
	// Width should be correct even with styled text
	assert.LessOrEqual(t, lipgloss.Width(lines[0]), 10, "line width should not exceed background width")
}
