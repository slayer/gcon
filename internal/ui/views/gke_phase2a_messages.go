package views

import (
	"github.com/slayer/gcon/internal/gcp"
)

// === Nodes tab ===

// gkeNodesPoolLoadedMsg carries one pool's worth of nodes back to the
// Nodes sub-view. Multiple of these arrive in parallel; the sub-view
// dedupes by Name and re-sorts on each. `gen` is the fan-out generation
// captured when fetchMIG was scheduled — stale responses (from a prior
// Refresh that has since been superseded) are dropped at receipt.
type gkeNodesPoolLoadedMsg struct {
	gen      int
	poolName string
	nodes    []gcp.GKENode
}

// gkeNodesPoolErrorMsg signals that one pool's MIG fetch failed.
// The Nodes sub-view continues rendering other pools' nodes; the
// failed pool is surfaced as an inline warning. `gen` mirrors
// gkeNodesPoolLoadedMsg for the same stale-response guard.
type gkeNodesPoolErrorMsg struct {
	gen      int
	poolName string
	err      error
}

// gkeNodesComputeClientReadyMsg carries a lazily-constructed compute client
// back to the Nodes sub-view. Emitted by gkeNodes.initComputeClient when
// the parent details view wasn't seeded with a compute client (e.g. the
// user navigated straight to GKE from the sidebar). The sub-view stitches
// the client into its state and proceeds with fanOut. `gen` matches the
// pattern used by other sub-view responses so a slow init that completes
// after a Refresh (which bumped generation) doesn't fire a second
// concurrent fan-out.
type gkeNodesComputeClientReadyMsg struct {
	gen    int
	client *gcp.ComputeClient
}

// === Observability tab ===

// gkeObsMetricsLoadedMsg carries the result of one full metric fan-out.
// The warnings map is per-metric error fallback so a single broken
// metric doesn't blank the whole tab (mirrors loadbalancer_observability's
// lbObsWarning accumulator). `gen` is the request generation captured
// at fetch time — the handler drops responses whose gen doesn't match
// the sub-view's current generation (stale range or stale refresh).
type gkeObsMetricsLoadedMsg struct {
	gen      int
	metrics  gcp.GKEMetrics
	warnings map[string]error // metric type key -> error
}

// gkeObsRefreshTickMsg is the auto-refresh tick. The sub-view's handler
// double-checks (autoRefresh && tabActive) on receipt — tea.Tick messages
// survive context switches (per .claude/rules/component-patterns.md).
type gkeObsRefreshTickMsg struct{}

// === Logs tab ===

// gkeLogsLoadedMsg carries the first page of log entries back to the Logs
// sub-view (replaces existing entries). The gcp.LogEntry type is shared
// with the dedicated Logs Explorer view. nextPageToken seeds the
// infinite-scroll LoadMore loop. `gen` is the query generation captured
// at fetch time — responses from a superseded toggle / resource / range
// change are dropped at receipt instead of stomping on the new state.
type gkeLogsLoadedMsg struct {
	gen           int
	entries       []gcp.LogEntry
	nextPageToken string
}

// gkeLogsMoreLoadedMsg carries a follow-up page from LoadMore — entries
// are APPENDED to whatever is already showing, not replaced.
type gkeLogsMoreLoadedMsg struct {
	gen           int
	entries       []gcp.LogEntry
	nextPageToken string
}

type gkeLogsErrorMsg struct {
	gen int
	err error
}

type gkeLogsRefreshTickMsg struct{}
