package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

func TestNewLoadBalancerObservabilityDefaults(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", "https_lb_rule", nil)
	assert.Equal(t, "rule-x", obs.forwardingRuleName)
	assert.Equal(t, 24*time.Hour, obs.timeRange)
	assert.True(t, obs.autoRefresh)
	assert.NotNil(t, obs.requestCountChart)
	assert.NotNil(t, obs.latencyChart)
	assert.NotNil(t, obs.errorRateChart)
	assert.NotNil(t, obs.backendLatChart)
	assert.NotNil(t, obs.throughputChart)
}

func TestLoadBalancerObservabilityViewLoading(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", "https_lb_rule", nil)
	obs.metricsLoading = true
	out := obs.View()
	assert.Contains(t, out, "Loading metrics")
}

func TestLatencyChartWiringOnLoad(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", "https_lb_rule", nil)
	now := time.Now()
	obs.Update(lbMetricsLoadedMsg{
		metrics: &gcp.LBMetrics{
			Latency50: []gcp.DataPoint{{Timestamp: now, Value: 100}},
			Latency95: []gcp.DataPoint{{Timestamp: now, Value: 200}},
			Latency99: []gcp.DataPoint{{Timestamp: now, Value: 500}},
		},
	})
	out := obs.View()
	assert.Contains(t, out, "Request Latency")
}

func TestRequestCountChartWiringOnLoad(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", "https_lb_rule", nil)
	now := time.Now()
	obs.Update(lbMetricsLoadedMsg{
		metrics: &gcp.LBMetrics{
			RequestCount: []gcp.DataPoint{
				{Timestamp: now.Add(-2 * time.Minute), Value: 100},
				{Timestamp: now.Add(-1 * time.Minute), Value: 150},
			},
		},
	})
	out := obs.View()
	assert.Contains(t, out, "Request Count")
}

func TestPercentRateBasic(t *testing.T) {
	now := time.Now()
	total := []gcp.DataPoint{
		{Timestamp: now.Add(-2 * time.Minute), Value: 100},
		{Timestamp: now.Add(-1 * time.Minute), Value: 200},
	}
	errs := []gcp.DataPoint{
		{Timestamp: now.Add(-2 * time.Minute), Value: 5},
		{Timestamp: now.Add(-1 * time.Minute), Value: 10},
	}
	got := percentRate(errs, total)
	require.Len(t, got, 2)
	assert.InDelta(t, 5.0, got[0].Value, 0.001)
	assert.InDelta(t, 5.0, got[1].Value, 0.001)
}

func TestPercentRateZeroTotalIsZero(t *testing.T) {
	now := time.Now()
	total := []gcp.DataPoint{{Timestamp: now, Value: 0}}
	errs := []gcp.DataPoint{{Timestamp: now, Value: 5}}
	got := percentRate(errs, total)
	require.Len(t, got, 1)
	assert.Equal(t, 0.0, got[0].Value)
}

func TestPercentRateMissingErrs(t *testing.T) {
	now := time.Now()
	total := []gcp.DataPoint{{Timestamp: now, Value: 100}}
	got := percentRate(nil, total)
	assert.Nil(t, got)
}

func TestBackendLatencyChartWiring(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", "https_lb_rule", nil)
	now := time.Now()
	obs.Update(lbMetricsLoadedMsg{
		metrics: &gcp.LBMetrics{
			BackendLat50: []gcp.DataPoint{{Timestamp: now, Value: 30}},
		},
	})
	out := obs.View()
	assert.Contains(t, out, "Backend Latency")
}

func TestThroughputChartWiring(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", "https_lb_rule", nil)
	now := time.Now()
	obs.Update(lbMetricsLoadedMsg{
		metrics: &gcp.LBMetrics{
			RequestBytes:  []gcp.DataPoint{{Timestamp: now, Value: 1024}},
			ResponseBytes: []gcp.DataPoint{{Timestamp: now, Value: 8192}},
		},
	})
	out := obs.View()
	assert.Contains(t, out, "Throughput")
}

func TestObservabilityTimeRangeKeys(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", "https_lb_rule", nil)
	cases := []struct {
		key  string
		want time.Duration
	}{
		{"1", 1 * time.Hour},
		{"2", 6 * time.Hour},
		{"3", 24 * time.Hour},
		{"4", 7 * 24 * time.Hour},
		{"5", 30 * 24 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			_, handled := obs.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
			assert.True(t, handled)
			assert.Equal(t, tc.want, obs.timeRange)
		})
	}
}

func TestObservabilityAutoRefreshToggle(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", "https_lb_rule", nil)
	assert.True(t, obs.autoRefresh) // default on
	obs.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.False(t, obs.autoRefresh)
}

func TestStaleTickIsDropped(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", "https_lb_rule", nil)
	obs.tabActive = false
	obs.autoRefresh = true
	cmd := obs.Update(lbObsTickMsg{})
	assert.Nil(t, cmd, "tick must produce no command when tab inactive")
}

func TestTickWhenInactiveProducesNoCmd(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", "https_lb_rule", nil)
	obs.autoRefresh = false
	obs.tabActive = true
	cmd := obs.tickAutoRefresh()
	assert.Nil(t, cmd)
}
