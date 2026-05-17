# GKE Phase 2a — Nodes + Observability + Logs tabs

## Goal

Add three new tabs to the cluster details view shipped in Phase 1:

- **Nodes** — flat table of every node across all pools, derived from each
  pool's underlying Managed Instance Group via Compute Engine API. No
  Kubernetes API access. Enter on a node row navigates to the existing
  Compute Engine instance details view.
- **Observability** — five cluster-scope charts (CPU%, Memory%, Node
  count, Pod count, Network rx/tx) using the existing `metricchart`
  package. GCP has no `kubernetes.io/cluster/*` family, so cluster
  aggregates are computed by cross-series reduction over
  `kubernetes.io/node/*` and `kubernetes.io/pod/*` metrics — see
  `.claude/rules/gcp-api-gotchas.md` for the per-metric mapping. Time
  range selector (`1`–`5`) and 30 s auto-refresh.
- **Logs** — filterable embedded log viewer reading from Cloud Logging
  for `k8s_cluster` / `k8s_node` / `k8s_pod` / `k8s_container` resources.
  Severity toggles (`I` / `W` / `E`), resource-type cycler (`R` advances
  through the four resource types and wraps), 15 s auto-refresh (opt-in),
  `L` to jump to the dedicated Logs Explorer view pre-filtered to this
  cluster.

All read-only. No node drain/cordon, no kubectl-style workload browsing,
no Kubernetes API client.

This is **Phase 2a of two** within Phase 2:

- **Phase 2a (this design)**: Nodes + Observability + Logs tabs.
- **Phase 2b** (separate brainstorm later): node pool create/delete +
  cluster/pool resize + master + node-pool upgrade. Mutations only.

Splitting 2 into 2a (read-only) and 2b (mutations) isolates the
heterogeneous risk: mutations have async lifecycles and edge cases that
shouldn't gate ship of the safe read-only surface.

## Non-goals

- Kubernetes API access (workloads, pods, services, exec, logs-via-kubectl).
- Node-level mutations (drain, cordon, delete). VM-level actions
  (stop/start/reset) work via the existing instance details view.
- Per-node CPU/memory charts on the Observability tab. Would require
  one Monitoring query per node per metric — out of scope for 2a.
- Node pool create/delete/resize, cluster/pool upgrade — Phase 2b.

## Tab layout after Phase 2a

| # | Tab          | Phase  | Notes                                                |
|---|--------------|--------|------------------------------------------------------|
| 1 | Overview     | 1      | Unchanged.                                           |
| 2 | Node Pools   | 1      | Unchanged. Phase 2b adds actions here.               |
| 3 | Nodes        | 2a     | New. Flat node list across all pools.                |
| 4 | Observability| 2a     | New. Five charts, range selector, auto-refresh.      |
| 5 | Logs         | 2a     | New. Filterable embedded log viewer.                 |

## Scope checklist

| Item | In Phase 2a? |
|---|---|
| Nodes list (MIG-derived) | Yes |
| Per-node status / IP / age | Yes |
| Per-node CPU / memory charts | Phase 2b+ |
| Sort + filter on Nodes tab | Yes |
| Navigate to Compute instance details from a node row | Yes |
| Observability tab — 5 cluster-scope charts | Yes |
| Time range selector (1h / 6h / 24h / 7d / 30d) | Yes |
| Auto-refresh on Observability (30 s, default on) | Yes |
| Logs tab — severity toggles + resource-type filter | Yes |
| Logs tab — auto-refresh (15 s, opt-in) | Yes |
| `L` opens dedicated Logs Explorer with cluster pre-filtered | Yes |
| Operation history view | No |
| Node drain / cordon | Out of scope (Kubernetes API) |

## Architecture

### New GCP-client methods

**`internal/gcp/monitoring_gke.go` (new file)** — mirrors the per-feature
monitoring layer pattern from `monitoring_lb.go` and `monitoring_cloudrun.go`:

