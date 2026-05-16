# GKE Phase 1 — Inventory, Details, Delete

## Goal

Ship the first slice of Kubernetes Engine support in gcon: a clusters list
view, a cluster details view with Overview and Node Pools tabs, and a
type-to-confirm delete flow. Cover both Autopilot and Standard clusters,
both zonal and regional locations. No edits, no create, no observability.

This is **Phase 1 of three**:

- **Phase 1 (this design)**: inventory + details + delete.
- **Phase 2** (separate brainstorm later): observability tab, node-pool
  create/delete, cluster/node-pool resize, master + node-pool upgrade.
- **Phase 3** (separate brainstorm later): full cluster create wizard —
  the form has 30+ knobs with cross-section dependencies and benefits
  from being designed after real-world browsing of existing clusters.

The shape mirrors what just shipped for Load Balancers Phase 1.

## Non-goals

- Kubernetes API access (no `kubectl`-style workload browsing, no
  `client-go` dependency, no kubeconfig generation).
- Cluster create or any form-based edit (Phase 2 / Phase 3).
- Observability metrics (Phase 2).
- Long-running operation tracking. Delete is fire-and-forget: the API
  call returns when GCP accepts the request, the operation continues
  server-side, and the user refreshes (`r`) to see status flip.

## Scope checklist

| Item | In Phase 1? |
|---|---|
| Clusters list with filter / sort | Yes |
| Cluster details Overview tab | Yes |
| Cluster details Node Pools tab | Yes |
| Delete cluster (type-to-confirm) | Yes |
| Autopilot and Standard clusters | Both |
| Zonal and regional clusters | Both |
| Sidebar entry + command palette | Yes |
| Node pool create / delete / resize | Phase 2 |
| Cluster upgrade / resize | Phase 2 |
| Observability | Phase 2 |
| Cluster create wizard | Phase 3 |
| Kubernetes workloads | Out of scope (separate brainstorm if pursued) |

## Architecture

### New GCP client — `internal/gcp/gke.go`

Wraps `google.golang.org/api/container/v1` (already a transitive dep of
`cloud.google.com/go`; add as a direct import).

```go
type ContainerClient struct {
    service   *container.Service
    projectID string
}

func NewContainerClient(ctx context.Context, projectID string) (*ContainerClient, error)

// "-" as the location returns clusters across all locations.
func (c *ContainerClient) ListClusters(ctx context.Context, projectID string) ([]Cluster, error)

func (c *ContainerClient) GetCluster(ctx context.Context, projectID, location, name string) (*ClusterDetails, error)
func (c *ContainerClient) ListNodePools(ctx context.Context, projectID, location, clusterName string) ([]NodePool, error)
func (c *ContainerClient) DeleteCluster(ctx context.Context, projectID, location, name string) error
```

`DeleteCluster` returns when the API accepts the request. The resulting
Operation is not polled — see the delete-flow section.

### Domain types

Mapped from the API at the client layer. UI never touches raw
`container.*` types.

```go
type Cluster struct {
    Name           string
    Location       string  // "us-central1-a" or "us-central1"
    LocationType   string  // "zone" | "region"
    Mode           string  // "AUTOPILOT" | "STANDARD"
    Status         string  // PROVISIONING / RUNNING / RECONCILING / STOPPING / ERROR / DEGRADED
    MasterVersion  string
    NodeVersion    string  // when uniform across pools; "(varies)" otherwise
    NodeCount      int     // sum across all node pools
    Network        string  // short name
    Subnetwork     string  // short name
    ReleaseChannel string  // RAPID / REGULAR / STABLE / "" (unspecified)
    Endpoint       string
    PrivateCluster bool
    CreatedAt      time.Time
}

type ClusterDetails struct {
    Cluster
    NodePools                []NodePool
    Addons                   AddonsSummary
    ClusterIPv4CIDR          string
    ServicesIPv4CIDR         string
    WorkloadIdentityPool     string  // "" when disabled
    MasterAuthorizedNetworks []string
    DatabaseEncryption       string  // "ENCRYPTED (key: name)" | "DECRYPTED"
}

type AddonsSummary struct {
    HTTPLoadBalancing  bool
    NetworkPolicy      bool
    PersistentDiskCSI  bool
    DNSCache           bool
}

type NodePool struct {
    Name           string
    MachineType    string
    DiskSizeGB     int64
    DiskType       string
    NodeCount      int   // current
    AutoscalingMin int
    AutoscalingMax int
    AutoscalingOn  bool
    NodeVersion    string
    Status         string
    AutoUpgrade    bool
    AutoRepair     bool
    Locations      []string  // zones the pool spans
}
```

