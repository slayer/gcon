package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/mouse"
)

const (
	// Standard padding for form views within the content area.
	formWidthPadding  = 4
	formHeightPadding = 4
)

// TableClickDelegate delegates Clickable interface methods to a table.Model.
// Embed in list views to avoid repeating identical 3-method delegation boilerplate.
type TableClickDelegate struct {
	Table *table.Model
}

func (d TableClickDelegate) UpdateRegions(offsetX, offsetY int) {
	if d.Table != nil {
		d.Table.UpdateRegions(offsetX, offsetY)
	}
}

func (d TableClickDelegate) GetRegions() []mouse.Region {
	if d.Table != nil {
		return d.Table.GetRegions()
	}
	return nil
}

func (d TableClickDelegate) HandleRegionClick(regionID string) tea.Cmd {
	if d.Table != nil {
		return d.Table.HandleRegionClick(regionID)
	}
	return nil
}

// renderLoading renders a standard loading message with a spinner.
func renderLoading(s spinner.Model, msg string) string {
	return fmt.Sprintf("\n  %s %s\n", s.View(), msg)
}

// renderSaving renders a saving/creating state with a GCP-blue styled message.
func renderSaving(s spinner.Model, msg string) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	return fmt.Sprintf("\n  %s %s\n", s.View(), style.Render(msg))
}

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

// truncateForSuffix truncates a name to fit within maxLen when suffix is added.
// GCP resource names have a 63 character limit.
func truncateForSuffix(name, suffix string, maxLen int) string {
	combined := name + suffix
	if len(combined) <= maxLen {
		return combined
	}
	maxNameLen := maxLen - len(suffix)
	if maxNameLen < 1 {
		maxNameLen = 1
	}
	return name[:maxNameLen] + suffix
}

// parseLabelsFromText parses labels from key=value text format.
// Used by creation views that accept labels as multiline text input.
func parseLabelsFromText(data any) map[string]string {
	labels := make(map[string]string)
	text, ok := data.(string)
	if !ok || text == "" {
		return labels
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" {
				labels[key] = value
			}
		}
	}
	return labels
}

// calculateStats computes average and peak values from metric data points.
func calculateStats(data []gcp.DataPoint) (avg, peak float64, peakTime time.Time) {
	if len(data) == 0 {
		return 0, 0, time.Time{}
	}

	sum := 0.0
	peak = data[0].Value
	peakTime = data[0].Timestamp

	for _, dp := range data {
		sum += dp.Value
		if dp.Value > peak {
			peak = dp.Value
			peakTime = dp.Timestamp
		}
	}

	avg = sum / float64(len(data))
	return avg, peak, peakTime
}

// formatDuration formats a time duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

// truncate truncates a string to maxLen runes, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
