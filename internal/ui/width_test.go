package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTerminalWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // Expected terminal width (lipgloss width + miscounted emoji count)
	}{
		{
			name:     "plain ASCII",
			input:    "hello world",
			expected: 11,
		},
		{
			name:     "with cloud emoji",
			input:    "☁ gcon",
			expected: 7, // lipgloss: 6, miscounted emoji: 1
		},
		{
			name:     "with hamburger menu",
			input:    "≡ Menu",
			expected: 6, // ≡ (U+2261) is consistently 1-cell wide, no adjustment needed
		},
		{
			name:     "with status emoji green",
			input:    "🟢 RUNNING",
			expected: 10, // lipgloss already counts 🟢 as 2, so no adjustment needed
		},
		{
			name:     "with status emoji red",
			input:    "🔴 STOPPED",
			expected: 10, // lipgloss already counts 🔴 as 2, so no adjustment needed
		},
		{
			name:     "multiple emojis mixed",
			input:    "☁ gcon • ≡ Menu • 🟢",
			expected: 21, // lipgloss: 20, miscounted emojis: 1 (☁); ≡ is 1-wide
		},
		{
			name:     "sidebar arrows",
			input:    "▶ Instances ▸",
			expected: 15, // lipgloss: 13, miscounted emojis: 2 (▶, ▸)
		},
		{
			name:     "active indicator",
			input:    "● Active",
			expected: 9, // lipgloss: 8, miscounted emoji: 1 (●)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TerminalWidth(tt.input)
			assert.Equal(t, tt.expected, got, "TerminalWidth(%q)", tt.input)
		})
	}
}

func TestSafeWidth(t *testing.T) {
	tests := []struct {
		name          string
		terminalWidth int
		content       string
		expected      int
	}{
		{
			name:          "no emojis",
			terminalWidth: 100,
			content:       "plain text",
			expected:      100,
		},
		{
			name:          "one emoji",
			terminalWidth: 100,
			content:       "☁ gcon",
			expected:      99,
		},
		{
			name:          "multiple emojis on same line",
			terminalWidth: 100,
			content:       "☁ gcon • ≡ Menu • 🟢 RUNNING",
			expected:      99, // 100 - 1 miscounted emoji (☁); ≡ is 1-wide, 🟢 is already 2-wide in lipgloss
		},
		{
			name:          "multiline uses max per line",
			terminalWidth: 100,
			content:       "☁ header\n≡ ● menu\nplain text",
			expected:      99, // Line 2 has 1 emoji (●); ≡ is 1-wide. Line 1 has 1 emoji (☁). Max = 1
		},
		{
			name:          "multiline emoji count not summed",
			terminalWidth: 100,
			content:       "☁ line1\n≡ line2\n● line3", // ☁ and ● are miscounted, ≡ is fine
			expected:      99,                          // Only reduce by 1 (max per line)
		},
		{
			name:          "minimum width enforced",
			terminalWidth: 12,
			content:       "☁ ≡ ● ▶", // 3 miscounted emojis (☁, ●, ▶); ≡ is fine
			expected:      10,        // Would be 9, but min is 10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeWidth(tt.terminalWidth, tt.content)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSafeWidthForEmojis(t *testing.T) {
	assert.Equal(t, 97, SafeWidthForEmojis(100, 3))
	assert.Equal(t, 10, SafeWidthForEmojis(12, 6)) // min enforced
	assert.Equal(t, 100, SafeWidthForEmojis(100, 0))
}

func TestCountWideEmojis(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"hello", 0},
		{"☁", 1},
		{"☁ ≡", 1},   // ≡ is not a wide emoji
		{"🟢🔴🟡", 0},   // lipgloss already counts these correctly
		{"▶ ▸ ◀", 3}, // lipgloss miscounts these
		{"● active", 1},
		{"no emojis here", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := countWideEmojis(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestMaxLineWidth(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "single line",
			content:  "hello",
			expected: 5,
		},
		{
			name:     "multiple lines",
			content:  "hello\nworld!",
			expected: 6, // "world!" is longest
		},
		{
			name:     "line with emoji is wider",
			content:  "hello\n☁ gcon",
			expected: 7, // "☁ gcon" = 6 + 1 emoji = 7
		},
		{
			name:     "trailing newline",
			content:  "hello\n",
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxLineWidth(tt.content)
			assert.Equal(t, tt.expected, got)
		})
	}
}
