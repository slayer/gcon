# Load Balancers — Phase 2 Design

## Scope

Two read-only additions to `LoadBalancerDetailsView`. No write paths.

**v2 (this spec)**

- **Live backend health on the Backends tab** — for every backend group on
  every backend service of the LB, call `backendServices.getHealth` in
  parallel and render an inline summary badge (`● 3/4 healthy`). The
  cursor can focus a group row and toggle a per-instance expansion that
  lists each member's HEALTHY / UNHEALTHY state, the failure reason if
  any, and how long since the last health probe.
- **Observability tab** (4th tab on the details view) — Cloud Monitoring
  charts for the LB, sourced from the `loadbalancing.googleapis.com/https/*`
  metric family. Five charts: request count, request latency
  (p50/p95/p99 overlay), error rate (4xx/5xx overlay), backend latency,
  and throughput (bytes in/out overlay). Time-range selector (1h/6h/24h/7d/30d),
  optional auto-refresh on a 30 s tick, manual `r` refresh.

**Explicitly deferred (out of scope)**

- Edit URL map host/path rules — separate spec.
- Edit backend service settings — separate spec.
- Attach / detach backends — separate spec.
- Create new LB — separate spec.
- Network LB (`l3/internal/*`, `l3/external/*`) metric coverage —
  separate spec; this phase shows a placeholder on Network LB details.
- Logical-LB grouping — possible v3 enhancement.
- SSL certificate management.

## UX entry points

Both additions live inside the existing `LoadBalancerDetailsView`. No new
sidebar entries, no new command-palette commands, no new top-level views.

The tab strip on the details view grows from three tabs to four:

```
Before:  [ Overview | Routing | Backends ]
After:   [ Overview | Routing | Backends | Observability ]
```

## Backends tab — live health

### Data source

The Compute API exposes `backendServices.getHealth` (and the regional
equivalent) which takes a `(backendService, group)` pair and returns
`HealthStatus[]` describing each instance in that group. A backend
service may have multiple groups; each group is a separate API call.

### Fetch strategy

When `lbBackendsLoadedMsg` lands, the view fans out one parallel
`tea.Cmd` per `(BackendService, Backend.Group)` pair. Results
accumulate into `map[string]groupHealth` keyed by `Backend.Group` URL.

Per-group state is one of:

- `groupHealthLoading` — call in flight, render `◌◌ loading…`
- `groupHealthOK` — call returned, render badge + cached `HealthStatus[]`
- `groupHealthError` — call failed, render `? error` (the call error is
  shown inline next to the badge, not as a fatal view error)
- `groupHealthSkipped` — the `Group` URL is not an instance group / NEG
  (e.g. GCS bucket backend, Cloud Run serverless NEG that has no per-
  instance health). Render `(no health — <kind> backend)`.

Health is best-effort, never blocks the rest of the tab. A failure on
one group does not prevent the others from rendering.

### Group kind detection

Backend `Group` URLs come in several shapes. The render path classifies
each URL by inspecting the path segments:

| URL segment containing      | Kind label                | Has health? |
|-----------------------------|---------------------------|-------------|
| `/instanceGroups/`          | Instance group            | yes         |
| `/networkEndpointGroups/`   | NEG (network/zonal)       | yes         |
| `/regionNetworkEndpointGroups/` for serverless backend | Serverless NEG | no |
| anything else (rare)        | (unknown)                 | skip        |

A NEG pointing at a serverless service (Cloud Run, Cloud Functions,
App Engine) has no per-instance state — the LB sees the service as a
single endpoint. The view detects this by inspecting the NEG's
`networkEndpointType` field (`SERVERLESS`) when the URL is fetched
during health resolution and marks the group as `groupHealthSkipped`.

### Rendering

Collapsed (default):

```
Backend service: api-backend
  Protocol: HTTPS  Timeout: 30s  Affinity: NONE

    Group: ig-prod-us-east1     ●●● 3/3 healthy
    Group: ig-prod-us-west1     ●●○ 2/3 healthy  ▸
    Group: neg-prod-eu          ◌◌ loading…
    Group: bucket-static        (no health — Cloud Storage backend)

    Health check: api-health  (HTTP /healthz :8080, interval 5s)
```

Expanded (after Tab / Enter on `ig-prod-us-west1`):

