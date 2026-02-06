package focus

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderAccent_Focused(t *testing.T) {
	content := "line one\nline two\nline three"
	result := RenderAccent(content, true)

	lines := strings.Split(result, "\n")
	assert.Len(t, lines, 3)
	// Each line should start with the accent bar (│ rendered with blue color)
	for _, line := range lines {
		assert.Contains(t, line, "│", "focused line should contain accent bar")
	}
}

func TestRenderAccent_Unfocused(t *testing.T) {
	content := "line one\nline two"
	result := RenderAccent(content, false)

	lines := strings.Split(result, "\n")
	assert.Len(t, lines, 2)
	for _, line := range lines {
		// Unfocused lines get a space prefix for alignment
		assert.True(t, strings.HasPrefix(line, " "), "unfocused line should start with space")
		assert.NotContains(t, line, "│", "unfocused line should not contain accent bar")
	}
}

func TestRenderAccent_SingleLine(t *testing.T) {
	result := RenderAccent("hello", true)
	assert.Contains(t, result, "│")
	assert.Contains(t, result, "hello")
	// Single line — no newlines in output
	assert.Equal(t, 0, strings.Count(result, "\n"))
}

func TestRenderAccent_EmptyContent(t *testing.T) {
	result := RenderAccent("", true)
	assert.Contains(t, result, "│")

	result = RenderAccent("", false)
	assert.Equal(t, " ", result)
}
