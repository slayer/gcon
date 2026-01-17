package focus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegionTypeString(t *testing.T) {
	tests := []struct {
		regionType RegionType
		want       string
	}{
		{RegionViewport, "viewport"},
		{RegionList, "list"},
		{RegionLinks, "links"},
		{RegionTabs, "tabs"},
		{RegionForm, "form"},
		{RegionButtons, "buttons"},
		{RegionType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.regionType.String())
		})
	}
}

func TestNewRegion(t *testing.T) {
	r := NewRegion("test-id", RegionTabs, "Test Label")

	assert.Equal(t, "test-id", r.ID)
	assert.Equal(t, RegionTabs, r.Type)
	assert.Equal(t, "Test Label", r.Label)
	assert.True(t, r.Enabled)
}

func TestNewDisabledRegion(t *testing.T) {
	r := NewDisabledRegion("disabled-id", RegionLinks, "Disabled Label")

	assert.Equal(t, "disabled-id", r.ID)
	assert.Equal(t, RegionLinks, r.Type)
	assert.Equal(t, "Disabled Label", r.Label)
	assert.False(t, r.Enabled)
}
