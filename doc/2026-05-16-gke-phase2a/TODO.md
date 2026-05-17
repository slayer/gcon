# GKE Phase 2a Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three new tabs (Nodes, Observability, Logs) to the GKE cluster details view shipped in Phase 1. All read-only; mutations land in Phase 2b.

**Architecture:** New `internal/gcp/monitoring_gke.go` adds the metric helpers (cluster CPU/memory/node count/pod count, node network rx/tx). `internal/gcp/compute.go` adds `ListManagedInstances` for node enumeration. Three new view files (`gke_nodes.go`, `gke_observability.go`, `gke_logs.go`) each owned by `GKEClusterDetailsView`, lazy-initialized on first tab visit. Auto-refresh + tabActive lifecycle mirrors LB Phase 2 / Cloud Run observability.

**Tech Stack:** Go 1.26+, Bubble Tea, `bubbles/viewport`, `metricchart` (HeightStandard, same as VM instance Observability), `cloud.google.com/go/monitoring/apiv3/v2`, `google.golang.org/api/container/v1`, `google.golang.org/api/compute/v1`.

**Reference docs:**
- Spec: `doc/2026-05-16-gke-phase2a/Design.md`
- Closest analogs: `internal/gcp/monitoring_lb.go`, `internal/ui/views/loadbalancer_observability.go`, `internal/ui/views/cloudrun_observability.go`, `internal/ui/views/instance_details.go` (HeightStandard charts)
- Required project rules: `.claude/rules/adding-new-views.md` (step 14 lazy creation; step 16 client chain), `.claude/rules/async-operations-error-handling.md` (SetError on action failures), `.claude/rules/bubble-tea-rendering.md` (viewport + Unicode width), `.claude/rules/component-patterns.md` (`tea.Tick` stale-message guard), `.claude/rules/gcp-api-gotchas.md` (single resource.type per filter)

> **⚠ Stale plan — read `.claude/rules/gcp-api-gotchas.md` before touching metric filters.**
> The original plan below references `kubernetes.io/cluster/*` filters
> (CPU, memory, node_count, pod_count). **That metric family does not
> exist in GCP** — the shipped implementation uses
> `kubernetes.io/node/*` and `kubernetes.io/pod/*` with cross-series
> reducers (REDUCE_MEAN for CPU/memory, REDUCE_COUNT for node/pod count).
> Source of truth: `internal/gcp/monitoring_gke.go` and the "GKE: No
> `kubernetes.io/cluster/*` metric family exists" entry in the gotchas
> rule. Do not re-introduce the cluster filters when updating Phase 2a.

**Pattern reminders worth re-reading before starting:**
- Sub-views are PLAIN STRUCTS, not `tea.Model` — they expose `Init() tea.Cmd`, `Update(tea.Msg) tea.Cmd`, `View() string`, `SetTabActive(bool)`, `Refresh() tea.Cmd`. The parent `GKEClusterDetailsView` orchestrates.
- `tea.Tick` messages survive context switches. Every tick handler must guard on `(autoRefresh && tabActive)` and the `tickAutoRefresh()` factory must return nil when either is false.
- Cells inside `bubbles/table` truncate by visual width via `runewidth` — ANSI escapes inflate the perceived width and the table slices mid-escape. Use the `symbols` package (emoji glyphs) for status indicators, never raw `lipgloss.NewStyle().Render("●")`. This is the Phase 1 status-column bug we already fixed; the lesson applies to the Nodes tab too.
- `metricchart.New(metricchart.HeightStandard)` (8 rows) for primary metrics — matches VM Instance Observability rendering.
- Charts get a per-chart error fallback ("metric fetch failed") so one failing metric doesn't blank the whole tab — mirrors `loadbalancer_observability.go`'s `lbObsWarning` accumulator.

---

## File Structure

**New files:**
- `internal/gcp/monitoring_gke.go` — metric helpers
- `internal/gcp/monitoring_gke_test.go` — filter-shape tests
- `internal/ui/views/gke_phase2a_messages.go` — internal sub-view messages
- `internal/ui/views/gke_nodes.go` — Nodes sub-view
- `internal/ui/views/gke_nodes_test.go`
- `internal/ui/views/gke_observability.go` — Observability sub-view
- `internal/ui/views/gke_observability_test.go`
- `internal/ui/views/gke_logs.go` — Logs sub-view
- `internal/ui/views/gke_logs_test.go`

**Modified files:**
- `internal/gcp/compute.go` — add `ListManagedInstances` + `MIGInstance`
- `internal/gcp/gke.go` — add `GKENode` (embeds `MIGInstance`, adds `Pool`)
- `internal/gcp/compute_test.go` — add `MIGInstance` projection test
- `internal/ui/views/gke_cluster_details.go` — extend tabs from 2 → 5, lazy sub-view creation, `tabActive` propagation, `r` dispatch, key routing for tab switches
- `internal/ui/views/gke_cluster_details_test.go` — tab cycling, lazy creation, `r` dispatch
- `internal/ui/app_navigation.go` — extend `handleInstanceSelected` client chain to include `gkeClusterDetailsView.GetComputeClient()`
- `CLAUDE.md`, `README.md`, `.claude/rules/key-bindings.md` — Phase 2a entries

---

## Task 1: `monitoring_gke.go` skeleton + filter helper + cluster CPU metric

**Files:**
- Create: `internal/gcp/monitoring_gke.go`
- Create: `internal/gcp/monitoring_gke_test.go`

- [ ] **Step 1: Write the failing filter-shape test**

```go
// internal/gcp/monitoring_gke_test.go
package gcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGKEFilter_ClusterCPU(t *testing.T) {
	f := gkeClusterFilter("prod", "us-central1", "kubernetes.io/cluster/cpu/allocatable_utilization")
	assert.Contains(t, f, `resource.type = "k8s_cluster"`)
	assert.Contains(t, f, `resource.labels.cluster_name = "prod"`)
	assert.Contains(t, f, `resource.labels.location = "us-central1"`)
	assert.Contains(t, f, `metric.type = "kubernetes.io/cluster/cpu/allocatable_utilization"`)
	// Regression: GCP rejects OR between resource.type values.
	assert.False(t, strings.Contains(f, " OR "), "filter must not contain OR between resource types")
}

func TestGKEFilter_NodeNetwork(t *testing.T) {
	f := gkeNodeFilter("prod", "us-central1", "kubernetes.io/node/network/received_bytes_count")
	assert.Contains(t, f, `resource.type = "k8s_node"`)
	assert.Contains(t, f, `resource.labels.cluster_name = "prod"`)
	assert.Contains(t, f, `metric.type = "kubernetes.io/node/network/received_bytes_count"`)
}
```

- [ ] **Step 2: Run, expect FAIL (undefined helpers)**

```bash
go test ./internal/gcp -run TestGKEFilter -v
```

- [ ] **Step 3: Implement `monitoring_gke.go` skeleton**

