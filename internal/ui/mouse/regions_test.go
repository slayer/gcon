package mouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRect_Contains(t *testing.T) {
	tests := []struct {
		name     string
		rect     Rect
		x, y     int
		expected bool
	}{
		{
			name:     "point inside rectangle",
			rect:     Rect{X: 10, Y: 20, Width: 30, Height: 40},
			x:        25,
			y:        35,
			expected: true,
		},
		{
			name:     "point at top-left corner (inclusive)",
			rect:     Rect{X: 10, Y: 20, Width: 30, Height: 40},
			x:        10,
			y:        20,
			expected: true,
		},
		{
			name:     "point at bottom-right corner (exclusive)",
			rect:     Rect{X: 10, Y: 20, Width: 30, Height: 40},
			x:        40,
			y:        60,
			expected: false,
		},
		{
			name:     "point one pixel before right edge",
			rect:     Rect{X: 10, Y: 20, Width: 30, Height: 40},
			x:        39,
			y:        35,
			expected: true,
		},
		{
			name:     "point one pixel before bottom edge",
			rect:     Rect{X: 10, Y: 20, Width: 30, Height: 40},
			x:        25,
			y:        59,
			expected: true,
		},
		{
			name:     "point left of rectangle",
			rect:     Rect{X: 10, Y: 20, Width: 30, Height: 40},
			x:        5,
			y:        35,
			expected: false,
		},
		{
			name:     "point right of rectangle",
			rect:     Rect{X: 10, Y: 20, Width: 30, Height: 40},
			x:        45,
			y:        35,
			expected: false,
		},
		{
			name:     "point above rectangle",
			rect:     Rect{X: 10, Y: 20, Width: 30, Height: 40},
			x:        25,
			y:        15,
			expected: false,
		},
		{
			name:     "point below rectangle",
			rect:     Rect{X: 10, Y: 20, Width: 30, Height: 40},
			x:        25,
			y:        65,
			expected: false,
		},
		{
			name:     "zero-width rectangle",
			rect:     Rect{X: 10, Y: 20, Width: 0, Height: 40},
			x:        10,
			y:        35,
			expected: false,
		},
		{
			name:     "zero-height rectangle",
			rect:     Rect{X: 10, Y: 20, Width: 30, Height: 0},
			x:        25,
			y:        20,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rect.Contains(tt.x, tt.y)
			assert.Equal(t, tt.expected, result, "Rect.Contains(%d, %d) = %v, want %v", tt.x, tt.y, result, tt.expected)
		})
	}
}

func TestRegionManager_AddAndCount(t *testing.T) {
	rm := NewRegionManager()
	assert.Equal(t, 0, rm.Count(), "New RegionManager should have 0 regions")

	rm.Add("region-1", Rect{X: 0, Y: 0, Width: 10, Height: 10}, nil)
	assert.Equal(t, 1, rm.Count(), "After adding 1 region, count should be 1")

	rm.Add("region-2", Rect{X: 10, Y: 0, Width: 10, Height: 10}, nil)
	assert.Equal(t, 2, rm.Count(), "After adding 2 regions, count should be 2")

	rm.Add("region-3", Rect{X: 0, Y: 10, Width: 10, Height: 10}, 42)
	assert.Equal(t, 3, rm.Count(), "After adding 3 regions, count should be 3")
}

func TestRegionManager_Clear(t *testing.T) {
	rm := NewRegionManager()
	rm.Add("region-1", Rect{X: 0, Y: 0, Width: 10, Height: 10}, nil)
	rm.Add("region-2", Rect{X: 10, Y: 0, Width: 10, Height: 10}, nil)
	assert.Equal(t, 2, rm.Count(), "Should have 2 regions before clear")

	rm.Clear()
	assert.Equal(t, 0, rm.Count(), "After clear, count should be 0")

	// Verify we can add regions after clearing
	rm.Add("region-3", Rect{X: 0, Y: 0, Width: 10, Height: 10}, nil)
	assert.Equal(t, 1, rm.Count(), "After clear and add, count should be 1")
}

