package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/ui/mouse"
)

// Clickable interface for components that handle mouse clicks
type Clickable interface {
	// UpdateRegions recalculates clickable regions based on current state
	// offsetX, offsetY are the component's top-left corner in screen coordinates
	// Called after View() but before mouse event processing
	UpdateRegions(offsetX, offsetY int)

	// GetRegions returns current clickable regions
	GetRegions() []mouse.Region

	// HandleRegionClick processes a click on a specific region
	// regionID identifies which region was clicked
	// Returns a tea.Cmd to execute in response to the click
	HandleRegionClick(regionID string) tea.Cmd
}