```
    Group: ig-prod-us-west1     ●●○ 2/3 healthy  ▾
        ┌─ Instance ──────────────┬─ Status ──┬─ Last check ─┐
        │ vm-prod-w1-a            │ ● HEALTHY │ 2s ago       │
        │ vm-prod-w1-b            │ ● HEALTHY │ 1s ago       │
        │ vm-prod-w1-c            │ ● UNHEALTHY  HTTP 503    │
        │                         │           │ (14s ago)    │
        └─────────────────────────┴───────────┴──────────────┘
```

- Solid filled dot `●` = healthy member (green).
- Hollow / dim dot `○` = unhealthy member (red).
- `◌` row = loading.
- The summary badge renders up to 5 dots; counts > 5 abbreviate to a
  numeric form (`12/15 healthy`) with no glyph row.

### Focus model

The Backends tab introduces a "group focus" mode parallel to the existing
tab / links / content focus levels. While in group-focus mode:

- `j` / `k` move the cursor across group rows on the tab.
- `Tab` / `Enter` toggles expansion on the focused group.
- `Esc` exits group-focus mode and returns to tab-strip focus.

This follows the same focus-ring pattern as `snapshot_details.go` /
`firewall_details.go` (links focus / content focus).

### Auto-refresh

Health badges respect the same auto-refresh tick as the Observability
tab — when auto-refresh is on, every 30 s the view re-fires the
`getHealth` fan-out (in addition to the metrics re-fetch). This avoids
two competing tickers.

When auto-refresh is off, `r` is the only way to refresh either signal.

## Observability tab

### Lazy creation

The observability sub-state is created on first tab visit, mirroring
the lazy-init pattern documented in `adding-new-views.md`:

```go
case tabs.TabChangedMsg:
    if v.tabs.ActiveTab().ID == "observability" {
        if v.observability == nil {
            v.observability = newLoadBalancerObservability(...)
        }
        v.updateViewportContent()
        return v.observability.Init()
    }
```

Tab strip remains visible regardless of LB type; the body shows either
the chart grid (HTTP(S)) or the deferred placeholder (Network LB / other).

### LB-type gating

```
       LB type derived in Phase 1
                  │
        ┌─────────┴─────────┐
        │                   │
  HTTPS / HTTP        Network LB / TCP / SSL / passthrough
  (external + internal)
        │                   │
        ▼                   ▼
  Observability tab     Placeholder body
  renders charts        ("Metrics for <kind> not yet supported")
```

Detection reuses the type-derivation helper added in phase 1
(`internal/ui/views/loadbalancers.go`). If the helper returns one of
`HTTPS (external)`, `HTTPS (internal)`, `HTTP (external)`, the tab
renders charts. All other types render the placeholder.

### Metric data source

A new `internal/gcp/monitoring_lb.go` mirrors the shape of
`monitoring_cloudrun.go`:

| Helper | Cloud Monitoring metric | Reducer / aligner |
|---|---|---|
| `GetLBRequestCount(fwd, dur)` | `loadbalancing.googleapis.com/https/request_count` | rate, SUM |
| `GetLBRequestCountByCodeClass(fwd, codeClass, dur)` | same, filtered on `response_code_class` | rate, SUM |
| `GetLBRequestLatencies(fwd, dur)` → p50/p95/p99 | `loadbalancing.googleapis.com/https/total_latencies` | distribution percentiles |
| `GetLBBackendLatencies(fwd, dur)` → p50/p95/p99 | `loadbalancing.googleapis.com/https/backend_latencies` | distribution percentiles |
| `GetLBRequestBytes(fwd, dur)` | `loadbalancing.googleapis.com/https/request_bytes_count` | rate, SUM |
| `GetLBResponseBytes(fwd, dur)` | `loadbalancing.googleapis.com/https/response_bytes_count` | rate, SUM |

All helpers take the forwarding-rule name (the LB identifier in the
metric label set) plus a `time.Duration` for the window. All return
ascending-by-timestamp `[]gcp.DataPoint`, sorted at the data layer
(GCP API gotcha — see `.claude/rules/gcp-api-gotchas.md`).

`monitoring_lb.go` is a new file rather than additions to an existing
one because Cloud Run's helpers are tightly tied to the Cloud Run
filter shape (`service_name="..."`). LB metrics use a different
resource type (`l7_lb_rule` / `https_lb_rule`) and label set. A
separate file is the cleaner home.

### Charts

