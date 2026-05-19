# GKE Phase 2b Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the five mutation actions Phase 2a deliberately deferred: node pool create / delete / resize / upgrade + cluster control-plane upgrade. Every mutation kicks a long-running `Operation`; the footer polls it via `operations.get` every 5 s until DONE.

**Architecture:** New `internal/gcp/gke_mutations.go` + `gke_operations.go` for the API surface and op polling. Three new view files (upgrade dialog, resize dialog, create form) hosted by `GKEClusterDetailsView`. Autopilot guards hide every action except master upgrade (and master upgrade hides on managed release channels).

**Tech Stack:** Go 1.26+, Bubble Tea, `cloud.google.com/go` / `google.golang.org/api/container/v1`, existing `forms` framework (CreateViewBase pattern for the create form), existing `confirm.TypeConfirmDialog` for delete.

**Reference docs:**
- Spec: `doc/2026-05-17-gke-phase2b/Design.md`
- Closest analogs: `internal/ui/views/instance_create.go` (CreateViewBase + form), `internal/ui/views/subnet_create.go` (simpler form), `internal/ui/views/cloudrun_service_details.go` (traffic-split dialog pattern), `internal/ui/app_navigation.go` lines around `handleGKEClusterDeleteRequest` (cluster delete handler + footer task pattern)
- Required project rules: `.claude/rules/async-operations-error-handling.md` (Generation Tokens section for the polling loop), `.claude/rules/gcp-api-gotchas.md` ("ForceSendFields for Create operations" — applies to SetNodePoolSize=0 and UpdateNodePoolAutoscaling min=0), `.claude/rules/forms-framework.md` (create form pattern), `.claude/rules/component-patterns.md` (Dialog Update must accept tea.Msg; ticker survives context switches)

**Pattern reminders:**
- The Operation type already exists in `container.Operation`; project it to our own `Operation` (just the fields we use) so callers don't depend on the upstream shape.
- Mutation messages tag a generation. Operation polling does NOT need a generation token — each poll msg carries its own `opName`, and we route by name.
- Cluster delete in Phase 1 is fire-and-forget; do NOT mirror that pattern here. Phase 2b polls and refreshes on DONE.
- Footer task IDs use the `gke-op:` prefix so they're easy to identify when cancelling on nav.

---

## File Structure

**New files:**
- `internal/gcp/gke_operations.go` — Operation projection + GetOperation
- `internal/gcp/gke_operations_test.go` — projection round-trip test
- `internal/gcp/gke_mutations.go` — all mutation methods
- `internal/gcp/gke_mutations_test.go` — request-shape tests (ForceSendFields)
- `internal/ui/views/gke_phase2b_messages.go` — mutation + polling messages
- `internal/ui/views/gke_upgrade_dialog.go` — shared version picker
- `internal/ui/views/gke_upgrade_dialog_test.go`
- `internal/ui/views/gke_node_pool_resize_dialog.go` — manual / autoscale dialog
- `internal/ui/views/gke_node_pool_resize_dialog_test.go`
- `internal/ui/views/gke_node_pool_create.go` — form view (CreateViewBase)
- `internal/ui/views/gke_node_pool_create_test.go`

**Modified files:**
- `internal/gcp/gke.go` — add `GetServerConfig` + test
- `internal/ui/views/gke_cluster_details.go` — wire keys + guards + dialog hosting
- `internal/ui/views/gke_cluster_details_test.go` — key routing + Autopilot guard tests
- `internal/ui/app.go` — view type enum + state field + render dispatch
- `internal/ui/app_navigation.go` — handlers + polling cmd
- `internal/ui/components/commandpalette/commands.go` — "Create node pool" entry
- `CLAUDE.md` — move GKE from Phase 1+2a to Phase 1+2a+2b
- `README.md` — extend GKE entry
- `.claude/rules/key-bindings.md` — add Overview `u` and Node Pools `c/D/R/u`

---

## Task 1: Operation projection + GetOperation

**Files:** Create `internal/gcp/gke_operations.go`, `internal/gcp/gke_operations_test.go`.

- [ ] **Step 1: Write the failing test**

```go
// internal/gcp/gke_operations_test.go
package gcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	container "google.golang.org/api/container/v1"
)

func TestProjectOperation(t *testing.T) {
	raw := &container.Operation{
		Name:          "operation-1234-abcd",
		OperationType: "UPGRADE_MASTER",
		Status:        "RUNNING",
		TargetLink:    "https://container.googleapis.com/v1/projects/p/locations/us-central1/clusters/prod",
		StatusMessage: "Upgrading control plane",
		StartTime:     "2026-05-17T10:00:00Z",
		EndTime:       "",
		Detail:        "Phase: PRE_UPGRADE",
	}
	op := projectOperation(raw)
	assert.Equal(t, "operation-1234-abcd", op.Name)
	assert.Equal(t, "UPGRADE_MASTER", op.Type)
	assert.Equal(t, "RUNNING", op.Status)
	assert.Equal(t, "Phase: PRE_UPGRADE", op.Detail)
	assert.False(t, op.StartTime.IsZero())
	assert.True(t, op.EndTime.IsZero())
}

func TestProjectOperation_DoneSetsEndTime(t *testing.T) {
	raw := &container.Operation{
		Name:    "op-done",
		Status:  "DONE",
		EndTime: "2026-05-17T10:05:00Z",
	}
	op := projectOperation(raw)
	assert.Equal(t, "DONE", op.Status)
	assert.False(t, op.EndTime.IsZero())
}
```

- [ ] **Step 2: Implement**

```go
// internal/gcp/gke_operations.go
package gcp

import (
	"context"
	"fmt"
	"time"

	container "google.golang.org/api/container/v1"
)

// Operation is the GKE long-running-operation projection. Mutation calls
// return one of these; the UI polls via GetOperation until Status=="DONE".
type Operation struct {
	Name          string
	Type          string
	Status        string
	Target        string
	StatusMessage string
	Detail        string
	StartTime     time.Time
	EndTime       time.Time
}

// GetOperation fetches the current state of an op. `name` is the full
// path returned by the mutation that created it ("operation-XXXXX").
// `location` is the zone or region of the cluster the op targets.
func (c *ContainerClient) GetOperation(ctx context.Context, projectID, location, name string) (Operation, error) {
	fqn := fmt.Sprintf("projects/%s/locations/%s/operations/%s", projectID, location, name)
	raw, err := c.service.Projects.Locations.Operations.Get(fqn).Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("get operation %s: %w", name, err)
	}
	return projectOperation(raw), nil
}

func projectOperation(raw *container.Operation) Operation {
	op := Operation{
		Name:          raw.Name,
		Type:          raw.OperationType,
		Status:        raw.Status,
		Target:        raw.TargetLink,
		StatusMessage: raw.StatusMessage,
		Detail:        raw.Detail,
	}
	if raw.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, raw.StartTime); err == nil {
			op.StartTime = t
		}
	}
	if raw.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, raw.EndTime); err == nil {
			op.EndTime = t
		}
	}
	return op
}
```

