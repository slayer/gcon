# GKE Phase 2c — cluster + node pool edit

## Goal

Add `e` key edit flows for existing GKE clusters and node pools — the
fields most commonly tweaked post-create, with a diff preview before
deploy. Like Phase 2b mutations, every edit kicks a long-running
`Operation`; the footer polls it via the existing `pollGKEOperation`
helper until DONE.

## Non-goals

- **Immutable fields** — name, location, network, subnetwork, IP family,
  initial pod range, autopilot flag (cluster); machine type, disk
  type/size, preemptible flag (pool). The form does not surface these.
- **Already-shipped mutations** — node count (Phase 2b
  `SetNodePoolSize`), autoscale (`UpdateNodePoolAutoscaling`),
  version upgrades (Phase 2b). The edit form does NOT duplicate them
  to keep field overlap minimal.
- **Recurring maintenance windows + exclusions** — only daily window for
  MVP. RRULE-based recurrence and exclusion intervals are deferred.
- **Master-authorized-networks, network policy toggle, release channel
  change, shielded-nodes toggle** — power-user controls; deferred.
- **Image type change, pool zone (locations) change** — both trigger
  node recreations and are higher-risk; deferred until we add a
  dedicated "replace pool" flow.

## Editable fields (MVP scope)

### Cluster

| Section | Field | UI | GCP API |
|---|---|---|---|
| Basic | resource labels | key/value list | `Clusters.SetResourceLabels` (separate endpoint, requires `LabelFingerprint`) |
| Maintenance | window mode | dropdown: none / daily | `Clusters.SetMaintenancePolicy` |
| Maintenance | daily start time | text "HH:MM" UTC (validated) | same |
| Observability | logging service | dropdown: none / SYSTEM / SYSTEM_AND_WORKLOAD | `Clusters.Update` (`DesiredLoggingService`) |
| Observability | monitoring service | dropdown: none / SYSTEM / SYSTEM_AND_WORKLOAD | `Clusters.Update` (`DesiredMonitoringService`) |

Cluster edit is allowed on **both Autopilot and Standard** — these
fields are top-level cluster knobs, not pool-specific. The existing
`canMutatePools()` Phase 2b guard does NOT apply.

### Node pool

| Section | Field | UI | GCP API |
|---|---|---|---|
| Labels | k8s labels (NodeConfig.Labels) | key/value list | `NodePools.Update` (NodeConfig.Labels) |
| Taints | taint list ({key, value, effect}) | repeating row: text key, text value, effect dropdown | `NodePools.Update` (NodeConfig.Taints) |
| Management | autoUpgrade | toggle | `NodePools.SetManagement` |
| Management | autoRepair | toggle | `NodePools.SetManagement` |
| Upgrade settings | strategy | dropdown: SURGE / BLUE_GREEN | `NodePools.Update` (UpgradeSettings) |
| Upgrade settings | max surge | int (0-N) | same |
| Upgrade settings | max unavailable | int (0-N) | same |

Pool edit is **Standard only** — Autopilot manages pools internally
and `canMutatePools()` returns false there.

## Architecture

### GCP layer (`internal/gcp/`)

Two new files:

```
gke_edit.go        — edit method wrappers + edit-shape projections
gke_edit_test.go   — request builder shape tests
```

Method surface (all return `Operation`):

```go
func (c *ContainerClient) UpdateClusterBasic(
    ctx, projectID, location, clusterName, edit ClusterEdit,
) (Operation, error)

func (c *ContainerClient) SetClusterMaintenancePolicy(
    ctx, projectID, location, clusterName, mw MaintenanceWindow,
) (Operation, error)

func (c *ContainerClient) UpdateNodePoolFields(
    ctx, projectID, location, clusterName, poolName, edit NodePoolEdit,
) (Operation, error)

func (c *ContainerClient) SetNodePoolManagement(
    ctx, projectID, location, clusterName, poolName, m NodePoolManagement,
) (Operation, error)
```

### Edit projections

Pointer fields encode "no change" (`nil`) vs "set to value" (`*v`).
Map/list fields are always whole-replacements when non-nil (the API
field mask handles this).

```go
type ClusterEdit struct {
    Description       *string
    ResourceLabels    *map[string]string
    LoggingService    *string
    MonitoringService *string
}

type MaintenanceWindow struct {
    Kind  MaintenanceKind // "none" | "daily"
    Daily string          // "HH:MM" UTC, when Kind=="daily"
}

type NodePoolEdit struct {
    Labels          *map[string]string
    Taints          *[]NodeTaint
    Tags            *[]string  // accepted in API but not exposed in MVP form
    UpgradeSettings *UpgradeSettings
}

type NodeTaint struct {
    Key, Value, Effect string // Effect: NO_SCHEDULE / PREFER_NO_SCHEDULE / NO_EXECUTE
}

type UpgradeSettings struct {
    MaxSurge       int64
    MaxUnavailable int64
    Strategy       string // "SURGE" | "BLUE_GREEN"
}

type NodePoolManagement struct {
    AutoUpgrade, AutoRepair bool
}
```

### UI layer (`internal/ui/views/`)

Two new views (form + diff preview, same two-state pattern as
`instance_editor.go`):

```
gke_cluster_edit.go         — cluster edit form view
gke_cluster_edit_test.go
gke_node_pool_edit.go       — node pool edit form view
gke_node_pool_edit_test.go
gke_edit_messages.go        — Open / Request / Result / Canceled msgs
```

Each view holds a snapshot of the initial values at construction time
(captured from `gcp.ClusterDetails` / `gcp.NodePool`). On submit:

1. Compare each form field against its initial value.
2. Build a `ClusterEdit` / `NodePoolEdit` with non-nil pointers ONLY
   for changed fields.