```go
type GKEMetrics struct {
    CPUUtilization      []DataPoint // % of allocatable (0–100)
    MemoryUtilization   []DataPoint // % of allocatable (0–100)
    NodeCount           []DataPoint // integer count
    PodCount            []DataPoint // integer count
    NetworkRxBytes      []DataPoint // bytes/sec
    NetworkTxBytes      []DataPoint // bytes/sec
    LastFetch           time.Time
}

func (c *MonitoringClient) GetGKEClusterCPUUtilization(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error)
func (c *MonitoringClient) GetGKEClusterMemoryUtilization(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error)
func (c *MonitoringClient) GetGKEClusterNodeCount(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error)
func (c *MonitoringClient) GetGKEClusterPodCount(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error)
func (c *MonitoringClient) GetGKEClusterNetworkBytes(ctx context.Context, location, clusterName string, duration time.Duration) (rx, tx []DataPoint, err error)
```

Filter shape — single `resource.type` per fetch per the
`gcp-api-gotchas.md` rule (no `OR` between resource types):

```go
// Cluster-scope metrics: k8s_cluster
filter := fmt.Sprintf(
    `resource.type = "k8s_cluster" AND resource.labels.cluster_name = %q AND resource.labels.location = %q AND metric.type = %q`,
    clusterName, location, metricType,
)

// Node-scope metrics (network traffic): k8s_node, REDUCE_SUM across nodes
filter := fmt.Sprintf(
    `resource.type = "k8s_node" AND resource.labels.cluster_name = %q AND resource.labels.location = %q AND metric.type = %q`,
    clusterName, location, metricType,
)
```

Metric type strings:

| Field            | Metric type                                                     | Resource     | Aggregation        |
|------------------|----------------------------------------------------------------|--------------|--------------------|
| CPUUtilization   | `kubernetes.io/cluster/cpu/allocatable_utilization`             | `k8s_cluster`| ALIGN_MEAN / MEAN  |
| MemoryUtilization| `kubernetes.io/cluster/memory/allocatable_utilization`          | `k8s_cluster`| ALIGN_MEAN / MEAN  |
| NodeCount        | `kubernetes.io/cluster/node_count`                              | `k8s_cluster`| ALIGN_MEAN / SUM   |
| PodCount         | `kubernetes.io/cluster/pod_count`                               | `k8s_cluster`| ALIGN_MEAN / SUM   |
| NetworkRxBytes   | `kubernetes.io/node/network/received_bytes_count`               | `k8s_node`   | ALIGN_RATE / SUM   |
| NetworkTxBytes   | `kubernetes.io/node/network/sent_bytes_count`                   | `k8s_node`   | ALIGN_RATE / SUM   |

The `allocatable_utilization` metrics are already in the range 0.0–1.0 —
multiply by 100 at the data-layer (in `GetGKEClusterCPUUtilization`) so
chart consumers see 0–100 and can use `SetYRange(0, 100)`.

**`internal/gcp/compute.go` addition** — `ListManagedInstances` returns
raw instance metadata; it does NOT know about GKE node pools:

```go
// MIGInstance is the subset of compute.Instance fields we need for
// node enumeration. Kept in the compute layer (not gke.go) because
// listing MIG instances is a generic Compute Engine capability.
type MIGInstance struct {
    Name         string
    Zone         string
    Status       string  // PROVISIONING / STAGING / RUNNING / STOPPING / TERMINATED
    InternalIP   string
    ExternalIP   string  // "" when no external NIC
    CreatedAt    string  // raw CreationTimestamp pass-through
}

// ListManagedInstances enumerates the VMs in a Managed Instance Group.
// MIG instance URLs are followed and projected to MIGInstance. Used by
// GKE Phase 2a for node enumeration.
func (c *ComputeClient) ListManagedInstances(ctx context.Context, projectID, zone, migName string) ([]MIGInstance, error)
```

**`internal/gcp/gke.go` addition** — `GKENode` adds the pool name on top
of `MIGInstance`. The pool isn't derived from the MIG name (pool names
can contain hyphens, making `gke-<cluster>-<pool>-<hash>-grp` parsing
ambiguous); instead, the caller in `gke_nodes.go` knows which pool a
MIG belongs to from context — it's iterating `pool.InstanceGroupUrls`.

