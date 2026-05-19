# GKE Phase 2b — node pool create/delete/resize + master/pool upgrade

## Goal

Add the six mutation actions Phase 2a deliberately deferred:

| Action               | Where               | Key | Async op |
|----------------------|---------------------|-----|----------|
| Create node pool     | Node Pools tab      | `c` | yes      |
| Delete node pool     | Node Pools tab      | `D` | yes      |
| Resize node pool     | Node Pools tab      | `R` | yes      |
| Upgrade node pool    | Node Pools tab      | `u` | yes      |
| Upgrade control plane| Overview tab        | `u` | yes      |
| (Cluster resize)     | — out of scope      | —   | —        |

Cluster-level "resize" doesn't exist for Standard clusters (resize means
resize-a-pool); for Autopilot it's fully managed. So the user-facing
mutation surface is the five rows above.

Every mutation returns a long-running `Operation`. The footer task
indicator polls `operations.get` every 5 s until DONE, then triggers a
refresh of the affected view. The user can keep working.

Autopilot clusters hide every action except master upgrade. Even master
upgrade is hidden when the cluster is on a fully-managed release
channel (RAPID / REGULAR / STABLE with `autoUpgrade.enabled` at the
node-pool level) — GKE manages upgrades automatically and the
`UpdateMaster` call returns an error.

## Non-goals

- Workload mutations (deployments, services, etc.) — never; that's
  Kubernetes API territory.
- Cluster-level rename / region migration — GCP doesn't support these
  in-place; user creates a new cluster.
- IAM / authorized networks edits — separate phase if useful.
- Image-type changes (cos → ubuntu) — requires draining + replacing
  the pool; better as a future "recreate node pool" guided flow.
- Cluster upgrade scheduling / maintenance windows — Phase 2c if asked.

## Architecture

### New GCP-layer files

**`internal/gcp/gke_mutations.go`** (new) — one method per mutation,
each returning the operation projection so the caller can kick off
polling:

```go
type Operation struct {
    Name          string    // full path: projects/X/locations/Y/operations/Z
    Type          string    // CREATE_NODE_POOL / DELETE_NODE_POOL / SET_NODE_POOL_SIZE / UPGRADE_MASTER / UPGRADE_NODES
    Status        string    // PENDING / RUNNING / DONE / ABORTING
    Target        string    // resource the operation acts on
    StatusMessage string    // human-readable error / progress message
    StartTime     time.Time
    EndTime       time.Time // zero until DONE
    Detail        string    // free-form progress detail when present
}

// Mutations. Each returns Operation + error.
func (c *ContainerClient) CreateNodePool(ctx, projectID, location, clusterName string, pool *container.NodePool) (Operation, error)
func (c *ContainerClient) DeleteNodePool(ctx, projectID, location, clusterName, poolName string) (Operation, error)
func (c *ContainerClient) SetNodePoolSize(ctx, projectID, location, clusterName, poolName string, nodeCount int64) (Operation, error)
func (c *ContainerClient) UpdateNodePoolAutoscaling(ctx, projectID, location, clusterName, poolName string, on bool, min, max int64) (Operation, error)
func (c *ContainerClient) UpgradeControlPlane(ctx, projectID, location, clusterName, masterVersion string) (Operation, error)
func (c *ContainerClient) UpgradeNodePool(ctx, projectID, location, clusterName, poolName, nodeVersion string) (Operation, error)
```

**`internal/gcp/gke_operations.go`** (new) — `GetOperation(ctx, project, location, operationName)` projecting the live op state. The full op name returned by the mutation calls already includes the location, so the polling layer just splits it on `/` to extract the components.

**`internal/gcp/gke.go`** (extend) — `GetServerConfig(ctx, project, location)` returning `[]ValidMasterVersions`, `[]ValidNodeVersions`, `DefaultClusterVersion`, channel defaults. Powers the version-picker dropdown.

### New view files

**`internal/ui/views/gke_node_pool_create.go`** — form-based creation
view. Embeds `CreateViewBase` for the standard lifecycle:

