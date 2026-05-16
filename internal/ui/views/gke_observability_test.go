package views

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

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