```go
type GKENode struct {
    MIGInstance         // embed
    Pool        string  // set by the caller from loop context
}
```

### New view files

| File                                       | Responsibility                                                              |
|--------------------------------------------|-----------------------------------------------------------------------------|
| `internal/ui/views/gke_nodes.go`           | Sub-view for the Nodes tab. Owns the nodes table.                           |
| `internal/ui/views/gke_observability.go`   | Sub-view for the Observability tab. Owns charts + range + auto-refresh.     |
| `internal/ui/views/gke_logs.go`            | Sub-view for the Logs tab. Owns severity toggles + resource dropdown.       |
| `internal/ui/views/gke_phase2a_messages.go`| Internal lifecycle messages for the three sub-views.                        |

Each sub-view is a struct, not a `tea.Model` — same pattern as
`loadbalancer_observability.go` and the existing GKE Phase 1 details
view. The parent `GKEClusterDetailsView` calls `Update(msg) tea.Cmd` and
`View() string` directly.

### State integration

```go
type GKEClusterDetailsView struct {
    // ... existing fields (Phase 1)
    observability *gkeObservability
    nodes         *gkeNodes
    logs          *gkeLogs
}
```

All three start nil. `tabs.TabChangedMsg` lazy-creates the active
sub-view (per `adding-new-views.md` step 14). Tab order grows to five:

```go
v.tabs = tabs.New([]tabs.Tab{
    {ID: "overview",      Label: "Overview"},
    {ID: "nodepools",     Label: "Node Pools"},
    {ID: "nodes",         Label: "Nodes"},
    {ID: "observability", Label: "Observability"},
    {ID: "logs",          Label: "Logs"},
})
```

## Tab specifications

### Nodes tab

Flat table over every node in every pool. Loading is a fan-out — one
`ListManagedInstances` per (pool × pool-location × MIG URL), deduped by
node name.

**Columns:**

| Column      | Source                                | Notes                                                                                                |
|-------------|---------------------------------------|------------------------------------------------------------------------------------------------------|
| Name        | `Instance.Name`                       | Full GKE-mangled name.                                                                               |
| Pool        | Stamped from loop context             | Pool name is the `NodePool.Name` of the iterator emitting this MIG — never parsed from the MIG name, since pool names can contain hyphens. |
| Zone        | From the MIG URL's `/zones/{zone}/` segment | Parsed in `parseMIGURL` (see `gke_nodes.go`).                                                  |
| Status      | `Instance.Status`                     | RUNNING / PROVISIONING / etc.                                                                        |
| Internal IP | `Instance.NetworkInterfaces[0].NetworkIP` | Empty during PROVISIONING.                                                                       |
| Age         | `Instance.CreationTimestamp`          | Render via `formatAge` helper.                                                                       |

**Keys** (see `key-bindings.md` update below):

| Key | Action |
|-----|--------|
| `Enter` | Navigate to Compute Engine instance details (cross-view). |
| `S` | Open sort menu (Name asc by default). |
| `/` | Filter (supports `pool:`, `status:`, `zone:`). |
| `r` | Refresh node list. |
| `Esc` | Go back. |

**Cross-view navigation message** — emit
`InstanceSelectedMsg{ProjectID, Zone, Name}` to reuse the existing
instance details navigation. Verify the handler exists; if not, follow
the precedent on the Compute Engine list view.

Per `adding-new-views.md` step 16, the app handler for
`InstanceSelectedMsg` must resolve `computeClient` from the GKE details
view (`gkeClusterDetailsView.GetComputeClient()`) if it isn't already.

**Loading shape**:

```
1. TabChangedMsg → sub-view created → Init() → kicks off fetches
2. For each NodePool in cluster:
     For each zone in pool.Locations:
       For each migURL in pool.InstanceGroupUrls:
         Emit a tea.Cmd that calls ListManagedInstances and returns
         gkeNodesPoolLoadedMsg{poolName, zone, nodes}
3. Sub-view accumulates results, deduplicates by Name, sorts.
4. When all pending fetches complete, loading flag clears.
```

