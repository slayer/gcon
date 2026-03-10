package metricchart

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

// Chart heights for different metric importance levels.
const (
	HeightStandard = 8 // CPU, request count, latency
	HeightCompact  = 5 // error rates, instance count
)

// DataSet represents a named series of data points with a color.
type DataSet struct {
	Name  string
	Data  []gcp.DataPoint
	Color string // lipgloss hex color, e.g. "#34A853"
}

// Chart wraps ntcharts timeserieslinechart for gcon's metric rendering.
// It's a stateful renderer, not a Bubble Tea model — no tea.Msg routing needed.
type Chart struct {
	width    int
	height   int
	datasets []DataSet
	hasData  bool

	// Fixed Y range override — when set, disables auto-scaling
	fixedYMin *float64
	fixedYMax *float64

	// Formatting for the stats line below the chart
	statsFormatter StatsFormatter

	// Y-axis label formatter — controls how axis tick values are displayed
	yLabelFormatter func(int, float64) string
}

// StatsFormatter renders a stats summary line from data points.
type StatsFormatter func(data []gcp.DataPoint) string

// New creates a chart with the given height.
func New(height int) *Chart {
	return &Chart{
		width:           60,
		height:          height,
		statsFormatter:  FormatGenericStats,
		yLabelFormatter: humanYLabel,
	}
}

// SetStatsFormatter sets a custom stats line formatter.
func (c *Chart) SetStatsFormatter(f StatsFormatter) *Chart {
	c.statsFormatter = f
	return c
}

// SetYLabelFormatter sets a custom Y-axis label formatter.
func (c *Chart) SetYLabelFormatter(f func(int, float64) string) *Chart {
	c.yLabelFormatter = f
	return c
}

// SetYRange sets a fixed Y-axis range, disabling auto-scaling.
// Use for percentage metrics (0-100) to prevent misleading auto-scaling.
func (c *Chart) SetYRange(minY, maxY float64) *Chart {
	c.fixedYMin = &minY
	c.fixedYMax = &maxY
	return c
}

// Resize updates the chart width.
func (c *Chart) Resize(width int) {
	if width > 0 {
		c.width = width
	}
}

// SetData sets a single series of data points (default dataset).
func (c *Chart) SetData(data []gcp.DataPoint) {
	c.hasData = len(data) > 0
	c.datasets = nil
	if c.hasData {
		c.datasets = []DataSet{{Name: "default", Data: data, Color: "#4285F4"}}
	}
}

// SetDataSets sets multiple named series for overlay rendering.
func (c *Chart) SetDataSets(sets []DataSet) {
	c.datasets = sets
	c.hasData = false
	for _, ds := range sets {
		if len(ds.Data) > 0 {
			c.hasData = true
			break
		}
	}
}

// View renders the chart + legend + stats as a string.
func (c *Chart) View() string {
	if !c.hasData || len(c.datasets) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		return mutedStyle.Render("  No data available") + "\n"
	}

	var b strings.Builder

	b.WriteString(c.renderChart())

	// Legend for multi-dataset charts
	if len(c.datasets) > 1 {
		b.WriteString(c.renderLegend())
	}

	// Stats line for single-dataset charts
	if c.statsFormatter != nil && len(c.datasets) == 1 {
		b.WriteString("  ")
		b.WriteString(c.statsFormatter(c.datasets[0].Data))
		b.WriteString("\n")
	}

	return b.String()
}

