# GKE Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the first slice of Kubernetes Engine support in gcon: a clusters list view, a cluster details view with Overview and Node Pools tabs, and a type-to-confirm delete flow. Both Autopilot and Standard clusters, both zonal and regional locations.

**Architecture:** Mirrors Load Balancers Phase 1. New `internal/gcp/gke.go` wraps `google.golang.org/api/container/v1`. New `gke_clusters.go` (list, embeds `TableClickDelegate`) and `gke_cluster_details.go` (details, two tabs wrapped in `bubbles/viewport.Model`) live in `internal/ui/views/`. New top-level sidebar group "Kubernetes Engine". Delete is fire-and-forget with type-to-confirm and a footer task; no Operation polling.

**Tech Stack:** Go 1.26+, Bubble Tea, Lip Gloss, `google.golang.org/api/container/v1` (new direct import; already a transitive dep).

**Reference docs:**
- Spec: `doc/2026-05-15-gke-phase1/Design.md`
- Closest analog: LB Phase 1 commits `01fb98a` and the `loadbalancers.go` / `loadbalancers.go` (list) / `loadbalancer_details.go` / `loadbalancer_cascade.go` files
- Required project rules: `.claude/rules/adding-new-views.md`, `.claude/rules/component-patterns.md`, `.claude/rules/bubble-tea-rendering.md`, `.claude/rules/async-operations-error-handling.md`, `.claude/rules/forms-framework.md` (delete dialog), `.claude/rules/gcp-api-gotchas.md`

**Pattern reminders worth re-reading before starting:**
- View client is created on-demand by the view itself (see `LoadBalancersView.initClient`). The aggregator `gcp.Client` does not hold a `ContainerClient`.
- All views with text input must implement `HasTextInputFocused()`. The delete dialog has a text input, so the cluster details view must implement it.
- `Init()` must reset `loading=true` and `err=nil` (per adding-new-views step 11).
- App handlers for action results must call `view.SetError()` on failure (per async-operations rule).
- Tab-body content is wrapped in `bubbles/viewport.Model` even if Phase 1 tabs fit on screen — Phase 2 will add Observability and need it.

---

## File Structure

**New files:**
- `internal/gcp/gke.go` — types + convert helpers + client methods
- `internal/gcp/gke_test.go` — type conversion + helper tests
- `internal/ui/views/gke_messages.go` — internal + cross-view messages
- `internal/ui/views/gke_clusters.go` — clusters list view
- `internal/ui/views/gke_clusters_test.go` — list view tests
- `internal/ui/views/gke_cluster_details.go` — cluster details view (Overview + Node Pools)
- `internal/ui/views/gke_cluster_details_test.go` — details view tests

**Modified files:**
- `go.mod` / `go.sum` — add `google.golang.org/api/container/v1` direct import
- `internal/ui/app.go` — ViewType enum, App struct fields, getCurrentViewModel, updateViewSizes, Update handlers
- `internal/ui/app_render.go` — `renderCurrentView` switch arm, header breadcrumbs
- `internal/ui/app_navigation.go` — navigation handlers, clearAllViews, sidebar guards, updateSidebarActiveView, network/subnet client resolution chain
- `internal/ui/components/sidebar/menu.go` — new top-level "Kubernetes Engine" entry + child "Clusters"
- `internal/ui/components/sidebar/sidebar.go` (or wherever `ViewType` is defined) — `sidebar.ViewGKEClusters` constant
- `internal/ui/components/commandpalette/commands.go` — `ViewGKEClusters` view type + icon + nav command
- `CLAUDE.md` — implemented-features list entry
- `README.md` — feature mention
- `.claude/rules/key-bindings.md` — new section for GKE list + details

---

## Task 1: Add `container/v1` dependency and create skeleton types

**Files:**
- Modify: `go.mod`
- Create: `internal/gcp/gke.go`

- [ ] **Step 1: Add the direct dependency**

Run: `go get google.golang.org/api/container/v1@latest`
Expected: dep added; `go.sum` updated.

- [ ] **Step 2: Create the skeleton with types only (no functions yet)**

```go
// internal/gcp/gke.go
package gcp

import (
	"time"

	"google.golang.org/api/container/v1"
)

// Cluster is the list-view projection of a GKE cluster.
type Cluster struct {
	Name           string
	Location       string // "us-central1-a" or "us-central1"
	LocationType   string // "zone" | "region"
	Mode           string // "AUTOPILOT" | "STANDARD"
	Status         string // PROVISIONING / RUNNING / RECONCILING / STOPPING / ERROR / DEGRADED
	MasterVersion  string
	NodeVersion    string // "(varies)" when non-uniform across pools
	NodeCount      int    // sum across node pools
	Network        string
	Subnetwork     string
	ReleaseChannel string // RAPID / REGULAR / STABLE / "" (unspecified)
	Endpoint       string
	PrivateCluster bool
	CreatedAt      time.Time
}

// ClusterDetails is the full projection used by the details view.
type ClusterDetails struct {
	Cluster
	NodePools                []NodePool
	Addons                   AddonsSummary
	ClusterIPv4CIDR          string
	ServicesIPv4CIDR         string
	WorkloadIdentityPool     string // "" when disabled
	MasterAuthorizedNetworks []string
	DatabaseEncryption       string // "ENCRYPTED (key: name)" | "DECRYPTED"
}

// AddonsSummary captures the four addons surfaced in Phase 1.
type AddonsSummary struct {
	HTTPLoadBalancing bool
	NetworkPolicy     bool
	PersistentDiskCSI bool
	DNSCache          bool
}

// NodePool is the per-pool projection used by the Node Pools tab.
type NodePool struct {
	Name           string
	MachineType    string
	DiskSizeGB     int64
	DiskType       string
	NodeCount      int
	AutoscalingMin int
	AutoscalingMax int
	AutoscalingOn  bool
	NodeVersion    string
	Status         string
	AutoUpgrade    bool
	AutoRepair     bool
	Locations      []string // zones the pool spans
}

// ContainerClient wraps the GKE container API.
type ContainerClient struct {
	service *container.Service
}
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/gcp/gke.go
git commit -m "2026-05-15: GKE phase 1 — types skeleton + container/v1 dep"
```

---

## Task 2: `locationType` helper + tests (TDD)

**Files:**
- Modify: `internal/gcp/gke.go`
- Create: `internal/gcp/gke_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/gcp/gke_test.go
package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocationType(t *testing.T) {
	cases := map[string]string{
		"us-central1-a": "zone",
		"us-central1-b": "zone",
		"europe-west2-c": "zone",
		"us-central1":   "region",
		"europe-west2":  "region",
		"":              "region", // empty defaults to region; harmless since no API call hits this path
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, locationType(in))
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gcp -run TestLocationType -v`
Expected: FAIL — `undefined: locationType`.

- [ ] **Step 3: Implement `locationType`**

Add to `internal/gcp/gke.go`:

```go
import "strings"

// locationType returns "zone" for fully-qualified zones (us-central1-a) and
// "region" otherwise. A GKE location with two or more "-" segments is a
// zone; with one or zero segments it is a region.
func locationType(location string) string {
	if strings.Count(location, "-") >= 2 {
		return "zone"
	}
	return "region"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gcp -run TestLocationType -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gcp/gke.go internal/gcp/gke_test.go
git commit -m "2026-05-15: GKE phase 1 — locationType helper"
```

---

## Task 3: `convertCluster` + tests (TDD)

**Files:**
- Modify: `internal/gcp/gke.go`, `internal/gcp/gke_test.go`

- [ ] **Step 1: Write the failing test (autopilot + regional, plus standard + zonal)**

Add to `internal/gcp/gke_test.go`:

