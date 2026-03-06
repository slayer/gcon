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
	uierrors "github.com/slayer/gcon/internal/ui/errors"
)

// Internal messages for Cloud Run observability data loading.
type crMetricsLoadedMsg struct {
	metrics  *gcp.CloudRunMetrics
	warnings []string // non-fatal per-metric errors
}
type crMetricsErrorMsg struct{ err error }
type crLogsLoadedMsg struct{ logs []gcp.LogEntry }
type crLogsErrorMsg struct{ err error }
type crRefreshTickMsg struct{}

// cloudRunObservability manages observability state for the Cloud Run service
// details view: metrics loading, log loading, severity filtering, time range
// selection, and auto-refresh.
type cloudRunObservability struct {
	projectID   string
	serviceName string
	gcpClient   *gcp.Client

	// Metrics state
	metrics         *gcp.CloudRunMetrics
	metricsLoading  bool
	metricsError    error
	metricsWarnings []string // non-fatal per-metric fetch errors

	// Logs state
	logs        []gcp.LogEntry
	logsLoading bool
	logsError   error

	// Severity filter: which severities to show in log output
	severityEnabled map[string]bool

	// Time range and refresh
	timeRange   time.Duration
	autoRefresh bool

	spinner spinner.Model
	width   int

	// Metric charts
	requestCountChart  *metricchart.Chart
	latencyChart       *metricchart.Chart // multi-dataset: p50/p95/p99
	errorRateChart     *metricchart.Chart // multi-dataset: 4xx/5xx
	cpuChart           *metricchart.Chart
	billableTimeChart  *metricchart.Chart
	instanceCountChart *metricchart.Chart
}

func newCloudRunObservability(projectID, serviceName string, gcpClient *gcp.Client) *cloudRunObservability {
	reqChart := metricchart.New("Request Count", metricchart.HeightStandard)
	reqChart.SetStatsFormatter(metricchart.FormatCountStats)

	latChart := metricchart.New("Latency", metricchart.HeightStandard)
	latChart.SetStatsFormatter(nil).SetYLabelFormatter(metricchart.LatencyYLabel)

	errChart := metricchart.New("Error Rate", metricchart.HeightCompact)
	errChart.SetStatsFormatter(nil)

	cpuChart := metricchart.New("CPU", metricchart.HeightStandard)
	cpuChart.SetStatsFormatter(metricchart.FormatVCPUStats).SetYLabelFormatter(metricchart.VCPUYLabel)

	billChart := metricchart.New("Billable Time", metricchart.HeightCompact)
	billChart.SetStatsFormatter(metricchart.FormatGenericStats)

	instChart := metricchart.New("Instance Count", metricchart.HeightCompact)
	instChart.SetStatsFormatter(metricchart.FormatInstanceCountStats)

	return &cloudRunObservability{
		projectID:   projectID,
		serviceName: serviceName,
		gcpClient:   gcpClient,
		severityEnabled: map[string]bool{
			"INFO":    true,
			"WARNING": true,
			"ERROR":   true,
		},
		timeRange:          time.Hour,
		autoRefresh:        true,
		spinner:            components.NewGCPSpinner(),
		requestCountChart:  reqChart,
		latencyChart:       latChart,
		errorRateChart:     errChart,
		cpuChart:           cpuChart,
		billableTimeChart:  billChart,
		instanceCountChart: instChart,
	}
}

// resizeCharts updates all chart widths to match the current view width.
func (o *cloudRunObservability) resizeCharts() {
	chartWidth := o.width - 4 // account for padding
	for _, ch := range []*metricchart.Chart{
		o.requestCountChart, o.latencyChart, o.errorRateChart,
		o.cpuChart, o.billableTimeChart, o.instanceCountChart,
	} {
		if ch != nil {
			ch.Resize(chartWidth)
		}
	}
}

// Init resets loading state and kicks off data fetching.
func (o *cloudRunObservability) Init() tea.Cmd {
	o.metricsLoading = true
	o.metricsError = nil
	o.logsLoading = true
	o.logsError = nil
	return tea.Batch(o.spinner.Tick, o.loadMetrics(), o.loadLogs())
}