### Mode detection

`Mode` is derived once in `convertCluster` from `cluster.autopilot.enabled`.
UI branches off `cluster.Mode`, never re-reads the raw API.

### Location type detection

`LocationType` is derived in `convertCluster`. A location with two or
more `-` segments is a zone (e.g. `us-central1-a`); otherwise it is a
region (e.g. `us-central1`). Helper:

```go
func locationType(location string) string {
    if strings.Count(location, "-") >= 2 {
        return "zone"
    }
    return "region"
}
```

This is identical in spirit to the `groupScope` helper used by NEG
classification, parsing the kind from the input directly rather than
guessing later.

## Views

### `internal/ui/views/gke_clusters.go` — list

Embeds `TableClickDelegate`. Table columns:

| Name | Location | Type | Mode | Master version | Nodes | Status |
|---|---|---|---|---|---|---|
| prod-api | us-central1 | regional | Standard | 1.30.5 | 12 | ● RUNNING |
| stage-api | us-central1-a | zonal | Autopilot | 1.30.5 | (managed) | ● RUNNING |

- Filter: `/` opens the standard filter bar. `field:value` syntax
  supported, including `mode:autopilot`, `status:running`,
  `location:us-central1`.
- Sort: `S` opens the sort menu. Default sort: Name asc.
- Refresh: `r`.
- Enter: opens details.
- `D`: delete with type-to-confirm.
- `Esc`: back.

Autopilot clusters show `(managed)` in the Nodes column because GKE does
not expose a meaningful single number for them in the cluster summary.

### `internal/ui/views/gke_cluster_details.go` — details

Two tabs in a `bubbles/viewport.Model`-wrapped body (per the
bubble-tea-rendering rule on tall tab content). Both tabs fit on a
typical terminal today, but applying the viewport pattern up front
costs nothing and avoids a follow-up retrofit when Phase 2 adds
Observability with five charts.

#### Overview tab

```
Cluster: prod-api
─────────────────────────────────────────────────────
  Mode:                 Standard       (or "Autopilot")
  Status:               ● RUNNING
  Location:             us-central1 (regional)
  Master version:       1.30.5-gke.1014001
  Node version:         1.30.5-gke.1014001       (or "(varies — see Node Pools)")
  Release channel:      REGULAR
  Created:              2025-08-12 14:03 UTC

Networking
─────────────────────────────────────────────────────
  Network:              default          [link → VPC details]
  Subnetwork:           default-uscent1  [link → subnet details]
  Cluster IPv4 CIDR:    10.4.0.0/14
  Services IPv4 CIDR:   10.8.0.0/20
  Endpoint:             34.123.45.67
  Private cluster:      No

Security
─────────────────────────────────────────────────────
  Workload Identity:    prod.svc.id.goog    (or "(off)")
  Database encryption:  ENCRYPTED (key: my-key)    (or "DECRYPTED")
  Authorized networks:  10.0.0.0/8, 192.168.0.0/16  (or "(none)")

Add-ons
─────────────────────────────────────────────────────
  HTTP load balancing:  Enabled
  Network policy:       Disabled
  Persistent disk CSI:  Enabled
  DNS cache:            Disabled
```

Network and Subnetwork rows use the `links` component (`Tab` to focus,
`Enter` to navigate). Cross-view navigation reuses the existing
`NetworkSelectedMsg` / subnet selection handlers in `app_navigation.go`.
Per the adding-new-views rule, the handler's client resolution chain
must include `gkeClusterDetailsView.GetComputeClient()`.

#### Node Pools tab

Embedded `table.Model`:

| Name | Machine type | Nodes | Autoscale | Version | Status | Auto-upgrade | Auto-repair |
|---|---|---|---|---|---|---|---|
| default | e2-medium | 3 | on (1–10) | 1.30.5-gke.1014001 | ● RUNNING | ✓ | ✓ |
| gpu-pool | g2-standard-4 | 2 | off | 1.30.5-gke.1014001 | ● RUNNING | ✓ | ✓ |

For Autopilot clusters, system pools render with a muted
`[managed by Autopilot]` suffix in the Name column and `—` in the
autoscale / version columns where the values do not apply. Node count
and Status still display for transparency.