```go
// internal/gcp/monitoring_gke.go
package gcp

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GKEMetrics aggregates the time-series data the GKE Observability tab renders.
type GKEMetrics struct {
	CPUUtilization    []DataPoint // 0–100 (already scaled from the API's 0–1 allocatable ratio)
	MemoryUtilization []DataPoint // 0–100
	NodeCount         []DataPoint // integer count
	PodCount          []DataPoint // integer count
	NetworkRxBytes    []DataPoint // bytes/sec (sum across nodes)
	NetworkTxBytes    []DataPoint // bytes/sec
	LastFetch         time.Time
}

// gkeClusterFilter builds a Cloud Monitoring filter scoped to one cluster
// at the k8s_cluster resource level (CPU, memory, node count, pod count).
// GCP rejects OR between resource.type values (see .claude/rules/gcp-api-gotchas.md),
// so we pick one resource type per fetch and dispatch from the caller.
func gkeClusterFilter(clusterName, location, metricType string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`resource.type = "k8s_cluster" AND resource.labels.cluster_name = "%s" AND resource.labels.location = "%s" AND metric.type = "%s"`,
		clusterName, location, metricType,
	)
}

// gkeNodeFilter builds a Cloud Monitoring filter scoped to all nodes of
// one cluster (network rx/tx aggregated via REDUCE_SUM).
func gkeNodeFilter(clusterName, location, metricType string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`resource.type = "k8s_node" AND resource.labels.cluster_name = "%s" AND resource.labels.location = "%s" AND metric.type = "%s"`,
		clusterName, location, metricType,
	)
}

// fetchGKEMetric is the rate/mean fetcher mirrored from fetchLBMetric.
func (c *MonitoringClient) fetchGKEMetric(ctx context.Context, filter string, duration time.Duration, aligner monitoringpb.Aggregation_Aligner, reducer monitoringpb.Aggregation_Reducer) ([]DataPoint, error) {
	endTime := time.Now()
	startTime := endTime.Add(-duration)

	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   fmt.Sprintf("projects/%s", c.projectID),
		Filter: filter,
		Interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(startTime),
			EndTime:   timestamppb.New(endTime),
		},
		Aggregation: &monitoringpb.Aggregation{
			AlignmentPeriod:    durationpb.New(60 * time.Second),
			PerSeriesAligner:   aligner,
			CrossSeriesReducer: reducer,
		},
	}

	points, err := c.collectDataPoints(ctx, req)
	if err != nil {
		return nil, err
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Timestamp.Before(points[j].Timestamp) })
	return points, nil
}

const gkeMetricClusterCPU = "kubernetes.io/cluster/cpu/allocatable_utilization"

// GetGKEClusterCPUUtilization fetches cluster-scope CPU as a 0–100
// percentage. The raw GCP metric is a 0–1 ratio; we scale at the data
// layer so chart consumers can use SetYRange(0, 100) directly.
func (c *MonitoringClient) GetGKEClusterCPUUtilization(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeClusterFilter(clusterName, location, gkeMetricClusterCPU)
	points, err := c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_MEAN)
	if err != nil {
		return nil, err
	}
	for i := range points {
		points[i].Value *= 100
	}
	return points, nil
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
go test ./internal/gcp -run TestGKEFilter -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/gcp/monitoring_gke.go internal/gcp/monitoring_gke_test.go
git commit -m "2026-05-16: GKE phase 2a — monitoring_gke skeleton + cluster CPU"
```

---

## Task 2: Memory + Node count + Pod count metrics

**Files:** Modify `internal/gcp/monitoring_gke.go`.

- [ ] **Step 1: Add the three metric methods**

```go
const (
	gkeMetricClusterMemory = "kubernetes.io/cluster/memory/allocatable_utilization"
	gkeMetricNodeCount     = "kubernetes.io/cluster/node_count"
	gkeMetricPodCount      = "kubernetes.io/cluster/pod_count"
)

// GetGKEClusterMemoryUtilization fetches cluster-scope memory as a 0–100
// percentage (scaled from the API's 0–1 ratio).
func (c *MonitoringClient) GetGKEClusterMemoryUtilization(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeClusterFilter(clusterName, location, gkeMetricClusterMemory)
	points, err := c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_MEAN)
	if err != nil {
		return nil, err
	}
	for i := range points {
		points[i].Value *= 100
	}
	return points, nil
}

// GetGKEClusterNodeCount fetches current node count over time.
func (c *MonitoringClient) GetGKEClusterNodeCount(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeClusterFilter(clusterName, location, gkeMetricNodeCount)
	return c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_SUM)
}

// GetGKEClusterPodCount fetches running pod count over time.
func (c *MonitoringClient) GetGKEClusterPodCount(ctx context.Context, location, clusterName string, duration time.Duration) ([]DataPoint, error) {
	filter := gkeClusterFilter(clusterName, location, gkeMetricPodCount)
	return c.fetchGKEMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_MEAN, monitoringpb.Aggregation_REDUCE_SUM)
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/gcp/monitoring_gke.go
git commit -m "2026-05-16: GKE phase 2a — memory + node count + pod count metrics"
```

---

## Task 3: Network rx/tx metric

**Files:** Modify `internal/gcp/monitoring_gke.go`.

- [ ] **Step 1: Add the network method**

```go
const (
	gkeMetricNodeNetworkRx = "kubernetes.io/node/network/received_bytes_count"
	gkeMetricNodeNetworkTx = "kubernetes.io/node/network/sent_bytes_count"
)

// GetGKEClusterNetworkBytes returns aggregate rx/tx bytes-per-second across
// every node in the cluster. Uses the k8s_node resource type with REDUCE_SUM
// so a single time series surfaces total cluster network throughput.
func (c *MonitoringClient) GetGKEClusterNetworkBytes(ctx context.Context, location, clusterName string, duration time.Duration) (rx, tx []DataPoint, err error) {
	rxFilter := gkeNodeFilter(clusterName, location, gkeMetricNodeNetworkRx)
	rx, err = c.fetchGKEMetric(ctx, rxFilter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch network rx: %w", err)
	}
	txFilter := gkeNodeFilter(clusterName, location, gkeMetricNodeNetworkTx)
	tx, err = c.fetchGKEMetric(ctx, txFilter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch network tx: %w", err)
	}
	return rx, tx, nil
}
```

- [ ] **Step 2: Verify build + lint**

```bash
go build ./...
make lint
```

- [ ] **Step 3: Commit**

```bash
git add internal/gcp/monitoring_gke.go
git commit -m "2026-05-16: GKE phase 2a — network rx/tx metric"
```

---

## Task 4: `ListManagedInstances` + `MIGInstance` + `GKENode`

**Files:** Modify `internal/gcp/compute.go`, `internal/gcp/gke.go`, `internal/gcp/compute_test.go`.

- [ ] **Step 1: Add `MIGInstance` type and `ListManagedInstances` to `compute.go`**

```go
// MIGInstance is the subset of compute.Instance fields needed for
// Managed Instance Group enumeration (GKE node listing, in particular).
type MIGInstance struct {
	Name       string
	Zone       string
	Status     string // PROVISIONING / STAGING / RUNNING / STOPPING / TERMINATED
	InternalIP string
	ExternalIP string // "" when no external NIC
	CreatedAt  string // raw CreationTimestamp pass-through
}

// ListManagedInstances enumerates the VMs in a Managed Instance Group
// via instanceGroupManagers.listManagedInstances + a follow-up batch
// instances.get for the metadata. Returns MIGInstance projections.
func (c *ComputeClient) ListManagedInstances(ctx context.Context, projectID, zone, migName string) ([]MIGInstance, error) {
	resp, err := c.service.InstanceGroupManagers.ListManagedInstances(projectID, zone, migName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list managed instances %s/%s: %w", zone, migName, err)
	}
	out := make([]MIGInstance, 0, len(resp.ManagedInstances))
	for _, mi := range resp.ManagedInstances {
		name := shortName(mi.Instance)
		inst, err := c.service.Instances.Get(projectID, zone, name).Context(ctx).Do()
		if err != nil {
			// Instance may be mid-creation/deletion — surface a stub with what
			// we know rather than failing the whole pool fetch.
			out = append(out, MIGInstance{
				Name:   name,
				Zone:   zone,
				Status: instanceStatusOrTransient(mi),
			})
			continue
		}
		out = append(out, projectMIGInstance(inst, zone))
	}
	return out, nil
}

func instanceStatusOrTransient(mi *compute.ManagedInstance) string {
	if mi.InstanceStatus != "" {
		return mi.InstanceStatus
	}
	// CurrentAction is "CREATING" / "DELETING" / "RESTARTING" / etc.
	return mi.CurrentAction
}

func projectMIGInstance(inst *compute.Instance, zone string) MIGInstance {
	out := MIGInstance{
		Name:      inst.Name,
		Zone:      zone,
		Status:    inst.Status,
		CreatedAt: inst.CreationTimestamp,
	}
	if len(inst.NetworkInterfaces) > 0 {
		nic := inst.NetworkInterfaces[0]
		out.InternalIP = nic.NetworkIP
		if len(nic.AccessConfigs) > 0 {
			out.ExternalIP = nic.AccessConfigs[0].NatIP
		}
	}
	return out
}
```