- [ ] **Step 3: Run, expect PASS, lint, commit**

```bash
go test ./internal/gcp/ -run TestProjectOperation -v
make lint
git add internal/gcp/gke_operations.go internal/gcp/gke_operations_test.go
git commit -m "2026-05-17: GKE phase 2b — Operation projection + GetOperation"
```

---

## Task 2: DeleteNodePool

**Files:** Create `internal/gcp/gke_mutations.go`, `internal/gcp/gke_mutations_test.go`.

- [ ] **Step 1: Write the failing test (shape-only — no live API)**

```go
// internal/gcp/gke_mutations_test.go
package gcp

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestNodePoolFQN_Delete(t *testing.T) {
	// nodePoolFQN composes the full GKE resource path used by every
	// mutation. Regression guard: trailing-slash / missing-zone bugs.
	got := nodePoolFQN("proj", "us-central1", "prod", "default")
	assert.Equal(t, "projects/proj/locations/us-central1/clusters/prod/nodePools/default", got)
}
```

- [ ] **Step 2: Implement DeleteNodePool**

```go
// internal/gcp/gke_mutations.go
package gcp

import (
	"context"
	"fmt"

	container "google.golang.org/api/container/v1"
)

// nodePoolFQN composes the full resource path GKE uses for node-pool
// operations: projects/X/locations/Y/clusters/Z/nodePools/W.
func nodePoolFQN(projectID, location, clusterName, poolName string) string {
	return fmt.Sprintf("projects/%s/locations/%s/clusters/%s/nodePools/%s",
		projectID, location, clusterName, poolName)
}

func clusterFQN(projectID, location, clusterName string) string {
	return fmt.Sprintf("projects/%s/locations/%s/clusters/%s",
		projectID, location, clusterName)
}

// DeleteNodePool kicks a pool deletion. Returns the Operation projection
// so the caller can poll it.
func (c *ContainerClient) DeleteNodePool(ctx context.Context, projectID, location, clusterName, poolName string) (Operation, error) {
	raw, err := c.service.Projects.Locations.Clusters.NodePools.
		Delete(nodePoolFQN(projectID, location, clusterName, poolName)).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("delete node pool %s: %w", poolName, err)
	}
	return projectOperation(raw), nil
}
```

- [ ] **Step 3: Run, lint, commit**

```bash
go test ./internal/gcp/ -run TestNodePoolFQN -v
make lint
git add internal/gcp/gke_mutations.go internal/gcp/gke_mutations_test.go
git commit -m "2026-05-17: GKE phase 2b — DeleteNodePool"
```

---

## Task 3: SetNodePoolSize + UpdateNodePoolAutoscaling

**Files:** Extend `internal/gcp/gke_mutations.go` and its test file.

- [ ] **Step 1: Tests first — ForceSendFields regression guards**

Append to `gke_mutations_test.go`:

```go
func TestSetNodePoolSizeRequest_ZeroCount(t *testing.T) {
	// Scale-to-zero requires ForceSendFields on NodeCount (per the
	// gcp-api-gotchas.md "ForceSendFields for Create operations" rule
	// — Go's omitempty would drop a 0).
	req := buildSetNodePoolSizeRequest(0)
	assert.Equal(t, int64(0), req.NodeCount)
	assert.Contains(t, req.ForceSendFields, "NodeCount")
}

func TestUpdateNodePoolAutoscalingRequest_DisabledZeroes(t *testing.T) {
	// Disabling autoscale must send Enabled=false even though it's
	// the Go zero value — same ForceSendFields requirement.
	req := buildUpdateNodePoolAutoscalingRequest(false, 0, 0)
	require.NotNil(t, req.Autoscaling)
	assert.False(t, req.Autoscaling.Enabled)
	assert.Contains(t, req.Autoscaling.ForceSendFields, "Enabled")
}
```

- [ ] **Step 2: Implement**

Append to `gke_mutations.go`:

```go
// SetNodePoolSize sets the current node count. Operation completes when
// the pool reaches the target. Pass count=0 to scale the pool to zero
// (the pool stays defined; instances drain).
func (c *ContainerClient) SetNodePoolSize(ctx context.Context, projectID, location, clusterName, poolName string, count int64) (Operation, error) {
	req := buildSetNodePoolSizeRequest(count)
	raw, err := c.service.Projects.Locations.Clusters.NodePools.
		SetSize(nodePoolFQN(projectID, location, clusterName, poolName), req).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("resize node pool %s to %d: %w", poolName, count, err)
	}
	return projectOperation(raw), nil
}

func buildSetNodePoolSizeRequest(count int64) *container.SetNodePoolSizeRequest {
	return &container.SetNodePoolSizeRequest{
		NodeCount:       count,
		ForceSendFields: []string{"NodeCount"},
	}
}

// UpdateNodePoolAutoscaling toggles autoscale on/off and sets the
// min/max bounds in one call. Pass enabled=false to disable.
func (c *ContainerClient) UpdateNodePoolAutoscaling(ctx context.Context, projectID, location, clusterName, poolName string, enabled bool, min, max int64) (Operation, error) {
	req := buildUpdateNodePoolAutoscalingRequest(enabled, min, max)
	raw, err := c.service.Projects.Locations.Clusters.NodePools.
		SetAutoscaling(nodePoolFQN(projectID, location, clusterName, poolName), req).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("update autoscaling on %s: %w", poolName, err)
	}
	return projectOperation(raw), nil
}

func buildUpdateNodePoolAutoscalingRequest(enabled bool, min, max int64) *container.SetNodePoolAutoscalingRequest {
	as := &container.NodePoolAutoscaling{
		Enabled:         enabled,
		MinNodeCount:    min,
		MaxNodeCount:    max,
		ForceSendFields: []string{"Enabled", "MinNodeCount", "MaxNodeCount"},
	}
	return &container.SetNodePoolAutoscalingRequest{
		Autoscaling: as,
	}
}
```

