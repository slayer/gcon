package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestNewFooter(t *testing.T) {
	f := NewFooter()

	assert.NotNil(t, f)
	assert.Equal(t, 0, f.width)
	// All slots should be nil initially
	assert.Nil(t, f.Left1)
	assert.Nil(t, f.Left2)
	assert.Nil(t, f.Left3)
	assert.Nil(t, f.Center1)
	assert.Nil(t, f.Right1)
	assert.Nil(t, f.Right2)
	assert.Nil(t, f.Right3)
}

func TestFooter_SetWidth(t *testing.T) {
	f := NewFooter()
	f.SetWidth(100)
	assert.Equal(t, 100, f.width)
}

func TestFooter_SetAndClearSlots(t *testing.T) {
	f := NewFooter()

	// Set slots
	f.SetLeft1("left1")
	f.SetLeft2("left2")
	f.SetLeft3("left3")
	f.SetCenter1("center1")
	f.SetRight1("right1")

	assert.NotNil(t, f.Left1)
	assert.Equal(t, "left1", *f.Left1)
	assert.NotNil(t, f.Left2)
	assert.Equal(t, "left2", *f.Left2)
	assert.NotNil(t, f.Left3)
	assert.Equal(t, "left3", *f.Left3)
	assert.NotNil(t, f.Center1)
	assert.Equal(t, "center1", *f.Center1)
	assert.NotNil(t, f.Right1)
	assert.Equal(t, "right1", *f.Right1)

	// Clear slots
	f.ClearLeft1()
	f.ClearLeft2()
	f.ClearLeft3()
	f.ClearCenter()
	f.ClearRight1()

	assert.Nil(t, f.Left1)
	assert.Nil(t, f.Left2)
	assert.Nil(t, f.Left3)
	assert.Nil(t, f.Center1)
	assert.Nil(t, f.Right1)
}

func TestFooter_View_EmptyWidth(t *testing.T) {
	f := NewFooter()
	// Width is 0, should return empty string
	assert.Equal(t, "", f.View())
}

func TestFooter_View_LeftGroupOnly(t *testing.T) {
	f := NewFooter()
	f.SetWidth(80)
	f.SetLeft1("nav")
	f.SetLeft2("mode")

	view := f.View()

	// Should contain the slot values
	assert.Contains(t, view, "nav")
	assert.Contains(t, view, "mode")
	// Should contain powerline separator
	assert.Contains(t, view, SepRight)
}

func TestFooter_View_RightGroupOnly(t *testing.T) {
	f := NewFooter()
	f.SetWidth(80)
	f.SetRight1("project")

	view := f.View()

	// Should contain the slot value
	assert.Contains(t, view, "project")
	// Should contain powerline separator (left-pointing for right group)
	assert.Contains(t, view, SepLeft)
}

func TestFooter_View_AllGroups(t *testing.T) {
	f := NewFooter()
	f.SetWidth(120)
	f.SetLeft1("back")
	f.SetLeft2("sidebar")
	f.SetLeft3("help")
	f.SetCenter1("status")
	f.SetRight1("my-project")
	f.SetRight2("task")

	view := f.View()

	// All slot values should be present
	assert.Contains(t, view, "back")
	assert.Contains(t, view, "sidebar")
	assert.Contains(t, view, "help")
	assert.Contains(t, view, "status")
	assert.Contains(t, view, "my-project")
	assert.Contains(t, view, "task")
}

func TestFooter_View_StyledRight(t *testing.T) {
	f := NewFooter()
	f.SetWidth(80)

	// Pre-rendered styled content (simulating task status)
	styledContent := lipgloss.NewStyle().
		Background(lipgloss.Color("#34A853")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1).
		Render("✓ Done")

	f.SetRight2Styled(styledContent, lipgloss.Color("#34A853"))

	view := f.View()

	// The styled content should be included
	assert.Contains(t, view, "Done")
}

func TestFooter_View_WidthRespected(t *testing.T) {
	f := NewFooter()
	f.SetWidth(60)
	f.SetLeft1("a")
	f.SetRight1("b")

	view := f.View()

	// The rendered width should not exceed the set width
	// (accounting for ANSI codes, we check lipgloss.Width)
	viewWidth := lipgloss.Width(view)
	assert.LessOrEqual(t, viewWidth, 60, "Footer width should not exceed set width")
}

func TestFooter_PowerlineSeparators(t *testing.T) {
	// Verify powerline constants are defined
	assert.NotEmpty(t, SepRight)
	assert.NotEmpty(t, SepRightThin)
	assert.NotEmpty(t, SepLeft)
	assert.NotEmpty(t, SepLeftThin)
}

func TestFooter_View_CenterGroupWithThinSeparators(t *testing.T) {
	f := NewFooter()
	f.SetWidth(100)
	f.SetCenter1("a")
	f.SetCenter2("b")
	f.SetCenter3("c")

	view := f.View()

	// Center group items should be separated by thin separators
	assert.Contains(t, view, SepRightThin)
}

func TestFooter_View_NoOverlap(t *testing.T) {
	f := NewFooter()
	f.SetWidth(80)
	f.SetLeft1("left-content")
	f.SetRight1("right-content")

	view := f.View()

	// Both should be present (no overlap causing truncation)
	assert.Contains(t, view, "left-content")
	assert.Contains(t, view, "right-content")
}