- [ ] **Step 2: Add `GKENode` wrapper to `internal/gcp/gke.go`**

Append to `gke.go`:

```go
// GKENode is a cluster-aware view of a MIG instance. The Pool field is
// set by the caller in views/gke_nodes.go from loop context (we iterate
// each NodePool's InstanceGroupUrls), not parsed from the MIG name —
// pool names can contain hyphens, making MIG-name parsing ambiguous.
type GKENode struct {
	MIGInstance
	Pool string
}
```

- [ ] **Step 3: Write the failing test for `projectMIGInstance`**

```go
// internal/gcp/compute_test.go (append)
func TestProjectMIGInstance(t *testing.T) {
	raw := &compute.Instance{
		Name:              "gke-prod-default-7f3a-abcd",
		Status:            "RUNNING",
		CreationTimestamp: "2026-05-01T12:00:00Z",
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				NetworkIP: "10.128.0.12",
				AccessConfigs: []*compute.AccessConfig{
					{NatIP: "34.1.2.3"},
				},
			},
		},
	}
	got := projectMIGInstance(raw, "us-central1-a")
	assert.Equal(t, "gke-prod-default-7f3a-abcd", got.Name)
	assert.Equal(t, "us-central1-a", got.Zone)
	assert.Equal(t, "RUNNING", got.Status)
	assert.Equal(t, "10.128.0.12", got.InternalIP)
	assert.Equal(t, "34.1.2.3", got.ExternalIP)
	assert.Equal(t, "2026-05-01T12:00:00Z", got.CreatedAt)
}

func TestProjectMIGInstance_NoExternalIP(t *testing.T) {
	raw := &compute.Instance{
		Name:   "gke-prod-private-pool-aaaa-bbbb",
		Status: "RUNNING",
		NetworkInterfaces: []*compute.NetworkInterface{
			{NetworkIP: "10.128.0.99"},
		},
	}
	got := projectMIGInstance(raw, "us-central1-b")
	assert.Equal(t, "10.128.0.99", got.InternalIP)
	assert.Equal(t, "", got.ExternalIP)
}
```

- [ ] **Step 4: Run, expect PASS (helpers already added in Step 1)**

```bash
go test ./internal/gcp -run TestProjectMIGInstance -v
```

- [ ] **Step 5: Verify build + lint**

```bash
go build ./...
make lint
```

- [ ] **Step 6: Commit**

```bash
git add internal/gcp/compute.go internal/gcp/gke.go internal/gcp/compute_test.go
git commit -m "2026-05-16: GKE phase 2a — ListManagedInstances + MIGInstance + GKENode"
```

---

## Task 5: `gke_phase2a_messages.go` — internal sub-view messages

**Files:** Create `internal/ui/views/gke_phase2a_messages.go`.

- [ ] **Step 1: Write the file**

```go
// internal/ui/views/gke_phase2a_messages.go
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

type gkeNodesPoolErrorMsg struct {
	poolName string
	err      error
}

// === Observability tab ===

type gkeObsMetricsLoadedMsg struct {
	metrics  gcp.GKEMetrics
	warnings map[string]error // metric type -> error; empty when all good
}

type gkeObsRefreshTickMsg struct{}

// === Logs tab ===

type gkeLogsLoadedMsg struct {
	entries []gcp.LogEntry // type already exists in gcp package
}

type gkeLogsErrorMsg struct {
	err error
}

type gkeLogsRefreshTickMsg struct{}

// InstanceSelectedMsg cross-view message — verify name vs. existing
// instances.go usage before committing this file; reuse the existing
// type if it's already exported. If the existing message has a
// different shape, this file should NOT redeclare it.
//
// Phase 2a's Nodes tab emits the existing instance-selected message
// (whatever it's named) to navigate to the Compute Engine instance
// details view. The handler MUST resolve computeClient with the GKE
// details view added to the chain.
```

The comment about `InstanceSelectedMsg` is intentional — the implementer
should grep for the existing instance-selection message before this task
lands and confirm its name/shape. If it doesn't exist, declare a new
`InstanceSelectedMsg` here. The shape needed is `{ProjectID, Zone, Name}`.

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

The build may complain about unused message types — that's fine, Tasks
6–10 use them.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/gke_phase2a_messages.go
git commit -m "2026-05-16: GKE phase 2a — sub-view messages"
```

---

## Task 6: `gke_nodes.go` skeleton (struct + Init + lazy fetch fan-out)

**Files:** Create `internal/ui/views/gke_nodes.go`.

- [ ] **Step 1: Write the skeleton**

```go
// internal/ui/views/gke_nodes.go
package views

import (
	gocontext "context"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
)

// gkeNodes is the Nodes sub-view: a flat table over every node across
// every pool in the cluster, derived by fanning out ListManagedInstances
// over each (pool × pool.Locations × pool.InstanceGroupUrls) tuple.
type gkeNodes struct {
	projectID     string
	details       *gcp.ClusterDetails
	computeClient *gcp.ComputeClient

	table   table.Model
	spinner spinner.Model

	nodes        []gcp.GKENode // accumulated across pool fetches
	pendingMIGs  int           // counts outstanding ListManagedInstances calls
	warnings     map[string]error
	loading      bool
	err          error
	tabActive    bool
	width, height int
}

func newGKENodes(projectID string, details *gcp.ClusterDetails, computeClient *gcp.ComputeClient) *gkeNodes {
	columns := []table.Column{
		{Title: "Name", Width: 40},
		{Title: "Pool", Width: 16},
		{Title: "Zone", Width: 18},
		{Title: "Status", Width: 14},
		{Title: "Internal IP", Width: 16},
		{Title: "Age", Width: 8},
	}
	t := table.NewWithColumns(columns, "")
	return &gkeNodes{
		projectID:     projectID,
		details:       details,
		computeClient: computeClient,
		table:         t,
		spinner:       components.NewGCPSpinner(),
		warnings:      map[string]error{},
	}
}

func (s *gkeNodes) SetTabActive(active bool) { s.tabActive = active }

func (s *gkeNodes) Init() tea.Cmd {
	s.loading = true
	s.err = nil
	s.nodes = nil
	s.pendingMIGs = 0
	s.warnings = map[string]error{}
	return tea.Batch(s.spinner.Tick, s.fanOut())
}

// fanOut emits one tea.Cmd per (pool, zone, migURL) tuple. Each cmd
// returns gkeNodesPoolLoadedMsg with that MIG's nodes (pool name stamped
// from loop context) or gkeNodesPoolErrorMsg on failure.
func (s *gkeNodes) fanOut() tea.Cmd {
	if s.details == nil || s.computeClient == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, pool := range s.details.NodePools {
		for _, migURL := range pool.InstanceGroupUrls {
			zone, migName := parseMIGURL(migURL)
			if zone == "" || migName == "" {
				continue
			}
			s.pendingMIGs++
			poolName := pool.Name
			cmds = append(cmds, s.fetchMIG(poolName, zone, migName))
		}
	}
	if len(cmds) == 0 {
		s.loading = false
		return nil
	}
	return tea.Batch(cmds...)
}

func (s *gkeNodes) fetchMIG(poolName, zone, migName string) tea.Cmd {
	return func() tea.Msg {
		ctx := gocontext.Background()
		mis, err := s.computeClient.ListManagedInstances(ctx, s.projectID, zone, migName)
		if err != nil {
			return gkeNodesPoolErrorMsg{poolName: poolName, err: err}
		}
		nodes := make([]gcp.GKENode, 0, len(mis))
		for _, mi := range mis {
			nodes = append(nodes, gcp.GKENode{MIGInstance: mi, Pool: poolName})
		}
		return gkeNodesPoolLoadedMsg{poolName: poolName, nodes: nodes}
	}
}