func TestRegionManager_FindRegion(t *testing.T) {
	rm := NewRegionManager()
	rm.Add("row-0", Rect{X: 0, Y: 0, Width: 100, Height: 1}, 0)
	rm.Add("row-1", Rect{X: 0, Y: 1, Width: 100, Height: 1}, 1)
	rm.Add("row-2", Rect{X: 0, Y: 2, Width: 100, Height: 1}, 2)

	tests := []struct {
		name          string
		x, y          int
		expectedID    string
		expectedFound bool
		expectedData  any
	}{
		{
			name:          "point in first region",
			x:             50,
			y:             0,
			expectedID:    "row-0",
			expectedFound: true,
			expectedData:  0,
		},
		{
			name:          "point in second region",
			x:             25,
			y:             1,
			expectedID:    "row-1",
			expectedFound: true,
			expectedData:  1,
		},
		{
			name:          "point in third region",
			x:             75,
			y:             2,
			expectedID:    "row-2",
			expectedFound: true,
			expectedData:  2,
		},
		{
			name:          "point outside all regions (below)",
			x:             50,
			y:             5,
			expectedFound: false,
		},
		{
			name:          "point outside all regions (right)",
			x:             150,
			y:             1,
			expectedFound: false,
		},
		{
			name:          "point outside all regions (above)",
			x:             50,
			y:             -1,
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			region := rm.FindRegion(tt.x, tt.y)
			if tt.expectedFound {
				assert.NotNil(t, region, "FindRegion(%d, %d) should find a region", tt.x, tt.y)
				assert.Equal(t, tt.expectedID, region.ID, "Found region ID should match expected")
				assert.Equal(t, tt.expectedData, region.Data, "Found region data should match expected")
			} else {
				assert.Nil(t, region, "FindRegion(%d, %d) should not find a region", tt.x, tt.y)
			}
		})
	}
}

func TestRegionManager_FindRegion_OverlappingRegions(t *testing.T) {
	// When regions overlap, FindRegion should return the first matching region
	rm := NewRegionManager()
	rm.Add("large", Rect{X: 0, Y: 0, Width: 100, Height: 100}, "large-data")
	rm.Add("small", Rect{X: 10, Y: 10, Width: 20, Height: 20}, "small-data")

	// Point in small region (which overlaps with large)
	region := rm.FindRegion(15, 15)
	assert.NotNil(t, region)
	// Should return the first one added (large)
	assert.Equal(t, "large", region.ID)
	assert.Equal(t, "large-data", region.Data)
}

func TestRegionManager_GetRegions(t *testing.T) {
	rm := NewRegionManager()
	rm.Add("region-1", Rect{X: 0, Y: 0, Width: 10, Height: 10}, 1)
	rm.Add("region-2", Rect{X: 10, Y: 0, Width: 10, Height: 10}, 2)
	rm.Add("region-3", Rect{X: 0, Y: 10, Width: 10, Height: 10}, 3)

	regions := rm.GetRegions()
	assert.Equal(t, 3, len(regions), "GetRegions should return all regions")

	// Verify region IDs
	assert.Equal(t, "region-1", regions[0].ID)
	assert.Equal(t, "region-2", regions[1].ID)
	assert.Equal(t, "region-3", regions[2].ID)

	// Verify region data
	assert.Equal(t, 1, regions[0].Data)
	assert.Equal(t, 2, regions[1].Data)
	assert.Equal(t, 3, regions[2].Data)

	// Verify bounds
	assert.Equal(t, Rect{X: 0, Y: 0, Width: 10, Height: 10}, regions[0].Bounds)
	assert.Equal(t, Rect{X: 10, Y: 0, Width: 10, Height: 10}, regions[1].Bounds)
	assert.Equal(t, Rect{X: 0, Y: 10, Width: 10, Height: 10}, regions[2].Bounds)
}

func TestRegionManager_EmptyManager(t *testing.T) {
	rm := NewRegionManager()

	assert.Equal(t, 0, rm.Count(), "Empty manager should have count 0")
	assert.Nil(t, rm.FindRegion(0, 0), "FindRegion should return nil on empty manager")
	assert.Empty(t, rm.GetRegions(), "GetRegions should return empty slice on empty manager")
}

func TestRegionManager_MultipleClears(t *testing.T) {
	rm := NewRegionManager()

	// Add, clear, add, clear multiple times
	rm.Add("r1", Rect{X: 0, Y: 0, Width: 10, Height: 10}, nil)
	assert.Equal(t, 1, rm.Count())

	rm.Clear()
	assert.Equal(t, 0, rm.Count())

	rm.Add("r2", Rect{X: 0, Y: 0, Width: 10, Height: 10}, nil)
	rm.Add("r3", Rect{X: 10, Y: 0, Width: 10, Height: 10}, nil)
	assert.Equal(t, 2, rm.Count())

	rm.Clear()
	assert.Equal(t, 0, rm.Count())

	rm.Add("r4", Rect{X: 0, Y: 0, Width: 10, Height: 10}, nil)
	assert.Equal(t, 1, rm.Count())
}
