package views

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/metricchart"
)

var errMonitoringClientUnavailable = errors.New("monitoring client unavailable")

// Internal messages.
type lbMetricsLoadedMsg struct {
	metrics  *gcp.LBMetrics
	warnings []string // non-fatal per-metric fetch errors
}
type lbMetricsErrorMsg struct{ err error }
type lbObsTickMsg struct{}

// loadBalancerObservability owns the Observability tab's state for a
// single LB: metrics, range selection, auto-refresh, and chart instances.
type loadBalancerObservability struct {
	projectID          string
	forwardingRuleName string
	gcpClient          *gcp.Client

	metrics         *gcp.LBMetrics
	metricsLoading  bool
	metricsError    error
	metricsWarnings []string

	timeRange   time.Duration
	autoRefresh bool
	tabActive   bool

	spinner spinner.Model
	width   int

	requestCountChart *metricchart.Chart
	latencyChart      *metricchart.Chart // p50/p95/p99
	errorRateChart    *metricchart.Chart // 4xx/5xx percent
	backendLatChart   *metricchart.Chart
	throughputChart   *metricchart.Chart // in/out
}

func newLoadBalancerObservability(projectID, forwardingRuleName string, gcpClient *gcp.Client) *loadBalancerObservability {
	req := metricchart.New(metricchart.HeightStandard)
	req.SetStatsFormatter(metricchart.FormatCountStats)

	lat := metricchart.New(metricchart.HeightStandard)
	lat.SetStatsFormatter(nil).SetYLabelFormatter(metricchart.LatencyYLabel)

	errc := metricchart.New(metricchart.HeightCompact)
	errc.SetStatsFormatter(nil).SetYLabelFormatter(metricchart.PercentYLabel)
	errc.SetYRange(0, 10)

	be := metricchart.New(metricchart.HeightStandard)
	be.SetStatsFormatter(nil).SetYLabelFormatter(metricchart.LatencyYLabel)

	thr := metricchart.New(metricchart.HeightCompact)
	thr.SetStatsFormatter(nil)

	return &loadBalancerObservability{
		projectID:          projectID,
		forwardingRuleName: forwardingRuleName,
		gcpClient:          gcpClient,
		timeRange:          24 * time.Hour,
		autoRefresh:        true,
		spinner:            components.NewGCPSpinner(),
		requestCountChart:  req,
		latencyChart:       lat,
		errorRateChart:     errc,
		backendLatChart:    be,
		throughputChart:    thr,
	}
}

// Init triggers the first metrics fetch.
func (o *loadBalancerObservability) Init() tea.Cmd {
	o.metricsLoading = true
	o.metricsError = nil
	return tea.Batch(o.spinner.Tick, o.fetchAllMetrics())
}

