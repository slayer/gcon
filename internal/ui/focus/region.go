// Package focus provides a unified focus management system for views with
// multiple interactive regions (tabs, links, viewport, etc.).
package focus

// RegionType identifies the behavior of a focusable area.
// Each type has specific key bindings that apply when focused.
type RegionType int

const (
	// RegionViewport scrolls with j/k keys
	RegionViewport RegionType = iota
	// RegionList navigates rows with j/k keys
	RegionList
	// RegionLinks navigates link items with j/k keys
	RegionLinks
	// RegionTabs switches tabs with h/l or 1-9 keys
	RegionTabs
	// RegionForm navigates form fields with j/k keys (future)
	RegionForm
	// RegionButtons navigates buttons with h/l keys (future)
	RegionButtons
)

// String returns a human-readable name for the region type.
func (r RegionType) String() string {
	switch r {
	case RegionViewport:
		return "viewport"
	case RegionList:
		return "list"
	case RegionLinks:
		return "links"
	case RegionTabs:
		return "tabs"
	case RegionForm:
		return "form"
	case RegionButtons:
		return "buttons"
	default:
		return "unknown"
	}
}

// Region represents a focusable area within a view.
type Region struct {
	// ID uniquely identifies this region within a view
	ID string
	// Type determines which keys are routed to this region
	Type RegionType
	// Label is shown in help text (e.g., "Disks" for a links region)
	Label string
	// Enabled indicates whether this region can receive focus.
	// Disabled regions are skipped during Tab cycling.
	Enabled bool
}

// NewRegion creates a new focusable region.
func NewRegion(id string, regionType RegionType, label string) Region {
	return Region{
		ID:      id,
		Type:    regionType,
		Label:   label,
		Enabled: true,
	}
}

// NewDisabledRegion creates a region that starts disabled.
// Useful for regions that become available after data loads.
func NewDisabledRegion(id string, regionType RegionType, label string) Region {
	return Region{
		ID:      id,
		Type:    regionType,
		Label:   label,
		Enabled: false,
	}
}