`gkeNodesPoolLoadedMsg` carries one pool's worth so the table can
render incrementally — same shape as the LB backend health fan-out.

### Observability tab

Five charts stacked vertically. The tab body is rendered into a
`viewport.Model`-wrapped container (Phase 1 already wired the viewport
on the details view; this tab gains scrollability for free).

**Charts** — each chart uses `metricchart.New(metricchart.HeightStandard)`
(8 rows of braille-time-series, matching the VM Instances Observability
tab). NOT compact single-line sparklines. The tab body scrolls inside
the existing `bubbles/viewport` wrap, so multiple full-height charts
stack cleanly:

```
[1h] [6h] [24h] [7d] [30d]                                            ● auto-refresh (a)

Cluster CPU utilization                                                (allocatable)
 100% ┤
  75% ┤                              ⢀⣀⡀
  50% ┤                  ⢀⡠⠤⠒⠉⠁         ⠉⠒⠤⢄⡀
  25% ┤    ⡠⠔⠊⠉⠒⠤⢄⣀⡀⢀⡠⠊                       ⠉⠒⠤⢀⣀⡀
   0% ┴─────────────────────────────────────────────────────
        1h ago                                              now
   current: 38.4%   avg: 31.2%   max: 47.1%

Cluster Memory utilization                                             (allocatable)
 100% ┤
  75% ┤
  50% ┤              ⡠⠔⠉⠉⠉⠉⠒⠤⠤⠤⠤⠤⠤⠒⠉⠉⠉⠒⠤⢄
  25% ┤  ⡠⠔⠉⠉⠒⠒⠊⠁                            ⠉⠉⠉⠉⠉⠒⠤
   0% ┴─────────────────────────────────────────────────────
        1h ago                                              now
   current: 54.7%   avg: 51.8%   max: 58.2%

Node count
                                                ⢀⡀⡀⢀⡀
   12 ┤  ⡠⠔⠉⠉⠒⠒⠒⠉⠉⠉⠒⠒⠊⠉⠉⠒⠊⠉⠁
   …
   current: 12   avg: 12   max: 12

Pod count                                                              (running)
   ⋯ 8-row braille chart, same formatters as VM instance details ⋯
   current: 142   avg: 118   max: 158

Network traffic                                                        (rx green, tx yellow)
   ⋯ 8-row braille chart, 2-series overlay via SetDataSets() ⋯
   rx 142 MB/s   tx 89 MB/s
```
- CPU & Memory: `SetYRange(0, 100)`, `PercentYLabel`, percentage stats formatter.
- Node count, Pod count: integer-display, default formatter.
- Network: 2-series overlay (`SetDataSets`) — `rx` (green `#34A853`) and `tx` (yellow `#FBBC04`), `humanYLabel`.

**Fetch coordinator** — single tea.Cmd that issues all six metric calls
sequentially within one goroutine and returns
`gkeObsMetricsLoadedMsg{gen, GKEMetrics, warnings}`. Per-metric errors
don't abort the whole tab; the sub-view holds a warnings accumulator
(mirrors `loadbalancer_observability.go`). Sequential rather than
parallel: total latency is the sum of 6 calls (~1–2s in practice), and
the simpler control flow keeps the gen-token race guard trivial. A
parallel variant is reasonable future work but isn't required for
Phase 2a's load profile.

**Keys:**

| Key | Action |
|-----|--------|
| `1`–`5` | Time range 1h / 6h / 24h / 7d / 30d. |
| `a` | Toggle auto-refresh (default on). |
| `r` | Manual refresh. |

**Auto-refresh:** `tea.Tick(30*time.Second, ...)` guarded by
`(autoRefresh && tabActive)` — same shape as LB Phase 2. The
component-patterns rule on `tea.Tick` stale-message survival applies
directly.

**Empty/loading states:** while the first fetch is in flight, render a
spinner above the first chart and "Loading metrics…" below each
placeholder. Per-chart errors render inline as a single muted line in
place of the sparkline.