// View renders the tab. Filled out in later tasks.
func (o *loadBalancerObservability) View() string {
	var b strings.Builder
	titleStyle := lipgloss.NewStyle().Bold(true)
	b.WriteString(titleStyle.Render("Observability"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", max(0, min(o.width-4, 60))))
	b.WriteString("\n\n")
	if o.metricsLoading {
		fmt.Fprintf(&b, "  %s Loading metrics...\n", o.spinner.View())
		return b.String()
	}
	if o.metricsError != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		b.WriteString(errStyle.Render(fmt.Sprintf("  Error loading metrics: %v", o.metricsError)))
		b.WriteString("\n  Press 'r' to retry\n")
		return b.String()
	}
	if o.metrics == nil {
		b.WriteString("  (no metric data)\n")
		return b.String()
	}
	o.renderTimeRangeSelector(&b)
	b.WriteString("\n")
	o.renderRequestCount(&b)
	o.renderLatency(&b)
	o.renderErrorRate(&b)
	o.renderBackendLatency(&b)
	return b.String()
}

// resizeCharts propagates width changes to every chart.
func (o *loadBalancerObservability) resizeCharts() {
	w := o.width - 2
	if w < 10 {
		w = 10
	}
	o.requestCountChart.Resize(w)
	o.latencyChart.Resize(w)
	o.errorRateChart.Resize(w)
	o.backendLatChart.Resize(w)
	o.throughputChart.Resize(w)
}

// fetchAllMetrics fetches all LB metrics in parallel.
func (o *loadBalancerObservability) fetchAllMetrics() tea.Cmd {
	if o.gcpClient == nil {
		return func() tea.Msg {
			return lbMetricsErrorMsg{err: errMonitoringClientUnavailable}
		}
	}
	rule := o.forwardingRuleName
	projectID := o.projectID
	duration := o.timeRange
	client := o.gcpClient
	return func() tea.Msg {
		mc, err := client.GetMonitoringClient(projectID)
		if err != nil {
			return lbMetricsErrorMsg{err: fmt.Errorf("init monitoring client: %w", err)}
		}
		ctx := context.Background()
		out := &gcp.LBMetrics{LastFetch: time.Now()}
		var warnings []string

		if rc, err := mc.GetLBRequestCount(ctx, rule, duration); err != nil {
			warnings = append(warnings, fmt.Sprintf("request count: %v", err))
		} else {
			out.RequestCount = rc
		}
		if p50, p95, p99, err := mc.GetLBRequestLatencies(ctx, rule, duration); err != nil {
			warnings = append(warnings, fmt.Sprintf("request latencies: %v", err))
		} else {
			out.Latency50, out.Latency95, out.Latency99 = p50, p95, p99
		}
		if r4, err := mc.GetLBRequestCountByCodeClass(ctx, rule, "4xx", duration); err != nil {
			warnings = append(warnings, fmt.Sprintf("4xx count: %v", err))
		} else {
			out.RequestCount4xx = r4
		}
		if r5, err := mc.GetLBRequestCountByCodeClass(ctx, rule, "5xx", duration); err != nil {
			warnings = append(warnings, fmt.Sprintf("5xx count: %v", err))
		} else {
			out.RequestCount5xx = r5
		}
		if p50, p95, p99, err := mc.GetLBBackendLatencies(ctx, rule, duration); err != nil {
			warnings = append(warnings, fmt.Sprintf("backend latencies: %v", err))
		} else {
			out.BackendLat50, out.BackendLat95, out.BackendLat99 = p50, p95, p99
		}
		return lbMetricsLoadedMsg{metrics: out, warnings: warnings}
	}
}

// StartAutoRefresh marks the tab active. Real ticker comes in Task 24.
func (o *loadBalancerObservability) StartAutoRefresh() tea.Cmd {
	o.tabActive = true
	return nil
}

// StopAutoRefresh marks the tab inactive.
func (o *loadBalancerObservability) StopAutoRefresh() {
	o.tabActive = false
}

// Update routes messages.
func (o *loadBalancerObservability) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case lbMetricsLoadedMsg:
		o.metrics = m.metrics
		o.metricsWarnings = m.warnings
		o.metricsLoading = false
		o.metricsError = nil
		if o.metrics != nil {
			o.requestCountChart.SetData(o.metrics.RequestCount)
			o.latencyChart.SetDataSets([]metricchart.DataSet{
				{Name: "p50", Data: o.metrics.Latency50, Color: "#34A853"},
				{Name: "p95", Data: o.metrics.Latency95, Color: "#FBBC04"},
				{Name: "p99", Data: o.metrics.Latency99, Color: "#EA4335"},
			})
			err4xx := percentRate(o.metrics.RequestCount4xx, o.metrics.RequestCount)
			err5xx := percentRate(o.metrics.RequestCount5xx, o.metrics.RequestCount)
			o.errorRateChart.SetDataSets([]metricchart.DataSet{
				{Name: "4xx", Data: err4xx, Color: "#FBBC04"},
				{Name: "5xx", Data: err5xx, Color: "#EA4335"},
			})
			o.backendLatChart.SetDataSets([]metricchart.DataSet{
				{Name: "p50", Data: o.metrics.BackendLat50, Color: "#34A853"},
				{Name: "p95", Data: o.metrics.BackendLat95, Color: "#FBBC04"},
				{Name: "p99", Data: o.metrics.BackendLat99, Color: "#EA4335"},
			})
		}
		return nil
	case lbMetricsErrorMsg:
		o.metricsError = m.err
		o.metricsLoading = false
		return nil
	case spinner.TickMsg:
		if o.metricsLoading {
			var cmd tea.Cmd
			o.spinner, cmd = o.spinner.Update(m)
			return cmd
		}
	}
	return nil
}

