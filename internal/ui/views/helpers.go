package views

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
