package views

import (
	gocontext "context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/metricchart"
)

// gkeObservability is the Observability sub-view. Owns five charts and
// the auto-refresh ticker. tabActive guard prevents stale ticks
// (per .claude/rules/component-patterns.md — tea.Tick survives
// context switches).
type gkeObservability struct {
	projectID, location, clusterName string
	gcpClient                        *gcp.Client

	cpuChart    *metricchart.Chart
	memoryChart *metricchart.Chart
	nodeChart   *metricchart.Chart
	podChart    *metricchart.Chart
	netChart    *metricchart.Chart

	metrics     gcp.GKEMetrics
	warnings    map[string]error
	rangeIdx    int // 0..4 → 1h/6h/24h/7d/30d
	autoRefresh bool
	tabActive   bool

	// generation increments on every Refresh / Init. Each fetch captures
	// the current value and tags its response message with it; the Update
	// handler drops responses whose tag is stale. Without this, a slow 7d
	// response could land after the user has switched to 1h and overwrite
	// the new chart with old-range data.
	generation int

	spinner spinner.Model
	loading bool
	err     error
	width   int
}

var gkeObsRanges = []time.Duration{
	1 * time.Hour,
	6 * time.Hour,
	24 * time.Hour,
	7 * 24 * time.Hour,
	30 * 24 * time.Hour,
}

var gkeObsRangeLabels = []string{"1h", "6h", "24h", "7d", "30d"}

func newGKEObservability(projectID, location, clusterName string, client *gcp.Client) *gkeObservability {
	cpu := metricchart.New(metricchart.HeightStandard).
		SetYRange(0, 100).
		SetStatsFormatter(metricchart.FormatPercentageStats).
		SetYLabelFormatter(metricchart.PercentYLabel)
	mem := metricchart.New(metricchart.HeightStandard).
		SetYRange(0, 100).
		SetStatsFormatter(metricchart.FormatPercentageStats).
		SetYLabelFormatter(metricchart.PercentYLabel)
	node := metricchart.New(metricchart.HeightStandard)
	pod := metricchart.New(metricchart.HeightStandard)
	net := metricchart.New(metricchart.HeightStandard) // multi-series via SetDataSets
	return &gkeObservability{
		projectID:   projectID,
		location:    location,
		clusterName: clusterName,
		gcpClient:   client,
		cpuChart:    cpu,
		memoryChart: mem,
		nodeChart:   node,
		podChart:    pod,
		netChart:    net,
		warnings:    map[string]error{},
		autoRefresh: true,
		spinner:     components.NewGCPSpinner(),
	}
}

// SetTabActive marks the tab as visible/hidden. While inactive, auto-refresh
// ticks are dropped (tea.Tick messages survive context switches).
func (s *gkeObservability) SetTabActive(active bool) { s.tabActive = active }

// Init triggers the first metrics fetch and schedules the first auto-refresh tick.
func (s *gkeObservability) Init() tea.Cmd {
	s.loading = true
	s.generation++
	return tea.Batch(s.spinner.Tick, s.fetchAllMetrics(), s.tickAutoRefresh())
}

// Refresh re-fetches all metrics, clearing prior warnings/errors AND
// the cached metric series so the View() falls back to its loading
// state. Without the metric clear, switching time ranges would keep
// the previous range's charts on screen for the duration of the new
// fetch — disorienting because the chart values silently swap once
// the new data arrives.
func (s *gkeObservability) Refresh() tea.Cmd {
	s.loading = true
	s.err = nil
	s.warnings = map[string]error{}
	s.metrics = gcp.GKEMetrics{}
	s.generation++
	return tea.Batch(s.spinner.Tick, s.fetchAllMetrics())
}