// parseMIGURL extracts (zone, name) from a MIG URL of the form
// .../zones/{zone}/instanceGroupManagers/{name}.
func parseMIGURL(url string) (zone, name string) {
	// (intentionally simple — reuses the same shape as groupScope in
	// loadbalancer_backends.go; can be promoted to a shared helper if a
	// third caller appears.)
	// Implementation: split on "/", find "zones" then next, and
	// "instanceGroupManagers" then next.
	parts := splitURLPath(url)
	for i, p := range parts {
		switch p {
		case "zones":
			if i+1 < len(parts) {
				zone = parts[i+1]
			}
		case "instanceGroupManagers":
			if i+1 < len(parts) {
				name = parts[i+1]
			}
		}
	}
	return zone, name
}

// splitURLPath is a stdlib-only path splitter that avoids importing net/url
// for a single helper. Reused if/when a similar helper exists in views.
func splitURLPath(url string) []string {
	// Implement inline; do not import strings just for Split if strings is
	// not yet imported. (It IS imported by other files in this package;
	// safe to use here too.)
	// ...keep simple: import strings and call strings.Split(url, "/")
	return nil // replaced in real impl with strings.Split — left here only
	             // to show intent; the engineer will inline it.
}
```

Replace the `splitURLPath` stub with `strings.Split(url, "/")` directly in
`parseMIGURL` — the inline helper is shown above purely to express the
intent, not as the final code.

- [ ] **Step 2: Verify build**

```bash
go build ./internal/ui/...
```

If `table.NewWithColumns`, `components.NewGCPSpinner`, or any other symbol
diverges, mirror what `gke_clusters.go` did during Phase 1.

Also add stub methods so `gkeNodes` satisfies whatever shape the parent
view expects:

```go
func (s *gkeNodes) Update(msg tea.Msg) tea.Cmd { _ = msg; return nil } // TODO: Task 7
func (s *gkeNodes) View() string                { return "" }          // TODO: Task 7
func (s *gkeNodes) Refresh() tea.Cmd            { return s.Init() }
func (s *gkeNodes) SetSize(w, h int)            { s.width = w; s.height = h; s.table.SetSize(w, h-4) }
func (s *gkeNodes) HasTextInputFocused() bool   { return s.table.HasTextInputFocused() }
```

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/gke_nodes.go
git commit -m "2026-05-16: GKE phase 2a — Nodes sub-view skeleton + fan-out"
```

---

## Task 7: `gke_nodes.go` Update + View + rendering + tests

**Files:** Modify `internal/ui/views/gke_nodes.go`, create `internal/ui/views/gke_nodes_test.go`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/ui/views/gke_nodes_test.go
package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func gkeNodesWithFixtures() *gkeNodes {
	s := newGKENodes("proj", &gcp.ClusterDetails{
		Cluster: gcp.Cluster{Name: "prod", Location: "us-central1"},
		NodePools: []gcp.NodePool{
			{Name: "default"},
			{Name: "gpu-pool"},
		},
	}, nil)
	s.loading = false
	s.SetTabActive(true)
	s.nodes = []gcp.GKENode{
		{MIGInstance: gcp.MIGInstance{Name: "gke-prod-default-7f3a-abcd", Zone: "us-central1-a", Status: "RUNNING", InternalIP: "10.128.0.12", CreatedAt: "2026-05-01T12:00:00Z"}, Pool: "default"},
		{MIGInstance: gcp.MIGInstance{Name: "gke-prod-default-7f3a-efgh", Zone: "us-central1-b", Status: "RUNNING", InternalIP: "10.128.0.13", CreatedAt: "2026-05-01T12:00:00Z"}, Pool: "default"},
		{MIGInstance: gcp.MIGInstance{Name: "gke-prod-gpu-pool-2c1f-xyz1", Zone: "us-central1-a", Status: "STAGING", InternalIP: "10.128.0.27", CreatedAt: "2026-05-15T08:00:00Z"}, Pool: "gpu-pool"},
	}
	s.refreshTable()
	return s
}

func TestGKENodes_RendersRows(t *testing.T) {
	s := gkeNodesWithFixtures()
	out := s.View()
	assert.Contains(t, out, "gke-prod-default-7f3a-abcd")
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "gpu-pool")
	assert.Contains(t, out, "RUNNING")
	assert.Contains(t, out, "STAGING")
	assert.Contains(t, out, "10.128.0.12")
}

func TestGKENodes_FilterByPool(t *testing.T) {
	s := gkeNodesWithFixtures()
	s.table.SetFilter("pool:gpu-pool")
	out := s.View()
	assert.Contains(t, out, "gke-prod-gpu-pool-2c1f-xyz1")
	assert.NotContains(t, out, "gke-prod-default-7f3a-abcd")
}

func TestGKENodes_EnterEmitsInstanceSelected(t *testing.T) {
	s := gkeNodesWithFixtures()
	// Find or construct the existing instance-selection message; this test
	// asserts the cmd produced by Enter, after sending the appropriate
	// tea.KeyMsg through Update. The implementer adjusts the message type
	// name to match what's already in the codebase.
	// Example:
	//   cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	//   msg := cmd().(views.InstanceSelectedMsg)
	//   assert.Equal(t, "us-central1-a", msg.Zone)
	//   assert.Equal(t, "gke-prod-default-7f3a-abcd", msg.Name)
}
```

The Enter test is skeleton — the implementer fills it in once they know
the exact message-type name used elsewhere for instance navigation.
Leave it as a `t.Skip("populate after confirming InstanceSelectedMsg type")`
if the message name resolution is non-trivial.

- [ ] **Step 2: Run, expect FAIL (no refreshTable, no Update, no View)**

```bash
go test ./internal/ui/views -run TestGKENodes -v
```

- [ ] **Step 3: Implement `Update`, `View`, `refreshTable`, `handleKey`, `cursorNode`**

```go
import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/symbols"
)

func (s *gkeNodes) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gkeNodesPoolLoadedMsg:
		s.pendingMIGs--
		s.nodes = append(s.nodes, m.nodes...)
		s.dedupeAndSort()
		s.refreshTable()
		if s.pendingMIGs <= 0 {
			s.loading = false
		}
		return nil
	case gkeNodesPoolErrorMsg:
		s.pendingMIGs--
		s.warnings[m.poolName] = m.err
		if s.pendingMIGs <= 0 {
			s.loading = false
		}
		return nil
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

func (s *gkeNodes) handleKey(m tea.KeyMsg) tea.Cmd {
	switch m.String() {
	case "enter":
		n := s.cursorNode()
		if n == nil {
			return nil
		}
		// REPLACE THIS WITH THE ACTUAL CROSS-VIEW NAV MESSAGE TYPE.
		// See instances.go / app_navigation.go for the existing message.
		return func() tea.Msg {
			return InstanceSelectedMsg{ProjectID: s.projectID, Zone: n.Zone, Name: n.Name}
		}
	}
	_, cmd := s.table.Update(m)
	return cmd
}

func (s *gkeNodes) View() string {
	if s.loading && len(s.nodes) == 0 {
		return renderLoading(s.spinner, "Loading nodes...")
	}
	if s.err != nil && len(s.nodes) == 0 {
		return components.RenderError(s.err)
	}
	out := s.table.View()
	if len(s.warnings) > 0 {
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		var b strings.Builder
		b.WriteString("\n")
		for pool, err := range s.warnings {
			b.WriteString(muted.Render(fmt.Sprintf("  ⚠ %s: %v", pool, err)))
			b.WriteString("\n")
		}
		out += b.String()
	}
	return out
}

