package mouse

// Rect represents a rectangular area in screen coordinates
type Rect struct {
	X, Y, Width, Height int
}

// Contains checks if the given point (x, y) is within the rectangle
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width &&
		y >= r.Y && y < r.Y+r.Height
}

// Region represents a clickable area with an identifier
type Region struct {
	ID     string // "row-0", "tab-overview", "sidebar-item-1"
	Bounds Rect   // Screen-absolute coordinates
	Data   any    // Optional metadata (row index, tab ID, etc.)
}

// RegionManager tracks clickable regions for a component
type RegionManager struct {
	regions []Region
}

// NewRegionManager creates a new region manager
func NewRegionManager() *RegionManager {
	return &RegionManager{regions: []Region{}}
}

// Clear removes all tracked regions
func (rm *RegionManager) Clear() {
	rm.regions = rm.regions[:0]
}

// Add adds a new clickable region
func (rm *RegionManager) Add(id string, bounds Rect, data any) {
	rm.regions = append(rm.regions, Region{ID: id, Bounds: bounds, Data: data})
}

// FindRegion finds the first region that contains the given point
// Returns nil if no region contains the point
func (rm *RegionManager) FindRegion(x, y int) *Region {
	for i := range rm.regions {
		if rm.regions[i].Bounds.Contains(x, y) {
			return &rm.regions[i]
		}
	}
	return nil
}

// GetRegions returns all tracked regions
func (rm *RegionManager) GetRegions() []Region {
	return rm.regions
}

// Count returns the number of tracked regions
func (rm *RegionManager) Count() int {
	return len(rm.regions)
}