3. Compute a human-readable diff (per-field "old → new" lines).
4. Switch to `stateDiff`; show the diff for confirmation.
5. On Enter: emit `GKEClusterEditRequestMsg` / `GKENodePoolEditRequestMsg`,
   transition to `stateSaving`.
6. On Esc: back to `stateForm`.

States: `stateForm` / `stateDiff` / `stateSaving`. Matches the existing
edit-view pattern in `instance_editor.go` and `cloudrun_edit.go`.

If no fields changed at submit, surface a "No changes" inline error
and stay in `stateForm` rather than spinning up a no-op API call.

### Two API calls per submit (when both basic + maintenance change)

`ClusterEdit` and `MaintenanceWindow` go to different endpoints. The
form may dirty fields in both groups in one submission. Approach:

- Issue them sequentially from the app handler.
- One task ID per submission: `gke-op:edit-cluster:<name>`.
- If basic+maintenance both dirty, run basic first → poll until DONE
  → run maintenance → poll until DONE → finish task. Status text
  shows "Updating cluster X (1/2)..." while basic is polling.
- Per-call failure: short-circuit, set error on the cluster details
  view, finish the task with the error.

The pool edit is similar: `NodePoolFields` (labels/taints/upgrade
settings) and `NodePoolManagement` (autoUpgrade/autoRepair) go to
different endpoints; same sequential-with-counter pattern.

This composite-poll lives in `app_navigation.go` next to `pollGKEOperation`:

```go
// pollGKEEditSequence runs a list of operation polls sequentially.
// Each entry is (op-name, op-location, status-prefix). Footer status
// updates as each step completes. On any step error, the whole
// sequence aborts and onError fires.
type gkeEditStep struct {
    name, location string
    label          string // shown in footer e.g. "basic" / "maintenance"
}
```

### Key wiring

`internal/ui/views/gke_cluster_details.go`:

- **Overview tab + `e`**: open cluster edit. Allowed on Autopilot.
- **Node Pools tab + `e`**: open pool edit for the focused pool. Gated
  by `canMutatePools()` (Standard only).

Per `.claude/rules/adding-new-views.md` checklist:

- Add `ViewGKEClusterEdit`, `ViewGKENodePoolEdit` view types.
- Wire `getCurrentViewModel`, `clearAllViews`, `updateViewSizes`,
  `app_render.go renderCurrentView`, `updateSidebarActiveView`,
  sidebar guards (include the new views in the GKE Clusters guard
  list).
- `HasTextInputFocused` on both edit views (they contain text inputs).
- `IsMenuOpen` not relevant — these are full-screen views, not overlays.

### Message types (`gke_edit_messages.go`)

```go
// Nav signals from cluster details
type GKEClusterEditOpenMsg  struct { ProjectID, Location, ClusterName string }
type GKENodePoolEditOpenMsg struct { ProjectID, Location, ClusterName, PoolName string }

// Form submits — handler dispatches to API.
type GKEClusterEditRequestMsg struct {
    ProjectID, Location, ClusterName string
    Basic       *gcp.ClusterEdit        // nil = no basic changes
    Maintenance *gcp.MaintenanceWindow  // nil = no maintenance changes
}

type GKENodePoolEditRequestMsg struct {
    ProjectID, Location, ClusterName, PoolName string
    Fields     *gcp.NodePoolEdit       // nil = no field changes
    Management *gcp.NodePoolManagement // nil = no management changes
}

// Lifecycle
type GKEClusterEditCanceledMsg  struct{}
type GKENodePoolEditCanceledMsg struct{}

type GKEClusterEditResultMsg struct {
    TaskID, ClusterName string
    Error error
}
type GKENodePoolEditResultMsg struct {
    TaskID, ClusterName, PoolName string
    Error error
}
```

## Edge cases + decisions

- **No-op submit** (no fields dirty): show "No changes to apply" inline
  on the form; do not navigate to diff or call API.
- **Partial dirty** (e.g. only description changed): only the basic
  call fires; maintenance call is skipped; one step in the sequence.
- **API path failure mid-sequence**: stop, refresh details (to capture
  any partial state), surface error via cluster-details `SetError`.
- **Tags field not in pool form (MVP)**: `Tags` is part of the
  `NodePoolEdit` struct so the API layer is complete, but the form
  does not surface it. Future PR can add the section without churning
  the API surface.
- **Map / list ordering**: labels and taints render alphabetical by
  key for deterministic diff output (per `component-patterns.md`).
- **Diff layout**: two-column "Before │ After" for scalar fields, then
  a "Changed entries" subsection for maps/lists showing added /
  removed / changed lines (color-coded green / red / yellow).
- **Saving during a sequence**: footer task status updates as each
  step completes. The user can cancel only between steps (Esc during
  saving is forwarded to the parent for nav-back; the in-flight
  operation continues server-side regardless).

## Reused infra (no new code)

- `pollGKEOperationOnce` + `gkeOperationPollResultMsg` from Phase 2b.
- `registerRunningTask` / `updateRunningTask` / `finishTask` footer
  task framework.
- `borrowContainerClient` helper.
- `CreateViewBase` is NOT used here — edit views have a 3-state
  machine (form / diff / saving), not the create-view's 2-state. They
  hand-roll the lifecycle like `instance_editor.go` does.

## Open questions (call out at review time)

- Should taints support inline validation of effect (only the 3
  GCP-allowed values)? Yes — dropdown forces it.
- Maintenance window kind: when changing from daily → none, the API
  expects an empty MaintenancePolicy. Confirm with a small integration
  test once we wire the form.
- Resource labels on Autopilot: should be supported per docs, but
  worth verifying we don't get a 400 on submit. Track during smoke.