No node-pool actions in Phase 1.

### `internal/ui/views/gke_messages.go` — internal & cross-view messages

```go
// Navigation
type GKEClusterSelectedMsg struct { ProjectID, Location, Name string }
type GKEClusterDeleteRequestMsg struct { ProjectID, Location, Name string }
type GKEClusterActionResultMsg struct { Action string; Error error }

// Internal — emitted only by gke_cluster_details.go
type gkeClusterLoadedMsg struct { details *gcp.ClusterDetails }
type gkeClusterErrorMsg struct { err error }
```

`GetCluster` returns node pools inline (`cluster.nodePools[]`), so a
single load message populates both tabs — no separate node-pool fetch
is needed for the initial render. The standalone `ListNodePools`
method on `ContainerClient` exists for Phase 2 (per-tab refresh, node
pool create/delete flows) and is unused in Phase 1.

### App wiring (`internal/ui/app.go`, `app_render.go`, `app_navigation.go`)

Per `adding-new-views.md` checklist:

1. `ViewGKEClusters`, `ViewGKEClusterDetails` added to ViewType enum.
2. `gkeClustersView`, `gkeClusterDetailsView` fields on `App`.
3. Case arms in `getCurrentViewModel()` and `renderCurrentView()`.
4. Update() message handlers: `GKEClusterSelectedMsg`,
   `GKEClusterDeleteRequestMsg`, `GKEClusterActionResultMsg`.
5. Navigation handlers in `app_navigation.go`.
6. `clearAllViews()` clears the two new fields.
7. `updateViewSizes()` sets context on both.
8. `updateSidebarActiveView()` switch maps both view types to the new
   sidebar entry.
9. Sidebar guard for both views in the GKE feature-hierarchy block.
10. `gkeClusterDetailsView.GetComputeClient()` added to the resolution
    chain in `handleNetworkSelected` and `handleSubnetSelected`.

### Sidebar & command palette

- **`internal/ui/components/sidebar/menu.go`**: new top-level group
  **"Kubernetes Engine"** with a single child entry **Clusters**. GKE
  is large enough to stand alone (multiple resource families in
  Phase 2+) and matches GCP-console navigation grouping.
- **`internal/ui/components/commandpalette/commands.go`**: adds
  `ViewGKEClusters` constant, icon (☸ — same symbol GCP and other tools
  use for Kubernetes; will fall back via ASCII flag), and the nav
  command `"Kubernetes Engine: Clusters"`.

## Delete flow

### Trigger

- `D` on a row in the clusters list, or
- `D` on the details view, or
- Action menu (`.`) entry on details view.

### Dialog

Uses the existing `confirmdialog` component (same as instance and LB
delete). Body warns explicitly about the non-obvious cleanup gap:

```
Delete cluster "prod-api"?
─────────────────────────────────────────────────────
This will permanently delete the cluster, its node pools,
and all workloads running in it. The operation takes 2–5
minutes and runs server-side after this call returns.

NOT auto-deleted by GKE:
  • Persistent volumes from dynamic provisioning unless
    the StorageClass uses reclaimPolicy=Delete
  • External load balancers created by Service of type
    LoadBalancer (deleted only if cluster removal succeeds)
  • Cloud DNS entries managed by the cluster

Type the cluster name to confirm:  [______________]
                                   [Cancel] [Delete]
```

### Behavior

1. Call `DeleteCluster` — returns when the API accepts the request.
2. Register a footer task via `registerRunningTask("Deleting cluster X...")`
   immediately before the call; `finishTask()` runs in the result
   handler regardless of outcome (per the async-operations-error-handling
   rule).
3. On success → navigate back to the clusters list and kick a refresh.
   Cluster appears with `STOPPING` status until GCP completes.
4. On error → emit `GKEClusterActionResultMsg{Action: "delete", Error: err}`.
   The app handler calls `SetError()` on the active view (per the same
   rule) so the user sees the error inline instead of being stranded in
   the saving state.

No polling of the Operation resource in Phase 1. Refresh is the user's
signal — matches how every other delete action in gcon already works
(instance delete, LB delete, Cloud Run service delete).

### No cluster-wide cascade walk

Unlike LB delete (which stitches forwarding rule → target proxy → URL
map → backend services → health checks), GKE's delete is atomic at the
API level. One call removes the cluster and everything inside it. The
warning copy about externally-created Persistent Disks and LB resources
is informational — gcon does not pre-scan or pre-delete them. The user
remains responsible for those if their `reclaimPolicy` is `Retain`.