// fetchAllMetrics issues all six monitoring queries sequentially within
// a single tea.Cmd goroutine. Per-metric errors are collected as warnings
// so one broken metric doesn't blank the whole tab (mirrors the LB pattern).
// The current generation is captured at call time and tagged into the
// response, so the Update handler can drop responses from a superseded
// range/refresh.
func (s *gkeObservability) fetchAllMetrics() tea.Cmd {
	if s.gcpClient == nil {
		return nil
	}
	duration := gkeObsRanges[s.rangeIdx]
	gen := s.generation
	return func() tea.Msg {
		ctx := gocontext.Background()
		mc, err := s.gcpClient.GetMonitoringClient(s.projectID)
		if err != nil {
			return gkeObsMetricsLoadedMsg{gen: gen, warnings: map[string]error{"client": err}}
		}
		var (
			metrics  gcp.GKEMetrics
			warnings = map[string]error{}
		)
		if v, err := mc.GetGKEClusterCPUUtilization(ctx, s.location, s.clusterName, duration); err != nil {
			warnings["cpu"] = err
		} else {
			metrics.CPUUtilization = v
		}
		if v, err := mc.GetGKEClusterMemoryUtilization(ctx, s.location, s.clusterName, duration); err != nil {
			warnings["memory"] = err
		} else {
			metrics.MemoryUtilization = v
		}
		if v, err := mc.GetGKEClusterNodeCount(ctx, s.location, s.clusterName, duration); err != nil {
			warnings["nodecount"] = err
		} else {
			metrics.NodeCount = v
		}
		if v, err := mc.GetGKEClusterPodCount(ctx, s.location, s.clusterName, duration); err != nil {
			warnings["podcount"] = err
		} else {
			metrics.PodCount = v
		}
		if rx, tx, err := mc.GetGKEClusterNetworkBytes(ctx, s.location, s.clusterName, duration); err != nil {
			warnings["network"] = err
		} else {
			metrics.NetworkRxBytes = rx
			metrics.NetworkTxBytes = tx
		}
		metrics.LastFetch = time.Now()
		return gkeObsMetricsLoadedMsg{gen: gen, metrics: metrics, warnings: warnings}
	}
}

// tickAutoRefresh schedules a single auto-refresh tick. The handler at
// receive time double-checks (autoRefresh && tabActive) — tea.Tick
// messages survive context switches and cannot be canceled.
func (s *gkeObservability) tickAutoRefresh() tea.Cmd {
	if !s.autoRefresh || !s.tabActive {
		return nil
	}
	return tea.Tick(30*time.Second, func(_ time.Time) tea.Msg { return gkeObsRefreshTickMsg{} })
}

// SetSize updates the rendering width and propagates it to all charts.
func (s *gkeObservability) SetSize(w, h int) {
	s.width = w
	_ = h
	cw := w - 2
	if cw < 10 {
		cw = 10
	}
	s.cpuChart.Resize(cw)
	s.memoryChart.Resize(cw)
	s.nodeChart.Resize(cw)
	s.podChart.Resize(cw)
	s.netChart.Resize(cw)
}

// HasTextInputFocused reports whether the sub-view owns a focused text input.
// The Observability tab has no text inputs.
func (s *gkeObservability) HasTextInputFocused() bool { return false }

// Update routes messages.
func (s *gkeObservability) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gkeObsMetricsLoadedMsg:
		// Drop responses from a superseded request — e.g. a slow 7d
		// fetch that lands after the user switched to 1h and a new
		// fetch is already in flight. Without this guard the stale
		// response would overwrite the chart with wrong-range data
		// while the range bar advertises the new range.
		if m.gen != s.generation {
			return nil
		}
		s.loading = false
		s.metrics = m.metrics
		s.warnings = m.warnings
		if s.warnings == nil {
			s.warnings = map[string]error{}
		}
		s.applyDataToCharts()
		return nil
	case gkeObsRefreshTickMsg:
		if !s.autoRefresh || !s.tabActive {
			return nil
		}
		return tea.Batch(s.fetchAllMetrics(), s.tickAutoRefresh())
	case spinner.TickMsg:
		if !s.loading {
			return nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(m)
		return cmd
	case tea.KeyMsg:
		return s.handleKey(m)
	}
	return nil
}

