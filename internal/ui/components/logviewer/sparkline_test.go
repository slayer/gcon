package logviewer

import (
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestSparklineEmpty(t *testing.T) {
	result := RenderSparkline(nil, 20, 0)
	assert.Contains(t, result, "No data")
}

func TestSparklineBasic(t *testing.T) {
	data := []gcp.DataPoint{
		{Timestamp: time.Now().Add(-5 * time.Minute), Value: 0},
		{Timestamp: time.Now().Add(-4 * time.Minute), Value: 50},
		{Timestamp: time.Now().Add(-3 * time.Minute), Value: 100},
		{Timestamp: time.Now().Add(-2 * time.Minute), Value: 50},
		{Timestamp: time.Now().Add(-1 * time.Minute), Value: 0},
	}
	result := RenderSparkline(data, 40, 12345)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "12,345")
}

func TestSparklineAllSameValue(t *testing.T) {
	data := []gcp.DataPoint{
		{Timestamp: time.Now().Add(-3 * time.Minute), Value: 50},
		{Timestamp: time.Now().Add(-2 * time.Minute), Value: 50},
		{Timestamp: time.Now().Add(-1 * time.Minute), Value: 50},
	}
	result := RenderSparkline(data, 20, 100)
	assert.NotEmpty(t, result)
}

func TestFormatResultCount(t *testing.T) {
	tests := []struct {
		count    int64
		expected string
	}{
		{0, "0 results"},
		{1, "1 result"},
		{999, "999 results"},
		{1000, "1,000 results"},
		{1234567, "1,234,567 results"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatResultCount(tt.count))
	}
}

func TestBucketize(t *testing.T) {
	data := []gcp.DataPoint{
		{Value: 10}, {Value: 20}, {Value: 30}, {Value: 40},
	}
	buckets := bucketize(data, 2)
	assert.Equal(t, 2, len(buckets))
	// First bucket averages 10+20=15, second 30+40=35
	assert.InDelta(t, 15.0, buckets[0], 0.1)
	assert.InDelta(t, 35.0, buckets[1], 0.1)
}