// Update handles observability-specific messages and key events.
// Returns (cmd, handled) — if handled is true, the caller should not process
// the message further.
func (o *cloudRunObservability) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case crMetricsLoadedMsg:
		o.metricsLoading = false
		o.metricsError = nil
		o.metrics = msg.metrics
		o.metricsWarnings = msg.warnings
		return nil, true

	case crMetricsErrorMsg:
		o.metricsLoading = false
		o.metricsError = msg.err
		return nil, true

	case crLogsLoadedMsg:
		o.logsLoading = false
		o.logsError = nil
		o.logs = msg.logs
		return nil, true

	case crLogsErrorMsg:
		o.logsLoading = false
		o.logsError = msg.err
		return nil, true

	case crRefreshTickMsg:
		// Drop stale ticks if auto-refresh was turned off while waiting
		if !o.autoRefresh {
			return nil, true
		}
		o.metricsLoading = true
		o.logsLoading = true
		return tea.Batch(o.loadMetrics(), o.loadLogs(), o.tickAutoRefresh()), true

	case spinner.TickMsg:
		var cmd tea.Cmd
		o.spinner, cmd = o.spinner.Update(msg)
		return cmd, true

	case tea.KeyMsg:
		return o.handleKey(msg)
	}

	return nil, false
}

// handleKey processes keyboard shortcuts for the observability tab.
func (o *cloudRunObservability) handleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	// Time range selection (1-5)
	case "1":
		return o.setTimeRange(components.PredefinedTimeRanges[0].Duration), true
	case "2":
		return o.setTimeRange(components.PredefinedTimeRanges[1].Duration), true
	case "3":
		return o.setTimeRange(components.PredefinedTimeRanges[2].Duration), true
	case "4":
		return o.setTimeRange(components.PredefinedTimeRanges[3].Duration), true
	case "5":
		return o.setTimeRange(components.PredefinedTimeRanges[4].Duration), true

	// Auto-refresh toggle
	case "a":
		o.autoRefresh = !o.autoRefresh
		if o.autoRefresh {
			return o.StartAutoRefresh(), true
		}
		o.StopAutoRefresh()
		return nil, true

	// Severity filter toggles
	case "I":
		o.toggleSeverity("INFO")
		return nil, true
	case "W":
		o.toggleSeverity("WARNING")
		return nil, true
	case "E":
		o.toggleSeverity("ERROR")
		return nil, true
	}

	return nil, false
}

// setTimeRange updates the time range and reloads metrics+logs.
func (o *cloudRunObservability) setTimeRange(d time.Duration) tea.Cmd {
	if o.timeRange == d {
		return nil
	}
	o.timeRange = d
	o.metricsLoading = true
	o.logsLoading = true
	return tea.Batch(o.loadMetrics(), o.loadLogs())
}

// View renders the full observability tab content.
//
//nolint:gocognit // Observability rendering with multiple sections
func (o *cloudRunObservability) View() string {
	var b strings.Builder

	// Styles matching the instance details observability tab
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Bold(true)

	// Title
	b.WriteString(titleStyle.Render("Observability"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", max(0, min(o.width-4, 60))))
	b.WriteString("\n\n")

	// Time range selector
	lastFetch := time.Time{}
	if o.metrics != nil {
		lastFetch = o.metrics.LastFetch
	}
	selector := components.RenderTimeRangeSelector(o.timeRange, o.autoRefresh, lastFetch)
	b.WriteString(selector)
	b.WriteString("\n\n")

	// Loading state (first load, no cached data)
	if o.metricsLoading && o.metrics == nil {
		fmt.Fprintf(&b, "  %s Loading metrics...\n\n", o.spinner.View())
		return b.String()
	}

	// Error state without cached metrics
	if o.metricsError != nil && o.metrics == nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  ✗ Error loading metrics: %s", o.metricsError.Error())))
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("  Press 'r' to retry"))
		b.WriteString("\n")
		return b.String()
	}

	// Render metric sections (use cached metrics during background refresh)
	if o.metrics != nil {
		o.renderRequestMetrics(&b, sectionStyle, mutedStyle)
		o.renderResourceMetrics(&b, sectionStyle, mutedStyle)
	}

	// Show warnings for partially failed metric fetches
	if len(o.metricsWarnings) > 0 {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))
		b.WriteString(warnStyle.Render(fmt.Sprintf("  ⚠ Some metrics unavailable: %s", strings.Join(o.metricsWarnings, "; "))))
		b.WriteString("\n\n")
	}

	// Logs section
	o.renderLogs(&b, sectionStyle, mutedStyle, errorStyle)

	// Key hints
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  1-5: time range  a: auto-refresh  I/W/E: toggle severity  r: refresh"))
	b.WriteString("\n")

	return b.String()
}