```go
import (
	"time"

	"google.golang.org/api/container/v1"
)

func TestConvertCluster_AutopilotRegional(t *testing.T) {
	raw := &container.Cluster{
		Name:               "prod",
		Location:           "us-central1",
		Status:             "RUNNING",
		CurrentMasterVersion: "1.30.5-gke.1014001",
		CurrentNodeVersion:   "1.30.5-gke.1014001",
		CurrentNodeCount:     12,
		Network:            "default",
		Subnetwork:         "default-uscent1",
		Endpoint:           "34.123.45.67",
		CreateTime:         "2025-08-12T14:03:00Z",
		Autopilot:          &container.Autopilot{Enabled: true},
		ReleaseChannel:     &container.ReleaseChannel{Channel: "REGULAR"},
		PrivateClusterConfig: &container.PrivateClusterConfig{EnablePrivateNodes: true},
	}
	got := convertCluster(raw)

	assert.Equal(t, "prod", got.Name)
	assert.Equal(t, "us-central1", got.Location)
	assert.Equal(t, "region", got.LocationType)
	assert.Equal(t, "AUTOPILOT", got.Mode)
	assert.Equal(t, "RUNNING", got.Status)
	assert.Equal(t, "1.30.5-gke.1014001", got.MasterVersion)
	assert.Equal(t, "1.30.5-gke.1014001", got.NodeVersion)
	assert.Equal(t, 12, got.NodeCount)
	assert.Equal(t, "default", got.Network)
	assert.Equal(t, "default-uscent1", got.Subnetwork)
	assert.Equal(t, "REGULAR", got.ReleaseChannel)
	assert.Equal(t, "34.123.45.67", got.Endpoint)
	assert.True(t, got.PrivateCluster)
	assert.Equal(t, 2025, got.CreatedAt.Year())
}

func TestConvertCluster_StandardZonal_MinimalFields(t *testing.T) {
	raw := &container.Cluster{
		Name:     "dev",
		Location: "us-central1-a",
		Status:   "RUNNING",
		// No Autopilot block → STANDARD
		// No ReleaseChannel block → empty
		// No PrivateClusterConfig → false
	}
	got := convertCluster(raw)

	assert.Equal(t, "STANDARD", got.Mode)
	assert.Equal(t, "zone", got.LocationType)
	assert.Equal(t, "", got.ReleaseChannel)
	assert.False(t, got.PrivateCluster)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gcp -run TestConvertCluster -v`
Expected: FAIL — `undefined: convertCluster`.

- [ ] **Step 3: Implement `convertCluster`**

Add to `internal/gcp/gke.go`:

```go
// convertCluster maps the raw container.Cluster into our list-view Cluster.
// Mode is derived from the Autopilot block, never from the raw API in the UI.
func convertCluster(c *container.Cluster) Cluster {
	mode := "STANDARD"
	if c.Autopilot != nil && c.Autopilot.Enabled {
		mode = "AUTOPILOT"
	}
	releaseChannel := ""
	if c.ReleaseChannel != nil {
		releaseChannel = c.ReleaseChannel.Channel
	}
	private := false
	if c.PrivateClusterConfig != nil {
		private = c.PrivateClusterConfig.EnablePrivateNodes
	}
	created, _ := time.Parse(time.RFC3339, c.CreateTime) //nolint:errcheck // zero-value on parse failure is acceptable
	return Cluster{
		Name:           c.Name,
		Location:       c.Location,
		LocationType:   locationType(c.Location),
		Mode:           mode,
		Status:         c.Status,
		MasterVersion:  c.CurrentMasterVersion,
		NodeVersion:    c.CurrentNodeVersion,
		NodeCount:      int(c.CurrentNodeCount),
		Network:        c.Network,
		Subnetwork:     c.Subnetwork,
		ReleaseChannel: releaseChannel,
		Endpoint:       c.Endpoint,
		PrivateCluster: private,
		CreatedAt:      created,
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gcp -run TestConvertCluster -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gcp/gke.go internal/gcp/gke_test.go
git commit -m "2026-05-15: GKE phase 1 — convertCluster + tests"
```

---

## Task 4: `convertNodePool` + tests (TDD)

**Files:**
- Modify: `internal/gcp/gke.go`, `internal/gcp/gke_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestConvertNodePool_AutoscalingOn(t *testing.T) {
	raw := &container.NodePool{
		Name:           "default",
		InitialNodeCount: 3,
		Locations:      []string{"us-central1-a", "us-central1-b"},
		Version:        "1.30.5-gke.1014001",
		Status:         "RUNNING",
		Config: &container.NodeConfig{
			MachineType: "e2-medium",
			DiskSizeGb:  100,
			DiskType:    "pd-balanced",
		},
		Autoscaling: &container.NodePoolAutoscaling{
			Enabled:      true,
			MinNodeCount: 1,
			MaxNodeCount: 10,
		},
		Management: &container.NodeManagement{AutoUpgrade: true, AutoRepair: true},
	}
	got := convertNodePool(raw)

	assert.Equal(t, "default", got.Name)
	assert.Equal(t, "e2-medium", got.MachineType)
	assert.Equal(t, int64(100), got.DiskSizeGB)
	assert.Equal(t, "pd-balanced", got.DiskType)
	assert.Equal(t, 3, got.NodeCount)
	assert.True(t, got.AutoscalingOn)
	assert.Equal(t, 1, got.AutoscalingMin)
	assert.Equal(t, 10, got.AutoscalingMax)
	assert.Equal(t, "1.30.5-gke.1014001", got.NodeVersion)
	assert.Equal(t, "RUNNING", got.Status)
	assert.True(t, got.AutoUpgrade)
	assert.True(t, got.AutoRepair)
	assert.Equal(t, []string{"us-central1-a", "us-central1-b"}, got.Locations)
}

func TestConvertNodePool_AutoscalingOff_NoManagement(t *testing.T) {
	raw := &container.NodePool{
		Name:             "gpu-pool",
		InitialNodeCount: 2,
		Status:           "RUNNING",
		Config:           &container.NodeConfig{MachineType: "g2-standard-4"},
		// No Autoscaling, no Management
	}
	got := convertNodePool(raw)

	assert.False(t, got.AutoscalingOn)
	assert.Equal(t, 0, got.AutoscalingMin)
	assert.Equal(t, 0, got.AutoscalingMax)
	assert.False(t, got.AutoUpgrade)
	assert.False(t, got.AutoRepair)
}
```

- [ ] **Step 2: Run tests, expect FAIL (`undefined: convertNodePool`)**

Run: `go test ./internal/gcp -run TestConvertNodePool -v`

- [ ] **Step 3: Implement `convertNodePool`**

```go
func convertNodePool(p *container.NodePool) NodePool {
	out := NodePool{
		Name:       p.Name,
		NodeCount:  int(p.InitialNodeCount),
		NodeVersion: p.Version,
		Status:     p.Status,
		Locations:  p.Locations,
	}
	if p.Config != nil {
		out.MachineType = p.Config.MachineType
		out.DiskSizeGB = p.Config.DiskSizeGb
		out.DiskType = p.Config.DiskType
	}
	if p.Autoscaling != nil && p.Autoscaling.Enabled {
		out.AutoscalingOn = true
		out.AutoscalingMin = int(p.Autoscaling.MinNodeCount)
		out.AutoscalingMax = int(p.Autoscaling.MaxNodeCount)
	}
	if p.Management != nil {
		out.AutoUpgrade = p.Management.AutoUpgrade
		out.AutoRepair = p.Management.AutoRepair
	}
	return out
}
```

- [ ] **Step 4: Run tests, expect PASS**

Run: `go test ./internal/gcp -run TestConvertNodePool -v`

- [ ] **Step 5: Commit**

```bash
git add internal/gcp/gke.go internal/gcp/gke_test.go
git commit -m "2026-05-15: GKE phase 1 — convertNodePool + tests"
```

---

## Task 5: `NewContainerClient` + `ListClusters`

**Files:** Modify `internal/gcp/gke.go`.

These methods call the GCP API and are not unit-tested in this codebase (mirrors LB Phase 1 — no mocks for network methods).

- [ ] **Step 1: Implement constructor + ListClusters**

Add to `internal/gcp/gke.go`:

```go
import (
	"context"
	"fmt"
	"sort"

	"google.golang.org/api/option"
)

// NewContainerClient creates a GKE container API client using ADC.
func NewContainerClient(ctx context.Context) (*ContainerClient, error) {
	svc, err := container.NewService(ctx, option.WithScopes(container.CloudPlatformScope))
	if err != nil {
		return nil, fmt.Errorf("create container client: %w", err)
	}
	return &ContainerClient{service: svc}, nil
}

// ListClusters returns all clusters across all locations for the project.
// Location "-" is the GKE wildcard for "any location".
func (c *ContainerClient) ListClusters(ctx context.Context, projectID string) ([]Cluster, error) {
	parent := fmt.Sprintf("projects/%s/locations/-", projectID)
	resp, err := c.service.Projects.Locations.Clusters.List(parent).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}
	out := make([]Cluster, 0, len(resp.Clusters))
	for _, raw := range resp.Clusters {
		out = append(out, convertCluster(raw))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/gcp/gke.go
git commit -m "2026-05-15: GKE phase 1 — NewContainerClient + ListClusters"
```

