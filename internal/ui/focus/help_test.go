package focus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelpForRegion(t *testing.T) {
	tests := []struct {
		name       string
		regionType RegionType
		label      string
		wantKeys   []string // Expected key bindings to contain these keys
	}{
		{
			name:       "viewport region",
			regionType: RegionViewport,
			label:      "",
			wantKeys:   []string{"j/k", "tab"},
		},
		{
			name:       "links region with label",
			regionType: RegionLinks,
			label:      "disk",
			wantKeys:   []string{"j/k", "enter", "tab"},
		},
		{
			name:       "links region without label",
			regionType: RegionLinks,
			label:      "",
			wantKeys:   []string{"j/k", "enter", "tab"},
		},
		{
			name:       "tabs region",
			regionType: RegionTabs,
			label:      "",
			wantKeys:   []string{"h/l", "1-9", "tab"},
		},
		{
			name:       "list region",
			regionType: RegionList,
			label:      "item",
			wantKeys:   []string{"j/k", "enter", "tab"},
		},
		{
			name:       "form region",
			regionType: RegionForm,
			label:      "",
			wantKeys:   []string{"j/k", "enter", "tab"},
		},
		{
			name:       "buttons region",
			regionType: RegionButtons,
			label:      "",
			wantKeys:   []string{"h/l", "enter", "tab"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bindings := HelpForRegion(tt.regionType, tt.label)
			assert.NotEmpty(t, bindings)

			// Check that expected keys are present
			foundKeys := make(map[string]bool)
			for _, b := range bindings {
				foundKeys[b.Key] = true
			}

			for _, wantKey := range tt.wantKeys {
				assert.True(t, foundKeys[wantKey], "expected key %s to be present", wantKey)
			}
		})
	}
}

func TestHelpForRegionLinksLabel(t *testing.T) {
	// Test that custom label is used in description
	bindings := HelpForRegion(RegionLinks, "disk")
	var foundSelectDisk bool
	for _, b := range bindings {
		if b.Key == "j/k" && b.Desc == "select disk" {
			foundSelectDisk = true
		}
	}
	assert.True(t, foundSelectDisk, "expected 'select disk' description")

	// Test default label when empty
	bindings = HelpForRegion(RegionLinks, "")
	var foundSelectItem bool
	for _, b := range bindings {
		if b.Key == "j/k" && b.Desc == "select item" {
			foundSelectItem = true
		}
	}
	assert.True(t, foundSelectItem, "expected 'select item' description when label is empty")
}

func TestFormatHelp(t *testing.T) {
	tests := []struct {
		name     string
		bindings []HelpBinding
		want     string
	}{
		{
			name:     "empty bindings",
			bindings: []HelpBinding{},
			want:     "",
		},
		{
			name: "single binding",
			bindings: []HelpBinding{
				{Key: "j/k", Desc: "scroll"},
			},
			want: "j/k: scroll",
		},
		{
			name: "multiple bindings",
			bindings: []HelpBinding{
				{Key: "j/k", Desc: "scroll"},
				{Key: "tab", Desc: "next region"},
			},
			want: "j/k: scroll • tab: next region",
		},
		{
			name: "three bindings",
			bindings: []HelpBinding{
				{Key: "h/l", Desc: "switch tab"},
				{Key: "1-9", Desc: "go to tab"},
				{Key: "tab", Desc: "next region"},
			},
			want: "h/l: switch tab • 1-9: go to tab • tab: next region",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatHelp(tt.bindings)
			assert.Equal(t, tt.want, got)
		})
	}
}