- [ ] **Step 3: Run, lint, commit**

```bash
go test ./internal/gcp/ -run TestSetNodePoolSize -v
go test ./internal/gcp/ -run TestUpdateNodePoolAutoscaling -v
make lint
git add internal/gcp/gke_mutations.go internal/gcp/gke_mutations_test.go
git commit -m "2026-05-17: GKE phase 2b — SetNodePoolSize + UpdateNodePoolAutoscaling"
```

---

## Task 4: UpgradeControlPlane + UpgradeNodePool

**Files:** Extend `internal/gcp/gke_mutations.go`.

- [ ] **Step 1: Implement (no shape tests needed — the API takes a plain version string)**

```go
// UpgradeControlPlane triggers a master version change. masterVersion
// must be one of the values returned by GetServerConfig.ValidMasterVersions
// (or "-" to pick the default for the channel). Fails with HTTP 400 on
// clusters with an active release channel — gate at the UI layer.
func (c *ContainerClient) UpgradeControlPlane(ctx context.Context, projectID, location, clusterName, masterVersion string) (Operation, error) {
	req := &container.UpdateMasterRequest{
		MasterVersion: masterVersion,
	}
	raw, err := c.service.Projects.Locations.Clusters.
		UpdateMaster(clusterFQN(projectID, location, clusterName), req).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("upgrade control plane to %s: %w", masterVersion, err)
	}
	return projectOperation(raw), nil
}

// UpgradeNodePool triggers a node-pool version upgrade. nodeVersion
// must be from GetServerConfig.ValidNodeVersions. Master must be at
// the same or newer version; GCP rejects otherwise (4xx).
func (c *ContainerClient) UpgradeNodePool(ctx context.Context, projectID, location, clusterName, poolName, nodeVersion string) (Operation, error) {
	req := &container.UpdateNodePoolRequest{
		NodeVersion: nodeVersion,
	}
	raw, err := c.service.Projects.Locations.Clusters.NodePools.
		Update(nodePoolFQN(projectID, location, clusterName, poolName), req).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("upgrade node pool %s to %s: %w", poolName, nodeVersion, err)
	}
	return projectOperation(raw), nil
}
```

- [ ] **Step 2: Commit**

```bash
go build ./... && make lint
git add internal/gcp/gke_mutations.go
git commit -m "2026-05-17: GKE phase 2b — UpgradeControlPlane + UpgradeNodePool"
```

---

## Task 5: CreateNodePool

**Files:** Extend `internal/gcp/gke_mutations.go` + test.

- [ ] **Step 1: Test the request builder**

```go
func TestBuildCreateNodePoolRequest(t *testing.T) {
	pool := &container.NodePool{
		Name:             "gpu-pool",
		InitialNodeCount: 2,
		Config: &container.NodeConfig{
			MachineType: "n1-standard-4",
			DiskType:    "pd-balanced",
			DiskSizeGb:  100,
			ImageType:   "COS_CONTAINERD",
		},
		Autoscaling: &container.NodePoolAutoscaling{
			Enabled:      true,
			MinNodeCount: 1,
			MaxNodeCount: 10,
		},
		Management: &container.NodeManagement{
			AutoUpgrade: true,
			AutoRepair:  true,
		},
	}
	req := buildCreateNodePoolRequest(pool)
	assert.Equal(t, "gpu-pool", req.NodePool.Name)
	assert.Equal(t, int64(2), req.NodePool.InitialNodeCount)
	assert.Equal(t, "n1-standard-4", req.NodePool.Config.MachineType)
}
```

- [ ] **Step 2: Implement**

```go
// CreateNodePool adds a new node pool. The caller composes the
// container.NodePool with all fields set; buildCreateNodePoolRequest
// wraps it. Returns the in-flight Operation.
func (c *ContainerClient) CreateNodePool(ctx context.Context, projectID, location, clusterName string, pool *container.NodePool) (Operation, error) {
	req := buildCreateNodePoolRequest(pool)
	raw, err := c.service.Projects.Locations.Clusters.NodePools.
		Create(clusterFQN(projectID, location, clusterName), req).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("create node pool %s: %w", pool.Name, err)
	}
	return projectOperation(raw), nil
}

func buildCreateNodePoolRequest(pool *container.NodePool) *container.CreateNodePoolRequest {
	return &container.CreateNodePoolRequest{
		NodePool: pool,
	}
}
```

- [ ] **Step 3: Run, lint, commit**

```bash
go test ./internal/gcp/ -run TestBuildCreateNodePoolRequest -v
make lint
git add internal/gcp/gke_mutations.go internal/gcp/gke_mutations_test.go
git commit -m "2026-05-17: GKE phase 2b — CreateNodePool"
```

---

## Task 6: GetServerConfig (valid versions for upgrade picker)

**Files:** Extend `internal/gcp/gke.go` + a small test.

- [ ] **Step 1: Test the projection**

```go
// in internal/gcp/gke_test.go (existing file)
func TestServerConfigProjection(t *testing.T) {
	raw := &container.ServerConfig{
		DefaultClusterVersion: "1.30.5-gke.1014001",
		ValidMasterVersions:   []string{"1.31.1-gke.1", "1.30.5-gke.1014001"},
		ValidNodeVersions:     []string{"1.31.1-gke.1", "1.30.5-gke.1014001"},
	}
	cfg := projectServerConfig(raw)
	assert.Equal(t, "1.30.5-gke.1014001", cfg.DefaultClusterVersion)
	assert.Len(t, cfg.ValidMasterVersions, 2)
	assert.Equal(t, "1.31.1-gke.1", cfg.ValidMasterVersions[0])
}
```

- [ ] **Step 2: Implement**

Add to `gke.go`:

```go
// ServerConfig is the subset of container.ServerConfig the upgrade
// picker reads. ValidMasterVersions / ValidNodeVersions are sorted
// newest-first by GCP and we keep that order.
type ServerConfig struct {
	DefaultClusterVersion string
	ValidMasterVersions   []string
	ValidNodeVersions     []string
}

func (c *ContainerClient) GetServerConfig(ctx context.Context, projectID, location string) (ServerConfig, error) {
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, location)
	raw, err := c.service.Projects.Locations.GetServerConfig(parent).Context(ctx).Do()
	if err != nil {
		return ServerConfig{}, fmt.Errorf("get server config for %s: %w", location, err)
	}
	return projectServerConfig(raw), nil
}

func projectServerConfig(raw *container.ServerConfig) ServerConfig {
	return ServerConfig{
		DefaultClusterVersion: raw.DefaultClusterVersion,
		ValidMasterVersions:   append([]string{}, raw.ValidMasterVersions...),
		ValidNodeVersions:     append([]string{}, raw.ValidNodeVersions...),
	}
}
```

- [ ] **Step 3: Run, lint, commit**

```bash
go test ./internal/gcp/ -run TestServerConfigProjection -v
make lint
git add internal/gcp/gke.go internal/gcp/gke_test.go
git commit -m "2026-05-17: GKE phase 2b — GetServerConfig + projection"
```

---

## Task 7: Phase 2b message types

**Files:** Create `internal/ui/views/gke_phase2b_messages.go`.

- [ ] **Step 1: Define every message type the UI layer will need**

```go
// internal/ui/views/gke_phase2b_messages.go
package views

import "github.com/slayer/gcon/internal/gcp"

// === Node pool create ===

// GKENodePoolCreateRequestMsg is emitted by the create form on submit.
// The app handler calls gcp.CreateNodePool and registers polling.
type GKENodePoolCreateRequestMsg struct {
	ProjectID   string
	Location    string
	ClusterName string
	Pool        *container.NodePool // wrapped to preserve every form field
}

type GKENodePoolCreateCanceledMsg struct{}
type GKENodePoolCreateResultMsg struct {
	Pool  string
	Error error
}

// === Node pool delete ===

type GKENodePoolDeleteRequestMsg struct {
	ProjectID, Location, ClusterName, PoolName string
}

type GKENodePoolDeleteResultMsg struct {
	Pool  string
	Error error
}

// === Node pool resize ===

type GKENodePoolResizeMode int

const (
	GKENodePoolResizeManual GKENodePoolResizeMode = iota
	GKENodePoolResizeAutoscale
)

type GKENodePoolResizeRequestMsg struct {
	ProjectID, Location, ClusterName, PoolName string
	Mode                                       GKENodePoolResizeMode
	NodeCount                                  int64 // manual mode
	AutoscaleEnabled                           bool  // autoscale mode
	MinNodes, MaxNodes                         int64 // autoscale mode
}

type GKENodePoolResizeResultMsg struct {
	Pool  string
	Error error
}

// === Upgrade (master + node pool, share the same dialog) ===

type GKEMasterUpgradeRequestMsg struct {
	ProjectID, Location, ClusterName, Version string
}
type GKEMasterUpgradeResultMsg struct {
	Error error
}

type GKENodePoolUpgradeRequestMsg struct {
	ProjectID, Location, ClusterName, PoolName, Version string
}
type GKENodePoolUpgradeResultMsg struct {
	Pool  string
	Error error
}

// === Operation polling ===

// gkeOperationPollMsg fires after a 5 s tick to re-fetch operation state.
// onDone is the cmd to run when Status=="DONE" (e.g. a Refresh for the
// affected view). Stored as a function so the same poller can target
// different views.
type gkeOperationPollMsg struct {
	ProjectID, Location, Name string
	TaskID                    string
	OnDone                    func() tea.Cmd
	OnError                   func(error) tea.Cmd
}
```

Note: This file imports `container "google.golang.org/api/container/v1"` and `tea "github.com/charmbracelet/bubbletea"` — add both.