---

## Task 6: `GetCluster` (with addon + auth-network + encryption mapping)

**Files:** Modify `internal/gcp/gke.go`, `internal/gcp/gke_test.go`.

- [ ] **Step 1: Implement `GetCluster`**

```go
// GetCluster fetches a single cluster and projects it to ClusterDetails.
// Node pools come from the response's NodePools field — no separate list call.
func (c *ContainerClient) GetCluster(ctx context.Context, projectID, location, name string) (*ClusterDetails, error) {
	fqn := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, name)
	raw, err := c.service.Projects.Locations.Clusters.Get(fqn).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get cluster %s/%s: %w", location, name, err)
	}
	out := &ClusterDetails{
		Cluster:                  convertCluster(raw),
		ClusterIPv4CIDR:          raw.ClusterIpv4Cidr,
		Addons:                   convertAddons(raw.AddonsConfig),
		WorkloadIdentityPool:     workloadIdentityPool(raw),
		MasterAuthorizedNetworks: authorizedNetworks(raw),
		DatabaseEncryption:       databaseEncryption(raw),
	}
	if raw.IpAllocationPolicy != nil {
		out.ServicesIPv4CIDR = raw.IpAllocationPolicy.ServicesIpv4CidrBlock
	}
	for _, np := range raw.NodePools {
		out.NodePools = append(out.NodePools, convertNodePool(np))
	}
	// Reconcile NodeVersion to "(varies)" when pools disagree.
	if !uniformNodeVersion(out.NodePools) {
		out.NodeVersion = "(varies)"
	}
	return out, nil
}

func convertAddons(a *container.AddonsConfig) AddonsSummary {
	if a == nil {
		return AddonsSummary{}
	}
	return AddonsSummary{
		HTTPLoadBalancing: a.HttpLoadBalancing != nil && !a.HttpLoadBalancing.Disabled,
		NetworkPolicy:     a.NetworkPolicyConfig != nil && !a.NetworkPolicyConfig.Disabled,
		PersistentDiskCSI: a.GcePersistentDiskCsiDriverConfig != nil && a.GcePersistentDiskCsiDriverConfig.Enabled,
		DNSCache:          a.DnsCacheConfig != nil && a.DnsCacheConfig.Enabled,
	}
}

func workloadIdentityPool(c *container.Cluster) string {
	if c.WorkloadIdentityConfig == nil {
		return ""
	}
	return c.WorkloadIdentityConfig.WorkloadPool
}

func authorizedNetworks(c *container.Cluster) []string {
	if c.MasterAuthorizedNetworksConfig == nil || !c.MasterAuthorizedNetworksConfig.Enabled {
		return nil
	}
	out := make([]string, 0, len(c.MasterAuthorizedNetworksConfig.CidrBlocks))
	for _, b := range c.MasterAuthorizedNetworksConfig.CidrBlocks {
		out = append(out, b.CidrBlock)
	}
	return out
}

func databaseEncryption(c *container.Cluster) string {
	if c.DatabaseEncryption == nil || c.DatabaseEncryption.State != "ENCRYPTED" {
		return "DECRYPTED"
	}
	if c.DatabaseEncryption.KeyName == "" {
		return "ENCRYPTED"
	}
	return fmt.Sprintf("ENCRYPTED (key: %s)", shortName(c.DatabaseEncryption.KeyName))
}

func uniformNodeVersion(pools []NodePool) bool {
	if len(pools) <= 1 {
		return true
	}
	v := pools[0].NodeVersion
	for _, p := range pools[1:] {
		if p.NodeVersion != v {
			return false
		}
	}
	return true
}
```

`shortName` already exists in `internal/gcp/` (used by LB code) — re-use it.

- [ ] **Step 2: Add a small test for `databaseEncryption` and `uniformNodeVersion`**

```go
func TestDatabaseEncryption(t *testing.T) {
	cases := []struct {
		name string
		in   *container.Cluster
		want string
	}{
		{"nil", &container.Cluster{}, "DECRYPTED"},
		{"decrypted-state", &container.Cluster{DatabaseEncryption: &container.DatabaseEncryption{State: "DECRYPTED"}}, "DECRYPTED"},
		{"encrypted-no-key", &container.Cluster{DatabaseEncryption: &container.DatabaseEncryption{State: "ENCRYPTED"}}, "ENCRYPTED"},
		{"encrypted-with-key", &container.Cluster{DatabaseEncryption: &container.DatabaseEncryption{State: "ENCRYPTED", KeyName: "projects/p/locations/global/keyRings/r/cryptoKeys/my-key"}}, "ENCRYPTED (key: my-key)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, databaseEncryption(tc.in))
		})
	}
}

func TestUniformNodeVersion(t *testing.T) {
	assert.True(t, uniformNodeVersion(nil))
	assert.True(t, uniformNodeVersion([]NodePool{{NodeVersion: "1.30"}}))
	assert.True(t, uniformNodeVersion([]NodePool{{NodeVersion: "1.30"}, {NodeVersion: "1.30"}}))
	assert.False(t, uniformNodeVersion([]NodePool{{NodeVersion: "1.30"}, {NodeVersion: "1.29"}}))
}
```

- [ ] **Step 3: Run tests, expect PASS**

Run: `go test ./internal/gcp -run "TestDatabaseEncryption|TestUniformNodeVersion" -v`

- [ ] **Step 4: Commit**

```bash
git add internal/gcp/gke.go internal/gcp/gke_test.go
git commit -m "2026-05-15: GKE phase 1 — GetCluster with addons/encryption/auth networks"
```

---

## Task 7: `ListNodePools` + `DeleteCluster`

**Files:** Modify `internal/gcp/gke.go`.

