package views

import (
	"context"
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
	b.WriteString("  (charts pending)\n")
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

// fetchAllMetrics is the parallel fetch. Real implementation in Task 18.
func (o *loadBalancerObservability) fetchAllMetrics() tea.Cmd {
	return func() tea.Msg {
		return lbMetricsLoadedMsg{metrics: &gcp.LBMetrics{LastFetch: time.Now()}}
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

// Compile-time use to avoid an unused-import error during the skeleton commit;
// removed by Task 18 when fetchAllMetrics is rewritten to use context.
var _ = context.Background