```
Observability
────────────────────────────────────────────────────────────

  [1h] [6h]  24h  [7d]  [30d]      ⟳ auto-refresh ON   r refresh

Request Count
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  [braille time series, single series]

  current 124 r/s   peak 410 r/s   total 4.2M reqs (24h)

Request Latency (p50 / p95 / p99)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  [braille time series, 3 overlay series]

  ● p50 142ms   ● p95 480ms   ● p99 1.1s

Error Rate (4xx / 5xx)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  [braille time series, 2 overlay series]

  ● 4xx 0.4% (peak 2.1%)   ● 5xx 0.0% (peak 0.3%)

Backend Latency (origin response time, p50 / p95 / p99)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  [braille time series, 3 overlay series]

  ● p50 38ms   ● p95 145ms   ● p99 290ms

Throughput (bytes in / out)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  [braille time series, 2 overlay series]

  ● in 1.2 MB/s   ● out 8.4 MB/s

  1-5: time range   a: auto-refresh   r: refresh
```

Chart heights and y-label formatters (with constants from `metricchart`):

| Chart | Height | Y-label formatter |
|---|---|---|
| Request count | `HeightStandard` (8) | `humanYLabel` (SI suffixes) |
| Request latency | `HeightStandard` | `LatencyYLabel` |
| Error rate | `HeightCompact` (5) | `PercentYLabel`, range 0–10 default |
| Backend latency | `HeightStandard` | `LatencyYLabel` |
| Throughput | `HeightCompact` | `humanYLabel` (bytes) |

Error rate is rendered as a percentage of total request count. The view
divides each code-class series by the request-count series at fetch
time. When request count is zero, error rate is zero (not NaN).

### Auto-refresh and stale-tick discipline

The Observability tab uses `tea.Tick(30*time.Second, ...)` for its
refresh cycle. Per the gotcha documented in
`.claude/rules/bubble-tea-rendering.md`, `tea.Tick` messages survive
context switches and must be guarded:

- `autoRefresh bool` — user toggle (default `true` on first open).
- `tabActive bool` — set true on tab enter, false on tab leave.
- Tick handler drops the refresh when either is false.
- Tick is only re-scheduled while both are true.

This is the same pattern Cloud Run observability uses today
(`cloudrun_observability.go`); the LB tab will copy it. The health
re-fetch on Backends tab piggybacks on the same tick when the tab
is visible.

### Key bindings

| Key | Action | Where |
|---|---|---|
| `j` / `k` | Move group focus | Backends tab |
| `Tab` / `Enter` | Toggle expansion on focused group | Backends tab |
| `Esc` | Exit group focus | Backends tab |
| `1`–`5` | Time range 1h/6h/24h/7d/30d | Observability tab |
| `a` | Toggle auto-refresh | Observability tab |
| `r` | Manual refresh metrics + health | Observability tab and Backends tab |

Existing tab-switching keys (`Tab`, `h/l`, `1/2/3` → `1/2/3/4`) and
the global delete shortcut (`D`) keep working unchanged. Number-key
overlap is resolved by focus: tab-strip focused → `1–4` switch tabs;
content focused on Observability → `1–5` change time range.

## Architecture

```
LoadBalancerDetailsView
├── tab "Overview"        (unchanged)
├── tab "Routing"         (unchanged)
├── tab "Backends"
│     ├── existing config render
│     └── + groupHealth fan-out  ← NEW
│           └── per-group state map[string]groupHealth
└── tab "Observability"   (NEW, lazy-init)
      └── loadBalancerObservability struct
            ├── metric fetch via gcp.MonitoringClient
            ├── 5 metricchart.Chart instances
            ├── 30s tea.Tick refresh loop
            └── tabActive + autoRefresh stale-tick guard

internal/gcp/
├── loadbalancers.go               (modify)
│     + GetBackendHealth(ctx, project, scope, bsName, groupURL) ([]InstanceHealth, error)
│     + GetNetworkEndpointGroup(ctx, project, zone, name) (*NEG, error)  // for serverless detection
└── monitoring_lb.go               (NEW)
      Methods modeled on monitoring_cloudrun.go shape.
```

### New types

```go
// internal/gcp/loadbalancers.go

// InstanceHealth is the per-member result of backendServices.getHealth.
type InstanceHealth struct {
    Instance       string // last segment of the instance URL
    HealthState    string // "HEALTHY" | "UNHEALTHY" | "UNKNOWN" | "DRAINING"
    IPAddress      string
    Port           int64
    FailureReason  string // e.g., "HTTP 503"; empty for HEALTHY
    LastCheckTime  time.Time
}

// NEG is a minimal projection of compute.NetworkEndpointGroup used only
// to detect the SERVERLESS type during health resolution.
type NEG struct {
    Name             string
    NetworkEndpointType string // "GCE_VM_IP_PORT" | "SERVERLESS" | ...
}
```

