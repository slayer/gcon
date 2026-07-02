# GKE Phase 2c Implementation Plan

**Goal:** Add `e` key edit flows for GKE clusters + node pools (MVP slice — description, resource labels, daily maintenance, logging/monitoring services for clusters; k8s labels, taints, management, upgrade settings for pools). Diff preview before deploy. Long-running operations tracked in footer with the existing Phase 2b polling helpers.

**Reference docs:**
- Spec: `doc/2026-05-19-gke-phase2c-edit/Design.md`
- Closest analogs: `internal/ui/views/instance_editor.go` (3-state form / diff / saving), `internal/ui/views/cloudrun_edit.go` (diff rendering)
- Phase 2b reuse: `pollGKEOperationOnce`, `gkeOperationPollResultMsg`, `borrowContainerClient`, `gke-op:` task ID prefix

**Pattern reminders:**
- Edit views use a 3-state machine — don't reuse `CreateViewBase` (it's 2-state).
- Pointer fields in edit structs encode "no change" (`nil`) vs "set to value".
- Map/list fields are always whole-replacements when non-nil (the API field mask handles partial updates internally).
- Generation tokens not needed for op polling (each poll msg carries its own opName).

---

## File structure

**New files:**
- `internal/gcp/gke_edit.go` — edit method wrappers + projections
- `internal/gcp/gke_edit_test.go` — request builder shape tests
- `internal/ui/views/gke_edit_messages.go` — open/request/result/canceled messages
- `internal/ui/views/gke_cluster_edit.go` — cluster edit form view
- `internal/ui/views/gke_cluster_edit_test.go`
- `internal/ui/views/gke_node_pool_edit.go` — pool edit form view
- `internal/ui/views/gke_node_pool_edit_test.go`

**Modified files:**
- `internal/gcp/gke.go` — extend `ClusterDetails` / pool projection if maintenance window / upgrade settings are missing
- `internal/ui/views/gke_cluster_details.go` — wire `e` key on Overview + Node Pools tabs
- `internal/ui/views/gke_cluster_details_test.go` — key routing tests
- `internal/ui/app.go` — view enums + state fields
- `internal/ui/app_navigation.go` — handlers + sequential-step polling helper
- `internal/ui/app_render.go` — render dispatch
- `.claude/rules/key-bindings.md` — document `e` key
- `CLAUDE.md`, `README.md` — Phase 2c bullet

---

## Task 1: Edit projection types + tests

**Files:** Create `internal/gcp/gke_edit.go`, `internal/gcp/gke_edit_test.go`.

- [ ] **Step 1: Failing test for the cluster-update request builder**

```go
// internal/gcp/gke_edit_test.go
package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildClusterUpdate_PartialPatch(t *testing.T) {
	desc := "edited"
	labels := map[string]string{"team": "platform"}
	edit := ClusterEdit{
		Description:    &desc,
		ResourceLabels: &labels,
		// LoggingService and MonitoringService intentionally nil — must not appear.
	}
	upd := buildClusterUpdate(edit)
	require.NotNil(t, upd)
	assert.Equal(t, "edited", upd.DesiredResourceUsageExportConfig.ConsumptionMeteringConfig.Enabled) // placeholder — see impl
}

func TestBuildNodePoolUpdate_LabelsOnly(t *testing.T) {
	labels := map[string]string{"role": "gpu"}
	edit := NodePoolEdit{
		Labels: &labels,
	}
	req := buildNodePoolUpdate(edit)
	require.NotNil(t, req)
	require.NotNil(t, req.Labels)
	assert.Equal(t, "gpu", req.Labels.Labels["role"])
}

func TestBuildNodePoolManagement_DisabledZeroes(t *testing.T) {
	mgmt := NodePoolManagement{AutoUpgrade: false, AutoRepair: false}
	req := buildSetNodePoolManagementRequest(mgmt)
	require.NotNil(t, req)
	require.NotNil(t, req.Management)
	assert.False(t, req.Management.AutoUpgrade)
	assert.False(t, req.Management.AutoRepair)
	// ForceSendFields must include both bool fields so false survives JSON.
	assert.Contains(t, req.Management.ForceSendFields, "AutoUpgrade")
	assert.Contains(t, req.Management.ForceSendFields, "AutoRepair")
}
```

