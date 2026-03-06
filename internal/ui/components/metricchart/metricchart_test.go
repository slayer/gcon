package metricchart

import (
	"strings"
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDataPoints(values []float64) []gcp.DataPoint {
	now := time.Now()
	points := make([]gcp.DataPoint, len(values))
	for i, v := range values {
		points[i] = gcp.DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     v,
		}
	}
	return points
}

func TestNew(t *testing.T) {
	c := New(HeightStandard)
	assert.Equal(t, HeightStandard, c.height)
	assert.Equal(t, 60, c.width)
	assert.False(t, c.hasData)
}

func TestSetData_SingleSeries(t *testing.T) {
	c := New(HeightStandard)
	data := makeDataPoints([]float64{10, 20, 30, 40, 50})
	c.SetData(data)

	assert.True(t, c.hasData)
	assert.Len(t, c.datasets, 1)
	assert.Equal(t, "default", c.datasets[0].Name)
	assert.Len(t, c.datasets[0].Data, 5)
}

func TestSetData_Empty(t *testing.T) {
	c := New(HeightStandard)
	c.SetData(nil)

	assert.False(t, c.hasData)
	assert.Empty(t, c.datasets)
}

func TestSetDataSets_MultiSeries(t *testing.T) {
	c := New(HeightStandard)
	c.SetDataSets([]DataSet{
		{Name: "p50", Data: makeDataPoints([]float64{10, 20}), Color: "#34A853"},
		{Name: "p95", Data: makeDataPoints([]float64{50, 60}), Color: "#FBBC04"},
		{Name: "p99", Data: makeDataPoints([]float64{90, 100}), Color: "#EA4335"},
	})

	assert.True(t, c.hasData)
	assert.Len(t, c.datasets, 3)
}

func TestSetDataSets_AllEmpty(t *testing.T) {
	c := New(HeightStandard)
	c.SetDataSets([]DataSet{
		{Name: "p50", Data: nil, Color: "#34A853"},
	})
	assert.False(t, c.hasData)
}

func TestView_NoData(t *testing.T) {
	c := New(HeightStandard)
	view := c.View()
	assert.Contains(t, view, "No data available")
}

func TestView_SingleSeries(t *testing.T) {
	c := New(HeightStandard)
	c.Resize(60)
	data := makeDataPoints([]float64{10, 20, 30, 40, 50, 40, 30})
	c.SetData(data)

	view := c.View()
	require.NotEmpty(t, view)
	// Should contain chart output (braille chars or axes)
	assert.NotContains(t, view, "No data available")
	// Should contain stats line
	assert.Contains(t, view, "Current:")
}

func TestView_MultiSeries_HasLegend(t *testing.T) {
	c := New(HeightStandard)
	c.Resize(60)
	c.SetStatsFormatter(nil) // no per-series stats for multi-dataset
	c.SetDataSets([]DataSet{
		{Name: "p50", Data: makeDataPoints([]float64{10, 20, 30}), Color: "#34A853"},
		{Name: "p95", Data: makeDataPoints([]float64{50, 60, 70}), Color: "#FBBC04"},
	})

	view := c.View()
	assert.Contains(t, view, "p50")
	assert.Contains(t, view, "p95")
	assert.Contains(t, view, "──")
}

func TestResize_NoPanic(t *testing.T) {
	c := New(HeightStandard)
	data := makeDataPoints([]float64{10, 20, 30})
	c.SetData(data)

	// Various sizes should not panic
	for _, w := range []int{10, 20, 40, 80, 120, 200} {
		c.Resize(w)
		assert.NotPanics(t, func() {
			c.View()
		}, "panicked at width %d", w)
	}
}

func TestResize_Zero(t *testing.T) {
	c := New(HeightStandard)
	c.Resize(0) // should be ignored
	assert.Equal(t, 60, c.width)
}

func TestSetYRange(t *testing.T) {
	c := New(HeightStandard)
	c.SetYRange(0, 100)
	assert.NotNil(t, c.fixedYMin)
	assert.NotNil(t, c.fixedYMax)
	assert.Equal(t, 0.0, *c.fixedYMin)
	assert.Equal(t, 100.0, *c.fixedYMax)
}

func TestAutoYRange(t *testing.T) {
	c := New(HeightStandard)
	c.SetData(makeDataPoints([]float64{10, 50, 30}))
	minY, maxY := c.autoYRange()
	assert.Equal(t, 0.0, minY) // floor at 0
	assert.InDelta(t, 55.0, maxY, 0.1) // 50 * 1.1 = 55
}

func TestAutoYRange_AllZero(t *testing.T) {
	c := New(HeightStandard)
	c.SetData(makeDataPoints([]float64{0, 0, 0}))
	_, maxY := c.autoYRange()
	assert.Equal(t, 1.0, maxY) // avoid zero-height chart
}

func TestFindTimeRange(t *testing.T) {
	now := time.Now()
	data := []gcp.DataPoint{
		{Timestamp: now, Value: 1},
		{Timestamp: now.Add(5 * time.Minute), Value: 2},
		{Timestamp: now.Add(10 * time.Minute), Value: 3},
	}
	c := New(HeightStandard)
	c.SetData(data)

	minT, maxT := c.findTimeRange()
	assert.Equal(t, now, minT)
	assert.Equal(t, now.Add(10*time.Minute), maxT)
}

func TestRenderLegend(t *testing.T) {
	c := New(HeightStandard)
	c.SetDataSets([]DataSet{
		{Name: "p50", Data: makeDataPoints([]float64{1}), Color: "#34A853"},
		{Name: "p95", Data: makeDataPoints([]float64{2}), Color: "#FBBC04"},
		{Name: "p99", Data: nil, Color: "#EA4335"}, // empty, should be skipped
	})

	legend := c.renderLegend()
	assert.Contains(t, legend, "p50")
	assert.Contains(t, legend, "p95")
	assert.NotContains(t, legend, "p99") // empty dataset omitted
}

