package views

import (
	"github.com/slayer/gcon/internal/gcp"
)

// === Nodes tab ===

// gkeNodesPoolLoadedMsg carries one pool's worth of nodes back to the
// Nodes sub-view. Multiple of these arrive in parallel; the sub-view
// dedupes by Name and re-sorts on each.
type gkeNodesPoolLoadedMsg struct {
	poolName string
	nodes    []gcp.GKENode
}

// gkeNodesPoolErrorMsg signals that one pool's MIG fetch failed.
// The Nodes sub-view continues rendering other pools' nodes; the
// failed pool is surfaced as an inline warning.
type gkeNodesPoolErrorMsg struct {
	poolName string
	err      error
}

// gkeNodesComputeClientReadyMsg carries a lazily-constructed compute client
// back to the Nodes sub-view. Emitted by gkeNodes.initComputeClient when
// the parent details view wasn't seeded with a compute client (e.g. the
// user navigated straight to GKE from the sidebar). The sub-view stitches
// the client into its state and proceeds with fanOut.
type gkeNodesComputeClientReadyMsg struct {
	client *gcp.ComputeClient
}

// === Observability tab ===

// gkeObsMetricsLoadedMsg carries the result of one full metric fan-out.
// The warnings map is per-metric error fallback so a single broken
// metric doesn't blank the whole tab (mirrors loadbalancer_observability's
// lbObsWarning accumulator).
type gkeObsMetricsLoadedMsg struct {
	metrics  gcp.GKEMetrics
	warnings map[string]error // metric type key -> error
}

// gkeObsRefreshTickMsg is the auto-refresh tick. The sub-view's handler
// double-checks (autoRefresh && tabActive) on receipt — tea.Tick messages
// survive context switches (per .claude/rules/component-patterns.md).
type gkeObsRefreshTickMsg struct{}

// === Logs tab ===

// gkeLogsLoadedMsg carries one page of log entries back to the Logs
// sub-view. The gcp.LogEntry type is shared with the dedicated Logs
// Explorer view.
type gkeLogsLoadedMsg struct {
	entries []gcp.LogEntry
}

type gkeLogsErrorMsg struct {
	err error
}

type gkeLogsRefreshTickMsg struct{}
