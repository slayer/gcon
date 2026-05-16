package views

import (
	gocontext "context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

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
	return tea.Batch(s.spinner.Tick, s.fetchAllMetrics(), s.tickAutoRefresh())
}

// Refresh re-fetches all metrics, clearing prior warnings/errors.
func (s *gkeObservability) Refresh() tea.Cmd {
	s.loading = true
	s.err = nil
	s.warnings = map[string]error{}
	return tea.Batch(s.spinner.Tick, s.fetchAllMetrics())
}

// fetchAllMetrics issues all six monitoring queries sequentially within
// a single tea.Cmd goroutine. Per-metric errors are collected as warnings
// so one broken metric doesn't blank the whole tab (mirrors the LB pattern).
func (s *gkeObservability) fetchAllMetrics() tea.Cmd {
	if s.gcpClient == nil {
		return nil
	}
	duration := gkeObsRanges[s.rangeIdx]
	return func() tea.Msg {
		ctx := gocontext.Background()
		mc, err := s.gcpClient.GetMonitoringClient(s.projectID)
		if err != nil {
			return gkeObsMetricsLoadedMsg{warnings: map[string]error{"client": err}}
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
		return gkeObsMetricsLoadedMsg{metrics: metrics, warnings: warnings}
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

// Stub Update/View/SetSize/HasTextInputFocused — replaced in Task 9.
func (s *gkeObservability) Update(msg tea.Msg) tea.Cmd { _ = msg; return nil } // TODO: Task 9
func (s *gkeObservability) View() string               { return "" }           // TODO: Task 9
func (s *gkeObservability) SetSize(w, h int)           { s.width = w; _ = h }
func (s *gkeObservability) HasTextInputFocused() bool  { return false }