func (s *gkeNodes) refreshTable() {
	rows := make([]table.Row, 0, len(s.nodes))
	for i := range s.nodes {
		n := &s.nodes[i]
		rows = append(rows, table.Row{
			ID: n.Zone + "/" + n.Name,
			Data: []string{
				n.Name,
				n.Pool,
				n.Zone,
				gkeNodeStatusBadge(n.Status),
				defaultIfEmpty(n.InternalIP, "-"),
				formatAge(n.CreatedAt),
			},
			FilterValue: strings.Join([]string{n.Name, n.Pool, n.Zone, n.Status, n.InternalIP}, " "),
		})
	}
	s.table.SetRows(rows)
}

func (s *gkeNodes) dedupeAndSort() {
	seen := make(map[string]struct{}, len(s.nodes))
	out := make([]gcp.GKENode, 0, len(s.nodes))
	for _, n := range s.nodes {
		key := n.Zone + "/" + n.Name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	s.nodes = out
}

func (s *gkeNodes) cursorNode() *gcp.GKENode {
	row := s.table.SelectedRow()
	if row == nil {
		return nil
	}
	for i := range s.nodes {
		n := &s.nodes[i]
		if n.Zone+"/"+n.Name == row.ID {
			return n
		}
	}
	return nil
}

// gkeNodeStatusBadge mirrors the Phase 1 status badge but maps Compute
// Engine instance statuses (RUNNING / STAGING / PROVISIONING / ...) onto
// the symbols package. Reuses the emoji-only output to dodge the bubbles/
// table byte-truncation trap.
func gkeNodeStatusBadge(status string) string {
	if status == "" {
		return "—"
	}
	return symbols.GetStatusSymbol(status) + " " + status
}

// formatAge renders an RFC3339 CreationTimestamp as a short age string.
// Empty timestamp returns "-". Implementation calls time.Parse and emits
// "<n>d" / "<n>h" / "<n>m" / "<n>s" based on the largest non-zero unit.
func formatAge(rfc3339 string) string {
	if rfc3339 == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	default:
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
}
```

`defaultIfEmpty` already exists in `helpers.go` (Phase 1). `symbols.GetStatusSymbol`
handles RUNNING / STAGING / PROVISIONING / TERMINATED / STOPPED /
SUSPENDED.

- [ ] **Step 4: Run tests, expect PASS**

```bash
go test ./internal/ui/views -run TestGKENodes -v
```

- [ ] **Step 5: Lint**

```bash
make lint
```

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/gke_nodes.go internal/ui/views/gke_nodes_test.go
git commit -m "2026-05-16: GKE phase 2a — Nodes tab rendering + tests"
```

---

## Task 8: `gke_observability.go` skeleton (struct + Init + fetch coordinator)

**Files:** Create `internal/ui/views/gke_observability.go`.

- [ ] **Step 1: Write the skeleton**

```go
// internal/ui/views/gke_observability.go
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
// the auto-refresh ticker. Tab-active guard prevents stale ticks.
type gkeObservability struct {
	projectID, location, clusterName string
	gcpClient                        *gcp.Client

	cpuChart    *metricchart.Chart
	memoryChart *metricchart.Chart
	nodeChart   *metricchart.Chart
	podChart    *metricchart.Chart
	netChart    *metricchart.Chart

	metrics     gcp.GKEMetrics
	warnings    map[string]error // metric type -> error
	rangeIdx    int              // 0..4 → 1h/6h/24h/7d/30d
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

func (s *gkeObservability) SetTabActive(active bool) { s.tabActive = active }

func (s *gkeObservability) Init() tea.Cmd {
	s.loading = true
	return tea.Batch(s.spinner.Tick, s.fetchAllMetrics(), s.tickAutoRefresh())
}

func (s *gkeObservability) Refresh() tea.Cmd {
	s.loading = true
	s.err = nil
	s.warnings = map[string]error{}
	return tea.Batch(s.spinner.Tick, s.fetchAllMetrics())
}

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

func (s *gkeObservability) tickAutoRefresh() tea.Cmd {
	if !s.autoRefresh || !s.tabActive {
		return nil
	}
	return tea.Tick(30*time.Second, func(_ time.Time) tea.Msg { return gkeObsRefreshTickMsg{} })
}

// Stub Update/View/SetSize — replaced in Task 9.
func (s *gkeObservability) Update(msg tea.Msg) tea.Cmd { _ = msg; return nil } // TODO: Task 9
func (s *gkeObservability) View() string                { return "" }          // TODO: Task 9
func (s *gkeObservability) SetSize(w, h int)            { s.width = w; _ = h }
func (s *gkeObservability) HasTextInputFocused() bool   { return false }
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

If `metricchart.FormatPercentageStats` / `PercentYLabel` don't match the
actual exported names, mirror what `loadbalancer_observability.go` does
for the LB latency chart. The percent-formatting helpers exist in the
metricchart package.

If `gcp.Client.GetMonitoringClient(projectID)` differs, look at how the
LB observability sub-view resolves the monitoring client.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/gke_observability.go
git commit -m "2026-05-16: GKE phase 2a — Observability sub-view skeleton"
```

---

## Task 9: `gke_observability.go` Update + View + handleKey + tests

**Files:** Modify `internal/ui/views/gke_observability.go`, create `internal/ui/views/gke_observability_test.go`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/ui/views/gke_observability_test.go
package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func TestGKEObservability_DefaultState(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	assert.Equal(t, 0, s.rangeIdx) // 1h
	assert.True(t, s.autoRefresh)
	assert.False(t, s.tabActive)
}

func TestGKEObservability_RangeCycle(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	for i := 1; i <= 5; i++ {
		// implementer: send tea.KeyMsg with rune '1'..'5' through Update;
		// after each, assert s.rangeIdx == i-1.
	}
}

func TestGKEObservability_TickGuardWhenAutoRefreshOff(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	s.tabActive = true
	s.autoRefresh = false
	cmd := s.tickAutoRefresh()
	assert.Nil(t, cmd, "tick should be nil when autoRefresh is off")
}

func TestGKEObservability_TickGuardWhenTabInactive(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	s.tabActive = false
	s.autoRefresh = true
	cmd := s.tickAutoRefresh()
	assert.Nil(t, cmd, "tick should be nil when tab is inactive")
}

func TestGKEObservability_StaleTickDropped(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	s.tabActive = false
	s.autoRefresh = true
	cmd := s.Update(gkeObsRefreshTickMsg{})
	assert.Nil(t, cmd, "Update must drop stale refresh ticks")
}

func TestGKEObservability_PerMetricErrorSurfaced(t *testing.T) {
	s := newGKEObservability("proj", "us-central1", "prod", nil)
	s.tabActive = true
	s.SetSize(120, 40)
	loadedMsg := gkeObsMetricsLoadedMsg{
		metrics: gcp.GKEMetrics{
			CPUUtilization: []gcp.DataPoint{{Value: 50}},
		},
		warnings: map[string]error{"memory": assertErr("api: insufficient permission")},
	}
	s.Update(loadedMsg)
	out := s.View()
	assert.Contains(t, out, "memory")
	assert.Contains(t, out, "insufficient permission")
}

// assertErr is a tiny test helper — already exists in the views package
// from Phase 1 tests, or add one in the test file if not.
func assertErr(msg string) error {
	// implementer: use errors.New or fmt.Errorf
	return nil
}
```

- [ ] **Step 2: Run, expect FAIL**

```bash
go test ./internal/ui/views -run TestGKEObservability -v
```

- [ ] **Step 3: Replace stubs**

```go
func (s *gkeObservability) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gkeObsMetricsLoadedMsg:
		s.loading = false
		s.metrics = m.metrics
		s.warnings = m.warnings
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

func (s *gkeObservability) handleKey(m tea.KeyMsg) tea.Cmd {
	switch m.String() {
	case "1", "2", "3", "4", "5":
		idx := int(m.String()[0] - '1')
		if idx >= 0 && idx < len(gkeObsRanges) && idx != s.rangeIdx {
			s.rangeIdx = idx
			return s.Refresh()
		}
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

func (s *gkeObservability) View() string {
	// Implementer: render range bar + auto-refresh state + 5 charts, with
	// per-chart warnings rendered inline when present.
	// Pattern reference: loadbalancer_observability.go View().
	// Pseudocode:
	//   b.WriteString(renderRangeBar(s.rangeIdx, s.autoRefresh))
	//   b.WriteString("\n\nCluster CPU utilization\n")
	//   if w, ok := s.warnings["cpu"]; ok { b.WriteString("⚠ " + w.Error()); } else { b.WriteString(s.cpuChart.View()) }
	//   ... repeat for the other four charts ...
	//   return b.String()
}
```

The renderRangeBar helper produces `[1h] [6h] [24h] [7d] [30d]` with the
active range bold. Either reuse a similar helper from LB observability or
inline.

- [ ] **Step 4: Run tests, expect PASS**

```bash
go test ./internal/ui/views -run TestGKEObservability -v
```

- [ ] **Step 5: Lint**

```bash
make lint
```

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/gke_observability.go internal/ui/views/gke_observability_test.go
git commit -m "2026-05-16: GKE phase 2a — Observability charts + range + auto-refresh"
```

---

## Task 10: `gke_logs.go` skeleton (struct + Init + LQL builder)

**Files:** Create `internal/ui/views/gke_logs.go`.

- [ ] **Step 1: Write the skeleton**

Inspect `internal/ui/views/cloudrun_observability.go` first — its log
viewer integration is the closest analog. The Phase 2a Logs tab reuses
the same components (`logviewer.Model`, `LoggingClient`) with a
GKE-specific filter builder.

```go
// internal/ui/views/gke_logs.go
package views

import (
	gocontext "context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
)

// gkeLogs is the Logs sub-view: an embedded log viewer with severity
// toggles, a resource-type dropdown, and 15 s opt-in auto-refresh.
type gkeLogs struct {
	projectID, location, clusterName string
	gcpClient                        *gcp.Client

	infoOn, warnOn, errOn bool
	resourceType          string // "k8s_cluster" by default
	autoRefresh           bool
	tabActive             bool

	entries []gcp.LogEntry  // type already exists; verify exact shape
	err     error
	loading bool

	width int
}

func newGKELogs(projectID, location, clusterName string, client *gcp.Client) *gkeLogs {
	return &gkeLogs{
		projectID:    projectID,
		location:     location,
		clusterName:  clusterName,
		gcpClient:    client,
		infoOn:       true,
		warnOn:       true,
		errOn:        true,
		resourceType: "k8s_cluster",
		autoRefresh:  false,
	}
}

func (s *gkeLogs) SetTabActive(active bool) { s.tabActive = active }

func (s *gkeLogs) Init() tea.Cmd {
	s.loading = true
	return tea.Batch(components.NewGCPSpinner().Tick, s.fetchLogs(), s.tickAutoRefresh())
}

func (s *gkeLogs) Refresh() tea.Cmd {
	s.loading = true
	s.err = nil
	return s.fetchLogs()
}

func (s *gkeLogs) fetchLogs() tea.Cmd {
	if s.gcpClient == nil {
		return nil
	}
	query := s.buildLQL()
	return func() tea.Msg {
		ctx := gocontext.Background()
		lc, err := s.gcpClient.GetLoggingClient(s.projectID)
		if err != nil {
			return gkeLogsErrorMsg{err: err}
		}
		entries, err := lc.ListLogEntries(ctx, query, 100) // adjust signature if different
		if err != nil {
			return gkeLogsErrorMsg{err: err}
		}
		return gkeLogsLoadedMsg{entries: entries}
	}
}

// buildLQL composes the Cloud Logging filter from the sub-view's toggle state.
func (s *gkeLogs) buildLQL() string {
	parts := []string{
		fmt.Sprintf(`resource.type = "%s"`, s.resourceType),
		fmt.Sprintf(`resource.labels.cluster_name = "%s"`, s.clusterName),
		fmt.Sprintf(`resource.labels.location = "%s"`, s.location),
	}
	// Severity: pick the lowest enabled level. If all three enabled → INFO.
	switch {
	case s.infoOn:
		parts = append(parts, `severity >= INFO`)
	case s.warnOn:
		parts = append(parts, `severity >= WARNING`)
	case s.errOn:
		parts = append(parts, `severity >= ERROR`)
	default:
		parts = append(parts, `severity >= INFO`) // when all disabled, fall back to all
	}
	return strings.Join(parts, " AND ")
}

func (s *gkeLogs) tickAutoRefresh() tea.Cmd {
	if !s.autoRefresh || !s.tabActive {
		return nil
	}
	return tea.Tick(15*time.Second, func(_ time.Time) tea.Msg { return gkeLogsRefreshTickMsg{} })
}

// Stub Update/View/SetSize/HasTextInputFocused — replaced in Task 11.
func (s *gkeLogs) Update(msg tea.Msg) tea.Cmd { _ = msg; return nil } // TODO: Task 11
func (s *gkeLogs) View() string                { return "" }          // TODO: Task 11
func (s *gkeLogs) SetSize(w, h int)            { s.width = w; _ = h }
func (s *gkeLogs) HasTextInputFocused() bool   { return false }
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

If `gcp.LogEntry` / `LoggingClient.ListLogEntries` differ, mirror what
`cloudrun_observability.go` does.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/gke_logs.go
git commit -m "2026-05-16: GKE phase 2a — Logs sub-view skeleton + LQL builder"
```

---

## Task 11: `gke_logs.go` Update + View + key handling + tests

**Files:** Modify `internal/ui/views/gke_logs.go`, create `internal/ui/views/gke_logs_test.go`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/ui/views/gke_logs_test.go
package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGKELogs_DefaultLQL(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	q := s.buildLQL()
	assert.Contains(t, q, `resource.type = "k8s_cluster"`)
	assert.Contains(t, q, `resource.labels.cluster_name = "prod"`)
	assert.Contains(t, q, `resource.labels.location = "us-central1"`)
	assert.Contains(t, q, `severity >= INFO`)
}

func TestGKELogs_SeverityToggle(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.infoOn = false
	s.warnOn = false
	q := s.buildLQL()
	assert.Contains(t, q, `severity >= ERROR`)
}

func TestGKELogs_ResourceTypeSwitch(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.resourceType = "k8s_pod"
	q := s.buildLQL()
	assert.Contains(t, q, `resource.type = "k8s_pod"`)
}

func TestGKELogs_AutoRefreshOffByDefault(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	assert.False(t, s.autoRefresh)
}

func TestGKELogs_StaleTickDropped(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.tabActive = false
	s.autoRefresh = true
	cmd := s.Update(gkeLogsRefreshTickMsg{})
	assert.Nil(t, cmd)
}
```

- [ ] **Step 2: Run, expect FAIL (no real Update yet)**

```bash
go test ./internal/ui/views -run TestGKELogs -v
```

- [ ] **Step 3: Replace stubs**

```go
func (s *gkeLogs) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gkeLogsLoadedMsg:
		s.loading = false
		s.entries = m.entries
		return nil
	case gkeLogsErrorMsg:
		s.loading = false
		s.err = m.err
		return nil
	case gkeLogsRefreshTickMsg:
		if !s.autoRefresh || !s.tabActive {
			return nil
		}
		return tea.Batch(s.fetchLogs(), s.tickAutoRefresh())
	case tea.KeyMsg:
		return s.handleKey(m)
	}
	return nil
}

func (s *gkeLogs) handleKey(m tea.KeyMsg) tea.Cmd {
	switch m.String() {
	case "I":
		s.infoOn = !s.infoOn
		return s.Refresh()
	case "W":
		s.warnOn = !s.warnOn
		return s.Refresh()
	case "E":
		s.errOn = !s.errOn
		return s.Refresh()
	case "a":
		s.autoRefresh = !s.autoRefresh
		if s.autoRefresh {
			return s.tickAutoRefresh()
		}
	case "r":
		return s.Refresh()
	case "L":
		// Emit a message that the parent (or app) handler routes to the
		// Logs Explorer view with this cluster's filter pre-populated.
		// Reuse an existing message type if there is one; otherwise add
		// a new exported message in gke_phase2a_messages.go.
		return func() tea.Msg {
			return LogsExplorerOpenMsg{Query: s.buildLQL()}
		}
	}
	return nil
}

func (s *gkeLogs) View() string {
	// Implementer: render severity toggle row + resource-type indicator +
	// log entries (via the existing logviewer component or simple inline
	// rendering — match Cloud Run observability's approach).
}
```

`LogsExplorerOpenMsg` may already be exported by the Logs Explorer view —
verify and reuse instead of redeclaring.

- [ ] **Step 4: Run tests, expect PASS**

```bash
go test ./internal/ui/views -run TestGKELogs -v
```

- [ ] **Step 5: Lint**

```bash
make lint
```

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/gke_logs.go internal/ui/views/gke_logs_test.go
git commit -m "2026-05-16: GKE phase 2a — Logs tab toggles + key handling"
```

---

## Task 12: Wire 5-tab structure into `gke_cluster_details.go`

**Files:** Modify `internal/ui/views/gke_cluster_details.go`.

- [ ] **Step 1: Extend the tabs array from 2 → 5**

```go
v.tabs = tabs.New([]tabs.Tab{
    {ID: "overview",      Label: "Overview"},
    {ID: "nodepools",     Label: "Node Pools"},
    {ID: "nodes",         Label: "Nodes"},
    {ID: "observability", Label: "Observability"},
    {ID: "logs",          Label: "Logs"},
})
```

- [ ] **Step 2: Add sub-view fields to the struct**

```go
type GKEClusterDetailsView struct {
    // ...existing
    nodes         *gkeNodes
    observability *gkeObservability
    logs          *gkeLogs
}
```

- [ ] **Step 3: Add gcpClient field if not already present**

The new sub-views need `*gcp.Client` (for `GetMonitoringClient` and
`GetLoggingClient`). Check the existing struct — Phase 1 may have only
`*gcp.ContainerClient` and `*gcp.ComputeClient`. If `*gcp.Client` isn't
already a field, add one and wire it through:

```go
gcpClient *gcp.Client  // for monitoring + logging clients on demand
```

And update `NewGKEClusterDetailsView` signature to accept it. App-level
construction (`app_navigation.go`) passes `a.gcpClient`.

- [ ] **Step 4: Lazy sub-view creation in `TabChangedMsg` handler**

```go
case tabs.TabChangedMsg:
    // Toggle tabActive off on all sub-views first.
    if v.nodes != nil { v.nodes.SetTabActive(false) }
    if v.observability != nil { v.observability.SetTabActive(false) }
    if v.logs != nil { v.logs.SetTabActive(false) }

    switch v.tabs.ActiveTab().ID {
    case "nodes":
        if v.nodes == nil {
            v.nodes = newGKENodes(v.projectID, v.details, v.computeClient)
            v.nodes.SetSize(v.width-4, v.height-8)
        }
        v.nodes.SetTabActive(true)
        v.viewport.GotoTop()
        return v.nodes.Init()
    case "observability":
        if v.observability == nil {
            v.observability = newGKEObservability(v.projectID, v.location, v.name, v.gcpClient)
            v.observability.SetSize(v.width-4, v.height-8)
        }
        v.observability.SetTabActive(true)
        v.viewport.GotoTop()
        return v.observability.Init()
    case "logs":
        if v.logs == nil {
            v.logs = newGKELogs(v.projectID, v.location, v.name, v.gcpClient)
            v.logs.SetSize(v.width-4, v.height-8)
        }
        v.logs.SetTabActive(true)
        v.viewport.GotoTop()
        return v.logs.Init()
    }
    v.viewport.GotoTop()
    return nil
```

Per `adding-new-views.md` step 14: the sub-view is created BEFORE the
viewport content update.

- [ ] **Step 5: Add sub-view rendering to the View body switch**

In the existing `case "overview"` / `case "nodepools"` body switch, add:

```go
case "nodes":
    if v.nodes != nil {
        body = v.nodes.View()
    } else {
        body = renderLoading(v.spinner, "Loading nodes...")
    }
case "observability":
    if v.observability != nil {
        body = v.observability.View()
    } else {
        body = renderLoading(v.spinner, "Loading observability...")
    }
case "logs":
    if v.logs != nil {
        body = v.logs.View()
    } else {
        body = renderLoading(v.spinner, "Loading logs...")
    }
```

- [ ] **Step 6: Route `Update` to the active sub-view**

After the existing message-type cases in the parent `Update`, add fallthrough
delegation:

```go
default:
    switch v.tabs.ActiveTab().ID {
    case "nodes":
        if v.nodes != nil {
            if cmd := v.nodes.Update(msg); cmd != nil {
                return cmd
            }
        }
    case "observability":
        if v.observability != nil {
            if cmd := v.observability.Update(msg); cmd != nil {
                return cmd
            }
        }
    case "logs":
        if v.logs != nil {
            if cmd := v.logs.Update(msg); cmd != nil {
                return cmd
            }
        }
    }
    return nil
```

The detail view's own message handlers (Phase 1 ones like
`gkeClusterLoadedMsg`) still run first.

