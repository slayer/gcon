package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColorFromString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"simple project", "my-project"},
		{"gcp project", "aytm-staging"},
		{"another project", "production-api-123"},
		{"numeric", "12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color := colorFromString(tt.input)

			// Should return a valid hex color
			assert.NotEmpty(t, string(color))
			assert.Regexp(t, `^#[0-9A-F]{6}$`, string(color),
				"Color should be a valid hex color")
		})
	}
}

func TestColorFromString_Consistency(t *testing.T) {
	// Same input should always produce the same color
	input := "my-project-id"
	color1 := colorFromString(input)
	color2 := colorFromString(input)

	assert.Equal(t, color1, color2, "Same input should produce same color")
}

func TestColorFromString_Uniqueness(t *testing.T) {
	// Different inputs should produce different colors (with high probability)
	inputs := []string{
		"project-a",
		"project-b",
		"project-c",
		"production",
		"staging",
		"development",
	}

	colors := make(map[string]bool)
	for _, input := range inputs {
		color := string(colorFromString(input))
		colors[color] = true
	}

	// With 6 distinct inputs, we expect at least 4 distinct colors
	// (allowing for some collision due to hash)
	assert.GreaterOrEqual(t, len(colors), 4,
		"Different inputs should produce mostly different colors")
}

func TestHueToRGB(t *testing.T) {
	// Test edge cases
	tests := []struct {
		name    string
		p, q, t float64
	}{
		{"t < 0", 0.2, 0.8, -0.1},
		{"t > 1", 0.2, 0.8, 1.1},
		{"t < 1/6", 0.2, 0.8, 0.1},
		{"t < 1/2", 0.2, 0.8, 0.3},
		{"t < 2/3", 0.2, 0.8, 0.6},
		{"t >= 2/3", 0.2, 0.8, 0.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hueToRGB(tt.p, tt.q, tt.t)
			// Result should be between 0 and 1
			assert.GreaterOrEqual(t, result, 0.0)
			assert.LessOrEqual(t, result, 1.0)
		})
	}
}