// renderRequestMetrics renders the request count, latency, and error rate sections.
func (o *cloudRunObservability) renderRequestMetrics(b *strings.Builder, sectionStyle, mutedStyle lipgloss.Style) {
	m := o.metrics

	// Request count
	b.WriteString(sectionStyle.Render("Request Count"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if len(m.RequestCount) > 0 {
		o.requestCountChart.SetData(m.RequestCount)
		b.WriteString(o.requestCountChart.View())
	} else {
		b.WriteString(mutedStyle.Render("  No request data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Latency (p50/p95/p99) — overlaid in one chart
	b.WriteString(sectionStyle.Render("Request Latency"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	hasLatency := len(m.Latency50) > 0 || len(m.Latency95) > 0 || len(m.Latency99) > 0
	if hasLatency {
		o.latencyChart.SetDataSets([]metricchart.DataSet{
			{Name: "p50", Data: m.Latency50, Color: "#34A853"},
			{Name: "p95", Data: m.Latency95, Color: "#FBBC04"},
			{Name: "p99", Data: m.Latency99, Color: "#EA4335"},
		})
		b.WriteString(o.latencyChart.View())
	} else {
		b.WriteString(mutedStyle.Render("  No latency data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Error rates (4xx/5xx) — overlaid in one chart
	b.WriteString(sectionStyle.Render("Error Rate"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	hasErrors := len(m.ErrorCount4xx) > 0 || len(m.ErrorCount5xx) > 0
	if hasErrors {
		o.errorRateChart.SetDataSets([]metricchart.DataSet{
			{Name: "4xx", Data: m.ErrorCount4xx, Color: "#FBBC04"},
			{Name: "5xx", Data: m.ErrorCount5xx, Color: "#EA4335"},
		})
		b.WriteString(o.errorRateChart.View())
	} else {
		b.WriteString(mutedStyle.Render("  No error data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// renderResourceMetrics renders CPU, memory, and instance count sections.
func (o *cloudRunObservability) renderResourceMetrics(b *strings.Builder, sectionStyle, mutedStyle lipgloss.Style) {
	m := o.metrics

	// CPU usage — values are vCPU-seconds/second (1.0 = 1 full vCPU)
	b.WriteString(sectionStyle.Render("CPU Usage"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if len(m.CPU) > 0 {
		o.cpuChart.SetData(m.CPU)
		b.WriteString(o.cpuChart.View())
	} else {
		b.WriteString(mutedStyle.Render("  No CPU data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Billable instance time — proxy for activity level
	b.WriteString(sectionStyle.Render("Billable Instance Time"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if len(m.BillableInstanceTime) > 0 {
		o.billableTimeChart.SetData(m.BillableInstanceTime)
		b.WriteString(o.billableTimeChart.View())
	} else {
		b.WriteString(mutedStyle.Render("  No billable instance data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Instance count
	b.WriteString(sectionStyle.Render("Instance Count"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if len(m.InstanceCount) > 0 {
		o.instanceCountChart.SetData(m.InstanceCount)
		b.WriteString(o.instanceCountChart.View())
	} else {
		b.WriteString(mutedStyle.Render("  No instance count data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// renderLogs renders the log section with severity filter pills and log entries.
func (o *cloudRunObservability) renderLogs(b *strings.Builder, sectionStyle, mutedStyle, errorStyle lipgloss.Style) {
	b.WriteString(sectionStyle.Render("Logs"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")

	// Severity filter pills
	b.WriteString("  Filter: ")
	for i, sev := range []struct {
		key   string
		label string
		color string
	}{
		{"INFO", "INFO", "#9AA0A6"},
		{"WARNING", "WARN", "#FBBC04"},
		{"ERROR", "ERROR", "#EA4335"},
	} {
		if i > 0 {
			b.WriteString(" ")
		}
		if o.severityEnabled[sev.key] {
			// Active: bold + colored
			style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(sev.color))
			b.WriteString(style.Render(fmt.Sprintf("[%s]", sev.label)))
		} else {
			// Inactive: faint gray
			style := lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("#5F6368"))
			b.WriteString(style.Render(fmt.Sprintf("[%s]", sev.label)))
		}
	}
	b.WriteString("\n\n")

	// Log entries
	switch {
	case o.logsLoading && len(o.logs) == 0:
		fmt.Fprintf(b, "  %s Loading logs...\n", o.spinner.View())
	case o.logsError != nil && len(o.logs) == 0:
		b.WriteString(errorStyle.Render(fmt.Sprintf("  ✗ Error loading logs: %s", o.logsError.Error())))
		b.WriteString("\n")
	default:
		filtered := o.filteredLogs()
		if len(filtered) > 0 {
			for _, log := range filtered {
				var severityColor string
				switch log.Severity {
				case "ERROR", "CRITICAL":
					severityColor = "#EA4335"
				case "WARNING":
					severityColor = "#FBBC04"
				default:
					severityColor = "#9AA0A6"
				}
				sevStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(severityColor)).Bold(true)
				fmt.Fprintf(b, "  %s [%s] %s\n",
					log.Timestamp.Format("15:04:05"),
					sevStyle.Render(log.Severity),
					truncate(log.Message, max(o.width-30, 20)))
			}
		} else {
			b.WriteString(mutedStyle.Render("  No logs matching current filter"))
			b.WriteString("\n")
		}
	}
}

// loadMetrics fetches all Cloud Run metrics asynchronously.
// Individual metric failures are non-fatal: we collect what we can.
func (o *cloudRunObservability) loadMetrics() tea.Cmd {
	projectID := o.projectID
	serviceName := o.serviceName
	timeRange := o.timeRange
	client := o.gcpClient

	return func() tea.Msg {
		if client == nil {
			return crMetricsErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		monClient, err := client.GetMonitoringClient(projectID)
		if err != nil {
			return crMetricsErrorMsg{err: fmt.Errorf("monitoring client: %w", err)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		metrics := &gcp.CloudRunMetrics{LastFetch: time.Now()}
		var warnings []string

		// Fetch each metric independently; failures are non-fatal but surfaced as warnings
		var metricErr error
		metrics.RequestCount, metricErr = monClient.GetCloudRunRequestCount(ctx, serviceName, timeRange)
		if metricErr != nil {
			warnings = append(warnings, fmt.Sprintf("request count: %v", metricErr))
		}
		metrics.Latency50, metrics.Latency95, metrics.Latency99, metricErr = monClient.GetCloudRunRequestLatencies(ctx, serviceName, timeRange)
		if metricErr != nil {
			warnings = append(warnings, fmt.Sprintf("latency: %v", metricErr))
		}
		metrics.ErrorCount4xx, metricErr = monClient.GetCloudRunRequestCountByCode(ctx, serviceName, "4xx", timeRange)
		if metricErr != nil {
			warnings = append(warnings, fmt.Sprintf("4xx errors: %v", metricErr))
		}
		metrics.ErrorCount5xx, metricErr = monClient.GetCloudRunRequestCountByCode(ctx, serviceName, "5xx", timeRange)
		if metricErr != nil {
			warnings = append(warnings, fmt.Sprintf("5xx errors: %v", metricErr))
		}
		metrics.CPU, metricErr = monClient.GetCloudRunCPUUtilization(ctx, serviceName, timeRange)
		if metricErr != nil {
			warnings = append(warnings, fmt.Sprintf("CPU: %v", metricErr))
		}
		metrics.BillableInstanceTime, metricErr = monClient.GetCloudRunBillableInstanceTime(ctx, serviceName, timeRange)
		if metricErr != nil {
			warnings = append(warnings, fmt.Sprintf("billable time: %v", metricErr))
		}
		metrics.InstanceCount, metricErr = monClient.GetCloudRunInstanceCount(ctx, serviceName, timeRange)
		if metricErr != nil {
			warnings = append(warnings, fmt.Sprintf("instance count: %v", metricErr))
		}

		return crMetricsLoadedMsg{metrics: metrics, warnings: warnings}
	}
}

// loadLogs fetches Cloud Run logs asynchronously.
func (o *cloudRunObservability) loadLogs() tea.Cmd {
	projectID := o.projectID
	serviceName := o.serviceName
	timeRange := o.timeRange
	client := o.gcpClient

	return func() tea.Msg {
		if client == nil {
			return crLogsErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return crLogsErrorMsg{err: fmt.Errorf("logging client: %w", err)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Fetch logs within the selected time range; client-side severity filtering applies later
		logs, err := logClient.GetCloudRunLogs(ctx, serviceName, nil, timeRange, 50)
		if err != nil {
			return crLogsErrorMsg{err: err}
		}
		return crLogsLoadedMsg{logs: logs}
	}
}

// tickAutoRefresh schedules a single auto-refresh tick using Bubble Tea's timing.
// Each tick reschedules the next one, avoiding goroutine leaks from long-lived tickers.
func (o *cloudRunObservability) tickAutoRefresh() tea.Cmd {
	if !o.autoRefresh {
		return nil
	}
	return tea.Tick(30*time.Second, func(_ time.Time) tea.Msg {
		return crRefreshTickMsg{}
	})
}

// StartAutoRefresh schedules the next auto-refresh tick (30s interval).
func (o *cloudRunObservability) StartAutoRefresh() tea.Cmd {
	if !o.autoRefresh {
		return nil
	}
	return o.tickAutoRefresh()
}

// StopAutoRefresh is a no-op — refresh ticks are dropped in Update when autoRefresh is false.
func (o *cloudRunObservability) StopAutoRefresh() {
	// No resources to clean up; tea.Tick-based approach is self-cleaning
}

// toggleSeverity flips the enabled state for a severity level.
func (o *cloudRunObservability) toggleSeverity(level string) {
	o.severityEnabled[level] = !o.severityEnabled[level]
}

// activeSeverities returns the list of currently enabled severity levels.
// When ERROR is enabled, CRITICAL is implicitly included.
func (o *cloudRunObservability) activeSeverities() []string {
	var result []string
	for _, sev := range []string{"INFO", "WARNING", "ERROR"} {
		if o.severityEnabled[sev] {
			result = append(result, sev)
			if sev == "ERROR" {
				result = append(result, "CRITICAL")
			}
		}
	}
	return result
}

// filteredLogs returns logs matching the current severity filter.
func (o *cloudRunObservability) filteredLogs() []gcp.LogEntry {
	active := make(map[string]bool)
	for _, sev := range o.activeSeverities() {
		active[sev] = true
	}

	var result []gcp.LogEntry
	for _, entry := range o.logs {
		if active[entry.Severity] {
			result = append(result, entry)
		}
	}
	return result
}

// formatLatency formats milliseconds into human-readable form (e.g., "123ms", "1.5s", "2m 30s")
func formatLatency(ms float64) string {
	switch {
	case ms < 1:
		return fmt.Sprintf("%.2fms", ms)
	case ms < 1000:
		return fmt.Sprintf("%.0fms", ms)
	case ms < 60000:
		return fmt.Sprintf("%.1fs", ms/1000)
	default:
		mins := int(ms / 60000)
		secs := int(ms/1000) % 60
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
}

// extractValues pulls float64 values from a slice of DataPoints.
func extractValues(data []gcp.DataPoint) []float64 {
	values := make([]float64, len(data))
	for i, dp := range data {
		values[i] = dp.Value
	}
	return values
}