`ListNodePools` is not used by Phase 1 view code (initial render uses `GetCluster`'s inline pools) but ships for Phase 2.

- [ ] **Step 1: Implement both methods**

```go
// ListNodePools returns the node pools for a single cluster. Phase 2 uses
// this for per-tab refresh; Phase 1 reads pools from GetCluster instead.
func (c *ContainerClient) ListNodePools(ctx context.Context, projectID, location, clusterName string) ([]NodePool, error) {
	parent := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, clusterName)
	resp, err := c.service.Projects.Locations.Clusters.NodePools.List(parent).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list node pools %s/%s: %w", location, clusterName, err)
	}
	out := make([]NodePool, 0, len(resp.NodePools))
	for _, raw := range resp.NodePools {
		out = append(out, convertNodePool(raw))
	}
	return out, nil
}

// DeleteCluster kicks off a cluster delete. Returns when the API accepts
// the request; the resulting Operation runs server-side and is not polled.
func (c *ContainerClient) DeleteCluster(ctx context.Context, projectID, location, name string) error {
	fqn := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, name)
	if _, err := c.service.Projects.Locations.Clusters.Delete(fqn).Context(ctx).Do(); err != nil {
		return fmt.Errorf("delete cluster %s/%s: %w", location, name, err)
	}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/gcp/gke.go
git commit -m "2026-05-15: GKE phase 1 — ListNodePools + DeleteCluster"
```

---

## Task 8: `gke_messages.go`

**Files:** Create `internal/ui/views/gke_messages.go`.

- [ ] **Step 1: Write the file**

```go
// internal/ui/views/gke_messages.go
package views

import "github.com/slayer/gcon/internal/gcp"

// GKEClusterSelectedMsg is emitted when a row is activated on the
// clusters list. The app handler navigates to the details view.
type GKEClusterSelectedMsg struct {
	ProjectID string
	Location  string
	Name      string
}

// GKEClusterDeleteRequestMsg is emitted after the user confirms the
// delete dialog. The app handler runs the API call.
type GKEClusterDeleteRequestMsg struct {
	ProjectID string
	Location  string
	Name      string
}

// GKEClusterActionResultMsg carries the outcome of a cluster action
// back to the view. Action is currently only "delete".
type GKEClusterActionResultMsg struct {
	Action string
	Error  error
}

// gkeClustersClientReadyMsg / gkeClustersErrorMsg / gkeClustersLoadedMsg
// are the internal load lifecycle for the list view.
type gkeClustersClientReadyMsg struct{ client *gcp.ContainerClient }
type gkeClustersErrorMsg struct{ err error }
type gkeClustersLoadedMsg struct{ clusters []gcp.Cluster }

// gkeClusterLoadedMsg / gkeClusterErrorMsg / gkeClusterClientReadyMsg are
// the internal load lifecycle for the details view.
type gkeClusterClientReadyMsg struct{ client *gcp.ContainerClient }
type gkeClusterLoadedMsg struct{ details *gcp.ClusterDetails }
type gkeClusterErrorMsg struct{ err error }
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/gke_messages.go
git commit -m "2026-05-15: GKE phase 1 — view message types"
```

---

## Task 9: `gke_clusters.go` list view skeleton (init + load)

**Files:** Create `internal/ui/views/gke_clusters.go`.

Pattern: copy the shape of `loadbalancers.go`. The list view embeds `TableClickDelegate`, owns a lazy `*gcp.ContainerClient`, loads clusters async, and renders a table.

- [ ] **Step 1: Write the skeleton (struct, constructor, Init, initClient, load, GetContainerClient, SetSize, SetContext, HasTextInputFocused)**

```go
// internal/ui/views/gke_clusters.go
package views

import (
	gocontext "context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
)

type GKEClustersView struct {
	TableClickDelegate

	projectID string
	client    *gcp.ContainerClient
	clusters  []gcp.Cluster
	table     table.Model
	spinner   spinner.Model
	loading   bool
	err       error
	width     int
	height    int
}

func NewGKEClustersView(projectID string, client *gcp.ContainerClient) *GKEClustersView {
	columns := []table.Column{
		{Title: "Name", Width: 28},
		{Title: "Location", Width: 18},
		{Title: "Type", Width: 8},
		{Title: "Mode", Width: 10},
		{Title: "Master version", Width: 22},
		{Title: "Nodes", Width: 8},
		{Title: "Status", Width: 14},
	}
	t := table.New(columns, "Kubernetes Engine › Clusters")
	v := &GKEClustersView{
		projectID: projectID,
		client:    client,
		table:     t,
		spinner:   components.NewGCPSpinner(),
	}
	v.Table = &v.table
	return v
}

func (v *GKEClustersView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	if v.client == nil {
		return tea.Batch(v.spinner.Tick, v.initClient())
	}
	return tea.Batch(v.spinner.Tick, v.load())
}

func (v *GKEClustersView) initClient() tea.Cmd {
	return func() tea.Msg {
		c, err := gcp.NewContainerClient(gocontext.Background())
		if err != nil {
			return gkeClustersErrorMsg{err: err}
		}
		return gkeClustersClientReadyMsg{client: c}
	}
}

func (v *GKEClustersView) load() tea.Cmd {
	return func() tea.Msg {
		if v.client == nil {
			return gkeClustersErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		out, err := v.client.ListClusters(gocontext.Background(), v.projectID)
		if err != nil {
			return gkeClustersErrorMsg{err: err}
		}
		return gkeClustersLoadedMsg{clusters: out}
	}
}

func (v *GKEClustersView) GetContainerClient() *gcp.ContainerClient { return v.client }

func (v *GKEClustersView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.table.SetSize(width, height-4)
}

func (v *GKEClustersView) SetContext(_ *context.ProgramContext) {}

func (v *GKEClustersView) HasTextInputFocused() bool { return v.table.HasTextInputFocused() }
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./internal/ui/...`
Expected: PASS (or fail with missing methods; if so, look at `loadbalancers.go` for the exact signatures of `table.Model.HasTextInputFocused` etc. — adjust to match).

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/gke_clusters.go
git commit -m "2026-05-15: GKE phase 1 — clusters list view skeleton"
```

---

## Task 10: List view Update + View + table rendering + test

**Files:** Modify `internal/ui/views/gke_clusters.go`; create `internal/ui/views/gke_clusters_test.go`.

- [ ] **Step 1: Write the failing test (mode column, autopilot row rendering, filter)**

```go
// internal/ui/views/gke_clusters_test.go
package views

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func gkeListWithFixtures() *GKEClustersView {
	v := NewGKEClustersView("proj", nil)
	v.SetSize(160, 40)
	v.clusters = []gcp.Cluster{
		{Name: "prod", Location: "us-central1", LocationType: "region", Mode: "STANDARD", MasterVersion: "1.30.5", NodeCount: 12, Status: "RUNNING", CreatedAt: time.Now()},
		{Name: "stage", Location: "us-central1-a", LocationType: "zone", Mode: "AUTOPILOT", MasterVersion: "1.30.5", NodeCount: 0, Status: "RUNNING", CreatedAt: time.Now()},
	}
	v.refreshTable()
	return v
}

func TestGKEClustersView_RendersRows(t *testing.T) {
	v := gkeListWithFixtures()
	out := v.View()
	assert.Contains(t, out, "prod")
	assert.Contains(t, out, "stage")
	assert.Contains(t, out, "Standard")
	assert.Contains(t, out, "Autopilot")
	// Autopilot shows "(managed)" rather than 0 in the Nodes column.
	assert.Contains(t, out, "(managed)")
}

func TestGKEClustersView_AutopilotFilter(t *testing.T) {
	v := gkeListWithFixtures()
	v.table.SetFilter("mode:autopilot")
	out := v.View()
	assert.Contains(t, out, "stage")
	assert.NotContains(t, out, "prod")
}
```

- [ ] **Step 2: Run, expect FAIL (no `refreshTable`, no `Update`/`View`)**

Run: `go test ./internal/ui/views -run TestGKEClustersView -v`

- [ ] **Step 3: Implement `Update`, `View`, `refreshTable` in `gke_clusters.go`**

```go
import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/ui"
	"github.com/slayer/gcon/internal/ui/keys"
	"github.com/slayer/gcon/internal/ui/symbols"
)

func (v *GKEClustersView) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gkeClustersClientReadyMsg:
		v.client = m.client
		return v.load()
	case gkeClustersLoadedMsg:
		v.loading = false
		v.clusters = m.clusters
		v.refreshTable()
		return nil
	case gkeClustersErrorMsg:
		v.loading = false
		v.err = m.err
		return nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		v.spinner, cmd = v.spinner.Update(m)
		return cmd
	case tea.KeyMsg:
		return v.handleKey(m)
	}
	return nil
}

func (v *GKEClustersView) handleKey(m tea.KeyMsg) tea.Cmd {
	if v.loading {
		if key.Matches(m, keys.Global.Cancel) {
			return func() tea.Msg { return ui.NavigateBackMsg{} }
		}
		return nil
	}
	switch m.String() {
	case "enter":
		c := v.cursorCluster()
		if c == nil {
			return nil
		}
		return func() tea.Msg {
			return GKEClusterSelectedMsg{ProjectID: v.projectID, Location: c.Location, Name: c.Name}
		}
	case "D":
		c := v.cursorCluster()
		if c == nil {
			return nil
		}
		// Wire the delete dialog in Task 14; for now route to the details
		// view's delete entry. (Or stub: ignored until Task 14 adds the
		// dialog here.)
		return func() tea.Msg {
			return GKEClusterSelectedMsg{ProjectID: v.projectID, Location: c.Location, Name: c.Name}
		}
	case "r":
		v.loading = true
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.load())
	}
	// Defer table-internal keys (sort menu, filter, j/k, mouse) to the table.
	_, cmd := v.table.Update(m)
	return cmd
}

func (v *GKEClustersView) View() string {
	if v.loading && len(v.clusters) == 0 {
		return renderLoading(v.spinner, "Loading clusters...")
	}
	if v.err != nil && len(v.clusters) == 0 {
		return components.RenderError(v.err)
	}
	return v.table.View()
}

func (v *GKEClustersView) refreshTable() {
	rows := make([]table.Row, 0, len(v.clusters))
	for _, c := range v.clusters {
		nodes := fmt.Sprintf("%d", c.NodeCount)
		if c.Mode == "AUTOPILOT" {
			nodes = "(managed)"
		}
		rows = append(rows, table.Row{
			c.Name,
			c.Location,
			locationBadge(c.LocationType),
			modeBadge(c.Mode),
			c.MasterVersion,
			nodes,
			statusBadge(c.Status),
			// Store the cluster key for filter-by-field lookups
			// (filter "mode:autopilot" matches the rendered string).
		})
	}
	v.table.SetRows(rows)
}

func (v *GKEClustersView) cursorCluster() *gcp.Cluster {
	idx := v.table.SelectedRowIndex()
	if idx < 0 || idx >= len(v.clusters) {
		return nil
	}
	return &v.clusters[idx]
}

func locationBadge(kind string) string {
	switch kind {
	case "zone":
		return "zonal"
	case "region":
		return "regional"
	}
	return ""
}

func modeBadge(mode string) string {
	switch mode {
	case "AUTOPILOT":
		return "Autopilot"
	case "STANDARD":
		return "Standard"
	}
	return mode
}