- [ ] **Step 7: Update `r` (refresh) to dispatch to active sub-view**

In `handleKey("r")`:

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
    // Phase 1 behaviour: reload cluster details.
    v.loading = true
    v.err = nil
    if v.client == nil {
        return tea.Batch(v.spinner.Tick, v.initClient())
    }
    return tea.Batch(v.spinner.Tick, v.load())
```

- [ ] **Step 8: Extend tab-switch shortcuts to 1..5**

In `handleKey`, the existing `case "h", "l", "1", "2"` row grows to
`"1", "2", "3", "4", "5"`. The `h/l` cyclers already work — `tabs.Update`
handles them.

- [ ] **Step 9: Update `HasTextInputFocused`**

```go
func (v *GKEClusterDetailsView) HasTextInputFocused() bool {
    if v.showConfirm && v.confirmDialog != nil {
        return v.confirmDialog.HasTextInputFocused()
    }
    if v.nodes != nil && v.nodes.HasTextInputFocused() { return true }
    if v.logs != nil && v.logs.HasTextInputFocused() { return true }
    return false
}
```

- [ ] **Step 10: Verify build + run existing tests**

```bash
go build ./...
go test ./internal/ui/views -run TestGKEClusterDetails -v
```

Existing Phase 1 tests should still pass.

- [ ] **Step 11: Lint**

```bash
make lint
```

- [ ] **Step 12: Commit**

```bash
git add internal/ui/views/gke_cluster_details.go
git commit -m "2026-05-16: GKE phase 2a — wire 5 tabs + lazy sub-views into details view"
```

---

## Task 13: App-level wiring for `InstanceSelectedMsg` from Nodes tab

**Files:** Modify `internal/ui/app_navigation.go` (handler client chain), `internal/ui/app.go` (Update message routing if needed).

- [ ] **Step 1: Confirm the existing instance-selection message type**

Run:
```bash
grep -rn "InstanceSelectedMsg\|instanceSelectedMsg" internal/ui/views/ internal/ui/
```

There should be an existing message used by the Compute Engine
instances list view. Note the exact name.

- [ ] **Step 2: Verify Nodes tab emits this exact type**

In `gke_nodes.go`'s `handleKey("enter")` (Task 7), the message returned
must be the same exact type identified in Step 1. If Task 7 used a
placeholder name, fix it now.

- [ ] **Step 3: Locate the handler in `app_navigation.go`**

Find `handleInstanceSelected` (or whatever the existing handler is
named) and inspect the client resolution chain.

- [ ] **Step 4: Extend the client chain to include the GKE details view**

Per `adding-new-views.md` step 16, the handler must resolve
`computeClient` from every view that can emit the message. Add:

```go
if computeClient == nil && a.gkeClusterDetailsView != nil {
    computeClient = a.gkeClusterDetailsView.GetComputeClient()
}
```

`GetComputeClient()` already exists on the GKE details view from Phase 1.

- [ ] **Step 5: Verify build + tests**

```bash
go build ./...
go test ./...
```

- [ ] **Step 6: Manual sanity check (no automated test required)**

Run the app, navigate to a GKE cluster → Nodes tab → Enter on a node →
verify the Compute Engine instance details view opens with the correct
instance.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/app_navigation.go
git commit -m "2026-05-16: GKE phase 2a — extend instance-selected client chain"
```

