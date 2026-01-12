package views

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstancesView_RenderingHeightConsistency(t *testing.T) {
	// Create view with a known height
	v := NewInstancesView("test-project")
	v.SetSize(80, 30) // width=80, height=30

	// Get loading view output
	loadingOutput := v.View()
	loadingLines := strings.Count(loadingOutput, "\n")

	t.Logf("View height set to: %d", v.height)
	t.Logf("Loading output newlines: %d", loadingLines)
	t.Logf("Loading output:\n%s", loadingOutput)

	// Sidebar outputs height-1 newlines (lipgloss Height renders n lines = n-1 newlines)
	// Content must match for proper horizontal join
	assert.Equal(t, v.height-1, loadingLines,
		"Loading view should output height-1 newlines to match sidebar")
}

func TestInstancesView_RenderLoadingHeight(t *testing.T) {
	v := NewInstancesView("test-project")

	tests := []struct {
		name             string
		height           int
		expectedNewlines int
	}{
		{"normal height", 30, 29}, // height-1 newlines
		{"small height", 15, 14},  // height-1 newlines
		{"zero height uses minimum", 0, 10},
		{"negative uses minimum", -5, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v.SetSize(80, tt.height)
			output := v.renderLoading("Loading...")
			actualNewlines := strings.Count(output, "\n")

			t.Logf("Height: %d, Expected newlines: %d, Actual newlines: %d",
				tt.height, tt.expectedNewlines, actualNewlines)
			t.Logf("Output:\n%q", output)

			assert.Equal(t, tt.expectedNewlines, actualNewlines,
				"renderLoading should output exactly %d newlines", tt.expectedNewlines)
		})
	}
}