```
┌─ Basic ─────────────────────────────────────────┐
│ Name           [_____________]                  │
│ Initial count  [3]                              │
│ Machine type   [e2-medium ▼]   (zone-scoped)    │
│ Disk type      [pd-balanced ▼]                  │
│ Disk size GB   [100]                            │
│ Image type     [COS_CONTAINERD ▼]               │
└─────────────────────────────────────────────────┘
┌─ Autoscaling ───────────────────────────────────┐
│ Enabled        [x]                              │
│ Min nodes      [1]                              │
│ Max nodes      [10]                             │
└─────────────────────────────────────────────────┘
┌─ Lifecycle ─────────────────────────────────────┐
│ Auto-upgrade   [x]                              │
│ Auto-repair    [x]                              │
│ Preemptible    [ ]                              │
└─────────────────────────────────────────────────┘
┌─ Labels ────────────────────────────────────────┐
│ (key=value, one per line)                       │
│ team=platform                                   │
└─────────────────────────────────────────────────┘
```

Validation:
- Name: GCP resource name (lowercase, hyphens, 1–40 chars).
- Initial count >= 1. >= Min when autoscale on.
- Max >= Min when autoscale on; reasonable upper bound (1000).
- Machine type: lazy-loaded per zone (reuse the existing machine-type
  cache from VM create view).
- Disk size: 10–65536.

Submit: emits `GKENodePoolCreateRequestMsg` → app handler calls
`CreateNodePool` → on success registers operation polling.

**`internal/ui/views/gke_node_pool_resize_dialog.go`** — inline dialog
spawned by `R` on a focused pool row. Has two modes (cycled with Tab):

- **Manual size**: number input, current → new (clamped to 0–1000).
- **Autoscale**: three inputs (enabled toggle, min, max).

On submit, picks the right API call (SetNodePoolSize vs
UpdateNodePoolAutoscaling). Both emit
`GKENodePoolResizeRequestMsg{Mode, Size, Min, Max, AutoscaleOn}`.

**`internal/ui/views/gke_upgrade_dialog.go`** — version-picker dialog
shared by master + node-pool upgrade. Constructor takes:

- Title (`"Upgrade control plane"` / `"Upgrade pool: <name>"`)
- Current version
- Available versions (from `GetServerConfig`)
- Submit message constructor

UI: list of versions newest-first, current version highlighted with
`(current)`. Select + Enter emits the constructor's message.

Includes inline warnings:
- "Master must be upgraded before node pools" when upgrading a pool
  whose version > master's available range.
- "This is a multi-minute operation" for any upgrade.

**`internal/ui/views/gke_node_pool_action_menu.go`** (optional) — A
small overlay action menu on `.` to host Delete / Resize / Upgrade for
the focused pool. Keeps the keyspace simple if direct keys are
preferred (`D`, `R`, `u`) — we'll see during implementation whether
the dialog suffices.

### Existing files modified

**`internal/ui/views/gke_cluster_details.go`**
- New keys on **Overview tab**: `u` opens master upgrade dialog (if
  not Autopilot + non-managed channel).
- New keys on **Node Pools tab**: `c` (create), `D` (delete), `R`
  (resize), `u` (upgrade). All hidden / no-op on Autopilot.
- Hosts `confirmDialog` / `resizeDialog` / `upgradeDialog` /
  `createView` instances per the existing dialog z-order rule.

**`internal/ui/app_navigation.go`**
- New handlers: `handleGKENodePoolCreate`, `handleGKENodePoolDelete`,
  `handleGKENodePoolResize`, `handleGKENodePoolUpgrade`,
  `handleGKEMasterUpgrade`.
- New view type: `ViewGKENodePoolCreate` for the form.
- Each handler:
  1. Registers footer task (`gke-pool-create:<cluster>:<pool>`).
  2. Calls the GCP method.
  3. On success, kicks off operation polling via `pollGKEOperation`.
  4. On error, calls `view.SetError` and clears the task.

**`internal/ui/app.go`** + **`internal/ui/views/gke_phase2b_messages.go`**
- Polling cmd: `pollGKEOperation(opName, location, ...)` returns
  `gkeOperationPollMsg{name, op}` after a 5 s tick. App handler
  refreshes the affected view on Status=DONE and updates the footer
  task message in-between.

### Operations polling