// renderChart builds the ntcharts model and returns the rendered string.
func (c *Chart) renderChart() string {
	chartWidth := c.width - 2 // 2-char left indent
	if chartWidth < 10 {
		chartWidth = 10
	}
	chartHeight := c.height
	if chartHeight < 3 {
		chartHeight = 3
	}

	minY, maxY := c.computeYRange()
	minTime, maxTime := c.findTimeRange()
	if minTime.Equal(maxTime) {
		maxTime = minTime.Add(time.Minute)
	}

	axisStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	opts := []timeserieslinechart.Option{
		timeserieslinechart.WithYRange(minY, maxY),
		timeserieslinechart.WithTimeRange(minTime, maxTime),
		timeserieslinechart.WithAxesStyles(axisStyle, labelStyle),
		timeserieslinechart.WithXYSteps(4, 3),
		timeserieslinechart.WithXLabelFormatter(selectTimeLabelFormatter(maxTime.Sub(minTime))),
	}
	if c.yLabelFormatter != nil {
		opts = append(opts, timeserieslinechart.WithYLabelFormatter(c.yLabelFormatter))
	}

	chart := timeserieslinechart.New(chartWidth, chartHeight, opts...)

	// Push data and set styles per dataset.
	// fillGaps inserts zero-value boundaries so ntcharts doesn't draw lines across data gaps.
	for _, ds := range c.datasets {
		if len(ds.Data) == 0 {
			continue
		}
		points := fillGaps(ds.Data)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(ds.Color))

		if ds.Name == "default" {
			chart.SetStyle(style)
			chart.SetLineStyle(runes.ArcLineStyle)
			for _, dp := range points {
				chart.Push(timeserieslinechart.TimePoint{Time: dp.Timestamp, Value: dp.Value})
			}
		} else {
			chart.SetDataSetStyle(ds.Name, style)
			chart.SetDataSetLineStyle(ds.Name, runes.ArcLineStyle)
			for _, dp := range points {
				chart.PushDataSet(ds.Name, timeserieslinechart.TimePoint{Time: dp.Timestamp, Value: dp.Value})
			}
		}
	}

	chart.DrawBrailleAll()

	// Indent each line
	lines := strings.Split(chart.View(), "\n")
	var result strings.Builder
	for i, line := range lines {
		result.WriteString("  ")
		result.WriteString(line)
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	result.WriteString("\n")
	return result.String()
}

// renderLegend renders a color-coded legend for multi-dataset charts.
func (c *Chart) renderLegend() string {
	var parts []string
	for _, ds := range c.datasets {
		if len(ds.Data) == 0 {
			continue
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(ds.Color))
		parts = append(parts, style.Render("──")+" "+ds.Name)
	}
	if len(parts) == 0 {
		return ""
	}
	return "  " + strings.Join(parts, "  ") + "\n"
}

// computeYRange returns the Y-axis bounds, using fixed range if set.
func (c *Chart) computeYRange() (minY, maxY float64) {
	if c.fixedYMin != nil && c.fixedYMax != nil {
		return *c.fixedYMin, *c.fixedYMax
	}
	return c.autoYRange()
}

// autoYRange computes min/max Y across all datasets with headroom.
func (c *Chart) autoYRange() (minY, maxY float64) {
	first := true

	for _, ds := range c.datasets {
		for _, dp := range ds.Data {
			if first {
				minY = dp.Value
				maxY = dp.Value
				first = false
				continue
			}
			if dp.Value < minY {
				minY = dp.Value
			}
			if dp.Value > maxY {
				maxY = dp.Value
			}
		}
	}

	// Metrics shouldn't go below 0
	if minY > 0 {
		minY = 0
	}

	// 10% headroom so peaks don't touch the top
	if maxY > 0 {
		maxY *= 1.1
	}
	if maxY == 0 {
		maxY = 1
	}

	return minY, maxY
}

// fillGaps inserts zero-value boundary points where consecutive data points
// are separated by more than gapThreshold * minIntervalInterval. This prevents
// ntcharts from drawing lines across periods with no data (e.g., when a
// Cloud Run service scales to zero).
func fillGaps(data []gcp.DataPoint) []gcp.DataPoint {
	if len(data) < 2 {
		return data
	}

	// Find the typical data interval (minimum non-zero gap between points).
	// This represents the actual sampling frequency; larger gaps are data holes.
	var minInterval time.Duration
	for i := 1; i < len(data); i++ {
		d := data[i].Timestamp.Sub(data[i-1].Timestamp)
		if d > 0 && (minInterval == 0 || d < minInterval) {
			minInterval = d
		}
	}
	if minInterval == 0 {
		return data
	}

	// Gap threshold: 3x the typical interval
	const gapMultiplier = 3
	threshold := minInterval * gapMultiplier

	result := make([]gcp.DataPoint, 0, len(data))
	result = append(result, data[0])
	for i := 1; i < len(data); i++ {
		gap := data[i].Timestamp.Sub(data[i-1].Timestamp)
		if gap > threshold {
			// Insert zero just after the last real point and just before the next
			result = append(result,
				gcp.DataPoint{Timestamp: data[i-1].Timestamp.Add(minInterval), Value: 0},
				gcp.DataPoint{Timestamp: data[i].Timestamp.Add(-minInterval), Value: 0},
			)
		}
		result = append(result, data[i])
	}
	return result
}