---

## Task 14: Detail view tests for tab cycling + lazy creation + r dispatch

**Files:** Modify `internal/ui/views/gke_cluster_details_test.go`.

- [ ] **Step 1: Write the failing tests**

```go
func TestGKEClusterDetails_AllFiveTabs(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	ids := []string{"overview", "nodepools", "nodes", "observability", "logs"}
	for _, id := range ids {
		v.tabs.SetActiveByID(id)
		out := v.View()
		assert.NotEmpty(t, out, "tab %s should render non-empty content", id)
	}
}

func TestGKEClusterDetails_LazyObservability(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	assert.Nil(t, v.observability, "observability should be nil before first visit")
	// Simulate tab switch — exact API depends on tabs.Model.
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{}) // ID inferred from active tab
	assert.NotNil(t, v.observability, "observability should be lazy-created on first visit")
}

func TestGKEClusterDetails_TabActiveToggling(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	assert.True(t, v.observability.tabActive)

	v.tabs.SetActiveByID("logs")
	v.Update(tabs.TabChangedMsg{})
	assert.False(t, v.observability.tabActive, "leaving observability must clear its tabActive flag")
	assert.True(t, v.logs.tabActive)
}

func TestGKEClusterDetails_RefreshOnObservability(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.observability)
	// Send 'r' through handleKey; assert observability.Refresh() ran.
	// The simplest assertion: observability.loading flips to true.
	v.observability.loading = false
	v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	assert.True(t, v.observability.loading, "r on observability tab should kick a refresh (loading=true)")
}
```