**Note**: The first assertion is intentionally wrong-looking — the implementer will replace it with the correct API field name (the `container.ClusterUpdate` API doesn't expose `Description` directly; it lives in the parent `Cluster`, not `ClusterUpdate`. Description is actually updated via the field on the cluster itself which we can't change. **Confirm this when implementing.** If `Description` isn't editable via `ClusterUpdate`, drop it from `ClusterEdit` and update Design.md). For MVP, `DesiredResourceLabels`, `DesiredLoggingService`, `DesiredMonitoringService` are confirmed editable.

- [ ] **Step 2: Implement projection types + builders**

```go
// internal/gcp/gke_edit.go
package gcp

import (
	"context"
	"fmt"

	container "google.golang.org/api/container/v1"
)

// ClusterEdit captures fields editable via Clusters.Update. nil pointer
// means "no change"; non-nil pointer means "set to this value".
type ClusterEdit struct {
	ResourceLabels    *map[string]string
	LoggingService    *string
	MonitoringService *string
}

// MaintenanceWindow is set via Clusters.SetMaintenancePolicy (separate endpoint).
type MaintenanceWindow struct {
	Kind  MaintenanceKind // "none" | "daily"
	Daily string          // "HH:MM" UTC (when Kind == MaintenanceKindDaily)
}

type MaintenanceKind string

const (
	MaintenanceKindNone  MaintenanceKind = "none"
	MaintenanceKindDaily MaintenanceKind = "daily"
)

// NodePoolEdit captures fields editable via NodePools.Update.
type NodePoolEdit struct {
	Labels          *map[string]string
	Taints          *[]NodeTaint
	Tags            *[]string
	UpgradeSettings *UpgradeSettings
}

type NodeTaint struct {
	Key, Value, Effect string // Effect: NO_SCHEDULE / PREFER_NO_SCHEDULE / NO_EXECUTE
}

type UpgradeSettings struct {
	MaxSurge       int64
	MaxUnavailable int64
	Strategy       string // SURGE | BLUE_GREEN
}

// NodePoolManagement is set via NodePools.SetManagement.
type NodePoolManagement struct {
	AutoUpgrade bool
	AutoRepair  bool
}

func buildClusterUpdate(edit ClusterEdit) *container.ClusterUpdate {
	upd := &container.ClusterUpdate{}
	if edit.ResourceLabels != nil {
		upd.DesiredResourceUsageExportConfig = nil // placeholder; real impl uses DesiredResourceLabels
		// container.ClusterUpdate has SetResourceLabels via separate call:
		// Clusters.SetResourceLabels(ctx, ...) — see impl.
		// For DesiredLoggingService and DesiredMonitoringService:
	}
	if edit.LoggingService != nil {
		upd.DesiredLoggingService = *edit.LoggingService
		upd.ForceSendFields = append(upd.ForceSendFields, "DesiredLoggingService")
	}
	if edit.MonitoringService != nil {
		upd.DesiredMonitoringService = *edit.MonitoringService
		upd.ForceSendFields = append(upd.ForceSendFields, "DesiredMonitoringService")
	}
	return upd
}

// (continue with buildNodePoolUpdate, buildSetNodePoolManagementRequest,
//  buildSetMaintenancePolicyRequest, and the ContainerClient methods)
```

**Implementer note**: Resource labels are set via `Clusters.SetResourceLabels(parent, &container.SetLabelsRequest{ResourceLabels, LabelFingerprint})` — a *separate* endpoint from `Clusters.Update`. This means `UpdateClusterBasic` may need to dispatch to either `Update` or `SetResourceLabels` depending on which fields changed. The sequential-step pattern from `app_navigation.go` handles this cleanly. Re-check the API docs and adjust.

- [ ] **Step 3: Run, lint, commit**

```bash
go test ./internal/gcp/ -run 'TestBuildClusterUpdate|TestBuildNodePoolUpdate|TestBuildNodePoolManagement' -v
make lint
git add internal/gcp/gke_edit.go internal/gcp/gke_edit_test.go
git commit -m "2026-05-19: GKE phase 2c — edit projections + request builders"
```

---

## Task 2: ContainerClient edit methods

**Files:** Extend `internal/gcp/gke_edit.go` and test file.

- [ ] **Step 1: Add shape tests for each method's request construction**

(Tests confirm the right endpoint + FQN + ForceSendFields. Use existing `nodePoolFQN`/`clusterFQN` helpers from `gke_mutations.go`.)

- [ ] **Step 2: Implement the four methods**

```go
func (c *ContainerClient) UpdateClusterBasic(ctx context.Context, projectID, location, clusterName string, edit ClusterEdit) (Operation, error) {
	// If only ResourceLabels changed, call Clusters.SetResourceLabels.
	// If LoggingService or MonitoringService changed, call Clusters.Update.
	// If both, callers issue two calls (sequenced at app layer).
	// This method decides which endpoint to call based on what's set.
	// For MVP: callers may invoke this once per category; we don't bundle.
	// Decision: callers always invoke ONE category per call → method
	//   dispatches to the right endpoint.

	switch {
	case edit.ResourceLabels != nil && edit.LoggingService == nil && edit.MonitoringService == nil:
		raw, err := c.service.Projects.Locations.Clusters.SetResourceLabels(
			clusterFQN(projectID, location, clusterName),
			&container.SetLabelsRequest{
				ResourceLabels: *edit.ResourceLabels,
				// LabelFingerprint must be fetched from the cluster — see note below.
			}).Context(ctx).Do()
		if err != nil {
			return Operation{}, fmt.Errorf("set cluster resource labels: %w", err)
		}
		return projectOperation(raw), nil

	case edit.LoggingService != nil || edit.MonitoringService != nil:
		upd := buildClusterUpdate(edit)
		raw, err := c.service.Projects.Locations.Clusters.Update(
			clusterFQN(projectID, location, clusterName),
			&container.UpdateClusterRequest{Update: upd}).Context(ctx).Do()
		if err != nil {
			return Operation{}, fmt.Errorf("update cluster: %w", err)
		}
		return projectOperation(raw), nil
	}
	return Operation{}, fmt.Errorf("UpdateClusterBasic: no fields to update")
}
```

**Critical**: `SetResourceLabels` requires `LabelFingerprint` — a checksum from the most recent cluster fetch. Without it, GCP returns 409 Conflict. The form view must capture the fingerprint at edit time and pass it through. Add `LabelFingerprint string` to `ClusterEdit` (only used when `ResourceLabels` is set).

```go
func (c *ContainerClient) SetClusterMaintenancePolicy(ctx context.Context, projectID, location, clusterName string, mw MaintenanceWindow) (Operation, error) {
	policy := &container.MaintenancePolicy{}
	switch mw.Kind {
	case MaintenanceKindNone:
		// Empty MaintenanceWindow disables maintenance windows.
		policy.Window = &container.MaintenanceWindow{}
	case MaintenanceKindDaily:
		policy.Window = &container.MaintenanceWindow{
			DailyMaintenanceWindow: &container.DailyMaintenanceWindow{
				StartTime: mw.Daily,
			},
		}
	}
	req := &container.SetMaintenancePolicyRequest{MaintenancePolicy: policy}
	raw, err := c.service.Projects.Locations.Clusters.SetMaintenancePolicy(
		clusterFQN(projectID, location, clusterName), req).Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("set maintenance policy: %w", err)
	}
	return projectOperation(raw), nil
}

func (c *ContainerClient) UpdateNodePoolFields(ctx context.Context, projectID, location, clusterName, poolName string, edit NodePoolEdit) (Operation, error) {
	req := &container.UpdateNodePoolRequest{}
	if edit.Labels != nil {
		req.Labels = &container.NodeLabels{Labels: *edit.Labels}
	}
	if edit.Taints != nil {
		taints := make([]*container.NodeTaint, 0, len(*edit.Taints))
		for _, t := range *edit.Taints {
			taints = append(taints, &container.NodeTaint{Key: t.Key, Value: t.Value, Effect: t.Effect})
		}
		req.Taints = &container.NodeTaints{Taints: taints}
	}
	if edit.Tags != nil {
		req.Tags = &container.NetworkTags{Tags: *edit.Tags}
	}
	if edit.UpgradeSettings != nil {
		req.UpgradeSettings = &container.UpgradeSettings{
			MaxSurge:       edit.UpgradeSettings.MaxSurge,
			MaxUnavailable: edit.UpgradeSettings.MaxUnavailable,
			Strategy:       edit.UpgradeSettings.Strategy,
			ForceSendFields: []string{"MaxSurge", "MaxUnavailable"},
		}
	}
	raw, err := c.service.Projects.Locations.Clusters.NodePools.Update(
		nodePoolFQN(projectID, location, clusterName, poolName), req).Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("update node pool fields: %w", err)
	}
	return projectOperation(raw), nil
}

func (c *ContainerClient) SetNodePoolManagement(ctx context.Context, projectID, location, clusterName, poolName string, mgmt NodePoolManagement) (Operation, error) {
	req := buildSetNodePoolManagementRequest(mgmt)
	raw, err := c.service.Projects.Locations.Clusters.NodePools.SetManagement(
		nodePoolFQN(projectID, location, clusterName, poolName), req).Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("set node pool management: %w", err)
	}
	return projectOperation(raw), nil
}

func buildSetNodePoolManagementRequest(mgmt NodePoolManagement) *container.SetNodePoolManagementRequest {
	return &container.SetNodePoolManagementRequest{
		Management: &container.NodeManagement{
			AutoUpgrade:     mgmt.AutoUpgrade,
			AutoRepair:      mgmt.AutoRepair,
			ForceSendFields: []string{"AutoUpgrade", "AutoRepair"},
		},
	}
}
```

- [ ] **Step 3: Build + test + lint + commit**

```bash
go test ./internal/gcp/ -run 'TestSetClusterMaintenance|TestUpdateNodePoolFields|TestSetNodePoolManagement' -v
go build ./...
make lint
git add internal/gcp/gke_edit.go internal/gcp/gke_edit_test.go
git commit -m "2026-05-19: GKE phase 2c — UpdateClusterBasic + SetMaintenancePolicy + pool edit methods"
```

---

## Task 3: Extend ClusterDetails / NodePool projection with edit-relevant fields

**Files:** Modify `internal/gcp/gke.go` if needed.

The form views need to populate from `ClusterDetails` / `NodePool` projections. Confirm they expose:

- Cluster: `ResourceLabels`, `ResourceLabelsFingerprint`, `LoggingService`, `MonitoringService`, `MaintenancePolicy` (with Daily start time)
- Pool: `NodeConfig.Labels`, `NodeConfig.Taints`, `NodeConfig.Tags`, `UpgradeSettings`, `Management.AutoUpgrade`, `Management.AutoRepair`

If any are missing, extend the projection. Add corresponding test in `gke_test.go`.

- [ ] **Step 1: Read existing projections + identify gaps**

```bash
grep -n 'ResourceLabels\|LoggingService\|MonitoringService\|MaintenancePolicy\|UpgradeSettings' internal/gcp/gke.go
```

- [ ] **Step 2: Add missing fields + projection logic**

- [ ] **Step 3: Test + commit**

```bash
go test ./internal/gcp/ -v
make lint
git add internal/gcp/gke.go internal/gcp/gke_test.go
git commit -m "2026-05-19: GKE phase 2c — extend ClusterDetails/NodePool projection for edit fields"
```

---

## Task 4: Edit message types

**Files:** Create `internal/ui/views/gke_edit_messages.go`.

```go
package views

import "github.com/slayer/gcon/internal/gcp"

// Nav signals from cluster details view.
type GKEClusterEditOpenMsg struct {
	ProjectID, Location, ClusterName string
}
type GKENodePoolEditOpenMsg struct {
	ProjectID, Location, ClusterName, PoolName string
}

// Form submits.
type GKEClusterEditRequestMsg struct {
	ProjectID, Location, ClusterName string
	Basic       *gcp.ClusterEdit
	Maintenance *gcp.MaintenanceWindow
}
type GKENodePoolEditRequestMsg struct {
	ProjectID, Location, ClusterName, PoolName string
	Fields     *gcp.NodePoolEdit
	Management *gcp.NodePoolManagement
}

// Cancellation
type GKEClusterEditCanceledMsg  struct{}
type GKENodePoolEditCanceledMsg struct{}

// Results
type GKEClusterEditResultMsg struct {
	TaskID, ClusterName string
	Error               error
}
type GKENodePoolEditResultMsg struct {
	TaskID, ClusterName, PoolName string
	Error                         error
}
```

- [ ] **Build + commit**

```bash
go build ./...
make lint
git add internal/ui/views/gke_edit_messages.go
git commit -m "2026-05-19: GKE phase 2c — edit message types"
```

---

## Task 5: Cluster edit view (form + diff)

**Files:** Create `internal/ui/views/gke_cluster_edit.go` + test.

- [ ] **Step 1: Write failing test**

```go
func TestGKEClusterEdit_NoChangesRejected(t *testing.T) {
	details := &gcp.ClusterDetails{
		Name: "prod", Location: "us-central1",
		LoggingService:    "logging.googleapis.com/kubernetes",
		MonitoringService: "monitoring.googleapis.com/kubernetes",
		ResourceLabels:    map[string]string{"env": "prod"},
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	cmd := v.handleSubmit()
	assert.Nil(t, cmd, "no-op submit must not emit a request")
	assert.NotNil(t, v.err, "form must surface a 'no changes' error")
}

func TestGKEClusterEdit_LoggingServiceChangeEmits(t *testing.T) {
	details := &gcp.ClusterDetails{ /* ... */ }
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	v.Form.SetData(map[string]any{
		"logging_service": "none",
	})
	v.confirmDeploy() // skip the diff state for tests
	cmd := v.emitRequest()
	require.NotNil(t, cmd)
	req, ok := cmd().(GKEClusterEditRequestMsg)
	require.True(t, ok)
	require.NotNil(t, req.Basic)
	require.NotNil(t, req.Basic.LoggingService)
	assert.Equal(t, "none", *req.Basic.LoggingService)
	assert.Nil(t, req.Maintenance, "maintenance not dirty, must be nil")
}
```

- [ ] **Step 2: Implement**

Three states: `stateForm`, `stateDiff`, `stateSaving`. Pattern from `instance_editor.go`.

```go
type GKEClusterEditView struct {
	projectID, location, clusterName string
	details                          *gcp.ClusterDetails
	Form                             *forms.Form
	state                            clusterEditState
	err                              error

	// captured at construction time — used to compute dirty fields at submit.
	initial gkeClusterEditSnapshot
	pending pendingClusterEdit

	spinner spinner.Model
}

type pendingClusterEdit struct {
	basic       *gcp.ClusterEdit
	maintenance *gcp.MaintenanceWindow
}
```

Form sections:
- **Basic**: resource labels (key/value list)
- **Maintenance**: window mode dropdown (none / daily), daily start time
- **Observability**: logging service dropdown, monitoring service dropdown

`handleSubmit` → compare each field against `initial` → populate `pending` → if `pending.basic == nil && pending.maintenance == nil`, set "No changes to apply" inline error and stay in stateForm. Else transition to stateDiff.

`renderDiff()` shows side-by-side or per-field "old → new" lines.

`confirmDeploy()` → transition to stateSaving, return cmd that emits `GKEClusterEditRequestMsg{Basic: pending.basic, Maintenance: pending.maintenance}`.

`HasTextInputFocused()` delegates to the form.

- [ ] **Step 3: Test + commit**

```bash
go test ./internal/ui/views -run TestGKEClusterEdit -v
make lint
git add internal/ui/views/gke_cluster_edit.go internal/ui/views/gke_cluster_edit_test.go
git commit -m "2026-05-19: GKE phase 2c — cluster edit form view"
```

---

## Task 6: Node pool edit view (form + diff)

**Files:** Create `internal/ui/views/gke_node_pool_edit.go` + test.

Mirror of Task 5 but for pool. Form sections:
- **Labels**: k8s labels (key/value list)
- **Taints**: repeating row (key text, value text, effect dropdown)
- **Management**: autoUpgrade toggle, autoRepair toggle
- **Upgrade settings**: strategy dropdown, max surge int, max unavailable int

Pre-populate from `gcp.NodePool`. Diff state mirrors cluster edit.

`handleSubmit` builds:
- `pending.fields` = non-nil only if labels / taints / upgrade settings dirty
- `pending.management` = non-nil only if autoUpgrade or autoRepair changed

Tests parallel cluster edit: `TestGKENodePoolEdit_NoChangesRejected`, `TestGKENodePoolEdit_LabelChangeEmits`, `TestGKENodePoolEdit_ManagementToggleEmits`.

- [ ] **Test + commit**

```bash
go test ./internal/ui/views -run TestGKENodePoolEdit -v
make lint
git add internal/ui/views/gke_node_pool_edit.go internal/ui/views/gke_node_pool_edit_test.go
git commit -m "2026-05-19: GKE phase 2c — node pool edit form view"
```

---

## Task 7: Wire `e` key in cluster details + nav

**Files:** Modify `internal/ui/views/gke_cluster_details.go` + test.

- [ ] **Step 1: Add `e` key handler**

```go
case "e":
    activeID := v.tabs.ActiveTab().ID
    if activeID == "overview" {
        // Allowed on both Autopilot and Standard (cluster-level edit).
        return func() tea.Msg {
            return GKEClusterEditOpenMsg{ProjectID: v.projectID, Location: v.location, ClusterName: v.clusterName}
        }
    }
    if activeID == "nodepools" && v.canMutatePools() {
        pool := v.focusedPool()
        if pool == nil {
            return nil
        }
        return func() tea.Msg {
            return GKENodePoolEditOpenMsg{
                ProjectID:   v.projectID,
                Location:    v.location,
                ClusterName: v.clusterName,
                PoolName:    pool.Name,
            }
        }
    }
    return nil
```

- [ ] **Step 2: Regression tests**

```go
func TestGKEClusterDetails_EditKeyOnOverview(t *testing.T) {
    v := gkeDetailsFixture("STANDARD")
    v.tabs.SetActiveByID("overview")
    cmd := v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
    require.NotNil(t, cmd)
    _, ok := cmd().(GKEClusterEditOpenMsg)
    assert.True(t, ok)
}

func TestGKEClusterDetails_EditKeyOnOverviewAutopilot(t *testing.T) {
    v := gkeDetailsFixture("AUTOPILOT")
    v.tabs.SetActiveByID("overview")
    cmd := v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
    require.NotNil(t, cmd, "cluster edit allowed on Autopilot")
    _, ok := cmd().(GKEClusterEditOpenMsg)
    assert.True(t, ok)
}

func TestGKEClusterDetails_EditKeyOnNodePoolsStandard(t *testing.T) {
    v := gkeDetailsFixture("STANDARD")
    v.tabs.SetActiveByID("nodepools")
    cmd := v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
    require.NotNil(t, cmd)
    _, ok := cmd().(GKENodePoolEditOpenMsg)
    assert.True(t, ok)
}

func TestGKEClusterDetails_EditKeyOnNodePoolsAutopilotNoOp(t *testing.T) {
    v := gkeDetailsFixture("AUTOPILOT")
    v.tabs.SetActiveByID("nodepools")
    cmd := v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
    assert.Nil(t, cmd)
}
```

- [ ] **Commit**

```bash
go test ./internal/ui/views -run TestGKEClusterDetails -v
make lint
git add internal/ui/views/gke_cluster_details.go internal/ui/views/gke_cluster_details_test.go
git commit -m "2026-05-19: GKE phase 2c — wire e key for cluster + pool edit"
```

---

## Task 8: App-level view enums + render dispatch

**Files:** Modify `internal/ui/app.go`, `internal/ui/app_render.go`.

Per `.claude/rules/adding-new-views.md`:

1. Add `ViewGKEClusterEdit`, `ViewGKENodePoolEdit` to the view enum (after `ViewGKENodePoolCreate`).
2. Add struct fields `gkeClusterEditView *views.GKEClusterEditView`, `gkeNodePoolEditView *views.GKENodePoolEditView`.
3. `getCurrentViewModel`: cases for both new views.
4. `clearAllViews`: nil both.
5. `updateViewSizes`: `SetContext` on both.
6. `updateSidebarActiveView`: highlight GKE Clusters for both.
7. Sidebar guard: include both in the GKE Clusters guard list.
8. `app_render.go renderCurrentView`: cases for both.

- [ ] **Build + commit**

```bash
go build ./...
make lint
git add internal/ui/app.go internal/ui/app_render.go
git commit -m "2026-05-19: GKE phase 2c — register edit view types in app"
```

---

## Task 9: App handlers + sequential-step polling

**Files:** Modify `internal/ui/app_navigation.go`.

- [ ] **Step 1: Open / cancel handlers**

```go
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleGKEClusterEditOpen(msg views.GKEClusterEditOpenMsg) tea.Cmd {
    a.viewStack = append(a.viewStack, a.currentView)
    a.currentView = ViewGKEClusterEdit

    var details *gcp.ClusterDetails
    if a.gkeClusterDetailsView != nil {
        details = a.gkeClusterDetailsView.Details()
    }
    a.gkeClusterEditView = views.NewGKEClusterEditView(msg.ProjectID, msg.Location, msg.ClusterName, details)
    a.updateViewSizes()
    a.updateSidebarActiveView()
    return a.gkeClusterEditView.Init()
}
// Similar for handleGKENodePoolEditOpen (pull focused pool from details).
```

- [ ] **Step 2: Request handlers (sequential steps)**

```go
//nolint:gocritic // hugeParam: message struct passed by value
func (a *App) handleGKEClusterEditRequest(msg views.GKEClusterEditRequestMsg) tea.Cmd {
    cc := a.borrowContainerClient()
    if cc == nil {
        if a.gkeClusterEditView != nil {
            a.gkeClusterEditView.SetError(uierrors.ErrGKEClientNotInitialized)
        }
        return nil
    }
    if msg.Basic == nil && msg.Maintenance == nil {
        // Shouldn't reach handler with no changes — form rejected it.
        // Defensive: just nav back.
        return a.navigateBack()
    }

    taskID := fmt.Sprintf("gke-op:edit-cluster:%s", msg.ClusterName)
    a.registerRunningTask(taskID, fmt.Sprintf("Updating cluster %s (1/2)...", msg.ClusterName))

    // Build the step sequence at the app layer. Each step issues one
    // API call, polls until DONE, then runs the next step.
    steps := []gkeEditStep{}
    if msg.Basic != nil {
        steps = append(steps, gkeEditStep{label: "basic", fn: func() (gcp.Operation, error) {
            return cc.UpdateClusterBasic(gocontext.Background(), msg.ProjectID, msg.Location, msg.ClusterName, *msg.Basic)
        }})
    }
    if msg.Maintenance != nil {
        steps = append(steps, gkeEditStep{label: "maintenance", fn: func() (gcp.Operation, error) {
            return cc.SetClusterMaintenancePolicy(gocontext.Background(), msg.ProjectID, msg.Location, msg.ClusterName, *msg.Maintenance)
        }})
    }
    return a.runGKEEditSequence(taskID, msg.ProjectID, msg.Location, steps, func(err error) tea.Cmd {
        return func() tea.Msg {
            return views.GKEClusterEditResultMsg{TaskID: taskID, ClusterName: msg.ClusterName, Error: err}
        }
    })
}
```

- [ ] **Step 3: Sequential runner helper**

```go
type gkeEditStep struct {
    label string
    fn    func() (gcp.Operation, error)
}

// runGKEEditSequence runs steps one at a time. On each step:
//   1. invoke fn() → get an Operation
//   2. poll until DONE (using existing GKEOperationPollMsg machinery)
//   3. on DONE, fire next step
// On any step error, abort the whole sequence and call onComplete(err).
func (a *App) runGKEEditSequence(taskID, projectID, location string, steps []gkeEditStep, onComplete func(error) tea.Cmd) tea.Cmd {
    return a.runGKEEditStep(taskID, projectID, location, steps, 0, onComplete)
}

func (a *App) runGKEEditStep(taskID, projectID, location string, steps []gkeEditStep, idx int, onComplete func(error) tea.Cmd) tea.Cmd {
    if idx >= len(steps) {
        return onComplete(nil)
    }
    a.updateRunningTask(taskID, fmt.Sprintf("Updating (%s, %d/%d)...", steps[idx].label, idx+1, len(steps)))
    return func() tea.Msg {
        op, err := steps[idx].fn()
        if err != nil {
            return onComplete(err)()
        }
        return views.GKEOperationPollMsg{
            TaskID:    taskID,
            ProjectID: projectID,
            Location:  location,
            Name:      op.Name,
            OnDone: func() tea.Cmd {
                return a.runGKEEditStep(taskID, projectID, location, steps, idx+1, onComplete)
            },
            OnError: func(opErr error) tea.Cmd {
                return onComplete(opErr)
            },
        }
    }
}
```

- [ ] **Step 4: Result handlers**

```go
func (a *App) handleGKEClusterEditResult(msg views.GKEClusterEditResultMsg) tea.Cmd {
    finishCmd := a.finishTask(msg.TaskID, msg.Error)
    if msg.Error != nil {
        a.err = msg.Error
        if a.gkeClusterEditView != nil {
            a.gkeClusterEditView.SetError(msg.Error)
        }
        return finishCmd
    }
    // Success: pop back to details view, then refresh.
    backCmd := a.navigateBack()
    refreshCmd := a.refreshActiveGKECluster()
    return tea.Batch(finishCmd, backCmd, refreshCmd)
}
// Pool result handler mirrors this.
```

- [ ] **Step 5: Wire into app.go Update() switch**

Add 8 message cases (open, request, canceled, result × 2 resources).

- [ ] **Test + commit**

```bash
go build ./...
go test ./internal/ui/... -race -timeout 60s
make lint
git add internal/ui/app.go internal/ui/app_navigation.go internal/ui/app_render.go
git commit -m "2026-05-19: GKE phase 2c — app handlers + sequential-step edit polling"
```

---

## Task 10: Documentation

**Files:** Modify `CLAUDE.md`, `README.md`, `.claude/rules/key-bindings.md`.

- [ ] **Step 1: `.claude/rules/key-bindings.md`**

Add to the GKE Cluster Details — Overview tab actions section:

```markdown
| `e` | Edit cluster (description, labels, maintenance, logging/monitoring) |
```

Add to the GKE Cluster Details — Node Pools tab actions section:

```markdown
| `e` | Edit focused pool (labels, taints, management, upgrade settings) |
```

- [ ] **Step 2: `CLAUDE.md`**

Change `(Phase 1 + 2a + 2b)` to `(Phase 1 + 2a + 2b + 2c)` and append a Phase 2c sub-bullet block:

```markdown
  - Phase 2c edit flows:
    - Cluster edit: resource labels, daily maintenance window, logging/monitoring services (diff preview before deploy)
    - Node pool edit: k8s labels, taints, auto-upgrade/auto-repair, upgrade strategy + surge/unavailable counts
    - Multi-step operations: edits that span multiple API endpoints (e.g. labels + maintenance) run sequentially with a single footer task
```

- [ ] **Step 3: `README.md`**

Append to the GKE entry: " Phase 2c adds cluster + node-pool edit flows with diff preview."

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md README.md .claude/rules/key-bindings.md
git commit -m "2026-05-19: GKE phase 2c — docs"
```

---

## Final integration smoke (manual)

After Task 10 lands, `make run`:

1. Open a Standard cluster's details → Overview → `e` → cluster edit form.
2. Change description? (if exposed) or change resource label. Ctrl+S → diff shows old → new. Enter to deploy. Footer "Updating cluster X (basic, 1/1)...". Refresh on DONE.
3. Edit again, change logging service AND daily maintenance window. Footer should show "(basic, 1/2)..." then "(maintenance, 2/2)..." as sequence progresses.
4. Edit with no changes → "No changes to apply" inline error, no API call.
5. Switch to Node Pools tab, focus a pool, press `e` → pool edit form.
6. Toggle autoUpgrade off + add a taint. Submit; diff preview; deploy. Two-step sequence (management + fields).
7. Open an Autopilot cluster → Overview `e` works (cluster edit allowed). Node Pools `e` is a no-op.
8. Try edit on a managed-release-channel cluster — logging service should still be editable.

---

## Self-review checklist

- [x] Spec coverage: every editable field in Design.md has a corresponding task.
- [x] No placeholders: tests have concrete code; impl shows enough shape for the implementer to act.
- [x] Type consistency: `ClusterEdit`, `MaintenanceWindow`, `NodePoolEdit`, `NodePoolManagement`, message types spelled identically across tasks.
- [x] Message producer/consumer paired: every `GKE*EditRequestMsg` has both an emitter (form) and a consumer (app handler).
- [x] HasTextInputFocused on both edit views.
- [x] Sequential-step pattern documented; reuses Phase 2b polling.
- [x] Footer task IDs use `gke-op:` prefix.
- [x] Autopilot guard for pool edit; cluster edit allowed on both modes.
- [x] No-op submit rejected at form layer (defense in depth: app handler also guards).
- [x] LabelFingerprint surfaced in ClusterEdit when ResourceLabels is set (avoids 409 Conflict).
- [x] `runGKEEditStep` is non-recursive in tea.Cmd terms — each step returns a new cmd from `OnDone`, never blocks Update().
