package views

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

var errGKEObsInsufficientPerm = errors.New("api: insufficient permission")

func TestGKEObservability_DefaultState(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	assert.Equal(t, 0, s.rangeIdx) // 1h
	assert.True(t, s.autoRefresh)
	assert.False(t, s.tabActive)
}

func TestGKEObservability_RangeCycle(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	for i := range 5 {
		key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune('1' + i)}}
		s.Update(key)
		assert.Equal(t, i, s.rangeIdx, "after pressing '%c', rangeIdx should be %d", '1'+i, i)
	}
}

func TestGKEObservability_TickGuardWhenAutoRefreshOff(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	s.tabActive = true
	s.autoRefresh = false
	cmd := s.tickAutoRefresh()
	assert.Nil(t, cmd, "tick should be nil when autoRefresh is off")
}

func TestGKEObservability_TickGuardWhenTabInactive(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	s.tabActive = false
	s.autoRefresh = true
	cmd := s.tickAutoRefresh()
	assert.Nil(t, cmd, "tick should be nil when tab is inactive")
}

func TestGKEObservability_StaleTickDropped(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	s.tabActive = false
	s.autoRefresh = true
	cmd := s.Update(gkeObsRefreshTickMsg{})
	assert.Nil(t, cmd, "Update must drop stale refresh ticks")
}

func TestGKEObservability_PerMetricErrorSurfaced(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	s.tabActive = true
	s.SetSize(120, 40)
	loadedMsg := gkeObsMetricsLoadedMsg{
		metrics: gcp.GKEMetrics{
			CPUUtilization: []gcp.DataPoint{{Value: 50}},
		},
		warnings: map[string]error{"memory": errGKEObsInsufficientPerm},
	}
	s.Update(loadedMsg)
	out := s.View()
	assert.Contains(t, out, "memory")
	assert.Contains(t, out, "insufficient permission")
}

// TestGKEObservability_StaleResponseDropped guards against the regression
// where a slow response from a superseded range could overwrite the
// chart with old-range data while the range bar advertised the new
// range. Generation token must drop the stale response.
func TestGKEObservability_StaleResponseDropped(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	s.tabActive = true
	s.SetSize(120, 40)
	// User loads 1h, sees data.
	s.generation = 1
	s.Update(gkeObsMetricsLoadedMsg{
		gen:     1,
		metrics: gcp.GKEMetrics{CPUUtilization: []gcp.DataPoint{{Value: 50}}},
	})
	require.Equal(t, 50.0, s.metrics.CPUUtilization[0].Value)

	// User presses '3' (24h); generation bumps to 2 via Refresh().
	s.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	require.Equal(t, 2, s.generation, "Refresh must bump generation")
	require.Empty(t, s.metrics.CPUUtilization, "Refresh must clear cached metrics")

	// The slow 1h fetch finally returns — must be dropped.
	s.Update(gkeObsMetricsLoadedMsg{
		gen:     1,
		metrics: gcp.GKEMetrics{CPUUtilization: []gcp.DataPoint{{Value: 999}}},
	})
	assert.Empty(t, s.metrics.CPUUtilization,
		"stale-gen response must not overwrite the cleared chart state")
	assert.True(t, s.loading, "stale-gen response must not flip loading=false")
}

// TestGKEObservability_RangeChangeShowsLoading guards against the
// regression where switching time ranges kept the previous range's
// charts on screen during the new fetch — disorienting because the
// chart values silently swapped once data arrived. Refresh now blanks
// s.metrics so View() falls into its loading branch.
func TestGKEObservability_RangeChangeShowsLoading(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	s.tabActive = true
	s.SetSize(120, 40)
	// Seed prior-range data so View() would otherwise show charts.
	// Gen=0 matches the fresh sub-view's generation, so the message
	// is accepted.
	s.Update(gkeObsMetricsLoadedMsg{
		gen: 0,
		metrics: gcp.GKEMetrics{
			CPUUtilization:    []gcp.DataPoint{{Value: 50}},
			MemoryUtilization: []gcp.DataPoint{{Value: 60}},
		},
	})
	require.False(t, s.loading, "fixture: loaded state")
	require.NotEmpty(t, s.metrics.CPUUtilization)

	// Press '3' to switch to the 24h range (rangeIdx 2).
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	assert.True(t, s.loading, "switching range must flip back to loading")
	assert.Empty(t, s.metrics.CPUUtilization,
		"switching range must clear cached metrics so View() shows loading state")
	assert.Empty(t, s.metrics.MemoryUtilization)

	out := s.View()
	assert.Contains(t, out, "Loading metrics",
		"View must show the loading message while the new range is fetching")
	assert.Contains(t, out, "24h",
		"loading message should name the range the user is waiting on")
}