## Keybindings

Adds the following section to `.claude/rules/key-bindings.md` once
implemented:

### GKE Clusters View

| Key | Action |
|---|---|
| `Enter` | View cluster details |
| `D` | Delete cluster (type-to-confirm) |
| `S` | Open sort menu |
| `/` | Filter clusters (`mode:`, `status:`, `location:` supported) |
| `r` | Refresh list |
| `Esc` | Go back |

### GKE Cluster Details View

| Key | Action |
|---|---|
| `D` | Delete cluster (type-to-confirm) |
| `.` | Open action menu |
| `r` | Refresh details and node pools |
| `Tab` | Switch focus (tabs / links / content) |
| `h/l` or `1/2` | Switch tabs (Overview / Node Pools) |
| `Enter` | Navigate via focused link (Network / Subnetwork) |
| `j/k` or `↑/↓` | Scroll content |
| `Esc` | Go back |

## Testing strategy

Mirrors LB Phase 1.

### `internal/gcp/gke_test.go`

- `convertCluster` — autopilot true vs false; regional vs zonal location;
  missing optional fields (no release channel, no workload identity,
  no master authorized networks, no database encryption).
- `convertNodePool` — autoscaling on vs off (the `autoscaling` block is
  sometimes nil); single-zone vs multi-zone pools; missing
  `Management` block (defaults to false / false for auto-upgrade
  / auto-repair).
- `locationType` helper — both shapes plus the empty string edge case.

### `internal/ui/views/gke_clusters_test.go`

- List renders with mock client.
- Mode column shows the right badge for autopilot vs standard.
- Filter `mode:autopilot` returns only autopilot rows.
- Sort by name and by location.

### `internal/ui/views/gke_cluster_details_test.go`

- Overview tab renders Network and Subnetwork as focusable link rows.
- Node Pools tab table contents match the converted pools.
- Autopilot detail rendering: system pool gets the
  `[managed by Autopilot]` suffix; autoscale / version cells show `—`.
- Delete dialog: type-to-confirm gating; correct cluster name accepts,
  wrong name rejects, submits the expected `GKEClusterDeleteRequestMsg`.
- `SetError()` resets state correctly (per the async rule).
- `Init()` is idempotent on second invocation (per `adding-new-views`
  step 11).

### `internal/ui/app_test.go` (or equivalent)

- Sidebar guard for GKE views: switching to Kubernetes Engine while
  already on `ViewGKEClusterDetails` does not pop into a nil parent
  view.

No new Cloud Monitoring tests in Phase 1.

## Dependencies

- New direct import: `google.golang.org/api/container/v1`. Already a
  transitive dep, so no `go.mod` size change beyond the explicit line.
- No new third-party packages.

## Risk & open questions

- **Status badge color mapping** for less-common statuses
  (`RECONCILING`, `DEGRADED`) — pick green for `RECONCILING` (matches
  GCP console treatment) and yellow for `DEGRADED`. Final palette
  decision happens at implementation time; no design impact.
- **Autopilot node count** — the cluster summary `currentNodeCount`
  field is unreliable for Autopilot. Phase 1 surfaces `(managed)` in
  the Nodes column for autopilot rows to avoid showing a misleading
  number. The Node Pools tab still lists whatever the API returns.
- **Deleted-but-still-listed window** — between hitting Delete and
  GCP fully removing the cluster, the status reads `STOPPING`.
  Refresh is the user's signal that it has cleared. No UI polling.

## Implementation order

Suggested commit sequence, sized after LB Phase 1:

1. `gcp/gke.go` skeleton + `Cluster` / `NodePool` / `ClusterDetails`
   types + `convertCluster` / `convertNodePool` (with tests).
2. `ContainerClient` constructor + `ListClusters` + `GetCluster` +
   `ListNodePools` + `DeleteCluster`.
3. `gke_clusters.go` list view + tests.
4. `gke_cluster_details.go` Overview tab + tests.
5. Node Pools tab on the details view + tests.
6. Delete dialog + result handler + tests.
7. App wiring (ViewType, switch arms, navigation handlers, sidebar
   guards, client resolution chain).
8. Sidebar entry + command palette entry.
9. README + CLAUDE.md + key-bindings.md updates.

Each step is its own commit; the full sequence lands as one PR (~10
commits), comparable to LB Phase 1.