func statusBadge(status string) string {
	// Match existing patterns from instances.go / loadbalancers.go.
	switch strings.ToUpper(status) {
	case "RUNNING", "RECONCILING":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853")).Render(symbols.Dot()) + " " + status
	case "PROVISIONING", "STOPPING":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04")).Render(symbols.Dot()) + " " + status
	case "ERROR", "DEGRADED":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Render(symbols.Dot()) + " " + status
	}
	return status
}
```

> **Note:** if `SelectedRowIndex`, `SetRows`, or `SetFilter` aren't exactly these names on your local `table.Model`, look at how `loadbalancers.go` uses the table and adapt.

- [ ] **Step 4: Run the list-view tests, expect PASS**

Run: `go test ./internal/ui/views -run TestGKEClustersView -v`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/views/gke_clusters.go internal/ui/views/gke_clusters_test.go
git commit -m "2026-05-15: GKE phase 1 — clusters list rendering + tests"
```

---

## Task 11: `gke_cluster_details.go` skeleton with viewport-wrapped tabs

**Files:** Create `internal/ui/views/gke_cluster_details.go`.

Pattern: copy the shape of `loadbalancer_details.go` Phase 2 (viewport-wrapped body, two tabs, `Init`, lazy client, etc.).

- [ ] **Step 1: Write the skeleton (struct, constructor, Init, lazy client, SetSize with viewport init, two tabs)**

```go
// internal/ui/views/gke_cluster_details.go
package views

import (
	gocontext "context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/confirmdialog"
	"github.com/slayer/gcon/internal/ui/components/links"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
)

type GKEClusterDetailsView struct {
	projectID    string
	location     string
	name         string
	client       *gcp.ContainerClient
	computeClient *gcp.ComputeClient // for cross-view nav (Network / Subnet links)
	details      *gcp.ClusterDetails
	tabs         tabs.Model
	viewport     viewport.Model
	viewportSize bool
	spinner      spinner.Model
	width, height int

	// Node Pools tab
	poolsTable table.Model

	// Overview tab links
	networkLink    *links.Component
	subnetworkLink *links.Component

	// Delete dialog
	confirmDialog *confirmdialog.Dialog
	showConfirm   bool
	deleting      bool

	err     error
	loading bool
}

func NewGKEClusterDetailsView(projectID, location, name string, container *gcp.ContainerClient, compute *gcp.ComputeClient) *GKEClusterDetailsView {
	v := &GKEClusterDetailsView{
		projectID:     projectID,
		location:      location,
		name:          name,
		client:        container,
		computeClient: compute,
		spinner:       components.NewGCPSpinner(),
		tabs: tabs.New([]tabs.Tab{
			{ID: "overview", Label: "Overview"},
			{ID: "nodepools", Label: "Node Pools"},
		}),
		networkLink:    links.New("Network"),
		subnetworkLink: links.New("Subnetwork"),
	}
	v.poolsTable = table.New([]table.Column{
		{Title: "Name", Width: 18},
		{Title: "Machine type", Width: 18},
		{Title: "Nodes", Width: 8},
		{Title: "Autoscale", Width: 14},
		{Title: "Version", Width: 22},
		{Title: "Status", Width: 14},
		{Title: "Auto-upgrade", Width: 12},
		{Title: "Auto-repair", Width: 12},
	}, "")
	return v
}

func (v *GKEClusterDetailsView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	if v.client == nil {
		return tea.Batch(v.spinner.Tick, v.initClient())
	}
	return tea.Batch(v.spinner.Tick, v.load())
}

func (v *GKEClusterDetailsView) initClient() tea.Cmd {
	return func() tea.Msg {
		c, err := gcp.NewContainerClient(gocontext.Background())
		if err != nil {
			return gkeClusterErrorMsg{err: err}
		}
		return gkeClusterClientReadyMsg{client: c}
	}
}

func (v *GKEClusterDetailsView) load() tea.Cmd {
	return func() tea.Msg {
		d, err := v.client.GetCluster(gocontext.Background(), v.projectID, v.location, v.name)
		if err != nil {
			return gkeClusterErrorMsg{err: err}
		}
		return gkeClusterLoadedMsg{details: d}
	}
}

func (v *GKEClusterDetailsView) Name() string { return v.name }

func (v *GKEClusterDetailsView) GetComputeClient() *gcp.ComputeClient { return v.computeClient }

func (v *GKEClusterDetailsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.poolsTable.SetSize(width-4, height-8)
	if !v.viewportSize {
		v.viewport = viewport.New(width-4, height-4)
		v.viewportSize = true
	} else {
		v.viewport.Width = width - 4
		v.viewport.Height = height - 4
	}
}

func (v *GKEClusterDetailsView) SetContext(_ *context.ProgramContext) {}

func (v *GKEClusterDetailsView) HasTextInputFocused() bool {
	if v.showConfirm && v.confirmDialog != nil {
		return v.confirmDialog.HasTextInputFocused()
	}
	return false
}

func (v *GKEClusterDetailsView) SetError(err error) {
	v.deleting = false
	v.err = err
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/ui/...`

If a referenced symbol differs locally (e.g., `links.New` signature), copy the exact pattern from `firewall_details.go` or `subnet_details.go`.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/gke_cluster_details.go
git commit -m "2026-05-15: GKE phase 1 — cluster details view skeleton (viewport, tabs)"
```

---

## Task 12: Overview tab render + tests

**Files:** Modify `internal/ui/views/gke_cluster_details.go`; create `internal/ui/views/gke_cluster_details_test.go`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/ui/views/gke_cluster_details_test.go
package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func gkeDetailsFixture(mode string) *GKEClusterDetailsView {
	v := NewGKEClusterDetailsView("proj", "us-central1", "prod", nil, nil)
	v.SetSize(160, 40)
	v.loading = false
	v.details = &gcp.ClusterDetails{
		Cluster: gcp.Cluster{
			Name:          "prod",
			Location:      "us-central1",
			LocationType:  "region",
			Mode:          mode,
			Status:        "RUNNING",
			MasterVersion: "1.30.5-gke.1014001",
			NodeVersion:   "1.30.5-gke.1014001",
			Network:       "default",
			Subnetwork:    "default-uscent1",
			ReleaseChannel: "REGULAR",
			Endpoint:       "34.123.45.67",
			PrivateCluster: false,
		},
		ClusterIPv4CIDR: "10.4.0.0/14",
		ServicesIPv4CIDR: "10.8.0.0/20",
		Addons: gcp.AddonsSummary{
			HTTPLoadBalancing: true,
			PersistentDiskCSI: true,
		},
		WorkloadIdentityPool: "prod.svc.id.goog",
		DatabaseEncryption:   "ENCRYPTED (key: my-key)",
		NodePools: []gcp.NodePool{
			{Name: "default", MachineType: "e2-medium", NodeCount: 3, AutoscalingOn: true, AutoscalingMin: 1, AutoscalingMax: 10, NodeVersion: "1.30.5-gke.1014001", Status: "RUNNING", AutoUpgrade: true, AutoRepair: true},
		},
	}
	return v
}

func TestGKEClusterDetails_OverviewRendersStandard(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.tabs.SetActiveByID("overview")
	out := v.View()
	assert.Contains(t, out, "Standard")
	assert.Contains(t, out, "us-central1 (regional)")
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "default-uscent1")
	assert.Contains(t, out, "REGULAR")
	assert.Contains(t, out, "prod.svc.id.goog")
	assert.Contains(t, out, "ENCRYPTED (key: my-key)")
	assert.Contains(t, out, "HTTP load balancing: Enabled")
	assert.Contains(t, out, "Network policy: Disabled")
}

func TestGKEClusterDetails_OverviewRendersAutopilot(t *testing.T) {
	v := gkeDetailsFixture("AUTOPILOT")
	v.tabs.SetActiveByID("overview")
	out := v.View()
	assert.Contains(t, out, "Autopilot")
}
```

- [ ] **Step 2: Run, expect FAIL (no `View` body yet)**

Run: `go test ./internal/ui/views -run TestGKEClusterDetails_Overview -v`

- [ ] **Step 3: Implement `View`, `renderOverview`, `Update` (load handlers)**

Add to `gke_cluster_details.go`:

```go
func (v *GKEClusterDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gkeClusterClientReadyMsg:
		v.client = m.client
		return v.load()
	case gkeClusterLoadedMsg:
		v.loading = false
		v.details = m.details
		// Wire link components
		v.networkLink.SetValue(v.details.Network)
		v.subnetworkLink.SetValue(v.details.Subnetwork)
		// Populate node pools table
		v.refreshNodePoolsTable()
		return nil
	case gkeClusterErrorMsg:
		v.loading = false
		v.err = m.err
		return nil
	case GKEClusterActionResultMsg:
		v.deleting = false
		if m.Error != nil {
			v.err = m.Error
		}
		return nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		v.spinner, cmd = v.spinner.Update(m)
		return cmd
	case tea.KeyMsg:
		return v.handleKey(m)
	}
	return nil
}

func (v *GKEClusterDetailsView) View() string {
	// Delete dialog renders above everything else, per the overlay Z-order rule.
	if v.showConfirm && v.confirmDialog != nil {
		// dialog rendering wired in Task 14
	}
	if v.loading && v.details == nil {
		return renderLoading(v.spinner, "Loading cluster...")
	}
	if v.err != nil && v.details == nil {
		return components.RenderError(v.err)
	}
	var b strings.Builder
	b.WriteString(v.tabs.View())
	b.WriteString("\n\n")
	var body string
	switch v.tabs.ActiveTab().ID {
	case "overview":
		body = v.renderOverview()
	case "nodepools":
		body = v.renderNodePools()
	}
	if v.viewportSize {
		v.viewport.SetContent(body)
		b.WriteString(v.viewport.View())
	} else {
		b.WriteString(body)
	}
	if v.deleting {
		b.WriteString("\n\nDeleting cluster...\n")
	}
	return b.String()
}

func (v *GKEClusterDetailsView) renderOverview() string {
	d := v.details
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	value := lipgloss.NewStyle()
	var b strings.Builder
	fmt.Fprintf(&b, "Cluster: %s\n", d.Name)
	b.WriteString(strings.Repeat("─", 60) + "\n")
	row := func(k, val string) {
		fmt.Fprintf(&b, "  %-22s%s\n", label.Render(k+":"), value.Render(val))
	}
	row("Mode", humanMode(d.Mode))
	row("Status", statusBadge(d.Status))
	row("Location", fmt.Sprintf("%s (%s)", d.Location, d.LocationType+"al"))
	row("Master version", d.MasterVersion)
	row("Node version", d.NodeVersion)
	row("Release channel", defaultIfEmpty(d.ReleaseChannel, "(unspecified)"))
	row("Created", d.CreatedAt.UTC().Format("2006-01-02 15:04 MST"))

	b.WriteString("\nNetworking\n")
	b.WriteString(strings.Repeat("─", 60) + "\n")
	row("Network", d.Network)
	row("Subnetwork", d.Subnetwork)
	row("Cluster IPv4 CIDR", d.ClusterIPv4CIDR)
	row("Services IPv4 CIDR", d.ServicesIPv4CIDR)
	row("Endpoint", d.Endpoint)
	row("Private cluster", yesNo(d.PrivateCluster))

	b.WriteString("\nSecurity\n")
	b.WriteString(strings.Repeat("─", 60) + "\n")
	row("Workload Identity", defaultIfEmpty(d.WorkloadIdentityPool, "(off)"))
	row("Database encryption", d.DatabaseEncryption)
	authNets := "(none)"
	if len(d.MasterAuthorizedNetworks) > 0 {
		authNets = strings.Join(d.MasterAuthorizedNetworks, ", ")
	}
	row("Authorized networks", authNets)

	b.WriteString("\nAdd-ons\n")
	b.WriteString(strings.Repeat("─", 60) + "\n")
	addon := func(k string, on bool) {
		state := "Disabled"
		if on {
			state = "Enabled"
		}
		fmt.Fprintf(&b, "  %s\n", muted.Render(fmt.Sprintf("%s: %s", k, state)))
	}
	addon("HTTP load balancing", d.Addons.HTTPLoadBalancing)
	addon("Network policy", d.Addons.NetworkPolicy)
	addon("Persistent disk CSI", d.Addons.PersistentDiskCSI)
	addon("DNS cache", d.Addons.DNSCache)
	return b.String()
}

func humanMode(m string) string {
	switch m {
	case "AUTOPILOT":
		return "Autopilot"
	case "STANDARD":
		return "Standard"
	}
	return m
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
```

`defaultIfEmpty` already exists in `internal/ui/views/helpers.go` — re-use.

- [ ] **Step 4: Run tests, expect PASS**

Run: `go test ./internal/ui/views -run TestGKEClusterDetails_Overview -v`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/views/gke_cluster_details.go internal/ui/views/gke_cluster_details_test.go
git commit -m "2026-05-15: GKE phase 1 — Overview tab render + tests"
```

---

## Task 13: Node Pools tab render (with Autopilot suffix) + tests

**Files:** Modify `internal/ui/views/gke_cluster_details.go`, `internal/ui/views/gke_cluster_details_test.go`.

- [ ] **Step 1: Write the failing tests**

```go
func TestGKEClusterDetails_NodePoolsRendersStandard(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.tabs.SetActiveByID("nodepools")
	v.refreshNodePoolsTable()
	out := v.View()
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "e2-medium")
	assert.Contains(t, out, "on (1–10)")
	assert.Contains(t, out, "1.30.5-gke.1014001")
}

func TestGKEClusterDetails_NodePoolsRendersAutopilotSuffix(t *testing.T) {
	v := gkeDetailsFixture("AUTOPILOT")
	// Inject a system-managed pool
	v.details.NodePools = []gcp.NodePool{
		{Name: "default-pool", MachineType: "e2-medium", NodeCount: 1, Status: "RUNNING"},
	}
	v.tabs.SetActiveByID("nodepools")
	v.refreshNodePoolsTable()
	out := v.View()
	assert.Contains(t, out, "default-pool [managed by Autopilot]")
	assert.Contains(t, out, "—") // autoscale / version cells
}
```

- [ ] **Step 2: Run, expect FAIL (no `renderNodePools` / `refreshNodePoolsTable`)**

Run: `go test ./internal/ui/views -run TestGKEClusterDetails_NodePools -v`

- [ ] **Step 3: Implement `renderNodePools` + `refreshNodePoolsTable`**

```go
func (v *GKEClusterDetailsView) renderNodePools() string {
	if v.details == nil {
		return ""
	}
	if len(v.details.NodePools) == 0 {
		return "(no node pools)"
	}
	return v.poolsTable.View()
}

func (v *GKEClusterDetailsView) refreshNodePoolsTable() {
	if v.details == nil {
		return
	}
	autopilot := v.details.Mode == "AUTOPILOT"
	rows := make([]table.Row, 0, len(v.details.NodePools))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	for _, p := range v.details.NodePools {
		nameCell := p.Name
		autoscale := "off"
		version := p.NodeVersion
		if p.AutoscalingOn {
			autoscale = fmt.Sprintf("on (%d–%d)", p.AutoscalingMin, p.AutoscalingMax)
		}
		if autopilot {
			nameCell = p.Name + " " + muted.Render("[managed by Autopilot]")
			autoscale = "—"
			version = "—"
		}
		rows = append(rows, table.Row{
			nameCell,
			p.MachineType,
			fmt.Sprintf("%d", p.NodeCount),
			autoscale,
			version,
			statusBadge(p.Status),
			checkmark(p.AutoUpgrade),
			checkmark(p.AutoRepair),
		})
	}
	v.poolsTable.SetRows(rows)
}

func checkmark(b bool) string {
	if b {
		return "✓"
	}
	return ""
}
```

- [ ] **Step 4: Run, expect PASS**

Run: `go test ./internal/ui/views -run TestGKEClusterDetails_NodePools -v`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/views/gke_cluster_details.go internal/ui/views/gke_cluster_details_test.go
git commit -m "2026-05-15: GKE phase 1 — Node Pools tab + autopilot suffix"
```

---

## Task 14: Delete dialog + result handling + tests

**Files:** Modify `internal/ui/views/gke_cluster_details.go`, `gke_clusters.go`, `gke_cluster_details_test.go`.

- [ ] **Step 1: Write the failing test (dialog gating + correct message emitted)**

