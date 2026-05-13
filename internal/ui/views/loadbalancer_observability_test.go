package views

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

func TestNewLoadBalancerObservabilityDefaults(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
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
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	obs.metricsLoading = true
	out := obs.View()
	assert.Contains(t, out, "Loading metrics")
}

func TestLatencyChartWiringOnLoad(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
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
	obs := newLoadBalancerObservability("p", "rule-x", nil)
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
	obs := newLoadBalancerObservability("p", "rule-x", nil)
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
	obs := newLoadBalancerObservability("p", "rule-x", nil)
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
