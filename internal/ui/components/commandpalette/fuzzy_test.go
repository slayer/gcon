package commandpalette

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScore(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		text     string
		wantZero bool // true if we expect no match (score = 0)
	}{
		{
			name:  "exact match",
			query: "VM instances",
			text:  "VM instances",
		},
		{
			name:  "exact match case insensitive",
			query: "vm instances",
			text:  "VM instances",
		},
		{
			name:  "prefix match",
			query: "VM",
			text:  "VM instances",
		},
		{
			name:  "prefix match case insensitive",
			query: "vm",
			text:  "VM instances",
		},
		{
			name:  "word boundary match",
			query: "instances",
			text:  "VM instances",
		},
		{
			name:  "word boundary after colon",
			query: "vm",
			text:  "Compute Engine: VM instances",
		},
		{
			name:  "contains match",
			query: "nsta",
			text:  "VM instances",
		},
		{
			name:     "no match",
			query:    "xyz",
			text:     "VM instances",
			wantZero: true,
		},
		{
			name:  "empty query matches everything",
			query: "",
			text:  "VM instances",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := Score(tt.query, tt.text)
			if tt.wantZero {
				assert.Equal(t, 0, score, "expected no match")
			} else {
				assert.Greater(t, score, 0, "expected match")
			}
		})
	}
}

func TestScoreRanking(t *testing.T) {
	text := "Compute Engine: VM instances"

	tests := []struct {
		name         string
		betterQuery  string
		worseQuery   string
		expectBetter bool
	}{
		{
			name:         "exact beats prefix",
			betterQuery:  "Compute Engine: VM instances",
			worseQuery:   "Compute",
			expectBetter: true,
		},
		{
			name:         "prefix beats word boundary",
			betterQuery:  "Compute",
			worseQuery:   "VM",
			expectBetter: true,
		},
		{
			name:         "word boundary beats contains",
			betterQuery:  "VM",
			worseQuery:   "ompu",
			expectBetter: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			betterScore := Score(tt.betterQuery, text)
			worseScore := Score(tt.worseQuery, text)
			if tt.expectBetter {
				assert.Greater(t, betterScore, worseScore,
					"expected %q to score higher than %q", tt.betterQuery, tt.worseQuery)
			}
		})
	}
}

func TestScoreMultiWord(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		text     string
		wantZero bool
	}{
		{
			name:  "two words both match",
			query: "compute vm",
			text:  "Compute Engine: VM instances",
		},
		{
			name:  "two words in different order",
			query: "vm compute",
			text:  "Compute Engine: VM instances",
		},
		{
			name:  "partial words",
			query: "ce vm",
			text:  "Compute Engine: VM instances",
		},
		{
			name:     "one word no match",
			query:    "compute xyz",
			text:     "Compute Engine: VM instances",
			wantZero: true,
		},
		{
			name:     "both words no match",
			query:    "abc xyz",
			text:     "Compute Engine: VM instances",
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := Score(tt.query, tt.text)
			if tt.wantZero {
				assert.Equal(t, 0, score, "expected no match")
			} else {
				assert.Greater(t, score, 0, "expected match")
			}
		})
	}
}

func TestFilter(t *testing.T) {
	commands := []Command{
		{ID: "1", Label: "Compute Engine: VM instances"},
		{ID: "2", Label: "Compute Engine: Disks"},
		{ID: "3", Label: "Cloud Storage: Buckets"},
		{ID: "4", Label: "VPC Network: VPC networks"},
		{ID: "5", Label: "Refresh"},
	}

	t.Run("empty query returns all", func(t *testing.T) {
		result := Filter(commands, "")
		assert.Equal(t, len(commands), len(result))
	})

	t.Run("filters to matching commands", func(t *testing.T) {
		result := Filter(commands, "vm")
		assert.Equal(t, 1, len(result))
		assert.Equal(t, "1", result[0].ID)
	})

	t.Run("filters multiple matches", func(t *testing.T) {
		result := Filter(commands, "Compute")
		assert.Equal(t, 2, len(result))
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		result := Filter(commands, "xyz")
		assert.Equal(t, 0, len(result))
	})

	t.Run("sorts by relevance", func(t *testing.T) {
		// "Refresh" should rank higher than others when searching "ref"
		// because it's a shorter, more specific match
		result := Filter(commands, "ref")
		assert.Equal(t, 1, len(result))
		assert.Equal(t, "5", result[0].ID)
	})

	t.Run("multi-word query", func(t *testing.T) {
		result := Filter(commands, "compute disk")
		assert.Equal(t, 1, len(result))
		assert.Equal(t, "2", result[0].ID)
	})
}

func TestFilterSorting(t *testing.T) {
	commands := []Command{
		{ID: "1", Label: "VPC Network: VPC networks"},
		{ID: "2", Label: "Compute Engine: VM instances"},
	}

	// "VM" should match "VM instances" at word boundary and rank higher
	// than "VPC" which also matches at word boundary but "VM" is more specific
	result := Filter(commands, "vm")
	assert.Equal(t, 1, len(result))
	assert.Equal(t, "2", result[0].ID)
}
