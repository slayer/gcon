package views

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
