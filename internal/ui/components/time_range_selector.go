package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	activeRangeStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4")) // Blue
	inactiveRangeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368"))            // Gray
)

// TimeRange represents a selectable time range
type TimeRange struct {
	Label    string
	Duration time.Duration
	Key      string
}

// PredefinedTimeRanges are the standard time ranges available
var PredefinedTimeRanges = []TimeRange{
	{Label: "1h", Duration: 1 * time.Hour, Key: "1"},
	{Label: "6h", Duration: 6 * time.Hour, Key: "2"},
	{Label: "24h", Duration: 24 * time.Hour, Key: "3"},
	{Label: "7d", Duration: 7 * 24 * time.Hour, Key: "4"},
	{Label: "30d", Duration: 30 * 24 * time.Hour, Key: "5"},
}

// RenderTimeRangeSelector renders a time range selector with highlighted active range
func RenderTimeRangeSelector(activeRange time.Duration, autoRefresh bool, lastUpdated time.Time) string {
	// Build range selector
	ranges := ""
	for i, tr := range PredefinedTimeRanges {
		var display string
		if tr.Duration == activeRange {
			display = activeRangeStyle.Render(fmt.Sprintf("[%s]", tr.Label))
		} else {
			display = inactiveRangeStyle.Render(fmt.Sprintf("[%s]", tr.Label))
		}

		ranges += display
		if i < len(PredefinedTimeRanges)-1 {
			ranges += " "
		}
	}

	// Format last updated time
	lastUpdatedStr := "Never"
	if !lastUpdated.IsZero() {
		lastUpdatedStr = lastUpdated.Format("3:04:05 PM")
	}

	// Auto-refresh status
	autoRefreshStr := "OFF"
	if autoRefresh {
		autoRefreshStr = "ON (15s)"
	}

	return fmt.Sprintf("%s  |  Last updated: %s  |  Auto-refresh: %s",
		ranges, lastUpdatedStr, autoRefreshStr)
}