func (o *loadBalancerObservability) renderTimeRangeSelector(b *strings.Builder) {
	options := []struct {
		label string
		d     time.Duration
	}{
		{"1h", 1 * time.Hour},
		{"6h", 6 * time.Hour},
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
	}
	active := lipgloss.NewStyle().Foreground(lipgloss.Color("#1A73E8")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString("  ")
	for _, opt := range options {
		if opt.d == o.timeRange {
			b.WriteString(active.Render("[" + opt.label + "]"))
		} else {
			b.WriteString(muted.Render(" " + opt.label + " "))
		}
		b.WriteString("  ")
	}
	state := "OFF"
	if o.autoRefresh {
		state = "ON"
	}
	b.WriteString(muted.Render(fmt.Sprintf("    auto-refresh %s    r refresh", state)))
	b.WriteString("\n")
}

func (o *loadBalancerObservability) renderLatency(b *strings.Builder) {
	section := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString(section.Render("Request Latency (p50 / p95 / p99)"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if o.metrics != nil && (len(o.metrics.Latency50) > 0 || len(o.metrics.Latency95) > 0 || len(o.metrics.Latency99) > 0) {
		b.WriteString(o.latencyChart.View())
	} else {
		b.WriteString(muted.Render("  No latency data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func (o *loadBalancerObservability) renderRequestCount(b *strings.Builder) {
	section := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString(section.Render("Request Count"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if o.metrics != nil && len(o.metrics.RequestCount) > 0 {
		b.WriteString(o.requestCountChart.View())
	} else {
		b.WriteString(muted.Render("  No request data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func (o *loadBalancerObservability) renderErrorRate(b *strings.Builder) {
	section := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString(section.Render("Error Rate (4xx / 5xx)"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if o.metrics != nil && (len(o.metrics.RequestCount4xx) > 0 || len(o.metrics.RequestCount5xx) > 0) {
		b.WriteString(o.errorRateChart.View())
	} else {
		b.WriteString(muted.Render("  No error data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func (o *loadBalancerObservability) renderBackendLatency(b *strings.Builder) {
	section := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString(section.Render("Backend Latency (origin response time)"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if o.metrics != nil && (len(o.metrics.BackendLat50) > 0 || len(o.metrics.BackendLat95) > 0 || len(o.metrics.BackendLat99) > 0) {
		b.WriteString(o.backendLatChart.View())
	} else {
		b.WriteString(muted.Render("  No backend latency data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// percentRate divides `errs[i]` by `total[i]` for each timestamp where
// the timestamps match (within 1 second tolerance). Result is a
// percentage 0–100. When total is zero, the percentage is zero (not
// NaN). Mismatched timestamps fall through with zero.
func percentRate(errs, total []gcp.DataPoint) []gcp.DataPoint {
	if len(errs) == 0 || len(total) == 0 {
		return nil
	}
	out := make([]gcp.DataPoint, 0, len(total))
	j := 0
	for i := range total {
		for j < len(errs) && errs[j].Timestamp.Before(total[i].Timestamp.Add(-time.Second)) {
			j++
		}
		var pct float64
		if total[i].Value > 0 && j < len(errs) {
			if errs[j].Timestamp.Sub(total[i].Timestamp).Abs() <= time.Second {
				pct = (errs[j].Value / total[i].Value) * 100
			}
		}
		out = append(out, gcp.DataPoint{Timestamp: total[i].Timestamp, Value: pct})
	}
	return out
}