- [ ] **Step 2: Run, expect PASS (Task 12 wired this)**

```bash
go test ./internal/ui/views -run TestGKEClusterDetails_AllFiveTabs -v
go test ./internal/ui/views -run TestGKEClusterDetails_LazyObservability -v
go test ./internal/ui/views -run TestGKEClusterDetails_TabActiveToggling -v
go test ./internal/ui/views -run TestGKEClusterDetails_RefreshOnObservability -v
```

If any fail, fix the wiring in `gke_cluster_details.go`.

- [ ] **Step 3: Lint**

```bash
make lint
```

- [ ] **Step 4: Commit**

```bash
git add internal/ui/views/gke_cluster_details_test.go
git commit -m "2026-05-16: GKE phase 2a — detail view tests for tabs + lazy + r dispatch"
```

---

## Task 15: Documentation (CLAUDE.md, README.md, key-bindings.md)

**Files:** Modify `CLAUDE.md`, `README.md`, `.claude/rules/key-bindings.md`.

- [ ] **Step 1: `.claude/rules/key-bindings.md` — extend the GKE section**

Replace the GKE Cluster Details View table with the expanded shape from
the design (see Design.md "Keybindings" section). Add three new sub-sections:
- GKE Cluster Details — Nodes tab
- GKE Cluster Details — Observability tab
- GKE Cluster Details — Logs tab

Exact content is in `doc/2026-05-16-gke-phase2a/Design.md` under
"Keybindings (documentation update)".

- [ ] **Step 2: `CLAUDE.md` — extend the GKE Phase 1 entry**

Move from "Phase 1" to a combined "Phase 1 + 2a" with sub-bullets for the
new tabs. Add a "Phase 2b" bullet to Planned Features.

```markdown
- [x] GKE cluster management (Phase 1 + 2a)
  - Clusters list across all locations (zonal + regional, Autopilot + Standard)
  - Cluster details with Overview, Node Pools, Nodes, Observability, and Logs tabs
  - Type-to-confirm cluster delete with explicit warning about non-auto-deleted
    resources (dynamic-provisioned PVs, LB Services, Cloud DNS)
  - Fire-and-forget delete: API call returns immediately; refresh shows status
  - Nodes tab: flat list across all pools derived from MIG enumeration; Enter
    opens Compute Engine instance details
  - Observability tab: five cluster-scope charts (CPU%, Memory%, Node count,
    Pod count, Network rx/tx) with time range selector and 30s auto-refresh
  - Logs tab: filterable embedded log viewer with severity toggles
    (INFO/WARNING/ERROR) and resource-type dropdown (cluster/node/pod/container)
```

- [ ] **Step 3: `README.md` — update the Kubernetes Engine entry**

Append the new tabs and capabilities to the existing GKE entry in the
features list.

- [ ] **Step 4: Verify build + run full test suite + lint**

```bash
go build ./...
go test ./...
make lint
```

All three must be clean.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md README.md .claude/rules/key-bindings.md
git commit -m "2026-05-16: GKE phase 2a — docs (CLAUDE.md, README.md, key-bindings)"
```

---

## Final integration smoke (manual)

After Task 15 lands, run:

```bash
make run
```

In the app:

1. Open a GKE cluster's details view.
2. Tab through all 5 tabs — each shows non-empty content.
3. Observability tab: verify charts render at HeightStandard (8 rows of braille), not single-line sparklines.
4. Observability: press `1`–`5` to cycle ranges; each triggers a fresh fetch.
5. Observability: press `a` to toggle auto-refresh; UI indicator updates.
6. Nodes tab: filter `pool:default` narrows visible rows.
7. Nodes tab: Enter on a node row opens Compute Engine instance details with the right instance.
8. Logs tab: toggle `I`, `W`, `E` and confirm severity changes affect the visible entries.
9. Logs tab: switch resource type via `R` dropdown.
10. Press `r` on each of Nodes / Observability / Logs — each kicks its own refresh, NOT a cluster-wide reload.

If anything reads wrong, check the bubble-tea-rendering rules — most
likely a missing `viewport.GotoTop()` on tab change, a missing
`tabActive` flag flip, or an ANSI-styled status cell that bubbles/table
truncated.

---

## Self-review checklist (controller fills in before handoff)

- [x] Spec coverage: every section of Design.md has a corresponding task.
- [x] No placeholders: all "implement later"-style sentences either show concrete code or explicitly mark an implementer-resolution step (e.g., "confirm exact message-type name").
- [x] Type consistency: `gcp.GKEMetrics`, `gcp.GKENode`, `gcp.MIGInstance`, `gkeNodesPoolLoadedMsg`, `gkeObsRefreshTickMsg`, `gkeLogsLoadedMsg`, etc. spelled identically across tasks.
- [x] Message producer/consumer paired: every new `gke*Msg` type has both an emitter (sub-view) and a consumer (sub-view's Update).
- [x] `HasTextInputFocused()` extended for new sub-views.
- [x] `Init()` resets state on each sub-view (per the idempotent-Init rule).
- [x] Tickers gated on `(autoRefresh && tabActive)` in BOTH the factory AND the message handler (per the `tea.Tick` stale-message rule).
- [x] Status badges use `symbols.GetStatusSymbol` not raw `lipgloss.Render("●")` (per the Phase 1 byte-truncation lesson).
- [x] `r` dispatches per active tab, not cluster-wide reload.
- [x] Cross-view client chain updated for `InstanceSelectedMsg` (Task 13).
- [x] Charts use `metricchart.New(metricchart.HeightStandard)` — full multi-row braille, matching VM Instance Observability (per user feedback).