### Logs tab

Embedded log viewer that fetches via the existing `LoggingClient` and
renders the resulting entries inline (single line per entry: timestamp +
severity + payload). The shared `logviewer` component was considered but
its focused-entry / expansion model is overkill for the cluster-scope
filter set; inline rendering keeps the sub-view ~250 lines and reuses
the parent details view's viewport for scroll. Infinite scroll appends
follow-up pages via `LoadMore()` when the viewport reaches the bottom.

**Default LQL** built at sub-view construction:

```
resource.type = "k8s_cluster"
AND resource.labels.cluster_name = "<name>"
AND resource.labels.location = "<location>"
AND timestamp >= "<now − 1h, RFC3339>"
AND (severity < WARNING OR severity = WARNING OR severity >= ERROR)
```

The 1h timestamp bound keeps each refresh cheap; the severity bucket is
built as a disjunction so toggling individual levels off genuinely
excludes them (no "lowest enabled threshold" leakage).

**Filter controls:**

| Key | Action |
|-----|--------|
| `I` | Toggle INFO/NOTICE/DEBUG bucket (default on). |
| `W` | Toggle WARNING (default on). |
| `E` | Toggle ERROR/CRITICAL/ALERT/EMERGENCY bucket (default on). |
| `R` | Cycle resource type: `k8s_cluster` → `k8s_node` → `k8s_pod` → `k8s_container` → wraps. |
| `a` | Toggle 15 s auto-refresh (default OFF — logs view is bursty). |
| `r` | Manual refresh (re-run query). |
| `L` | Open the full Logs Explorer view with this cluster's filter pre-populated. |
| `Esc` | Back. |

Severity toggles work by rewriting the `severity >=` clause and re-fetching.
Resource type changes rewrite `resource.type` and re-fetch.

`L` emits `LogsExplorerOpenMsg{Query: "<built LQL>"}` (or whatever the
existing Cloud Run observability uses — verify the message name at
implementation time).

