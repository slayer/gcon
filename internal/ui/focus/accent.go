package focus

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// accentBar is the blue vertical bar prepended to focused regions.
var accentBar = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#4285F4")).
	Render("│")

// RenderAccent prepends a blue │ bar to each line when focused.
// Unfocused content is returned with a single space prefix to maintain alignment.
func RenderAccent(content string, focused bool) string {
	lines := strings.Split(content, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if focused {
			b.WriteString(accentBar)
		} else {
			b.WriteByte(' ')
		}
		b.WriteString(line)
	}
	return b.String()
}