// findTimeRange computes the earliest and latest timestamps across all datasets.
func (c *Chart) findTimeRange() (minT, maxT time.Time) {
	first := true

	for _, ds := range c.datasets {
		for _, dp := range ds.Data {
			if first {
				minT = dp.Timestamp
				maxT = dp.Timestamp
				first = false
				continue
			}
			if dp.Timestamp.Before(minT) {
				minT = dp.Timestamp
			}
			if dp.Timestamp.After(maxT) {
				maxT = dp.Timestamp
			}
		}
	}

	return minT, maxT
}

// selectTimeLabelFormatter picks the right X-axis label format based on span.
func selectTimeLabelFormatter(span time.Duration) func(int, float64) string {
	if span <= 48*time.Hour {
		return timeserieslinechart.HourTimeLabelFormatter()
	}
	return timeserieslinechart.DateTimeLabelFormatter()
}

// humanYLabel formats Y-axis values with SI suffixes for readability.
// Examples: 0.5 → "0.5", 42 → "42", 1500 → "1.5K", 2300000 → "2.3M", 1000000000 → "1B"
func humanYLabel(_ int, v float64) string {
	return formatHumanNumber(v)
}

// PercentYLabel formats Y-axis values as percentages.
// Examples: 0 → "0%", 50 → "50%", 100 → "100%"
func PercentYLabel(_ int, v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f%%", v)
	}
	return fmt.Sprintf("%.1f%%", v)
}

// LatencyYLabel formats Y-axis values as latency (milliseconds).
// Examples: 0.5 → "0.5ms", 150 → "150ms", 1500 → "1.5s"
func LatencyYLabel(_ int, v float64) string {
	switch {
	case v < 1:
		return fmt.Sprintf("%.1fms", v)
	case v < 1000:
		return fmt.Sprintf("%.0fms", v)
	default:
		return fmt.Sprintf("%.1fs", v/1000)
	}
}

// VCPUYLabel formats Y-axis values as vCPU.
func VCPUYLabel(_ int, v float64) string {
	if v == 0 {
		return "0"
	}
	if v < 0.01 {
		return fmt.Sprintf("%.3f", v)
	}
	if v < 1 {
		return fmt.Sprintf("%.2f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

// formatHumanNumber renders a number with SI suffixes (K, M, B).
func formatHumanNumber(v float64) string {
	abs := math.Abs(v)
	sign := ""
	if v < 0 {
		sign = "-"
	}

	switch {
	case abs >= 1e9:
		return sign + trimTrailingZero(fmt.Sprintf("%.1f", abs/1e9)) + "B"
	case abs >= 1e6:
		return sign + trimTrailingZero(fmt.Sprintf("%.1f", abs/1e6)) + "M"
	case abs >= 1e3:
		return sign + trimTrailingZero(fmt.Sprintf("%.1f", abs/1e3)) + "K"
	case abs >= 100:
		return fmt.Sprintf("%s%.0f", sign, abs)
	case abs >= 1:
		return sign + trimTrailingZero(fmt.Sprintf("%.1f", abs))
	case abs > 0:
		return sign + trimTrailingZero(fmt.Sprintf("%.2f", abs))
	default:
		return "0"
	}
}

// trimTrailingZero removes ".0" suffix (e.g. "1.0K" → "1K").
func trimTrailingZero(s string) string {
	return strings.TrimSuffix(s, ".0")
}