**Page size:** 100 entries per fetch (matches existing Logs Explorer).
Older entries via infinite scroll: when the parent details view's
viewport hits the bottom on the Logs tab, it calls the sub-view's
`LoadMore()` which appends the next page using the captured query (so
the `timestamp >=` lower bound doesn't drift between pages). No `]`
keybinding — the scroll trigger is enough and matches how every other
tall tab in the app handles "more".

## Auto-refresh + tabActive lifecycle

Each sub-view that ticks holds:
- `autoRefresh bool` — user preference.
- `tabActive bool` — set by parent when this tab is active.

Parent `tabs.TabChangedMsg` handler:

```go
case tabs.TabChangedMsg:
    if v.observability != nil { v.observability.SetTabActive(false) }
    if v.logs != nil          { v.logs.SetTabActive(false) }
    if v.nodes != nil         { v.nodes.SetTabActive(false) }

    switch v.tabs.ActiveTab().ID {
    case "nodes":
        if v.nodes == nil { v.nodes = newGKENodes(v.projectID, v.details, v.computeClient) }
        v.nodes.SetTabActive(true)
        v.viewport.GotoTop()
        return v.nodes.Init()
    case "observability":
        if v.observability == nil { v.observability = newGKEObservability(v.projectID, v.location, v.name, v.gcpClient) }
        v.observability.SetTabActive(true)
        v.viewport.GotoTop()
        return v.observability.Init()
    case "logs":
        if v.logs == nil { v.logs = newGKELogs(v.projectID, v.location, v.name, v.gcpClient) }
        v.logs.SetTabActive(true)
        v.viewport.GotoTop()
        return v.logs.Init()
    default:
        v.viewport.GotoTop()
    }
```

Stale-tick guard per `component-patterns.md`:

```go
func (s *gkeObservability) tickAutoRefresh() tea.Cmd {
    if !s.autoRefresh || !s.tabActive {
        return nil
    }
    return tea.Tick(30*time.Second, func(_ time.Time) tea.Msg {
        return gkeObsRefreshTickMsg{}
    })
}

case gkeObsRefreshTickMsg:
    if !s.autoRefresh || !s.tabActive {
        return nil
    }
    return tea.Batch(s.fetchAllMetrics(), s.tickAutoRefresh())
```

Same pattern in `gkeLogs` with 15 s interval and default-off.

## Error handling

Per `async-operations-error-handling.md`:

- Each sub-view holds `err error` and a `SetError(err)` method.
- Per-metric failures (Observability) surface inline as muted text on
  the failing chart only — other charts render fine.
- Whole-fetch failures (Nodes, Logs) show the standard error block
  via `components.RenderError` and clear the loading state.
- Manual `r` clears `err` before re-fetching.

## Manual refresh routing

The parent's `handleKey("r")` dispatches to the active sub-view's
`Refresh()` when on Nodes/Observability/Logs. On Overview/Node Pools
(Phase 1 behavior preserved), `r` reloads cluster details via
`v.load()`.

```go
case "r":
    switch v.tabs.ActiveTab().ID {
    case "nodes":
        if v.nodes != nil { return v.nodes.Refresh() }
    case "observability":
        if v.observability != nil { return v.observability.Refresh() }
    case "logs":
        if v.logs != nil { return v.logs.Refresh() }
    }
    v.loading = true
    v.err = nil
    if v.client == nil {
        return tea.Batch(v.spinner.Tick, v.initClient())
    }
    return tea.Batch(v.spinner.Tick, v.load())
```

## Keybindings (documentation update)

The `.claude/rules/key-bindings.md` Phase 1 entries grow:

### GKE Cluster Details View

| Key | Action |
|-----|--------|
| `D` | Delete cluster (type-to-confirm) |
| `r` | Refresh active tab (Observability re-fetches metrics; Logs re-runs query; Nodes re-lists; Overview/Node Pools reload cluster) |
| `Tab` | Switch focus (tabs / content) |
| `h/l` or `1/2/3/4/5` | Switch tabs (Overview / Node Pools / Nodes / Observability / Logs) |
| `Enter` | Navigate via focused link (Network, Subnetwork, or — on Nodes tab — open instance details) |
| `j/k` or `↑/↓` or `PgUp/PgDn` | Scroll content |
| `Esc` | Go back |

### GKE Cluster Details — Nodes tab

| Key | Action |
|-----|--------|
| `Enter` | Open instance details for the focused node. |
| `S` | Open sort menu. |
| `/` | Filter (`pool:`, `status:`, `zone:`). |
| `r` | Refresh node list. |

### GKE Cluster Details — Observability tab

| Key | Action |
|-----|--------|
| `1`–`5` | Time range (1h / 6h / 24h / 7d / 30d). |
| `a` | Toggle auto-refresh (default on, 30 s). |
| `r` | Manual refresh. |

### GKE Cluster Details — Logs tab

| Key | Action |
|-----|--------|
| `I` / `W` / `E` | Toggle severity bucket (INFO+below / WARNING / ERROR+above). |
| `R` | Cycle resource type (k8s_cluster → k8s_node → k8s_pod → k8s_container, wraps). |
| `a` | Toggle auto-refresh (default off, 15 s). |
| `L` | Open full Logs Explorer pre-filtered for this cluster. |
| `r` | Manual refresh. |

## Testing strategy

### `internal/gcp/monitoring_gke_test.go`

- `TestGKEFilter_ClusterCPU` — filter contains `resource.type = "k8s_cluster"`, cluster_name, location, metric type; no `OR` between resource types.
- `TestGKEFilter_NodeNetwork` — uses `resource.type = "k8s_node"` with `REDUCE_SUM`.
- `TestGKEFilter_AllocatableScaledToPercent` — the helper that converts the 0–1 ratio to 0–100 percentage.

### `internal/gcp/compute_test.go` addition

- `TestListManagedInstances_ProjectsMIGInstance` — table-driven test on
  the projection helper that maps `compute.ManagedInstance` (or whatever
  the SDK shape is) into `MIGInstance`. Edge cases: PROVISIONING with no
  internal IP, TERMINATED with status field present, missing
  CreationTimestamp.

### `internal/ui/views/gke_nodes_test.go`

- Renders rows with mixed statuses.
- Filter `pool:default` narrows correctly.
- Sort by Age.
- Enter emits `InstanceSelectedMsg` with correct fields.
- All-or-nothing loading: while any MIG fetch is still in flight,
  `View()` keeps the loading indicator (with a `(completed/total
  instance groups)` counter) and does NOT leak partial rows from
  completed pools. Showing partial rows was the original plan but
  turned out to be misleading — the table would look "complete" then
  suddenly grow when the next pool landed.

### `internal/ui/views/gke_observability_test.go`

- Default state on construction: range = 1h, autoRefresh on, no data yet.
- Time range cycle via `1`–`5`.
- Stale-tick guard: `autoRefresh=false` → `tickAutoRefresh` returns nil; `tabActive=false` → tick handler drops msg.
- View renders loading placeholders for each chart while fetches are in flight.
- Per-metric error surfacing: one chart fails, others render.

### `internal/ui/views/gke_logs_test.go`

- Severity toggles (`I/W/E`) update the LQL filter string.
- Resource-type dropdown changes `resource.type` in the query.
- Auto-refresh off by default.
- `L` emits the LogsExplorerOpen-style message with the built LQL.

### `internal/ui/views/gke_cluster_details_test.go` additions

- Tab navigation cycles through all 5 tabs.
- Lazy sub-view creation: observability is nil before first visit, non-nil after.
- `tabActive` toggling on tab change.
- `r` dispatches to active sub-view's `Refresh()` when on Observability / Logs / Nodes (not the cluster-details `load()`).

## Risk & open questions

- **`kubernetes.io/cluster/pod_count`** may return no data on some
  clusters (very new clusters, certain Autopilot configurations).
  Phase 2a renders "(no data)" if the series is empty — no auto-fallback.
- **MIG enumeration cost** on big clusters: a 50-node, 10-pool cluster
  triggers 10 `ListManagedInstances` calls. Each fast (~200 ms) but
  fan-out concurrency matters. Pattern: all calls in parallel via
  `tea.Batch`, render incrementally as each pool returns.
- **Cross-tab `tabActive` race** if the user mashes tab keys: harmless
  because the guard double-checks on tick delivery. Worth a deliberate test.
- **`LoggingClient` cold-start** on first Logs tab visit (~500 ms).
  Same trade-off as Cloud Run observability — acceptable.
- **`InstanceSelectedMsg` cross-view nav** must include the GKE details
  view in the client resolution chain (`gkeClusterDetailsView.GetComputeClient()`)
  per `adding-new-views.md` step 16.
- **Future Phase 2b coupling**: node pool delete + resize will land on
  the existing Node Pools tab. 2a leaves that tab unchanged so 2b is
  purely additive.

## Implementation order

Suggested commit sequence:

1. `internal/gcp/monitoring_gke.go` — types + filter helper + first metric (CPU utilization) + tests.
2. Remaining metrics on `MonitoringClient` (Memory, NodeCount, PodCount, NetworkBytes).
3. `internal/gcp/compute.go` — `ListManagedInstances` + `MIGInstance` type (+ `internal/gcp/gke.go` `GKENode` wrapper) + tests.
4. `gke_phase2a_messages.go` — internal lifecycle messages.
5. `gke_nodes.go` — Nodes sub-view + tests.
6. `gke_observability.go` — Observability sub-view + tests.
7. `gke_logs.go` — Logs sub-view + tests.
8. Wire all three into `gke_cluster_details.go`: tabs grow, lazy
   sub-view creation, `tabActive` propagation, `r` dispatch, key
   routing.
9. App-level wiring for `InstanceSelectedMsg` from the Nodes tab (extend
   the client resolution chain).
10. `.claude/rules/key-bindings.md`, `CLAUDE.md`, `README.md` updates.

Each step is its own commit; the full sequence lands as one PR
(~12 commits), comparable to LB Phase 2.
