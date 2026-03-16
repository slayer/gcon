package metricchart

import (
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestComputeStats(t *testing.T) {
	now := time.Now()
	data := []gcp.DataPoint{
		{Timestamp: now, Value: 10},
		{Timestamp: now.Add(time.Minute), Value: 30},
		{Timestamp: now.Add(2 * time.Minute), Value: 50},
		{Timestamp: now.Add(3 * time.Minute), Value: 20},
	}

	s := computeStats(data)
	assert.Equal(t, 20.0, s.Current)     // last value
	assert.InDelta(t, 27.5, s.Avg, 0.01) // (10+30+50+20)/4
	assert.Equal(t, 50.0, s.Peak)
	assert.Equal(t, now.Add(2*time.Minute), s.PeakTime)
}

func TestComputeStats_Empty(t *testing.T) {
	s := computeStats(nil)
	assert.Equal(t, 0.0, s.Current)
	assert.Equal(t, 0.0, s.Avg)
	assert.Equal(t, 0.0, s.Peak)
	assert.True(t, s.PeakTime.IsZero())
}

func TestComputeStats_SinglePoint(t *testing.T) {
	now := time.Now()
	s := computeStats([]gcp.DataPoint{{Timestamp: now, Value: 42}})
	assert.Equal(t, 42.0, s.Current)
	assert.Equal(t, 42.0, s.Avg)
	assert.Equal(t, 42.0, s.Peak)
}

func TestFormatPercentageStats(t *testing.T) {
	data := makeDataPoints([]float64{25.5, 50.0, 75.3})
	result := FormatPercentageStats(data)

	assert.Contains(t, result, "75.3%")
	assert.Contains(t, result, "Current:")
	assert.Contains(t, result, "Avg:")
	assert.Contains(t, result, "Peak:")
}

func TestFormatCountStats(t *testing.T) {
	data := makeDataPoints([]float64{100, 200, 150})
	result := FormatCountStats(data)

	assert.Contains(t, result, "req/s")
	assert.Contains(t, result, "150.0 req/s")
}

func TestFormatLatencyStats(t *testing.T) {
	data := makeDataPoints([]float64{50, 100, 200})
	result := FormatLatencyStats(data)

	assert.Contains(t, result, "ms")
	assert.Contains(t, result, "200ms") // peak
}

func TestFormatVCPUStats(t *testing.T) {
	data := makeDataPoints([]float64{0.5, 1.0, 0.75})
	result := FormatVCPUStats(data)

	assert.Contains(t, result, "vCPU")
	assert.Contains(t, result, "0.75 vCPU")
}

func TestFormatInstanceCountStats(t *testing.T) {
	data := makeDataPoints([]float64{1, 3, 5})
	result := FormatInstanceCountStats(data)

	assert.Contains(t, result, "Current: 5")
	assert.Contains(t, result, "Peak: 5")
}

func TestFormatGenericStats(t *testing.T) {
	data := makeDataPoints([]float64{1.23, 45.6, 789.0})
	result := FormatGenericStats(data)

	assert.Contains(t, result, "Current:")
	assert.Contains(t, result, "789") // peak
}

func TestAutoFormat(t *testing.T) {
	tests := []struct {
		value    float64
		expected string
	}{
		{0.12, "0.12"},
		{5.67, "5.67"},
		{12.3, "12.3"},
		{99.9, "99.9"},
		{100.0, "100"},
		{1234.5, "1234"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, autoFormat(tt.value), "autoFormat(%v)", tt.value)
	}
}

func TestFormatLatency(t *testing.T) {
	tests := []struct {
		ms       float64
		expected string
	}{
		{0.5, "0.50ms"},
		{123, "123ms"},
		{1500, "1.5s"},
		{65000, "1m 5s"},
		{120000, "2m"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatLatency(tt.ms), "formatLatency(%v)", tt.ms)
	}
}
