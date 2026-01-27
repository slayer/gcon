package views

import "github.com/charmbracelet/lipgloss"

// renderRow formats a label-value pair for display in detail views.
// Empty or placeholder values are rendered in muted style.
func renderRow(labelStyle, valueStyle, mutedStyle lipgloss.Style, label, value string) string { //nolint:gocritic // Style params acceptable
	if value == "" || value == "None" || value == "—" {
		return labelStyle.Render(label+":") + " " + mutedStyle.Render(value) + "\n"
	}
	return labelStyle.Render(label+":") + " " + valueStyle.Render(value) + "\n"
}

// defaultIfEmpty returns the default value if the string is empty.
func defaultIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
