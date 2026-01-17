package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	barFilledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")) // Green
	barEmptyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368")) // Gray
)

// RenderMetricBar renders a horizontal bar chart for a metric
// Returns a multi-line string with the bar and stats
func RenderMetricBar(label string, currentValue, avgValue, peakValue float64, peakTime time.Time, width int) string {
	// Ensure width is reasonable
	if width < 20 {
		width = 20
	}

	barWidth := width - 10 // Leave space for percentage label
	if barWidth > 60 {
		barWidth = 60
	}

	// Calculate filled portion (percentage of bar width)
	percentage := currentValue
	if percentage > 100 {
		percentage = 100
	}
	if percentage < 0 {
		percentage = 0
	}

	filledCount := int((percentage / 100.0) * float64(barWidth))
	emptyCount := barWidth - filledCount

	// Build bar string
	filled := strings.Repeat("█", filledCount)
	empty := strings.Repeat("░", emptyCount)

	bar := barFilledStyle.Render(filled) + barEmptyStyle.Render(empty)

	// Format peak time
	peakTimeStr := ""
	if !peakTime.IsZero() {
		peakTimeStr = fmt.Sprintf(" (%s)", peakTime.Format("3:04 PM"))
	}

	// Build stats line
	stats := fmt.Sprintf("     Current: %.0f%%  |  Avg: %.0f%%  |  Peak: %.0f%%%s",
		currentValue, avgValue, peakValue, peakTimeStr)

	return fmt.Sprintf("%s\n%s", bar, stats)
}

// RenderMetricBarWithLabel renders a metric bar with a header label
func RenderMetricBarWithLabel(label string, currentValue, avgValue, peakValue float64, peakTime time.Time, width int) string {
	separator := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	if width > len(separator) {
		separator = strings.Repeat("━", width)
	}

	bar := RenderMetricBar(label, currentValue, avgValue, peakValue, peakTime, width)

	return fmt.Sprintf("%s\n%s\n  %s", label, separator, bar)
}
