package focus

import "github.com/charmbracelet/lipgloss"

// HelpBinding represents a key binding shown in help text.
type HelpBinding struct {
	Key  string
	Desc string
}

// HelpForRegion returns context-sensitive help bindings for the given region type.
// The label parameter can customize the description (e.g., "disk" instead of "item").
func HelpForRegion(regionType RegionType, label string) []HelpBinding {
	if label == "" {
		label = "item"
	}

	switch regionType {
	case RegionViewport:
		return []HelpBinding{
			{Key: "j/k", Desc: "scroll"},
			{Key: "tab", Desc: "next region"},
		}
	case RegionList:
		return []HelpBinding{
			{Key: "j/k", Desc: "select " + label},
			{Key: "enter", Desc: "open"},
			{Key: "tab", Desc: "next region"},
		}
	case RegionLinks:
		return []HelpBinding{
			{Key: "j/k", Desc: "select " + label},
			{Key: "enter", Desc: "open"},
			{Key: "tab", Desc: "next region"},
		}
	case RegionTabs:
		return []HelpBinding{
			{Key: "h/l", Desc: "switch tab"},
			{Key: "1-9", Desc: "go to tab"},
			{Key: "tab", Desc: "next region"},
		}
	case RegionForm:
		return []HelpBinding{
			{Key: "j/k", Desc: "next field"},
			{Key: "enter", Desc: "edit"},
			{Key: "tab", Desc: "next region"},
		}
	case RegionButtons:
		return []HelpBinding{
			{Key: "h/l", Desc: "select button"},
			{Key: "enter", Desc: "press"},
			{Key: "tab", Desc: "next region"},
		}
	default:
		return []HelpBinding{
			{Key: "tab", Desc: "next region"},
		}
	}
}

// regionBadgeStyle renders the active region label in bold blue.
var regionBadgeStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#4285F4")).
	Bold(true)

// FormatRegionBadge returns a styled "▸ Label" badge for the active region.
// Returns empty string if region is nil or has no label.
func FormatRegionBadge(region *Region) string {
	if region == nil || region.Label == "" {
		return ""
	}
	return regionBadgeStyle.Render("▸ " + region.Label)
}

// FormatHelp formats help bindings as a single string for display.
// Format: "key: desc • key: desc"
func FormatHelp(bindings []HelpBinding) string {
	if len(bindings) == 0 {
		return ""
	}

	result := ""
	for i, b := range bindings {
		if i > 0 {
			result += " • "
		}
		result += b.Key + ": " + b.Desc
	}
	return result
}