func TestFooter_View_SingleLine(t *testing.T) {
	f := NewFooter()
	f.SetWidth(100)
	f.SetLeft1("nav")
	f.SetLeft2("mode")
	f.SetRight1("project")
	f.SetRight2("status")

	view := f.View()

	// Footer should be a single line (no newlines)
	newlineCount := strings.Count(view, "\n")
	assert.Equal(t, 0, newlineCount, "Footer should be single line")
}

func TestFooter_terminalWidth(t *testing.T) {
	f := NewFooter()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty string", "", 0},
		{"plain text", "hello", 5},
		{"text with powerline right", "hello" + SepRight + "world", 12}, // 5+1+5 + 1 extra for powerline
		{"multiple powerlines", SepRight + SepLeft, 4},                  // 2 chars + 2 extra
		{"all powerline types", SepRight + SepRightThin + SepLeft + SepLeftThin, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.terminalWidth(tt.input)
			assert.Equal(t, tt.expected, got, "terminalWidth mismatch for %q", tt.input)
		})
	}
}

func TestFooter_View_TerminalWidthNotExceeded(t *testing.T) {
	// Test that footer doesn't exceed terminal width when accounting for powerline symbols
	widths := []int{60, 80, 100, 120, 200}

	for _, width := range widths {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			f := NewFooter()
			f.SetWidth(width)
			f.SetLeft1("esc back")
			f.SetLeft2("[ sidebar")
			f.SetLeft3(": cmd • ? help • q quit")
			f.SetRight1("my-test-project")
			f.SetRight2("Loading...")

			view := f.View()

			// Calculate actual terminal width
			termWidth := f.terminalWidth(view)

			assert.LessOrEqual(t, termWidth, width,
				"Footer terminal width %d exceeds set width %d", termWidth, width)
		})
	}
}

func TestFooter_View_NarrowWidth(t *testing.T) {
	// Test that footer handles very narrow widths gracefully
	f := NewFooter()
	f.SetWidth(30)
	f.SetLeft1("back")
	f.SetRight1("project")

	view := f.View()

	// Should not panic and should produce output
	assert.NotEmpty(t, view)
	// Should still be single line
	assert.Equal(t, 0, strings.Count(view, "\n"))
}

func TestFooter_View_ExactWidth(t *testing.T) {
	// Test that footer fills exactly to the width (no overflow, no underflow)
	f := NewFooter()
	width := 80
	f.SetWidth(width)
	f.SetLeft1("a")

	view := f.View()
	termWidth := f.terminalWidth(view)

	// The footer should fill exactly to width (accounting for powerline extra width)
	assert.Equal(t, width, termWidth,
		"Footer should fill exactly to width %d, got %d", width, termWidth)
}

func TestFooter_truncateToWidth(t *testing.T) {
	f := NewFooter()

	tests := []struct {
		name     string
		input    string
		maxWidth int
		maxLen   int // expected max terminal width
	}{
		{"plain text fits", "hello", 10, 5},
		{"plain text truncated", "hello world", 5, 5},
		{"powerline counted correctly", "ab" + SepRight + "cd", 5, 4}, // ab + separator(2) = 4
		{"empty string", "", 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.truncateToWidth(tt.input, tt.maxWidth)
			termWidth := f.terminalWidth(result)
			assert.LessOrEqual(t, termWidth, tt.maxWidth,
				"truncated width %d exceeds max %d", termWidth, tt.maxWidth)
		})
	}
}

func TestFooter_View_CenterDroppedWhenOverflow(t *testing.T) {
	// When left+center+right exceeds width, center should be dropped first
	f := NewFooter()
	f.SetWidth(50)
	f.SetLeft1("left content here")
	f.SetCenter1("center content")
	f.SetRight1("right content here")

	view := f.View()

	// Center should be dropped when there's overflow
	assert.NotContains(t, view, "center content",
		"Center should be dropped when content exceeds width")
	// Left and right should still be present (possibly truncated)
	// We just verify width doesn't exceed
	assert.LessOrEqual(t, f.terminalWidth(view), 50)
}

func TestFooter_View_PrerenderedTaskStatus(t *testing.T) {
	// Test that pre-rendered task status works correctly
	f := NewFooter()
	f.SetWidth(100)
	f.SetLeft1("nav")

	// Simulate task status with ANSI colors
	taskStatus := lipgloss.NewStyle().
		Background(lipgloss.Color("#4285F4")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1).
		Render("⠋ Loading instances...")

	f.SetRight2Styled(taskStatus, lipgloss.Color("#4285F4"))

	view := f.View()

	// Task status text should be present
	assert.Contains(t, view, "Loading instances")
	// Should be single line
	assert.Equal(t, 0, strings.Count(view, "\n"))
	// Should not exceed width
	assert.LessOrEqual(t, f.terminalWidth(view), 100)
}

func TestFooter_View_MultipleRightSections(t *testing.T) {
	f := NewFooter()
	f.SetWidth(120)
	f.SetRight1("project-id")
	f.SetRight2("status")
	f.SetRight3("extra")

	view := f.View()

	// All right sections should be present
	assert.Contains(t, view, "project-id")
	assert.Contains(t, view, "status")
	assert.Contains(t, view, "extra")
}

func TestFooter_View_OnlyRightSections(t *testing.T) {
	// Test footer with only right sections (no left, no center)
	f := NewFooter()
	f.SetWidth(80)
	f.SetRight1("only-right")

	view := f.View()

	assert.Contains(t, view, "only-right")
	assert.Equal(t, 80, f.terminalWidth(view))
}