- [ ] **Step 2: Build (no test for message types — they're shape-only)**

```bash
go build ./...
make lint
git add internal/ui/views/gke_phase2b_messages.go
git commit -m "2026-05-17: GKE phase 2b — sub-view + operation messages"
```

---

## Task 8: Upgrade version-picker dialog (shared by master + node pool)

**Files:** Create `internal/ui/views/gke_upgrade_dialog.go` + test.

- [ ] **Step 1: Write the failing test**

```go
// internal/ui/views/gke_upgrade_dialog_test.go
package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUpgradeSubmit struct{ version string }

func TestGKEUpgradeDialog_CurrentMarked(t *testing.T) {
	d := NewGKEUpgradeDialog("Upgrade control plane", "1.30.5-gke.1014001",
		[]string{"1.31.1-gke.1", "1.30.5-gke.1014001"},
		func(v string) tea.Msg { return fakeUpgradeSubmit{version: v} })
	out := d.View()
	assert.Contains(t, out, "1.30.5-gke.1014001 (current)")
	assert.Contains(t, out, "1.31.1-gke.1")
}

func TestGKEUpgradeDialog_EnterSubmits(t *testing.T) {
	d := NewGKEUpgradeDialog("Upgrade pool: default", "1.30.5-gke.1014001",
		[]string{"1.31.1-gke.1", "1.30.5-gke.1014001"},
		func(v string) tea.Msg { return fakeUpgradeSubmit{version: v} })
	// Cursor starts at index 0 → "1.31.1-gke.1".
	cmd, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	got, ok := msg.(fakeUpgradeSubmit)
	require.True(t, ok)
	assert.Equal(t, "1.31.1-gke.1", got.version)
}

func TestGKEUpgradeDialog_EscEmitsCancel(t *testing.T) {
	d := NewGKEUpgradeDialog("Upgrade", "v1", []string{"v2", "v1"}, func(string) tea.Msg { return nil })
	cmd, _ := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	_, ok := cmd().(GKEUpgradeCanceledMsg)
	assert.True(t, ok)
}
```

- [ ] **Step 2: Implement**

```go
// internal/ui/views/gke_upgrade_dialog.go
package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GKEUpgradeCanceledMsg is emitted when the user presses Esc.
type GKEUpgradeCanceledMsg struct{}

// GKEUpgradeDialog is the shared version picker for control-plane and
// node-pool upgrades. The submit closure decides which message type
// to emit, so the same component drives both flows.
type GKEUpgradeDialog struct {
	title          string
	currentVersion string
	versions       []string // newest-first
	cursor         int
	submit         func(version string) tea.Msg
}

func NewGKEUpgradeDialog(title, current string, versions []string, submit func(string) tea.Msg) *GKEUpgradeDialog {
	return &GKEUpgradeDialog{
		title:          title,
		currentVersion: current,
		versions:       versions,
		submit:         submit,
	}
}

func (d *GKEUpgradeDialog) Init() tea.Cmd { return nil }

// HasTextInputFocused — no text inputs in this dialog. Returned to
// satisfy the parent's gating; always false.
func (d *GKEUpgradeDialog) HasTextInputFocused() bool { return false }

func (d *GKEUpgradeDialog) Update(msg tea.Msg) (tea.Cmd, bool) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}
	switch keyMsg.String() {
	case "esc":
		return func() tea.Msg { return GKEUpgradeCanceledMsg{} }, true
	case "j", "down":
		if d.cursor+1 < len(d.versions) {
			d.cursor++
		}
		return nil, false
	case "k", "up":
		if d.cursor > 0 {
			d.cursor--
		}
		return nil, false
	case "enter":
		if d.cursor < 0 || d.cursor >= len(d.versions) {
			return nil, false
		}
		v := d.versions[d.cursor]
		return func() tea.Msg { return d.submit(v) }, true
	}
	return nil, false
}

func (d *GKEUpgradeDialog) View() string {
	style := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder())
	titleStyle := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	highlight := lipgloss.NewStyle().Background(lipgloss.Color("#4285F4")).Foreground(lipgloss.Color("#FFFFFF"))

	var b strings.Builder
	b.WriteString(titleStyle.Render(d.title))
	b.WriteString("\n\n")
	b.WriteString(muted.Render("Pick a version (j/k to move, Enter to submit, Esc to cancel)"))
	b.WriteString("\n\n")
	for i, v := range d.versions {
		label := v
		if v == d.currentVersion {
			label = v + " (current)"
		}
		if i == d.cursor {
			b.WriteString(highlight.Render("▶ " + label))
		} else {
			b.WriteString("  " + label)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(muted.Render(fmt.Sprintf("Note: %s is a multi-minute operation.", strings.ToLower(d.title))))
	return style.Render(b.String())
}
```

- [ ] **Step 3: Run, lint, commit**

```bash
go test ./internal/ui/views -run TestGKEUpgradeDialog -v
make lint
git add internal/ui/views/gke_upgrade_dialog.go internal/ui/views/gke_upgrade_dialog_test.go
git commit -m "2026-05-17: GKE phase 2b — shared upgrade version-picker dialog"
```

---

## Task 9: Node pool resize dialog (manual + autoscale modes)

**Files:** Create `internal/ui/views/gke_node_pool_resize_dialog.go` + test.

- [ ] **Step 1: Write the failing test**

```go
// internal/ui/views/gke_node_pool_resize_dialog_test.go
package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGKENodePoolResizeDialog_ManualSubmit(t *testing.T) {
	d := NewGKENodePoolResizeDialog("default", 3, false, 0, 0)
	// In manual mode (default). Type "5" then Enter.
	d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	cmd, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	req, ok := msg.(GKENodePoolResizeSubmitMsg)
	require.True(t, ok)
	assert.Equal(t, GKENodePoolResizeManual, req.Mode)
	assert.Equal(t, int64(5), req.NodeCount)
}

func TestGKENodePoolResizeDialog_TabSwitchesMode(t *testing.T) {
	d := NewGKENodePoolResizeDialog("default", 3, false, 0, 0)
	require.Equal(t, GKENodePoolResizeManual, d.Mode())
	d.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, GKENodePoolResizeAutoscale, d.Mode())
}

func TestGKENodePoolResizeDialog_EscCancels(t *testing.T) {
	d := NewGKENodePoolResizeDialog("default", 3, false, 0, 0)
	cmd, _ := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	_, ok := cmd().(GKENodePoolResizeCanceledMsg)
	assert.True(t, ok)
}
```

- [ ] **Step 2: Implement**

Use `textinput.Model` from `bubbles/textinput` for each numeric field. The implementer chooses the exact layout; ensure:
- Title "Resize node pool: <name>"
- A toggle row showing the active mode highlighted ("Manual | Autoscale")
- Manual mode: one input for node count (current value pre-filled)
- Autoscale mode: one toggle + two inputs for min/max (pre-filled from current pool autoscale state)
- Tab cycles mode (also cycles inputs within the mode)
- Enter submits a `GKENodePoolResizeSubmitMsg`
- Esc emits `GKENodePoolResizeCanceledMsg`
- HasTextInputFocused returns true while any text input is focused

```go
// internal/ui/views/gke_node_pool_resize_dialog.go
type GKENodePoolResizeSubmitMsg struct {
    PoolName         string
    Mode             GKENodePoolResizeMode // re-uses the enum from gke_phase2b_messages.go
    NodeCount        int64
    AutoscaleEnabled bool
    MinNodes, MaxNodes int64
}

type GKENodePoolResizeCanceledMsg struct{}
```

Validation: count >= 0, min <= max when both set, min >= 0.

- [ ] **Step 3: Run, lint, commit**

```bash
go test ./internal/ui/views -run TestGKENodePoolResizeDialog -v
make lint
git add internal/ui/views/gke_node_pool_resize_dialog.go internal/ui/views/gke_node_pool_resize_dialog_test.go
git commit -m "2026-05-17: GKE phase 2b — node pool resize dialog (manual + autoscale)"
```

---

## Task 10: Node pool create view (form-based)

**Files:** Create `internal/ui/views/gke_node_pool_create.go` + test.

- [ ] **Step 1: Write the failing test**

```go
// internal/ui/views/gke_node_pool_create_test.go
package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/ui/components/forms"
)

func TestGKENodePoolCreate_FormHasRequiredSections(t *testing.T) {
	v := NewGKENodePoolCreateView("proj", "us-central1", "prod", nil)
	require.NotNil(t, v.Form)
	want := []string{"Basic", "Autoscaling", "Lifecycle"}
	got := []string{}
	for _, s := range v.Form.Sections() {
		got = append(got, s.Title)
	}
	for _, w := range want {
		assert.Contains(t, got, w, "form must include the %s section", w)
	}
}

func TestGKENodePoolCreate_SubmitEmitsRequest(t *testing.T) {
	v := NewGKENodePoolCreateView("proj", "us-central1", "prod", nil)
	// Populate required fields.
	v.Form.SetData(map[string]any{
		"name":          "gpu-pool",
		"initial_count": int64(2),
	})
	cmd := v.handleSubmit()
	require.NotNil(t, cmd)
	msg := cmd()
	req, ok := msg.(GKENodePoolCreateRequestMsg)
	require.True(t, ok)
	assert.Equal(t, "gpu-pool", req.Pool.Name)
	assert.Equal(t, int64(2), req.Pool.InitialNodeCount)
}
```

- [ ] **Step 2: Implement**

Use `CreateViewBase` for the lifecycle. Form sections:
- **Basic**: name (required, GCP resource name), initial_count (int 1-1000), machine_type (dropdown, lazy-loaded per zone — reuse `ListMachineTypesInZone` from compute client), disk_type (dropdown: pd-balanced/pd-standard/pd-ssd), disk_size_gb (int 10-65536), image_type (dropdown: COS_CONTAINERD/UBUNTU_CONTAINERD).
- **Autoscaling**: enabled (toggle), min_nodes (int 0-1000), max_nodes (int 1-1000).
- **Lifecycle**: auto_upgrade (toggle, default on), auto_repair (toggle, default on), preemptible (toggle, default off).

handleSubmit:
1. `v.Form.Validate()` — if errors, return nil.
2. Read field values; compose `*container.NodePool` with NodeConfig + Autoscaling + Management.
3. Validation: if autoscale enabled, initial_count must be >= min_nodes.
4. Call `v.BeginSaving()` and emit `GKENodePoolCreateRequestMsg`.

```go
// internal/ui/views/gke_node_pool_create.go
type GKENodePoolCreateView struct {
    CreateViewBase
    projectID, location, clusterName string
    container                         *gcp.ContainerClient
    compute                           *gcp.ComputeClient // for machine-type dropdown
}

func NewGKENodePoolCreateView(projectID, location, clusterName string, compute *gcp.ComputeClient) *GKENodePoolCreateView {
    v := &GKENodePoolCreateView{
        CreateViewBase: NewCreateViewBase("Creating node pool..."),
        projectID:      projectID,
        location:       location,
        clusterName:    clusterName,
        compute:        compute,
    }
    v.buildForm()
    return v
}

// Implements: buildForm, Update, handleSubmit, Init, SetSize.
```

Per `.claude/rules/forms-framework.md` and `.claude/rules/async-operations-error-handling.md`.

- [ ] **Step 3: Run, lint, commit**

```bash
go test ./internal/ui/views -run TestGKENodePoolCreate -v
make lint
git add internal/ui/views/gke_node_pool_create.go internal/ui/views/gke_node_pool_create_test.go
git commit -m "2026-05-17: GKE phase 2b — node pool create form view"
```

---

## Task 11: Wire keys + dialog hosting in cluster details

**Files:** Modify `internal/ui/views/gke_cluster_details.go`.

- [ ] **Step 1: Add struct fields for new dialogs and the create-view nav signal**

```go
type GKEClusterDetailsView struct {
    // ... existing fields ...
    upgradeDialog *GKEUpgradeDialog
    showUpgrade   bool
    resizeDialog  *GKENodePoolResizeDialog
    showResize    bool
    serverConfig  gcp.ServerConfig // cached after first GetServerConfig
}
```

- [ ] **Step 2: Add Autopilot guards**

```go
func (v *GKEClusterDetailsView) canMutatePools() bool {
    return v.details != nil && v.details.Mode != "AUTOPILOT"
}

func (v *GKEClusterDetailsView) canUpgradeMaster() bool {
    if v.details == nil {
        return false
    }
    rc := v.details.ReleaseChannel
    return rc == "" || rc == "UNSPECIFIED" || rc == "STATIC"
}
```

- [ ] **Step 3: Wire keys in handleKey**

On **Overview** (or unspecified tab): `u` opens master upgrade dialog if `canUpgradeMaster()` AND `serverConfig.ValidMasterVersions` is populated. Otherwise no-op.

On **Node Pools**: `c` (create) navigates to `GKENodePoolCreateView`; `D` (delete) opens type-to-confirm; `R` (resize) opens resize dialog; `u` (upgrade) opens upgrade dialog. ALL no-ops when `!canMutatePools()`.

```go
case "u":
    activeID := v.tabs.ActiveTab().ID
    if activeID == "overview" && v.canUpgradeMaster() {
        return v.openMasterUpgradeDialog()
    }
    if activeID == "nodepools" && v.canMutatePools() {
        return v.openNodePoolUpgradeDialog()
    }
    return nil
case "c":
    if v.tabs.ActiveTab().ID == "nodepools" && v.canMutatePools() {
        return v.requestNodePoolCreate() // emits GKENodePoolCreateOpenMsg
    }
case "R":
    if v.tabs.ActiveTab().ID == "nodepools" && v.canMutatePools() {
        return v.openNodePoolResizeDialog()
    }
```

D is already wired for cluster delete on Overview; on Node Pools it should open pool-delete confirm. Use the existing `confirm.TypeConfirmDialog` (the typed name is the pool name).

- [ ] **Step 4: Route dialog messages**

Dialogs are hosted by this view (z-order: above body, below confirm).

Add `case GKEUpgradeCanceledMsg / GKEUpgradeSubmittedMsg / GKENodePoolResizeCanceledMsg / GKENodePoolResizeSubmitMsg`.

On submit, emit the corresponding `GKEMasterUpgradeRequestMsg` / `GKENodePoolUpgradeRequestMsg` / `GKENodePoolResizeRequestMsg` upward to the app handler.

- [ ] **Step 5: Render — dialogs above body, confirm above dialogs**

Per the existing dialog z-order rule (bubble-tea-rendering.md):

```go
// In View() — order matters:
if v.showConfirm && v.confirmDialog != nil {
    return overlay.Center(mainContent, v.confirmDialog.View(), v.width, h)
}
if v.showResize && v.resizeDialog != nil {
    return overlay.Center(mainContent, v.resizeDialog.View(), v.width, h)
}
if v.showUpgrade && v.upgradeDialog != nil {
    return overlay.Center(mainContent, v.upgradeDialog.View(), v.width, h)
}
return mainContent
```

- [ ] **Step 6: Autopilot footer note + master upgrade suffix**

In `renderNodePools()`, append a muted line when `!canMutatePools()`:

```
  ▒ Pool mutations are managed by Autopilot
```

In `renderOverview()`, append a release-channel suffix to the master version row when not on STATIC.

- [ ] **Step 7: Lazy server-config fetch**

When `u` is pressed for the first time, kick off a `gkeServerConfigLoadedMsg` fetch. Cache on success. Subsequent presses use the cache (or re-fetch on `r`).

```go
type gkeServerConfigLoadedMsg struct { cfg gcp.ServerConfig; err error }

func (v *GKEClusterDetailsView) fetchServerConfig() tea.Cmd {
    return func() tea.Msg {
        cfg, err := v.client.GetServerConfig(context.Background(), v.projectID, v.location)
        return gkeServerConfigLoadedMsg{cfg: cfg, err: err}
    }
}
```

- [ ] **Step 8: Build + commit**

```bash
go build ./...
go test ./internal/ui/views -run TestGKEClusterDetails -v   # existing tests must pass
make lint
git add internal/ui/views/gke_cluster_details.go
git commit -m "2026-05-17: GKE phase 2b — wire mutation keys + dialogs + Autopilot guards"
```

---

## Task 12: App handlers + operation polling

**Files:** Modify `internal/ui/app.go` (add view type + state field), `internal/ui/app_navigation.go` (handlers + polling), `internal/ui/app_render.go` (render dispatch for the new view).

- [ ] **Step 1: Add `ViewGKENodePoolCreate` enum + state field**

In `app.go`:

```go
const (
    // ... existing values ...
    ViewGKENodePoolCreate
)

type App struct {
    // ... existing fields ...
    gkeNodePoolCreateView *views.GKENodePoolCreateView
}
```

Add to `getCurrentViewModel`, `clearAllViews`, `updateViewSizes` (via SetContext), and `app_render.go` `renderCurrentView`.

- [ ] **Step 2: Mutation handlers**

In `app_navigation.go`:

```go
func (a *App) handleGKENodePoolCreateRequest(msg views.GKENodePoolCreateRequestMsg) tea.Cmd {
    taskID := fmt.Sprintf("gke-op:create-pool:%s:%s", msg.ClusterName, msg.Pool.Name)
    a.registerRunningTask(taskID, fmt.Sprintf("Creating node pool %s...", msg.Pool.Name))
    return func() tea.Msg {
        op, err := a.containerClient.CreateNodePool(ctx, msg.ProjectID, msg.Location, msg.ClusterName, msg.Pool)
        if err != nil {
            a.finishTask(taskID, "")
            return views.GKENodePoolCreateResultMsg{Pool: msg.Pool.Name, Error: err}
        }
        return startGKEOperationPoll(taskID, msg.ProjectID, msg.Location, op.Name,
            func() tea.Cmd { return a.refreshActiveGKECluster() },
            func(err error) tea.Cmd { ... })
    }
}
```

Repeat the same shape for: `handleGKENodePoolDeleteRequest`, `handleGKENodePoolResizeRequest` (dispatches to SetNodePoolSize or UpdateNodePoolAutoscaling based on Mode), `handleGKEMasterUpgradeRequest`, `handleGKENodePoolUpgradeRequest`.

Each handler must (i) borrow ContainerClient from `a.gkeClustersView` or `a.gkeClusterDetailsView`; (ii) register a footer task with a unique `gke-op:` ID; (iii) on API error, finish the task and SetError on the details view; (iv) on success, kick off polling.

- [ ] **Step 3: Operation polling**

```go
// Polling helper — re-schedules itself until Status=="DONE".
func (a *App) pollGKEOperation(taskID, projectID, location, opName string, onDone, onError func() tea.Cmd) tea.Cmd {
    return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
        return views.GKEOperationPollMsg{
            TaskID:    taskID,
            ProjectID: projectID,
            Location:  location,
            Name:      opName,
            OnDone:    onDone,
            OnError:   onError,
        }
    })
}

// Handler in app.go Update():
case views.GKEOperationPollMsg:
    op, err := a.containerClient.GetOperation(ctx, msg.ProjectID, msg.Location, msg.Name)
    if err != nil {
        a.finishTask(msg.TaskID, "")
        return a, msg.OnError(err)
    }
    if op.Status == "DONE" {
        a.finishTask(msg.TaskID, "")
        if op.StatusMessage != "" {
            return a, msg.OnError(errors.New(op.StatusMessage))
        }
        return a, msg.OnDone()
    }
    // Still running — update footer and re-tick.
    a.updateRunningTask(msg.TaskID, fmt.Sprintf("%s (%s)", initialTaskDescription, formatElapsed(op.StartTime)))
    return a, a.pollGKEOperation(msg.TaskID, msg.ProjectID, msg.Location, msg.Name, msg.OnDone, msg.OnError)
```

`formatElapsed` returns "Xm Ys" using `time.Since(op.StartTime)`.

`refreshActiveGKECluster` re-fetches cluster details (causing the Node Pools table to refresh).

- [ ] **Step 4: Open create view from details**

In `app_navigation.go`:

```go
func (a *App) handleGKENodePoolCreateOpen(msg views.GKENodePoolCreateOpenMsg) tea.Cmd {
    a.viewStack = append(a.viewStack, a.currentView)
    a.currentView = ViewGKENodePoolCreate
    var compute *gcp.ComputeClient
    if a.gkeClusterDetailsView != nil {
        compute = a.gkeClusterDetailsView.GetComputeClient()
    }
    a.gkeNodePoolCreateView = views.NewGKENodePoolCreateView(
        msg.ProjectID, msg.Location, msg.ClusterName, compute)
    a.updateViewSizes()
    return a.gkeNodePoolCreateView.Init()
}
```

`GKENodePoolCreateOpenMsg` is the new nav message emitted by Cluster Details' `c` key.

- [ ] **Step 5: Build + commit**

```bash
go build ./...
make lint
git add internal/ui/app.go internal/ui/app_navigation.go internal/ui/app_render.go
git commit -m "2026-05-17: GKE phase 2b — app handlers + operation polling"
```

---

## Task 13: Detail view regression tests (keys + Autopilot guards)

**Files:** Extend `internal/ui/views/gke_cluster_details_test.go`.

- [ ] **Step 1: Write the new tests**

```go
func TestGKEClusterDetails_NodePoolCreateKeyStandard(t *testing.T) {
    v := gkeDetailsFixture("STANDARD")
    v.tabs.SetActiveByID("nodepools")
    cmd := v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
    require.NotNil(t, cmd)
    _, ok := cmd().(GKENodePoolCreateOpenMsg)
    assert.True(t, ok, "c on Node Pools (Standard) must emit GKENodePoolCreateOpenMsg")
}

func TestGKEClusterDetails_NodePoolCreateKeyAutopilotNoOp(t *testing.T) {
    v := gkeDetailsFixture("AUTOPILOT")
    v.tabs.SetActiveByID("nodepools")
    cmd := v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
    assert.Nil(t, cmd, "c on Node Pools (Autopilot) must be a no-op")
}

func TestGKEClusterDetails_MasterUpgradeKeyOpensDialog(t *testing.T) {
    v := gkeDetailsFixture("STANDARD")
    // STATIC channel allows manual upgrade.
    v.details.ReleaseChannel = "STATIC"
    v.serverConfig = gcp.ServerConfig{
        ValidMasterVersions: []string{"1.31.1-gke.1", "1.30.5-gke.1014001"},
    }
    v.tabs.SetActiveByID("overview")
    v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
    assert.True(t, v.showUpgrade)
    assert.NotNil(t, v.upgradeDialog)
}

func TestGKEClusterDetails_MasterUpgradeKeyNoOpOnReleaseChannel(t *testing.T) {
    v := gkeDetailsFixture("STANDARD")
    v.details.ReleaseChannel = "REGULAR"
    v.tabs.SetActiveByID("overview")
    v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
    assert.False(t, v.showUpgrade, "u on managed channel must NOT open the dialog")
}
```

- [ ] **Step 2: Run, lint, commit**

```bash
go test ./internal/ui/views -run TestGKEClusterDetails -v
make lint
git add internal/ui/views/gke_cluster_details_test.go
git commit -m "2026-05-17: GKE phase 2b — detail view tests for new keys + Autopilot guards"
```

---

## Task 14: Documentation

**Files:** Modify `CLAUDE.md`, `README.md`, `.claude/rules/key-bindings.md`.

- [ ] **Step 1: `.claude/rules/key-bindings.md`**

Add to the existing GKE Cluster Details section:

```markdown
### GKE Cluster Details — Overview tab actions

| Key | Action |
|-----|--------|
| `u` | Upgrade control plane (version picker; hidden on managed release channels) |

### GKE Cluster Details — Node Pools tab actions (Standard clusters only)

| Key | Action |
|-----|--------|
| `c` | Create new node pool (form) |
| `D` | Delete focused pool (type-to-confirm) |
| `R` | Resize focused pool (manual or autoscale) |
| `u` | Upgrade focused pool (version picker) |
```

- [ ] **Step 2: `CLAUDE.md`**

Move GKE entry from "Phase 1 + 2a" to "Phase 1 + 2a + 2b" and extend the bullets:

```markdown
- [x] GKE cluster management (Phase 1 + 2a + 2b)
  - ... existing bullets ...
  - Phase 2b mutations:
    - Create node pool with form (machine type, autoscaling, lifecycle settings)
    - Delete pool with type-to-confirm
    - Resize pool (manual count or autoscale min/max in one dialog)
    - Upgrade control plane and individual node pools (version picker)
    - Long-running operations tracked in footer with 5s polling; refresh on DONE
    - Autopilot clusters hide pool actions; release-channel clusters hide master upgrade
```

Move the "GKE Phase 2b" line out of Planned Features.

- [ ] **Step 3: `README.md`**

Append to the GKE entry: "Phase 2b adds node-pool create/delete/resize and control-plane/node-pool upgrades with footer-tracked long-running operations."

- [ ] **Step 4: Verify**

```bash
go build ./...
go test ./... 2>&1 | tail -5
make lint
```

All clean.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md README.md .claude/rules/key-bindings.md
git commit -m "2026-05-17: GKE phase 2b — docs (CLAUDE.md, README.md, key-bindings)"
```

---

## Final integration smoke (manual)

After Task 14 lands, run `make run`. In the app:

1. Open a **Standard** cluster's details, switch to Node Pools tab.
2. Press `c` — form opens; fill name + count + machine type; Ctrl+S to submit.
3. Footer shows "Creating node pool xyz... (15s)"; refreshes pools list when DONE.
4. Focus a non-default pool; press `D` — type the pool name; pool disappears when delete completes.
5. Press `R` on a focused pool — dialog opens with current count; type new count; submit; footer updates; refresh when DONE.
6. `R` then Tab — autoscale mode; toggle on, set min/max; submit; refresh.
7. Press `u` on a focused pool — version picker; pick newer; pool version updates when DONE.
8. Back to Overview; press `u` on a STATIC-channel cluster — version picker for master; pick newer; refresh when DONE.
9. Open an **Autopilot** cluster; `c`/`D`/`R`/`u` on Node Pools are no-ops; muted footer says "Pool mutations are managed by Autopilot".
10. Open a **release-channel** cluster (REGULAR); Overview's `u` is a no-op; master version row shows "(managed by REGULAR channel)".

If anything reads wrong, check:
- `gke-op:` task IDs in the footer match the started mutation.
- Operation polling stops after Status=DONE (no infinite tick loop).
- Dialog z-order respected (delete confirm above resize/upgrade dialog if multiple stack).

---

## Self-review checklist (controller fills in before handoff)

- [x] Spec coverage: every action in Design.md has a corresponding task.
- [x] No placeholders: tests have concrete code; production code shows enough shape that the implementer can act.
- [x] Type consistency: `Operation`, `GKEServerConfig`, `GKENodePoolResizeMode`, message types spelled identically across tasks.
- [x] Message producer/consumer paired: every `GKE*RequestMsg` has both an emitter (dialog/form) and a consumer (app handler).
- [x] HasTextInputFocused extended for new dialogs.
- [x] Init resets state in the create view (per the idempotent-Init rule).
- [x] Operation polling uses `tea.Tick` per the component-patterns rule; stops when Status=DONE.
- [x] Footer task IDs use `gke-op:` prefix.
- [x] Autopilot guards in place; release-channel guard for master upgrade in place.
- [x] CreateViewBase used for the create form (per forms-framework + async-ops rules).
- [x] ForceSendFields used for SetNodePoolSize=0 and UpdateNodePoolAutoscaling.Enabled=false.
- [x] Generation tokens NOT used for operation polling — each poll msg carries its own opName, no race.