Single tick loop per in-flight operation, keyed by operation name:

```go
func (a *App) pollGKEOperation(opName, location string, onDone func() tea.Cmd) tea.Cmd {
    return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
        return gkeOperationPollMsg{opName: opName, location: location, onDone: onDone}
    })
}

// Handler:
case gkeOperationPollMsg:
    op, err := a.containerClient.GetOperation(ctx, projectID, location, opName)
    if err != nil {
        a.finishTask(msg.opName, "")
        return notifyError
    }
    if op.Status == "DONE" {
        a.finishTask(msg.opName, "")
        if op.StatusMessage != "" {
            // op failed server-side
            return notifyError
        }
        return msg.onDone()  // refresh the view
    }
    // Still running — update footer + re-schedule.
    a.updateRunningTask(msg.opName, formatOpProgress(op))
    return a.pollGKEOperation(...)
```

`formatOpProgress` returns e.g. `"Upgrading control plane (1m 30s)"`.
GKE's Operation type doesn't include a percentage; we use elapsed time
since the operation StartTime.

### Footer task keys

To make cancel-on-navigate behave correctly, every footer task ID
starts with `gke-op:`:

- `gke-op:create-pool:<cluster>:<pool>`
- `gke-op:delete-pool:<cluster>:<pool>`
- `gke-op:resize-pool:<cluster>:<pool>`
- `gke-op:upgrade-pool:<cluster>:<pool>`
- `gke-op:upgrade-master:<cluster>`

`clearRunningTasks()` already wipes everything in TaskRunning state on
nav; the prefix is for filtering / debug only.

### Autopilot guards

```go
func (v *GKEClusterDetailsView) canMutatePools() bool {
    return v.details != nil && v.details.Mode != "AUTOPILOT"
}

func (v *GKEClusterDetailsView) canUpgradeMaster() bool {
    if v.details == nil {
        return false
    }
    // STATIC channel is the only one where master upgrades are
    // user-initiated. All release channels (RAPID/REGULAR/STABLE) run
    // automatic upgrades on a schedule.
    return v.details.ReleaseChannel == "" || v.details.ReleaseChannel == "UNSPECIFIED"
}
```

When `canMutatePools()` is false, the Node Pools tab footer renders:

```
  ▒ Pool mutations are managed by Autopilot
```

When `canUpgradeMaster()` is false, the Overview tab's master version
row gets a muted suffix:

```
  Master version:  1.30.5-gke.1014001  (managed by REGULAR channel)
```

## Keybindings (documentation update)

Extend `.claude/rules/key-bindings.md` GKE section:

### GKE Cluster Details — Overview tab

| Key | Action |
|-----|--------|
| `u` | Upgrade control plane (version picker; hidden on managed channels) |

### GKE Cluster Details — Node Pools tab

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Move cursor between pools |
| `c` | Create new node pool (form) |
| `D` | Delete focused pool (type-to-confirm) |
| `R` | Resize focused pool (manual size or autoscale) |
| `u` | Upgrade focused pool (version picker) |

## Testing strategy

### `internal/gcp/gke_mutations_test.go`

Filter-shape tests only (no live API calls):
- `CreateNodePool` builds the right `CreateNodePoolRequest` from a
  `NodePool` struct.
- `UpdateNodePoolAutoscaling` correctly sets `ForceSendFields` so
  zero-value min/max are sent.
- `SetNodePoolSize` sends `nodeCount=0` when scaling to zero (needs
  `ForceSendFields` per the existing gotcha).

### `internal/ui/views/gke_node_pool_create_test.go`

- Form has every required section.
- Submit emits `GKENodePoolCreateRequestMsg` with autoscale toggled.
- Submit validates: name required, count >= min when autoscale on.
- Embeds `CreateViewBase` lifecycle — error from handler propagates
  back via `SetError`.

### `internal/ui/views/gke_upgrade_dialog_test.go`

- Renders current version with `(current)` suffix.
- Available-versions list excludes the current version.
- Enter on a focused version emits the constructor-supplied message.

### `internal/ui/views/gke_node_pool_resize_dialog_test.go`