```go
func TestGKEClusterDetails_DeleteDialogGatesOnName(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	// User presses D
	cmd := v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	_ = cmd
	assert.True(t, v.showConfirm)

	// Wrong name typed → submit ignored
	v.confirmDialog.SetInput("wrong-name")
	out := v.confirmDialog.HandleSubmit()
	assert.False(t, out.Confirmed)

	// Correct name typed → submit emits GKEClusterDeleteRequestMsg
	v.confirmDialog.SetInput("prod")
	out = v.confirmDialog.HandleSubmit()
	assert.True(t, out.Confirmed)
}

func TestGKEClusterDetails_SetErrorClearsDeleting(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.deleting = true
	v.SetError(fmt.Errorf("api: insufficient permission"))
	assert.False(t, v.deleting)
	assert.NotNil(t, v.err)
}
```

> **Note:** the exact `confirmdialog` API may differ. Look at an existing
> caller (e.g., `loadbalancer_details.go` cascade-delete, or `instances.go`
> delete) and copy the same call patterns. Adapt the test accordingly.

- [ ] **Step 2: Run, expect FAIL**

Run: `go test ./internal/ui/views -run TestGKEClusterDetails_Delete -v`

- [ ] **Step 3: Implement `handleKey` delete path + dialog build + result wiring**

```go
import "github.com/charmbracelet/bubbles/key"

func (v *GKEClusterDetailsView) handleKey(m tea.KeyMsg) tea.Cmd {
	if v.showConfirm && v.confirmDialog != nil {
		// Forward keys to the dialog; dialog emits its own submit/cancel.
		cmd, done := v.confirmDialog.Update(m)
		if done {
			v.showConfirm = false
		}
		return cmd
	}
	switch m.String() {
	case "D":
		v.openDeleteDialog()
		return nil
	case "r":
		v.loading = true
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.load())
	case "tab":
		v.tabs.Next()
		v.viewport.GotoTop()
		return nil
	case "esc":
		return func() tea.Msg { return ui.NavigateBackMsg{} }
	case "h", "1":
		v.tabs.SetActiveByID("overview")
		v.viewport.GotoTop()
	case "l", "2":
		v.tabs.SetActiveByID("nodepools")
		v.viewport.GotoTop()
	}
	if isViewportScrollKey(m) {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(m)
		return cmd
	}
	return nil
}

func (v *GKEClusterDetailsView) openDeleteDialog() {
	if v.details == nil {
		return
	}
	body := strings.Join([]string{
		"This will permanently delete the cluster, its node pools,",
		"and all workloads running in it. The operation takes 2–5",
		"minutes and runs server-side after this call returns.",
		"",
		"NOT auto-deleted by GKE:",
		"  • Persistent volumes from dynamic provisioning unless",
		"    the StorageClass uses reclaimPolicy=Delete",
		"  • External load balancers created by Service of type",
		"    LoadBalancer (deleted only if cluster removal succeeds)",
		"  • Cloud DNS entries managed by the cluster",
	}, "\n")
	v.confirmDialog = confirmdialog.New(
		fmt.Sprintf("Delete cluster %q?", v.details.Name),
		body,
		v.details.Name, // expected typed value
	)
	v.confirmDialog.SetOnConfirm(func() tea.Cmd {
		v.deleting = true
		return func() tea.Msg {
			return GKEClusterDeleteRequestMsg{ProjectID: v.projectID, Location: v.details.Location, Name: v.details.Name}
		}
	})
	v.showConfirm = true
}
```

Also wire the same `openDeleteDialog`-style entry point on the list view (`gke_clusters.go`) when the user presses `D` directly. Simpler approach: the list view's `D` keybinding emits `GKEClusterSelectedMsg` first, the app handler navigates to the details view, the user presses `D` again there. **Phase 1 keeps it simple — `D` on the list view navigates to details with a query parameter / state to auto-open the dialog, OR list view shows its own dialog.** Pick the auto-open path for fewer keystrokes; record this as task-internal state:

- Add a `pendingDelete bool` to `GKEClusterSelectedMsg` (optional bool field), or
- Just open the dialog at the list level by reusing `confirmdialog`.

To keep Phase 1 tight, **only the details view shows the delete dialog**. The list view's `D` simply emits `GKEClusterSelectedMsg` and the user presses `D` again. Document this in the keybindings.

- [ ] **Step 4: Run, expect PASS**

Run: `go test ./internal/ui/views -run TestGKEClusterDetails_Delete -v`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/views/gke_cluster_details.go internal/ui/views/gke_cluster_details_test.go internal/ui/views/gke_clusters.go
git commit -m "2026-05-15: GKE phase 1 — delete dialog + result wiring"
```

---

## Task 15: ViewType enum + App struct fields + renderCurrentView arm

**Files:** Modify `internal/ui/app.go`, `internal/ui/app_render.go`.

- [ ] **Step 1: Add the ViewType constants**

Find the ViewType iota in `internal/ui/app.go`. Add:

```go
const (
	// ...existing views...
	ViewGKEClusters
	ViewGKEClusterDetails
)
```

- [ ] **Step 2: Add App struct fields**

```go
type App struct {
	// ...
	gkeClustersView       *views.GKEClustersView
	gkeClusterDetailsView *views.GKEClusterDetailsView
}
```

- [ ] **Step 3: Add cases to `getCurrentViewModel()`**

```go
case ViewGKEClusters:
	return a.gkeClustersView
case ViewGKEClusterDetails:
	return a.gkeClusterDetailsView
```

- [ ] **Step 4: Add cases to `renderCurrentView()` in `app_render.go`**

```go
case ViewGKEClusters:
	if a.gkeClustersView != nil {
		return a.gkeClustersView.View()
	}
case ViewGKEClusterDetails:
	if a.gkeClusterDetailsView != nil {
		return a.gkeClusterDetailsView.View()
	}
```

- [ ] **Step 5: Add breadcrumb resource cases in `app_render.go` (look for the LB precedent at line ~162–167)**

```go
case ViewGKEClusters, ViewGKEClusterDetails:
	// breadcrumb root: "Kubernetes Engine"
case ViewGKEClusterDetails:
	if a.gkeClusterDetailsView != nil {
		resources = append(resources, a.gkeClusterDetailsView.Name())
	}
```

- [ ] **Step 6: Add cases to `updateViewSizes()` if SetContext is required (Phase 1 doesn't need EmojiWidthBudget, but call SetSize via the existing layout helper)**

Look at how `loadBalancersView` / `loadBalancerDetailsView` are sized. Mirror the calls.

- [ ] **Step 7: Verify build**

Run: `go build ./...`

- [ ] **Step 8: Commit**

```bash
git add internal/ui/app.go internal/ui/app_render.go
git commit -m "2026-05-15: GKE phase 1 — ViewType + App fields + render switch"
```

---

## Task 16: Navigation handlers + clearAllViews + Update message routing

**Files:** Modify `internal/ui/app.go` (Update message handlers), `internal/ui/app_navigation.go`.

- [ ] **Step 1: Add message handler cases in `App.Update`**

```go
case views.GKEClusterSelectedMsg:
	return a, a.handleGKEClusterSelected(msg)
case views.GKEClusterDeleteRequestMsg:
	return a, a.handleGKEClusterDeleteRequest(msg)
case views.GKEClusterActionResultMsg:
	return a, a.handleGKEClusterActionResult(msg)
```

- [ ] **Step 2: Implement the handlers in `app_navigation.go`**

```go
func (a *App) handleGKEClusterSelected(msg views.GKEClusterSelectedMsg) tea.Cmd {
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewGKEClusterDetails
	var container *gcp.ContainerClient
	if a.gkeClustersView != nil {
		container = a.gkeClustersView.GetContainerClient()
	}
	a.gkeClusterDetailsView = views.NewGKEClusterDetailsView(msg.ProjectID, msg.Location, msg.Name, container, a.getComputeClient())
	a.updateViewSizes()
	return a.gkeClusterDetailsView.Init()
}

func (a *App) handleGKEClusterDeleteRequest(msg views.GKEClusterDeleteRequestMsg) tea.Cmd {
	a.registerRunningTask(fmt.Sprintf("Deleting cluster %s...", msg.Name))
	return func() tea.Msg {
		client, err := gcp.NewContainerClient(a.gcpClient.Context())
		if err != nil {
			return views.GKEClusterActionResultMsg{Action: "delete", Error: err}
		}
		if err := client.DeleteCluster(a.gcpClient.Context(), msg.ProjectID, msg.Location, msg.Name); err != nil {
			return views.GKEClusterActionResultMsg{Action: "delete", Error: err}
		}
		return views.GKEClusterActionResultMsg{Action: "delete"}
	}
}

