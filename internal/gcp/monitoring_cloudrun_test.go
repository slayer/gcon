package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloudRunMetrics_ZeroValue(t *testing.T) {
	m := &CloudRunMetrics{}
	assert.Empty(t, m.RequestCount)
	assert.Empty(t, m.Latency50)
	assert.Empty(t, m.Latency95)
	assert.Empty(t, m.Latency99)
	assert.Empty(t, m.ErrorCount4xx)
	assert.Empty(t, m.ErrorCount5xx)
	assert.Empty(t, m.CPU)
	assert.Empty(t, m.BillableInstanceTime)
	assert.Empty(t, m.InstanceCount)
	assert.True(t, m.LastFetch.IsZero())
}

func TestScaleToMs(t *testing.T) {
	points := []DataPoint{
		{Value: 0.5},
		{Value: 1.0},
		{Value: 0.123},
	}

	scaleToMs(points)

	assert.InDelta(t, 500.0, points[0].Value, 0.001)
	assert.InDelta(t, 1000.0, points[1].Value, 0.001)
	assert.InDelta(t, 123.0, points[2].Value, 0.001)
}

func TestCloudRunFilter(t *testing.T) {
	filter := cloudRunFilter("my-service", "run.googleapis.com/request_count")
	assert.Contains(t, filter, `resource.type = "cloud_run_revision"`)
	assert.Contains(t, filter, `resource.labels.service_name = "my-service"`)
	assert.Contains(t, filter, `metric.type = "run.googleapis.com/request_count"`)
}