### New files

| File | Purpose |
|---|---|
| `internal/gcp/monitoring_lb.go` | LB metric helpers, modeled on `monitoring_cloudrun.go`. Filters keyed on `forwarding_rule_name`. Returns ascending-sorted `[]DataPoint`. |
| `internal/gcp/monitoring_lb_test.go` | Filter string assertions, ascending-sort guard, percentile path. |
| `internal/ui/views/loadbalancer_observability.go` | The lazy-init sub-view. Owns the five charts, the time range, the auto-refresh ticker, and metric fetch dispatch. |
| `internal/ui/views/loadbalancer_observability_test.go` | Range key handling, ticker start/stop on tab switch, stale-tick guard, multi-series wiring. |

### Modified files

- `internal/gcp/loadbalancers.go` — `GetBackendHealth`,
  `GetNetworkEndpointGroup`, `InstanceHealth` type, `NEG` type.
- `internal/gcp/loadbalancers_test.go` — new method shape coverage.
- `internal/ui/views/loadbalancer_details.go`:
  - Add `"observability"` tab to the constructor's tab list.
  - Add fields: `groupHealth map[string]groupHealthState`,
    `groupFocus int`, `groupExpanded map[string]bool`,
    `observability *loadBalancerObservability`.
  - Extend `Update()` for the new message types (
    `lbGroupHealthLoadedMsg`, `lbGroupHealthErrorMsg`,
    `lbObservabilityTickMsg`) and key handlers.
  - Trigger `getHealth` fan-out on `lbBackendsLoadedMsg`.
  - Modify `renderBackends()` to draw badges + expand state.
  - Add `renderObservability()` delegating to the sub-view.
  - Implement `MenuOpener` already exists; the new tab keeps it intact.
- `internal/ui/views/loadbalancer_details_test.go` — health badge
  rendering, expand toggle, tab visibility.
- `README.md`, `CLAUDE.md`, `.claude/rules/key-bindings.md` — docs.

## Testing

1. **`monitoring_lb.go` filter construction** — table-driven test
   asserting every filter string exactly matches the documented metric
   path and pins the resource type label (`https_lb_rule` /
   `internal_http_lb_rule`).

2. **Ascending-sort guard** — every public metric helper, on synthetic
   newest-first GCP responses, returns ascending-by-timestamp results.

3. **`GetBackendHealth` shape** — table-driven test for global vs.
   regional URL classification and the resulting API path taken.

4. **NEG serverless detection** — table-driven on
   `NetworkEndpointType` values; verifies `groupHealthSkipped` is set
   for `SERVERLESS` and `groupHealthOK` for `GCE_VM_IP_PORT`.

5. **Backends tab rendering**:
   - Mixed health states (healthy / unhealthy / unknown / draining)
     render the right glyph counts.
   - >5 members abbreviate to numeric form.
   - Loading shows `◌◌ loading…`, error shows `? error`, skipped shows
     `(no health — <kind>)`.
   - Tab/Enter toggles expansion; the table renders only when expanded.

6. **Observability tab rendering**:
   - Loading state shows the spinner and "Loading metrics…".
   - Per-metric error becomes a warning banner; sibling charts still
     render.
   - Range key (`1`–`5`) re-fires fetch with the new duration only when
     content is focused; tab-strip focus routes the key to tab switching.
   - Auto-refresh tick is dropped when `tabActive` is false (stale-tick
     guard).
   - Network LB type renders the placeholder, no chart elements.

7. **LB-type gating** — table-driven across every type label from the
   Phase 1 derivation helper; only HTTP / HTTPS / HTTPS internal route
   to charts; everything else routes to the placeholder.

8. **App routing unchanged** — sidebar guards (`adding-new-views.md`
   step 13) do not need an update because no new top-level view is
   added. Existing guards continue to work.

9. **Full suite green** + **lint clean** on Go 1.26.

## Out of scope

Repeated for clarity. The only new feature behavior introduced is:

- Backend group health on Backends tab.
- HTTP(S) LB observability tab.

Everything in `Design.md` from Phase 1 — list view, type derivation,
cascade delete — is unchanged.