func (a *App) handleGKEClusterActionResult(msg views.GKEClusterActionResultMsg) tea.Cmd {
	a.finishTask()
	if msg.Error != nil {
		a.err = msg.Error
		// Propagate to active view per the async-operations rule.
		if a.currentView == ViewGKEClusterDetails && a.gkeClusterDetailsView != nil {
			a.gkeClusterDetailsView.SetError(msg.Error)
		}
		return nil
	}
	// Success: navigate back to list and refresh.
	a.currentView = ViewGKEClusters
	a.gkeClusterDetailsView = nil
	if a.gkeClustersView != nil {
		return a.gkeClustersView.Init()
	}
	return nil
}
```

> `a.getComputeClient()` is whatever helper currently resolves a ComputeClient; copy the pattern from `handleLoadBalancerSelected`. If no such helper exists, instantiate inline.

- [ ] **Step 3: Add fields to `clearAllViews()`**

```go
func (a *App) clearAllViews() {
	// ...existing...
	a.gkeClustersView = nil
	a.gkeClusterDetailsView = nil
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/app.go internal/ui/app_navigation.go
git commit -m "2026-05-15: GKE phase 1 — navigation handlers + clearAllViews"
```

---

## Task 17: Sidebar entry + command palette + sidebar guards + cross-view nav

**Files:** Modify `internal/ui/components/sidebar/menu.go`, `internal/ui/components/sidebar/sidebar.go` (or wherever `ViewType` lives), `internal/ui/components/commandpalette/commands.go`, `internal/ui/app_navigation.go`.

- [ ] **Step 1: Add `sidebar.ViewGKEClusters` constant**

Find the sidebar `ViewType` iota and add `ViewGKEClusters`.

- [ ] **Step 2: Add "Kubernetes Engine" group with "Clusters" child in `menu.go`**

```go
{
	Label: "Kubernetes Engine",
	Items: []MenuItem{
		{Label: "Clusters", ViewType: ViewGKEClusters, Icon: "☸"},
	},
},
```

- [ ] **Step 3: Add `updateSidebarActiveView()` case**

```go
case ViewGKEClusters, ViewGKEClusterDetails:
	a.sidebar.SetActiveView(sidebar.ViewGKEClusters)
```

- [ ] **Step 4: Add sidebar guard for both views**

```go
case sidebar.ViewGKEClusters:
	if a.currentView != ViewGKEClusters && a.currentView != ViewGKEClusterDetails {
		a.currentView = ViewGKEClusters
		a.gkeClusterDetailsView = nil
		if a.gkeClustersView == nil {
			a.gkeClustersView = views.NewGKEClustersView(a.selectedProject.ID, nil)
		}
		cmd = a.gkeClustersView.Init()
	}
```

- [ ] **Step 5: Add to command palette**

In `commandpalette/commands.go`:

```go
// ViewType iota — add ViewGKEClusters before any sidebar trailers.
const (
	// ...existing...
	ViewGKEClusters
)

const IconGKE = "☸"

// In NavigationCommands():
{
	ID:       "nav:gke-clusters",
	Label:    "Kubernetes Engine: Clusters",
	Icon:     IconGKE,
	Type:     CommandTypeNavigation,
	ViewType: ViewGKEClusters,
	Enabled:  true,
},
```

- [ ] **Step 6: Update Network/Subnetwork cross-view nav handlers**

In `app_navigation.go`, find `handleNetworkSelected` and `handleSubnetSelected` (or equivalents) and add `gkeClusterDetailsView.GetComputeClient()` to the client resolution chain:

```go
if computeClient == nil && a.gkeClusterDetailsView != nil {
	computeClient = a.gkeClusterDetailsView.GetComputeClient()
}
```

- [ ] **Step 7: Verify build**

Run: `go build ./...`

- [ ] **Step 8: Commit**

```bash
git add internal/ui/components/sidebar/ internal/ui/components/commandpalette/ internal/ui/app_navigation.go
git commit -m "2026-05-15: GKE phase 1 — sidebar + command palette + cross-view nav"
```

---

## Task 18: Docs (CLAUDE.md, README.md, key-bindings.md)

**Files:** Modify `CLAUDE.md`, `README.md`, `.claude/rules/key-bindings.md`.

- [ ] **Step 1: `.claude/rules/key-bindings.md` — add two sections**

Append after the Load Balancers Observability section:

```markdown
## GKE Clusters View

| Key | Action |
|-----|--------|
| `Enter` | View cluster details |
| `D` | View cluster details (delete dialog opens in details view) |
| `S` | Open sort menu |
| `/` | Filter clusters (supports `mode:`, `status:`, `location:`) |
| `r` | Refresh list |
| `Esc` | Go back |

## GKE Cluster Details View

| Key | Action |
|-----|--------|
| `D` | Delete cluster (type-to-confirm) |
| `r` | Refresh details and node pools |
| `Tab` | Switch focus (tabs / content) |
| `h/l` or `1/2` | Switch tabs (Overview / Node Pools) |
| `Enter` | Navigate via focused link (Network / Subnetwork) |
| `j/k` or `↑/↓` or `PgUp/PgDn` | Scroll content |
| `Esc` | Go back |
```

- [ ] **Step 2: `CLAUDE.md` — add to the implemented-features list and remove from planned**

Move the GKE bullet from the **Planned Features** section to **Implemented Features**, replacing it with:

```markdown
- [x] GKE cluster management (Phase 1)
  - Clusters list across all locations (zonal + regional, Autopilot + Standard)
  - Cluster details view with Overview and Node Pools tabs
  - Network and Subnetwork link rows cross-link to existing VPC/subnet views
  - Type-to-confirm cluster delete with explicit warning about non-auto-deleted
    resources (dynamic-provisioned PVs, LB Services, Cloud DNS)
  - Fire-and-forget delete: API call returns immediately; refresh shows status
```

- [ ] **Step 3: `README.md` — add a one-line feature mention in the appropriate list**

```markdown
- [x] Kubernetes Engine (Phase 1): clusters list + details (Overview, Node Pools)
       + delete with type-to-confirm
```

- [ ] **Step 4: Verify build and run the full test suite**

```bash
go build ./...
go test ./...
```

- [ ] **Step 5: Run the linter**

```bash
make lint
```

Expected: 0 issues. Fix any complaints.

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md README.md .claude/rules/key-bindings.md
git commit -m "2026-05-15: GKE phase 1 — docs (CLAUDE.md, README.md, key-bindings)"
```

---

## Final integration smoke (manual)

After Task 18 lands, run:

```bash
make run
```

In the app:

1. Open the command palette (`:`), pick "Kubernetes Engine: Clusters". List loads.
2. Filter `mode:autopilot`. Confirm only Autopilot clusters show.
3. Press Enter on a Standard cluster. Overview tab renders; tab switch to Node Pools shows the pools table.
4. Switch to an Autopilot cluster. Node Pools tab shows `[managed by Autopilot]` suffix.
5. Press `D`. Dialog appears with the cluster name as the typed-confirm target. Cancel works. Submitting the wrong name does not delete.
6. Submit correct name on a throwaway test cluster. Status bar shows "Deleting cluster X...". List view re-loads with the cluster in `STOPPING` state.

If anything reads wrong on screen, check the bubble-tea-rendering rules first — most likely culprits are missing `viewport.GotoTop()` on tab change or a sidebar-guard miss when navigating between Kubernetes Engine and other groups.

---

## Self-review checklist (completed before handoff)

- [x] Spec coverage: every section of `Design.md` has a corresponding task.
- [x] No placeholders: all "implement later"-style sentences have been replaced with concrete code or explicit "Phase 2" notes.
- [x] Type consistency: `Cluster`, `ClusterDetails`, `NodePool`, `AddonsSummary`, `GKEClusterSelectedMsg`, `GKEClusterDeleteRequestMsg`, `GKEClusterActionResultMsg`, `gkeClusterLoadedMsg`, etc. are spelled identically across all tasks.
- [x] Message producer/consumer paired: every `*Msg` type defined has both an emitter (in a view) and a handler (in app code) — see Task 16.
- [x] `HasTextInputFocused()` implemented on the details view (delete dialog has text input) — Task 11 + Task 14.
- [x] `Init()` idempotent on both views — both reset `loading=true` and `err=nil`.
- [x] `SetError()` called from app handler on failure — Task 16.
- [x] Sidebar guard includes both `ViewGKEClusters` and `ViewGKEClusterDetails` — Task 17 step 4.
- [x] Cross-view client resolution chain updated — Task 17 step 6.
- [x] Tab body wrapped in `viewport.Model` even though Phase 1 fits — Task 11 + Task 12 View().