// handleKey processes keyboard input scoped to the Observability tab.
func (s *gkeObservability) handleKey(m tea.KeyMsg) tea.Cmd {
	switch m.String() {
	case "1", "2", "3", "4", "5":
		idx := int(m.String()[0] - '1')
		if idx < 0 || idx >= len(gkeObsRanges) {
			return nil
		}
		if idx != s.rangeIdx {
			s.rangeIdx = idx
			return s.Refresh()
		}
		// Same range pressed — keep rangeIdx unchanged.
		s.rangeIdx = idx
	case "a":
		s.autoRefresh = !s.autoRefresh
		if s.autoRefresh {
			return s.tickAutoRefresh()
		}
	case "r":
		return s.Refresh()
	}
	return nil
}

// applyDataToCharts pushes the latest metric data into each chart's
// internal state. Called after every successful fan-out.
func (s *gkeObservability) applyDataToCharts() {
	s.cpuChart.SetData(s.metrics.CPUUtilization)
	s.memoryChart.SetData(s.metrics.MemoryUtilization)
	s.nodeChart.SetData(s.metrics.NodeCount)
	s.podChart.SetData(s.metrics.PodCount)
	s.netChart.SetDataSets([]metricchart.DataSet{
		{Name: "rx", Data: s.metrics.NetworkRxBytes, Color: "#34A853"},
		{Name: "tx", Data: s.metrics.NetworkTxBytes, Color: "#FBBC04"},
	})
}

// View renders the Observability tab.
func (s *gkeObservability) View() string {
	if s.loading && len(s.metrics.CPUUtilization) == 0 && len(s.metrics.MemoryUtilization) == 0 {
		// Surface the range being loaded so the user knows what they're
		// waiting on after pressing 1-5.
		rangeLabel := gkeObsRangeLabels[s.rangeIdx]
		return renderLoading(s.spinner, "Loading metrics ("+rangeLabel+")...")
	}
	var b strings.Builder
	b.WriteString(s.renderRangeBar())
	b.WriteString("\n\n")
	// If Monitoring client construction itself failed, every metric is
	// unrenderable and the per-chart warnings won't fire. Surface it once
	// at the top so the user sees the auth/API error instead of five
	// empty charts.
	if clientErr, ok := s.warnings["client"]; ok {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		b.WriteString(errStyle.Render("  ⚠ Monitoring client: " + clientErr.Error()))
		b.WriteString("\n\n")
	}
	b.WriteString(s.renderChart("Cluster CPU utilization", "cpu", s.cpuChart))
	b.WriteString("\n")
	b.WriteString(s.renderChart("Cluster Memory utilization", "memory", s.memoryChart))
	b.WriteString("\n")
	b.WriteString(s.renderChart("Node count", "nodecount", s.nodeChart))
	b.WriteString("\n")
	b.WriteString(s.renderChart("Pod count", "podcount", s.podChart))
	b.WriteString("\n")
	b.WriteString(s.renderChart("Network traffic", "network", s.netChart))
	return b.String()
}

// renderRangeBar draws the [1h] [6h] [24h] [7d] [30d] strip with the
// active range highlighted, followed by the auto-refresh indicator.
func (s *gkeObservability) renderRangeBar() string {
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	active := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	var b strings.Builder
	for i, label := range gkeObsRangeLabels {
		if i == s.rangeIdx {
			b.WriteString(active.Render("[" + label + "]"))
		} else {
			b.WriteString(muted.Render("[" + label + "]"))
		}
		b.WriteString(" ")
	}
	autoState := "off"
	if s.autoRefresh {
		autoState = "on"
	}
	b.WriteString(muted.Render("    auto-refresh: " + autoState + " (a to toggle)"))
	return b.String()
}

// renderChart renders a single chart with its title. When a warning is
// present for the chart's metric key, the chart is replaced by an
// inline error message (mirrors loadbalancer_observability's pattern).
func (s *gkeObservability) renderChart(title, warnKey string, chart *metricchart.Chart) string {
	var b strings.Builder
	section := lipgloss.NewStyle().Bold(true)
	b.WriteString(section.Render(title))
	b.WriteString("\n")
	if err, ok := s.warnings[warnKey]; ok {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		b.WriteString(errStyle.Render("  ⚠ " + warnKey + ": " + err.Error()))
		b.WriteString("\n")
		return b.String()
	}
	b.WriteString(chart.View())
	b.WriteString("\n")
	return b.String()
}