- Manual mode submits SetNodePoolSize with the entered count.
- Autoscale mode submits UpdateNodePoolAutoscaling with min/max.
- Mode switch (Tab) preserves entered values.

### `internal/ui/views/gke_cluster_details_test.go` additions

- `c` on Node Pools tab (Standard cluster) navigates to create view.
- `c` on Node Pools tab (Autopilot cluster) is a no-op.
- `D` on focused pool opens the delete confirm dialog.
- `u` on Overview tab opens the upgrade dialog (when allowed).
- `u` on Overview tab is a no-op when the cluster is on a release channel.

### App-level (smoke)

- Footer task registered on mutation kick-off.
- Polling msg handler updates the task description.
- DONE status triggers a refresh on the affected view.

## Risk & open questions

- **Operations API auth scope** — `GetOperation` requires `container`
  scope which Phase 1 already has. No new scope.
- **Long-running upgrades** — the user may navigate away while an
  upgrade is in flight. Polling continues (task survives nav until
  `clearRunningTasks` runs). On nav back, the footer task is gone but
  the operation is still happening server-side. Acceptable for v1;
  could persist op state to a small in-memory map keyed by cluster
  later.
- **Master upgrade on managed channel** — GCP rejects the API call
  with HTTP 400. We pre-empt with the canUpgradeMaster guard, but if
  the user finds a way through (e.g. via a future command-palette
  entry), the error surfaces via `SetError` on the details view.
- **Concurrent mutations** — GKE allows only one operation per cluster
  at a time. If the user fires a second, GCP returns 409. The view's
  inline error surfaces it; we don't pre-validate client-side.
- **`SetNodePoolSize` to 0** — supported by GCP but effectively
  decommissions the pool. We don't gate this client-side beyond the
  min=0 lower bound on the number input.
- **NodePool Locations field** — for regional clusters, a pool spans
  every zone in the region. The create form does NOT expose this —
  we always use the cluster's default location list. Adding per-pool
  zone control is Phase 2c material.

## Implementation order

Suggested commit sequence:

1. `internal/gcp/gke_operations.go` — Operation type + GetOperation + polling helper.
2. `internal/gcp/gke_mutations.go` — DeleteNodePool first (simplest), test.
3. `internal/gcp/gke_mutations.go` — SetNodePoolSize + UpdateNodePoolAutoscaling, test.
4. `internal/gcp/gke_mutations.go` — UpgradeControlPlane + UpgradeNodePool, test.
5. `internal/gcp/gke_mutations.go` — CreateNodePool, test.
6. `internal/gcp/gke.go` — GetServerConfig, test.
7. `internal/ui/views/gke_phase2b_messages.go` — message types.
8. `internal/ui/views/gke_upgrade_dialog.go` — shared upgrade picker + tests.
9. `internal/ui/views/gke_node_pool_resize_dialog.go` + tests.
10. `internal/ui/views/gke_node_pool_create.go` — form + tests.
11. `internal/ui/views/gke_cluster_details.go` — wire keys + Autopilot guards + dialog z-order.
12. `internal/ui/app_navigation.go` — handlers + polling cmd + footer task IDs.
13. `internal/ui/views/gke_cluster_details_test.go` — key routing + Autopilot guard tests.
14. Docs — CLAUDE.md / README / key-bindings.md.

## Final integration smoke (manual)

After everything lands:

1. On a Standard cluster's Node Pools tab, `c` opens the create form;
   submit creates a new pool, footer shows "Creating node pool
   xyz...", refreshes pools when DONE.
2. `D` on a non-default pool: type-to-confirm; delete completes,
   pools list refreshes minus the row.
3. `R` on a focused pool: dialog opens with current size; submit
   resizes; footer updates with elapsed time; refresh on DONE.
4. `R` then Tab into autoscale mode: change min/max; submit; refresh.
5. `u` on a non-current node pool: version picker with current
   highlighted; pick newer; pool refreshes with new version when DONE.
6. `u` on Overview tab (STATIC channel only): version picker;
   submit; master version updates when DONE.
7. On an Autopilot cluster: `c` / `D` / `R` / `u` on Node Pools tab
   are no-ops; footer shows "Pool mutations are managed by
   Autopilot"; Overview master row shows the channel suffix.