func TestSelectTimeLabelFormatter(t *testing.T) {
	// Short span should use hour formatter
	f := selectTimeLabelFormatter(time.Hour)
	assert.NotNil(t, f)

	// Long span should use date formatter
	f = selectTimeLabelFormatter(7 * 24 * time.Hour)
	assert.NotNil(t, f)
}

func TestFormatHumanNumber(t *testing.T) {
	tests := []struct {
		value    float64
		expected string
	}{
		{0, "0"},
		{0.5, "0.50"},
		{0.12, "0.12"},
		{1, "1"},
		{1.5, "1.5"},
		{42, "42"},
		{99.9, "99.9"},
		{100, "100"},
		{999, "999"},
		{1000, "1K"},
		{1500, "1.5K"},
		{10000, "10K"},
		{999999, "1000K"},
		{1000000, "1M"},
		{2300000, "2.3M"},
		{39881237, "39.9M"},
		{1000000000, "1B"},
		{-1500, "-1.5K"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatHumanNumber(tt.value),
			"formatHumanNumber(%v)", tt.value)
	}
}

func TestHumanYLabel(t *testing.T) {
	assert.Equal(t, "1.5K", humanYLabel(0, 1500))
	assert.Equal(t, "0", humanYLabel(0, 0))
}

func TestPercentYLabel(t *testing.T) {
	assert.Equal(t, "0%", PercentYLabel(0, 0))
	assert.Equal(t, "50%", PercentYLabel(0, 50))
	assert.Equal(t, "100%", PercentYLabel(0, 100))
	assert.Equal(t, "33.3%", PercentYLabel(0, 33.3))
}

func TestLatencyYLabel(t *testing.T) {
	assert.Equal(t, "0.5ms", LatencyYLabel(0, 0.5))
	assert.Equal(t, "150ms", LatencyYLabel(0, 150))
	assert.Equal(t, "1.5s", LatencyYLabel(0, 1500))
}

func TestVCPUYLabel(t *testing.T) {
	assert.Equal(t, "0", VCPUYLabel(0, 0))
	assert.Equal(t, "0.50", VCPUYLabel(0, 0.5))
	assert.Equal(t, "1.0", VCPUYLabel(0, 1))
	assert.Equal(t, "0.005", VCPUYLabel(0, 0.005))
}

func TestFillGaps_NoGap(t *testing.T) {
	// Evenly spaced points — no gaps to fill
	now := time.Now()
	data := []gcp.DataPoint{
		{Timestamp: now, Value: 1},
		{Timestamp: now.Add(1 * time.Minute), Value: 2},
		{Timestamp: now.Add(2 * time.Minute), Value: 3},
	}
	result := fillGaps(data)
	assert.Len(t, result, 3, "no gap means no extra points")
}

func TestFillGaps_WithGap(t *testing.T) {
	// 1-minute intervals with a 10-minute gap in the middle
	now := time.Now()
	data := []gcp.DataPoint{
		{Timestamp: now, Value: 1},
		{Timestamp: now.Add(1 * time.Minute), Value: 2},
		{Timestamp: now.Add(2 * time.Minute), Value: 3},
		// gap: 10 minutes
		{Timestamp: now.Add(12 * time.Minute), Value: 4},
		{Timestamp: now.Add(13 * time.Minute), Value: 5},
	}
	result := fillGaps(data)
	// Should insert 2 zero-value boundary points at the gap
	assert.Len(t, result, 7, "gap should insert 2 zero-value points")

	// Check that the inserted points have Value=0
	assert.Equal(t, 0.0, result[3].Value, "first boundary should be zero")
	assert.Equal(t, 0.0, result[4].Value, "second boundary should be zero")

	// Boundaries should be near the gap edges
	assert.True(t, result[3].Timestamp.After(data[2].Timestamp))
	assert.True(t, result[4].Timestamp.Before(data[3].Timestamp))
}

func TestFillGaps_SinglePoint(t *testing.T) {
	now := time.Now()
	data := []gcp.DataPoint{{Timestamp: now, Value: 1}}
	result := fillGaps(data)
	assert.Len(t, result, 1)
}

func TestFillGaps_TwoPoints_NoGap(t *testing.T) {
	now := time.Now()
	data := []gcp.DataPoint{
		{Timestamp: now, Value: 1},
		{Timestamp: now.Add(1 * time.Minute), Value: 2},
	}
	result := fillGaps(data)
	assert.Len(t, result, 2)
}

func TestFillGaps_MultipleGaps(t *testing.T) {
	now := time.Now()
	data := []gcp.DataPoint{
		{Timestamp: now, Value: 1},
		{Timestamp: now.Add(1 * time.Minute), Value: 2},
		// gap 1
		{Timestamp: now.Add(11 * time.Minute), Value: 3},
		{Timestamp: now.Add(12 * time.Minute), Value: 4},
		// gap 2
		{Timestamp: now.Add(22 * time.Minute), Value: 5},
	}
	result := fillGaps(data)
	// 5 original + 2 boundary pairs = 9
	assert.Len(t, result, 9)
}

func TestView_IndentedOutput(t *testing.T) {
	c := New(HeightStandard)
	c.Resize(40)
	c.SetData(makeDataPoints([]float64{10, 20, 30, 40, 50}))

	view := c.View()
	lines := strings.Split(view, "\n")
	// All non-empty lines should start with 2-space indent
	for _, line := range lines {
		if len(line) > 0 {
			assert.True(t, strings.HasPrefix(line, "  "),
				"line should be indented: %q", line)
		}
	}
}
