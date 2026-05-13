# Load Balancers Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add live backend health on the Backends tab and an Observability tab (HTTP(S) charts) to `LoadBalancerDetailsView`. Read-only; no write paths.

**Architecture:** The Backends tab fans out `backendServices.getHealth` calls per backend group on first load and renders inline badges, with cursor-driven per-instance expansion. The Observability tab lazy-initializes on first visit and renders five `metricchart` charts driven by a new `internal/gcp/monitoring_lb.go` modeled on `monitoring_cloudrun.go`. Both share a single 30 s `tea.Tick` auto-refresh loop with the stale-tick guards documented in `.claude/rules/bubble-tea-rendering.md`.

**Tech Stack:** Go 1.26, Bubble Tea, Lip Gloss, `metricchart` (ntcharts wrapper), Cloud Monitoring v3, Compute Engine v1.

---

## File Structure

**Create:**
- `internal/gcp/monitoring_lb.go` — LB Cloud Monitoring helpers (request count, latencies, error code class, throughput).
- `internal/gcp/monitoring_lb_test.go`
- `internal/ui/views/loadbalancer_observability.go` — observability sub-view: charts, time range, auto-refresh ticker.
- `internal/ui/views/loadbalancer_observability_test.go`

**Modify:**
- `internal/gcp/loadbalancers.go` — `InstanceHealth`, `NEG`, `GetBackendHealth`, `GetNetworkEndpointGroup`.
- `internal/gcp/loadbalancers_test.go`
- `internal/ui/views/loadbalancer_details.go`:
  - Constructor now takes `*gcp.Client` (for monitoring) alongside the existing `*gcp.ComputeClient`.
  - Add `"observability"` tab.
  - Add `groupHealth` map, `groupFocus`, `groupExpanded`, and the observability sub-view field.
  - `renderBackends` shows badges; `renderObservability` delegates.
- `internal/ui/views/loadbalancer_details_test.go`
- `internal/ui/views/loadbalancer_messages.go` — internal `lbGroupHealthLoadedMsg`, `lbGroupHealthErrorMsg` (internal, lowercased — not cross-view).
- `internal/ui/app_navigation.go` — pass `a.gcpClient` into the new constructor.
- `README.md`, `CLAUDE.md`, `.claude/rules/key-bindings.md`.

---

## Phase A — GCP data layer

### Task 1: Add `InstanceHealth` and `NEG` types

**Files:**
- Modify: `internal/gcp/loadbalancers.go` (append type definitions after line 257, after `SSLCertificate`).
- Test: `internal/gcp/loadbalancers_test.go` (no test — types only; covered by Task 2/3 tests).

- [ ] **Step 1: Add types**

Append to `internal/gcp/loadbalancers.go` immediately after the `SSLCertificate` type:

```go
// InstanceHealth is the per-member result of backendServices.getHealth /
// regionBackendServices.getHealth. One entry per VM (or NEG endpoint)
// behind a backend group.
type InstanceHealth struct {
	Instance      string // last segment of the instance URL
	IPAddress     string
	Port          int64
	HealthState   string // "HEALTHY" | "UNHEALTHY" | "UNKNOWN" | "DRAINING"
	FailureReason string // empty when HealthState is HEALTHY
}

// NEG is a minimal projection of compute.NetworkEndpointGroup used to
// detect the SERVERLESS endpoint type during health resolution.
type NEG struct {
	Name                string
	SelfLink            string
	Zone                string // empty for global NEGs
	NetworkEndpointType string // "GCE_VM_IP_PORT" | "SERVERLESS" | "INTERNET_FQDN_PORT" | ...
}
```

- [ ] **Step 2: Build to verify it compiles**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/gcp/loadbalancers.go
git commit -m "2026-05-13: LB phase 2 — InstanceHealth and NEG types"
```

---

### Task 2: Implement `GetBackendHealth`

**Files:**
- Modify: `internal/gcp/loadbalancers.go`
- Test: `internal/gcp/loadbalancers_test.go`

`backendServices.getHealth` (global) and `regionBackendServices.getHealth` (regional) each take a `(projectID, backendServiceName, ResourceGroupReference{Group: groupURL})` payload and return a `BackendServiceGroupHealth` with `HealthStatus[]`. Scope follows the same `"global"` vs region pattern used by `GetBackendService`.

- [ ] **Step 1: Write the failing test**

Append to `internal/gcp/loadbalancers_test.go`:

```go
func TestBackendHealthScopeRouting(t *testing.T) {
	cases := []struct {
		name    string
		scope   string
		wantSvc string // expected service path segment
	}{
		{"global routes to BackendServices", "global", "BackendServices"},
		{"regional routes to RegionBackendServices", "us-central1", "RegionBackendServices"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// pure compile-time check that the routing exists; the test is
			// validated by the build, plus a sanity assertion below.
			assert.NotEmpty(t, tc.wantSvc)
		})
	}
}
```

This is a placeholder — the real test is below. Add:

```go
func TestInstanceHealthZeroValue(t *testing.T) {
	h := InstanceHealth{}
	assert.Empty(t, h.Instance)
	assert.Empty(t, h.HealthState)
	assert.Empty(t, h.FailureReason)
}
```

- [ ] **Step 2: Run tests (zero-value already passes; routing test passes trivially)**

Run: `go test ./internal/gcp/ -run "TestInstanceHealthZeroValue|TestBackendHealthScopeRouting" -v`
Expected: both PASS.

- [ ] **Step 3: Implement `GetBackendHealth`**

Append to `internal/gcp/loadbalancers.go`:

```go
// GetBackendHealth fetches per-instance health for one (backendService, group)
// pair. scope is "global" or a region. backendServiceName is the short name
// (no URL). groupURL is the full URL of the instance group or NEG.
func (c *ComputeClient) GetBackendHealth(ctx context.Context, projectID, scope, backendServiceName, groupURL string) ([]InstanceHealth, error) {
	ref := &compute.ResourceGroupReference{Group: groupURL}
	if scope == "global" {
		resp, err := c.service.BackendServices.GetHealth(projectID, backendServiceName, ref).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("get backend health %s: %w", backendServiceName, err)
		}
		return convertHealthStatuses(resp.HealthStatus), nil
	}
	resp, err := c.service.RegionBackendServices.GetHealth(projectID, scope, backendServiceName, ref).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get regional backend health %s/%s: %w", scope, backendServiceName, err)
	}
	return convertHealthStatuses(resp.HealthStatus), nil
}

func convertHealthStatuses(in []*compute.HealthStatus) []InstanceHealth {
	out := make([]InstanceHealth, 0, len(in))
	for _, hs := range in {
		out = append(out, InstanceHealth{
			Instance:      shortName(hs.Instance),
			IPAddress:     hs.IpAddress,
			Port:          hs.Port,
			HealthState:   hs.HealthState,
			FailureReason: deriveFailureReason(hs),
		})
	}
	return out
}

// deriveFailureReason produces a short human-readable reason string for an
// unhealthy member. The GCP HealthStatus response does not include a
// dedicated reason field; the closest signals are the health-check log
// and the IpAddress/Port pair. For v1 we just label by state — a
// follow-up can plumb richer reason text through Cloud Logging.
func deriveFailureReason(hs *compute.HealthStatus) string {
	if hs.HealthState == "HEALTHY" || hs.HealthState == "" {
		return ""
	}
	return hs.HealthState
}
```

- [ ] **Step 4: Replace placeholder test with a behavioral test**

Replace `TestBackendHealthScopeRouting` (added in Step 1) with:

```go
func TestConvertHealthStatuses(t *testing.T) {
	in := []*compute.HealthStatus{
		{Instance: "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/instances/vm-1", IpAddress: "10.0.0.5", Port: 80, HealthState: "HEALTHY"},
		{Instance: "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/instances/vm-2", IpAddress: "10.0.0.6", Port: 80, HealthState: "UNHEALTHY"},
	}
	got := convertHealthStatuses(in)
	require.Len(t, got, 2)
	assert.Equal(t, "vm-1", got[0].Instance)
	assert.Equal(t, "HEALTHY", got[0].HealthState)
	assert.Empty(t, got[0].FailureReason)
	assert.Equal(t, "vm-2", got[1].Instance)
	assert.Equal(t, "UNHEALTHY", got[1].HealthState)
	assert.Equal(t, "UNHEALTHY", got[1].FailureReason)
}
```

Ensure the test imports include `compute "google.golang.org/api/compute/v1"` (alias `compute` if needed for clarity) and `"github.com/stretchr/testify/require"`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/gcp/ -run "TestConvertHealthStatuses|TestInstanceHealthZeroValue" -v`
Expected: both PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gcp/loadbalancers.go internal/gcp/loadbalancers_test.go
git commit -m "2026-05-13: LB phase 2 — GetBackendHealth"
```

---

### Task 3: Implement `GetNetworkEndpointGroup`

**Files:**
- Modify: `internal/gcp/loadbalancers.go`
- Test: `internal/gcp/loadbalancers_test.go`

Needed to detect `SERVERLESS` NEGs and skip `GetBackendHealth` for them.

- [ ] **Step 1: Write the failing test**

Append to `internal/gcp/loadbalancers_test.go`:

```go
func TestConvertNEG(t *testing.T) {
	in := &compute.NetworkEndpointGroup{
		Name:                "my-neg",
		SelfLink:            "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/networkEndpointGroups/my-neg",
		NetworkEndpointType: "GCE_VM_IP_PORT",
	}
	got := convertNEG(in, "us-central1-a")
	assert.Equal(t, "my-neg", got.Name)
	assert.Equal(t, "us-central1-a", got.Zone)
	assert.Equal(t, "GCE_VM_IP_PORT", got.NetworkEndpointType)
}

func TestConvertNEGServerless(t *testing.T) {
	in := &compute.NetworkEndpointGroup{
		Name:                "cr-neg",
		NetworkEndpointType: "SERVERLESS",
	}
	got := convertNEG(in, "us-central1")
	assert.Equal(t, "SERVERLESS", got.NetworkEndpointType)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gcp/ -run TestConvertNEG -v`
Expected: FAIL with "convertNEG undefined".

- [ ] **Step 3: Implement `GetNetworkEndpointGroup` + `convertNEG`**

Append to `internal/gcp/loadbalancers.go`:

```go
// GetNetworkEndpointGroup fetches a NEG by zone (zonal NEGs) or global
// scope. scopeOrZone is either "global" or a zone name. groupURL is the
// full self-link of the NEG; the function extracts the name via
// shortName(groupURL).
func (c *ComputeClient) GetNetworkEndpointGroup(ctx context.Context, projectID, scopeOrZone, groupURL string) (*NEG, error) {
	name := shortName(groupURL)
	if scopeOrZone == "global" {
		neg, err := c.service.GlobalNetworkEndpointGroups.Get(projectID, name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("get global NEG %s: %w", name, err)
		}
		return convertNEG(neg, ""), nil
	}
	neg, err := c.service.NetworkEndpointGroups.Get(projectID, scopeOrZone, name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get NEG %s/%s: %w", scopeOrZone, name, err)
	}
	return convertNEG(neg, scopeOrZone), nil
}

func convertNEG(neg *compute.NetworkEndpointGroup, zone string) *NEG {
	return &NEG{
		Name:                neg.Name,
		SelfLink:            neg.SelfLink,
		Zone:                zone,
		NetworkEndpointType: neg.NetworkEndpointType,
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/gcp/ -run TestConvertNEG -v`
Expected: both PASS.

- [ ] **Step 5: Run full GCP test suite**

Run: `go test ./internal/gcp/ -v`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gcp/loadbalancers.go internal/gcp/loadbalancers_test.go
git commit -m "2026-05-13: LB phase 2 — GetNetworkEndpointGroup"
```

---

## Phase B — Monitoring helpers

### Task 4: Create `monitoring_lb.go` skeleton with filter helper and `LBMetrics` type

**Files:**
- Create: `internal/gcp/monitoring_lb.go`
- Create: `internal/gcp/monitoring_lb_test.go`

The skeleton mirrors `monitoring_cloudrun.go` — a per-resource filter helper, an aggregate struct, and we will fill in the per-metric methods in subsequent tasks.

For HTTP(S) external LBs the Cloud Monitoring resource type is `https_lb_rule`; for internal HTTPS LBs it is `internal_http_lb_rule`. The phase 2 spec scopes both. The forwarding-rule name appears in both label sets as `forwarding_rule_name`. Filtering on the rule name in a single filter requires using `(resource.type = "https_lb_rule" OR resource.type = "internal_http_lb_rule")`.

- [ ] **Step 1: Write the failing test**

Create `internal/gcp/monitoring_lb_test.go`:

```go
package gcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLBFilterContainsRuleAndMetric(t *testing.T) {
	f := lbFilter("my-rule", "loadbalancing.googleapis.com/https/request_count")
	assert.Contains(t, f, `forwarding_rule_name = "my-rule"`)
	assert.Contains(t, f, `metric.type = "loadbalancing.googleapis.com/https/request_count"`)
	// Both HTTP(S) resource types must be included so the filter matches
	// external HTTPS, internal HTTPS, and external HTTP load balancers.
	assert.True(t, strings.Contains(f, `https_lb_rule`))
	assert.True(t, strings.Contains(f, `internal_http_lb_rule`))
}

func TestLBFilterWithLabel(t *testing.T) {
	f := lbFilterWithLabel("my-rule", "loadbalancing.googleapis.com/https/request_count", "response_code_class", "4xx")
	assert.Contains(t, f, `forwarding_rule_name = "my-rule"`)
	assert.Contains(t, f, `metric.labels.response_code_class = "4xx"`)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/gcp/ -run "TestLBFilter" -v`
Expected: FAIL with "lbFilter undefined".

- [ ] **Step 3: Implement skeleton**

Create `internal/gcp/monitoring_lb.go`:

```go
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

// LBMetrics aggregates the time-series data the Observability tab renders
// for a single HTTP(S) load balancer.
type LBMetrics struct {
	RequestCount    []DataPoint // requests/sec
	RequestCount4xx []DataPoint // raw 4xx rate (requests/sec)
	RequestCount5xx []DataPoint // raw 5xx rate (requests/sec)
	Latency50       []DataPoint // total latency p50 (ms)
	Latency95       []DataPoint // total latency p95 (ms)
	Latency99       []DataPoint // total latency p99 (ms)
	BackendLat50    []DataPoint // backend latency p50 (ms)
	BackendLat95    []DataPoint // backend latency p95 (ms)
	BackendLat99    []DataPoint // backend latency p99 (ms)
	RequestBytes    []DataPoint // bytes/sec
	ResponseBytes   []DataPoint // bytes/sec
	LastFetch       time.Time
}

// lbFilter builds a Cloud Monitoring filter scoped to a single HTTP(S)
// load balancer keyed by forwarding-rule name. The filter matches both
// external (https_lb_rule) and internal (internal_http_lb_rule) resource
// types so external HTTPS, internal HTTPS, and external HTTP load
// balancers all resolve through a single helper.
func lbFilter(forwardingRuleName, metricType string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`(resource.type = "https_lb_rule" OR resource.type = "internal_http_lb_rule") AND resource.labels.forwarding_rule_name = "%s" AND metric.type = "%s"`,
		forwardingRuleName, metricType,
	)
}

// lbFilterWithLabel narrows lbFilter by a single metric label (e.g. a
// response_code_class label for error breakdowns).
func lbFilterWithLabel(forwardingRuleName, metricType, labelKey, labelValue string) string {
	return fmt.Sprintf( //nolint:gocritic // GCP filter syntax requires double quotes
		`%s AND metric.labels.%s = "%s"`,
		lbFilter(forwardingRuleName, metricType), labelKey, labelValue,
	)
}

// fetchLBMetric is the rate/sum/mean fetcher mirrored from fetchCloudRunMetric.
func (c *MonitoringClient) fetchLBMetric(ctx context.Context, filter string, duration time.Duration, aligner monitoringpb.Aggregation_Aligner, reducer monitoringpb.Aggregation_Reducer) ([]DataPoint, error) {
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
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})
	return points, nil
}

// fetchLBPercentile is the percentile aligner variant.
func (c *MonitoringClient) fetchLBPercentile(ctx context.Context, filter string, duration time.Duration, aligner monitoringpb.Aggregation_Aligner) ([]DataPoint, error) {
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
			CrossSeriesReducer: monitoringpb.Aggregation_REDUCE_MEAN,
		},
	}

	points, err := c.collectDataPoints(ctx, req)
	if err != nil {
		return nil, err
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})
	return points, nil
}
```

- [ ] **Step 4: Add `projectID` access if needed**

The above references `c.projectID`. Verify `MonitoringClient` already exposes it (see `monitoring.go` line 18; `projectID` is unexported but the file is in the same package). No change needed.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/gcp/ -run "TestLBFilter" -v`
Expected: both PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gcp/monitoring_lb.go internal/gcp/monitoring_lb_test.go
git commit -m "2026-05-13: LB phase 2 — monitoring_lb.go skeleton + lbFilter"
```

---

### Task 5: `GetLBRequestCount` and `GetLBRequestCountByCodeClass`

**Files:**
- Modify: `internal/gcp/monitoring_lb.go`
- Modify: `internal/gcp/monitoring_lb_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/gcp/monitoring_lb_test.go`:

```go
func TestGetLBRequestCountFilter(t *testing.T) {
	// We can't run a real GCP call in unit tests; assert the filter
	// shape on the helper that builds it. This is a compile + shape check
	// that protects against accidental metric-name typos.
	f := lbFilter("rule-x", "loadbalancing.googleapis.com/https/request_count")
	assert.Contains(t, f, "https/request_count")
	assert.Contains(t, f, `forwarding_rule_name = "rule-x"`)
}

func TestGetLBRequestCountByCodeClassFilter(t *testing.T) {
	f := lbFilterWithLabel("rule-x", "loadbalancing.googleapis.com/https/request_count", "response_code_class", "5xx")
	assert.Contains(t, f, `metric.labels.response_code_class = "5xx"`)
	assert.Contains(t, f, "https/request_count")
}
```

- [ ] **Step 2: Run tests (will pass — these only exercise the filter helpers already in place)**

Run: `go test ./internal/gcp/ -run "TestGetLBRequestCount" -v`
Expected: both PASS.

- [ ] **Step 3: Add the API methods**

Append to `internal/gcp/monitoring_lb.go`:

```go
const lbMetricRequestCount = "loadbalancing.googleapis.com/https/request_count"

// GetLBRequestCount fetches per-second request count for an HTTP(S) LB
// keyed by forwarding-rule name. ALIGN_RATE + REDUCE_SUM aggregate across
// all per-backend time series into one rate-of-requests series.
func (c *MonitoringClient) GetLBRequestCount(ctx context.Context, forwardingRuleName string, duration time.Duration) ([]DataPoint, error) {
	filter := lbFilter(forwardingRuleName, lbMetricRequestCount)
	return c.fetchLBMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
}

// GetLBRequestCountByCodeClass narrows request_count by response_code_class
// label (one of "2xx", "3xx", "4xx", "5xx"). Used to compute error rate.
func (c *MonitoringClient) GetLBRequestCountByCodeClass(ctx context.Context, forwardingRuleName, codeClass string, duration time.Duration) ([]DataPoint, error) {
	filter := lbFilterWithLabel(forwardingRuleName, lbMetricRequestCount, "response_code_class", codeClass)
	return c.fetchLBMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
}
```

- [ ] **Step 4: Build to verify**

Run: `go build ./internal/gcp/...`
Expected: no errors.

- [ ] **Step 5: Run full GCP suite**

Run: `go test ./internal/gcp/ -v`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gcp/monitoring_lb.go internal/gcp/monitoring_lb_test.go
git commit -m "2026-05-13: LB phase 2 — GetLBRequestCount + ByCodeClass"
```

---

### Task 6: `GetLBRequestLatencies` (p50/p95/p99)

**Files:**
- Modify: `internal/gcp/monitoring_lb.go`
- Modify: `internal/gcp/monitoring_lb_test.go`

Distribution metric. Pattern mirrors `GetCloudRunRequestLatencies` exactly — three percentile aligner calls. Values returned are **already in milliseconds** (per GCP API gotcha for `https/total_latencies` — do not scale).

- [ ] **Step 1: Write the failing test**

Append to `internal/gcp/monitoring_lb_test.go`:

```go
func TestGetLBRequestLatenciesFilter(t *testing.T) {
	f := lbFilter("rule-x", "loadbalancing.googleapis.com/https/total_latencies")
	assert.Contains(t, f, "https/total_latencies")
}
```

- [ ] **Step 2: Run test**

Run: `go test ./internal/gcp/ -run TestGetLBRequestLatenciesFilter -v`
Expected: PASS.

- [ ] **Step 3: Implement the API method**

Append to `internal/gcp/monitoring_lb.go`:

```go
const lbMetricRequestLatencies = "loadbalancing.googleapis.com/https/total_latencies"

// GetLBRequestLatencies fetches p50, p95, and p99 of total request latency
// for the LB. Total latency = backend latency + LB overhead. Values are
// returned in milliseconds (the GCP metric is already in ms — no scale).
func (c *MonitoringClient) GetLBRequestLatencies(ctx context.Context, forwardingRuleName string, duration time.Duration) (p50, p95, p99 []DataPoint, err error) {
	filter := lbFilter(forwardingRuleName, lbMetricRequestLatencies)

	p50, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_50)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p50 total latency: %w", err)
	}
	p95, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_95)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p95 total latency: %w", err)
	}
	p99, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_99)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p99 total latency: %w", err)
	}
	return p50, p95, p99, nil
}
```

- [ ] **Step 4: Build and run**

Run: `go test ./internal/gcp/ -v`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/gcp/monitoring_lb.go internal/gcp/monitoring_lb_test.go
git commit -m "2026-05-13: LB phase 2 — GetLBRequestLatencies"
```

---

### Task 7: `GetLBBackendLatencies` (p50/p95/p99)

**Files:**
- Modify: `internal/gcp/monitoring_lb.go`
- Modify: `internal/gcp/monitoring_lb_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/gcp/monitoring_lb_test.go`:

```go
func TestGetLBBackendLatenciesFilter(t *testing.T) {
	f := lbFilter("rule-x", "loadbalancing.googleapis.com/https/backend_latencies")
	assert.Contains(t, f, "https/backend_latencies")
}
```

- [ ] **Step 2: Run test**

Run: `go test ./internal/gcp/ -run TestGetLBBackendLatenciesFilter -v`
Expected: PASS.

- [ ] **Step 3: Implement the API method**

Append to `internal/gcp/monitoring_lb.go`:

```go
const lbMetricBackendLatencies = "loadbalancing.googleapis.com/https/backend_latencies"

// GetLBBackendLatencies fetches p50, p95, and p99 of backend-only latency
// (origin response time, excludes LB-introduced overhead). Values in ms.
func (c *MonitoringClient) GetLBBackendLatencies(ctx context.Context, forwardingRuleName string, duration time.Duration) (p50, p95, p99 []DataPoint, err error) {
	filter := lbFilter(forwardingRuleName, lbMetricBackendLatencies)

	p50, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_50)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p50 backend latency: %w", err)
	}
	p95, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_95)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p95 backend latency: %w", err)
	}
	p99, err = c.fetchLBPercentile(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_PERCENTILE_99)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch p99 backend latency: %w", err)
	}
	return p50, p95, p99, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/gcp/ -v`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/gcp/monitoring_lb.go internal/gcp/monitoring_lb_test.go
git commit -m "2026-05-13: LB phase 2 — GetLBBackendLatencies"
```

---

### Task 8: `GetLBRequestBytes` and `GetLBResponseBytes`

**Files:**
- Modify: `internal/gcp/monitoring_lb.go`
- Modify: `internal/gcp/monitoring_lb_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/gcp/monitoring_lb_test.go`:

```go
func TestGetLBThroughputFilters(t *testing.T) {
	reqF := lbFilter("rule-x", "loadbalancing.googleapis.com/https/request_bytes_count")
	respF := lbFilter("rule-x", "loadbalancing.googleapis.com/https/response_bytes_count")
	assert.Contains(t, reqF, "https/request_bytes_count")
	assert.Contains(t, respF, "https/response_bytes_count")
}
```

- [ ] **Step 2: Run test**

Run: `go test ./internal/gcp/ -run TestGetLBThroughputFilters -v`
Expected: PASS.

- [ ] **Step 3: Implement the API methods**

Append to `internal/gcp/monitoring_lb.go`:

```go
const (
	lbMetricRequestBytes  = "loadbalancing.googleapis.com/https/request_bytes_count"
	lbMetricResponseBytes = "loadbalancing.googleapis.com/https/response_bytes_count"
)

// GetLBRequestBytes returns request-bytes/sec rate-aligned series.
func (c *MonitoringClient) GetLBRequestBytes(ctx context.Context, forwardingRuleName string, duration time.Duration) ([]DataPoint, error) {
	filter := lbFilter(forwardingRuleName, lbMetricRequestBytes)
	return c.fetchLBMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
}

// GetLBResponseBytes returns response-bytes/sec rate-aligned series.
func (c *MonitoringClient) GetLBResponseBytes(ctx context.Context, forwardingRuleName string, duration time.Duration) ([]DataPoint, error) {
	filter := lbFilter(forwardingRuleName, lbMetricResponseBytes)
	return c.fetchLBMetric(ctx, filter, duration, monitoringpb.Aggregation_ALIGN_RATE, monitoringpb.Aggregation_REDUCE_SUM)
}
```

- [ ] **Step 4: Run all GCP tests**

Run: `go test ./internal/gcp/ -v`
Expected: all pass.

- [ ] **Step 5: Run linter**

Run: `make lint`
Expected: no new warnings in `monitoring_lb.go` or `monitoring_lb_test.go`.

- [ ] **Step 6: Commit**

```bash
git add internal/gcp/monitoring_lb.go internal/gcp/monitoring_lb_test.go
git commit -m "2026-05-13: LB phase 2 — GetLBRequestBytes + ResponseBytes"
```

---

## Phase C — Backends tab health

### Task 9: Plumb `*gcp.Client` into `LoadBalancerDetailsView`

The Observability tab (Task 17) needs a `*gcp.Client` to reach `GetMonitoringClient`. The current details-view constructor takes only `*gcp.ComputeClient`. Add a second parameter and update the single caller.

**Files:**
- Modify: `internal/ui/views/loadbalancer_details.go`
- Modify: `internal/ui/views/loadbalancer_details_test.go`
- Modify: `internal/ui/app_navigation.go` (line ~3604)

- [ ] **Step 1: Update existing tests to use the new signature**

In `internal/ui/views/loadbalancer_details_test.go`, the four existing call sites are:

```
NewLoadBalancerDetailsView("proj", "global", "front", nil)
```

Change each to:

```
NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
```

(passing `nil` for both clients — these tests don't exercise live fetch paths).

- [ ] **Step 2: Update the constructor signature**

In `internal/ui/views/loadbalancer_details.go`:

Replace:

```go
func NewLoadBalancerDetailsView(projectID, scope, name string, client *gcp.ComputeClient) *LoadBalancerDetailsView {
```

with:

```go
func NewLoadBalancerDetailsView(projectID, scope, name string, client *gcp.ComputeClient, gcpClient *gcp.Client) *LoadBalancerDetailsView {
```

Add the field to the struct (insert after the existing `client *gcp.ComputeClient` line near line 26):

```go
gcpClient *gcp.Client
```

In the constructor body, set the new field:

```go
return &LoadBalancerDetailsView{
    projectID: projectID,
    scope:     scope,
    name:      name,
    client:    client,
    gcpClient: gcpClient,
    tabs:      t,
    spinner:   components.NewGCPSpinner(),
    keys:      defaultLoadBalancerDetailsKeyMap(),
}
```

- [ ] **Step 3: Update the caller in `app_navigation.go`**

In `internal/ui/app_navigation.go` around line 3604, replace:

```go
a.loadBalancerDetailsView = views.NewLoadBalancerDetailsView(
    a.selectedProject.ID, msg.Scope, msg.Name, client,
)
```

with:

```go
a.loadBalancerDetailsView = views.NewLoadBalancerDetailsView(
    a.selectedProject.ID, msg.Scope, msg.Name, client, a.gcpClient,
)
```

- [ ] **Step 4: Build and run tests**

Run: `go build ./... && go test ./internal/ui/views/ -run TestLoadBalancerDetails -v`
Expected: build clean, existing details tests still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_details_test.go internal/ui/app_navigation.go
git commit -m "2026-05-13: LB phase 2 — plumb gcp.Client into details view constructor"
```

---

### Task 10: Add `groupHealth` state types

State only — no rendering or fetching yet.

**Files:**
- Modify: `internal/ui/views/loadbalancer_details.go`
- Modify: `internal/ui/views/loadbalancer_messages.go`

- [ ] **Step 1: Add internal message types and state enums**

Append to `internal/ui/views/loadbalancer_messages.go`:

```go
// Internal message types for backend health fan-out. Lowercase by
// convention — these never cross the view boundary.
type lbGroupHealthLoadedMsg struct {
	groupURL string
	statuses []gcp.InstanceHealth
}

type lbGroupHealthErrorMsg struct {
	groupURL string
	err      error
}

type lbGroupSkippedMsg struct {
	groupURL string
	reason   string // human-readable, e.g. "Cloud Storage backend"
}
```

Verify that `gcp` is already imported in `loadbalancer_messages.go`. If not, add the import.

- [ ] **Step 2: Add state types to the view**

In `internal/ui/views/loadbalancer_details.go`, after the `fetchState` type definition (around line 67), add:

```go
type groupHealthPhase int

const (
	groupHealthLoading groupHealthPhase = iota
	groupHealthOK
	groupHealthErrored
	groupHealthSkipped
)

type groupHealthState struct {
	phase    groupHealthPhase
	statuses []gcp.InstanceHealth
	err      error
	reason   string // populated when phase == groupHealthSkipped
}
```

Add fields to `LoadBalancerDetailsView` struct (insert after `checks []gcp.HealthCheck` line):

```go
// Backend group health (phase 2).
groupHealth   map[string]groupHealthState // keyed on Backend.Group URL
groupFocus    int                         // index into the flat list of group rows; -1 = no focus
groupExpanded map[string]bool             // group URL -> expanded?
```

In the constructor, initialize the maps:

```go
return &LoadBalancerDetailsView{
    projectID:     projectID,
    scope:         scope,
    name:          name,
    client:        client,
    gcpClient:     gcpClient,
    tabs:          t,
    spinner:       components.NewGCPSpinner(),
    keys:          defaultLoadBalancerDetailsKeyMap(),
    groupHealth:   map[string]groupHealthState{},
    groupExpanded: map[string]bool{},
    groupFocus:    -1,
}
```

- [ ] **Step 3: Build to verify it compiles**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_messages.go
git commit -m "2026-05-13: LB phase 2 — group health state types"
```

---

### Task 11: Group kind detection helper

Pure helper that classifies a `Backend.Group` URL into one of: instance group, NEG, serverless NEG, unsupported. The serverless determination is provisional from the URL only (any `/networkEndpointGroups/` is treated as "needs NEG GET" — Task 12 finishes the determination with the NEG type).

**Files:**
- Modify: `internal/ui/views/loadbalancer_details.go`
- Modify: `internal/ui/views/loadbalancer_details_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/views/loadbalancer_details_test.go`:

```go
func TestClassifyBackendGroupURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want backendGroupKind
	}{
		{
			name: "zonal instance group",
			url:  "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/instanceGroups/ig-prod",
			want: backendGroupInstanceGroup,
		},
		{
			name: "regional instance group",
			url:  "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/instanceGroups/ig-prod",
			want: backendGroupInstanceGroup,
		},
		{
			name: "zonal NEG",
			url:  "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/networkEndpointGroups/neg-prod",
			want: backendGroupNEG,
		},
		{
			name: "regional NEG (serverless)",
			url:  "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/networkEndpointGroups/cr-neg",
			want: backendGroupNEG,
		},
		{
			name: "empty url",
			url:  "",
			want: backendGroupUnknown,
		},
		{
			name: "random",
			url:  "https://example.com/foo/bar",
			want: backendGroupUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyBackendGroupURL(tc.url))
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/views/ -run TestClassifyBackendGroupURL -v`
Expected: FAIL with "classifyBackendGroupURL undefined".

- [ ] **Step 3: Implement the classifier**

Append to `internal/ui/views/loadbalancer_details.go` (near the bottom of the file, alongside the existing helpers like `collectBackendURLs`):

```go
type backendGroupKind int

const (
	backendGroupUnknown backendGroupKind = iota
	backendGroupInstanceGroup
	backendGroupNEG
)

// classifyBackendGroupURL inspects a Backend.Group URL and returns a coarse
// kind. Distinguishing serverless from VM-backed NEGs requires a separate
// API GET (see fetchGroupHealth in Task 12).
func classifyBackendGroupURL(url string) backendGroupKind {
	if url == "" {
		return backendGroupUnknown
	}
	switch {
	case strings.Contains(url, "/instanceGroups/"):
		return backendGroupInstanceGroup
	case strings.Contains(url, "/networkEndpointGroups/"):
		return backendGroupNEG
	default:
		return backendGroupUnknown
	}
}

// groupScope extracts the zone-or-region segment from a Backend.Group URL.
// Returns "global" if the URL contains "/global/" instead of a zone or
// region (rare, but possible for global NEGs).
func groupScope(url string) string {
	parts := strings.Split(url, "/")
	for i, p := range parts {
		switch p {
		case "zones", "regions":
			if i+1 < len(parts) {
				return parts[i+1]
			}
		case "global":
			return "global"
		}
	}
	return ""
}
```

Add a small test for `groupScope` too:

```go
func TestGroupScope(t *testing.T) {
	cases := map[string]string{
		"https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/instanceGroups/ig-prod": "us-central1-a",
		"https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/networkEndpointGroups/neg": "us-central1",
		"https://www.googleapis.com/compute/v1/projects/p/global/networkEndpointGroups/global-neg":       "global",
		"":                                                                                              "",
	}
	for url, want := range cases {
		t.Run(url, func(t *testing.T) {
			assert.Equal(t, want, groupScope(url))
		})
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ui/views/ -run "TestClassifyBackendGroupURL|TestGroupScope" -v`
Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_details_test.go
git commit -m "2026-05-13: LB phase 2 — classifyBackendGroupURL + groupScope helpers"
```

---

### Task 12: Fan-out health fetch on `lbBackendsLoadedMsg`

On the existing `lbBackendsLoadedMsg` handler, additionally fire one `tea.Cmd` per `(BackendService, Backend.Group)` pair. NEG groups first GET the NEG to detect `SERVERLESS`, then either call `GetBackendHealth` or emit a skip.

**Files:**
- Modify: `internal/ui/views/loadbalancer_details.go`

- [ ] **Step 1: Add helper that produces the fan-out `tea.Cmd` list**

Append to `internal/ui/views/loadbalancer_details.go`:

```go
// fetchAllBackendHealth returns one tea.Cmd per (backend service, group)
// pair across the freshly-loaded backend services. Each cmd ultimately
// emits one of: lbGroupHealthLoadedMsg, lbGroupHealthErrorMsg,
// lbGroupSkippedMsg. The view's groupHealth map is keyed by Backend.Group
// URL, so duplicate Group URLs (rare — same group referenced by multiple
// backend services) share state.
func (v *LoadBalancerDetailsView) fetchAllBackendHealth() tea.Cmd {
	if v.client == nil {
		return nil
	}
	var cmds []tea.Cmd
	seen := map[string]struct{}{}
	for i := range v.backends {
		bs := &v.backends[i]
		for _, be := range bs.Backends {
			if be.Group == "" {
				continue
			}
			if _, ok := seen[be.Group]; ok {
				continue
			}
			seen[be.Group] = struct{}{}
			v.groupHealth[be.Group] = groupHealthState{phase: groupHealthLoading}
			cmds = append(cmds, v.fetchGroupHealth(bs.Name, bs.Scope, be.Group))
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// fetchGroupHealth dispatches the health resolution for a single group.
// For NEGs, it first GETs the NEG to check for SERVERLESS endpoint type
// and skips health if so. For instance groups, it calls GetBackendHealth
// directly.
func (v *LoadBalancerDetailsView) fetchGroupHealth(backendServiceName, scope, groupURL string) tea.Cmd {
	kind := classifyBackendGroupURL(groupURL)
	switch kind {
	case backendGroupInstanceGroup:
		return v.fetchInstanceGroupHealth(backendServiceName, scope, groupURL)
	case backendGroupNEG:
		return v.fetchNEGHealth(backendServiceName, scope, groupURL)
	default:
		return func() tea.Msg {
			return lbGroupSkippedMsg{groupURL: groupURL, reason: "unsupported backend kind"}
		}
	}
}

func (v *LoadBalancerDetailsView) fetchInstanceGroupHealth(backendServiceName, scope, groupURL string) tea.Cmd {
	return func() tea.Msg {
		statuses, err := v.client.GetBackendHealth(gocontext.Background(), v.projectID, scope, backendServiceName, groupURL)
		if err != nil {
			return lbGroupHealthErrorMsg{groupURL: groupURL, err: err}
		}
		return lbGroupHealthLoadedMsg{groupURL: groupURL, statuses: statuses}
	}
}

func (v *LoadBalancerDetailsView) fetchNEGHealth(backendServiceName, scope, groupURL string) tea.Cmd {
	return func() tea.Msg {
		negScope := groupScope(groupURL)
		neg, err := v.client.GetNetworkEndpointGroup(gocontext.Background(), v.projectID, negScope, groupURL)
		if err != nil {
			return lbGroupHealthErrorMsg{groupURL: groupURL, err: err}
		}
		if neg.NetworkEndpointType == "SERVERLESS" {
			return lbGroupSkippedMsg{groupURL: groupURL, reason: "serverless NEG"}
		}
		statuses, err := v.client.GetBackendHealth(gocontext.Background(), v.projectID, scope, backendServiceName, groupURL)
		if err != nil {
			return lbGroupHealthErrorMsg{groupURL: groupURL, err: err}
		}
		return lbGroupHealthLoadedMsg{groupURL: groupURL, statuses: statuses}
	}
}
```

- [ ] **Step 2: Wire the fan-out into the `lbBackendsLoadedMsg` handler**

In `internal/ui/views/loadbalancer_details.go` find this block in `Update()`:

```go
case lbBackendsLoadedMsg:
    v.backends = m.services
    v.fetchState.backendsLoaded = true
    seen := map[string]struct{}{}
    hcs := []string{}
    for i := range m.services {
        for _, h := range m.services[i].HealthChecks {
            if _, ok := seen[h]; ok {
                continue
            }
            seen[h] = struct{}{}
            hcs = append(hcs, h)
        }
    }
    if len(hcs) == 0 {
        v.fetchState.checksLoaded = true
        return nil
    }
    return v.fetchHealthChecks(hcs)
```

Replace the final two return statements with batched fan-out:

```go
case lbBackendsLoadedMsg:
    v.backends = m.services
    v.fetchState.backendsLoaded = true
    seen := map[string]struct{}{}
    hcs := []string{}
    for i := range m.services {
        for _, h := range m.services[i].HealthChecks {
            if _, ok := seen[h]; ok {
                continue
            }
            seen[h] = struct{}{}
            hcs = append(hcs, h)
        }
    }
    healthCmd := v.fetchAllBackendHealth()
    if len(hcs) == 0 {
        v.fetchState.checksLoaded = true
        return healthCmd
    }
    return tea.Batch(v.fetchHealthChecks(hcs), healthCmd)
```

- [ ] **Step 3: Handle the new message types in `Update()`**

Add cases after `lbHealthChecksLoadedMsg` (around line 207):

```go
case lbGroupHealthLoadedMsg:
    v.groupHealth[m.groupURL] = groupHealthState{
        phase:    groupHealthOK,
        statuses: m.statuses,
    }
    return nil
case lbGroupHealthErrorMsg:
    v.groupHealth[m.groupURL] = groupHealthState{
        phase: groupHealthErrored,
        err:   m.err,
    }
    return nil
case lbGroupSkippedMsg:
    v.groupHealth[m.groupURL] = groupHealthState{
        phase:  groupHealthSkipped,
        reason: m.reason,
    }
    return nil
```

- [ ] **Step 4: Reset the maps in `Init()`**

In `Init()` (around line 102), add:

```go
v.groupHealth = map[string]groupHealthState{}
v.groupExpanded = map[string]bool{}
v.groupFocus = -1
```

These resets allow `r` (refresh) to redo the fan-out cleanly.

- [ ] **Step 5: Write a state-machine test**

Append to `internal/ui/views/loadbalancer_details_test.go`:

```go
func TestGroupHealthHandlersUpdateState(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.Update(lbGroupHealthLoadedMsg{
		groupURL: "ig-1",
		statuses: []gcp.InstanceHealth{{Instance: "vm-1", HealthState: "HEALTHY"}},
	})
	st, ok := v.groupHealth["ig-1"]
	require.True(t, ok)
	assert.Equal(t, groupHealthOK, st.phase)
	require.Len(t, st.statuses, 1)
	assert.Equal(t, "vm-1", st.statuses[0].Instance)

	v.Update(lbGroupHealthErrorMsg{groupURL: "ig-2", err: errors.New("boom")})
	st = v.groupHealth["ig-2"]
	assert.Equal(t, groupHealthErrored, st.phase)
	assert.EqualError(t, st.err, "boom")

	v.Update(lbGroupSkippedMsg{groupURL: "neg-1", reason: "serverless NEG"})
	st = v.groupHealth["neg-1"]
	assert.Equal(t, groupHealthSkipped, st.phase)
	assert.Equal(t, "serverless NEG", st.reason)
}
```

Ensure the test file imports `errors` and `github.com/slayer/gcon/internal/gcp`.

- [ ] **Step 6: Run the test**

Run: `go test ./internal/ui/views/ -run TestGroupHealthHandlersUpdateState -v`
Expected: PASS.

- [ ] **Step 7: Run full view-test suite**

Run: `go test ./internal/ui/views/`
Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_details_test.go
git commit -m "2026-05-13: LB phase 2 — backend health fan-out + state handlers"
```

---

### Task 13: Render badges (collapsed) on the Backends tab

**Files:**
- Modify: `internal/ui/views/loadbalancer_details.go`
- Modify: `internal/ui/views/loadbalancer_details_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/views/loadbalancer_details_test.go`:

```go
func TestRenderBackendsShowsHealthBadge(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.backendsLoaded = true
	v.backends = []gcp.BackendService{{
		Name:     "api-backend",
		Protocol: "HTTPS",
		Backends: []gcp.Backend{{Group: "https://example/zones/z/instanceGroups/ig-1"}},
	}}
	v.groupHealth["https://example/zones/z/instanceGroups/ig-1"] = groupHealthState{
		phase: groupHealthOK,
		statuses: []gcp.InstanceHealth{
			{Instance: "vm-1", HealthState: "HEALTHY"},
			{Instance: "vm-2", HealthState: "HEALTHY"},
			{Instance: "vm-3", HealthState: "UNHEALTHY"},
		},
	}
	out := v.renderBackends()
	assert.Contains(t, out, "ig-1")
	assert.Contains(t, out, "2/3 healthy")
}

func TestRenderBackendsLoadingBadge(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.backendsLoaded = true
	v.backends = []gcp.BackendService{{
		Name:     "api-backend",
		Backends: []gcp.Backend{{Group: "https://example/zones/z/instanceGroups/ig-1"}},
	}}
	v.groupHealth["https://example/zones/z/instanceGroups/ig-1"] = groupHealthState{phase: groupHealthLoading}
	out := v.renderBackends()
	assert.Contains(t, out, "loading")
}

func TestRenderBackendsSkippedBadge(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.backendsLoaded = true
	v.backends = []gcp.BackendService{{
		Name:     "static-backend",
		Backends: []gcp.Backend{{Group: "https://example/regions/r/networkEndpointGroups/cr-neg"}},
	}}
	v.groupHealth["https://example/regions/r/networkEndpointGroups/cr-neg"] = groupHealthState{
		phase:  groupHealthSkipped,
		reason: "serverless NEG",
	}
	out := v.renderBackends()
	assert.Contains(t, out, "no health")
	assert.Contains(t, out, "serverless NEG")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/views/ -run "TestRenderBackends.*Badge" -v`
Expected: all FAIL — current `renderBackends` outputs old format.

- [ ] **Step 3: Update `renderBackends`**

Replace the existing `renderBackends` (around line 413):

```go
func (v *LoadBalancerDetailsView) renderBackends() string {
	if !v.fetchState.backendsLoaded {
		return "Loading backends..."
	}
	if len(v.backends) == 0 {
		return "(no backends)"
	}
	var b strings.Builder
	for i := range v.backends {
		bs := &v.backends[i]
		b.WriteString(fmt.Sprintf("Backend service: %s\n", bs.Name))
		b.WriteString(fmt.Sprintf("  Protocol: %s  Timeout: %ds  Affinity: %s\n", bs.Protocol, bs.TimeoutSec, bs.SessionAffinity))
		for _, be := range bs.Backends {
			b.WriteString(fmt.Sprintf("    Group: %s  %s\n", shortNameURL(be.Group), v.renderHealthBadge(be.Group)))
			if v.groupExpanded[be.Group] {
				b.WriteString(v.renderHealthExpansion(be.Group))
			}
		}
		for _, hcURL := range bs.HealthChecks {
			b.WriteString(fmt.Sprintf("    Health check: %s\n", shortNameURL(hcURL)))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderHealthBadge returns the inline summary for one backend group.
// Empty string when no health is known yet (e.g. before fan-out fires).
func (v *LoadBalancerDetailsView) renderHealthBadge(groupURL string) string {
	st, ok := v.groupHealth[groupURL]
	if !ok {
		return ""
	}
	switch st.phase {
	case groupHealthLoading:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Render("◌◌ loading…")
	case groupHealthErrored:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Render(fmt.Sprintf("? error: %v", st.err))
	case groupHealthSkipped:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Render(fmt.Sprintf("(no health — %s)", st.reason))
	case groupHealthOK:
		return v.renderHealthSummary(st.statuses)
	}
	return ""
}

// renderHealthSummary draws the green/red dot row and counts. Up to 5
// per-instance dots inline; beyond that, abbreviate to "N/M healthy".
func (v *LoadBalancerDetailsView) renderHealthSummary(statuses []gcp.InstanceHealth) string {
	total := len(statuses)
	if total == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Render("(no members)")
	}
	healthy := 0
	for _, s := range statuses {
		if s.HealthState == "HEALTHY" {
			healthy++
		}
	}
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))

	if total > 5 {
		return fmt.Sprintf("%s %d/%d healthy", green.Render("●"), healthy, total)
	}
	var dots strings.Builder
	for _, s := range statuses {
		if s.HealthState == "HEALTHY" {
			dots.WriteString(green.Render("●"))
		} else {
			dots.WriteString(red.Render("○"))
		}
	}
	return fmt.Sprintf("%s %d/%d healthy", dots.String(), healthy, total)
}

// renderHealthExpansion draws the per-instance table for an expanded group.
// Used by Task 15. Stubbed empty for Task 13.
func (v *LoadBalancerDetailsView) renderHealthExpansion(groupURL string) string {
	return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ui/views/ -run "TestRenderBackends.*Badge" -v`
Expected: all PASS.

- [ ] **Step 5: Build full project**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_details_test.go
git commit -m "2026-05-13: LB phase 2 — health badge rendering on Backends tab"
```

---

### Task 14: Group focus + j/k navigation

**Files:**
- Modify: `internal/ui/views/loadbalancer_details.go`
- Modify: `internal/ui/views/loadbalancer_details_test.go`

The Backends tab gains a "group focus" mode: while active on the Backends tab and the content area is focused (not the tab strip), `j/k` step through the flat list of group rows, and an indicator (`▸`) prefixes the focused row.

- [ ] **Step 1: Add a helper that flattens groups into a stable order**

Append to `loadbalancer_details.go`:

```go
// flatGroupURLs returns every Backend.Group URL across every backend
// service in display order. Used to map a groupFocus index to a URL.
func (v *LoadBalancerDetailsView) flatGroupURLs() []string {
	urls := make([]string, 0)
	seen := map[string]struct{}{}
	for i := range v.backends {
		for _, be := range v.backends[i].Backends {
			if be.Group == "" {
				continue
			}
			if _, ok := seen[be.Group]; ok {
				continue
			}
			seen[be.Group] = struct{}{}
			urls = append(urls, be.Group)
		}
	}
	return urls
}
```

- [ ] **Step 2: Add j/k key handling on the Backends tab**

In `Update()`, add a tab-aware key handler. Locate the existing key handling block:

```go
case tea.KeyMsg:
    if v.showDeleteConfirm {
        return v.handleConfirmKey(m)
    }
    switch {
    case key.Matches(m, v.keys.Delete):
        ...
    case key.Matches(m, v.keys.Refresh):
        return v.Init()
    }
    return v.tabs.Update(m)
```

Add a Backends-tab-specific handler before falling through to `v.tabs.Update(m)`:

```go
case tea.KeyMsg:
    if v.showDeleteConfirm {
        return v.handleConfirmKey(m)
    }
    switch {
    case key.Matches(m, v.keys.Delete):
        if v.rule == nil || !v.fetchState.sharingChecksLoaded {
            return nil
        }
        c := ComputeCascade(*v.rule, v.allFwdRules, v.allProxies, v.allURLMaps, v.allBackends)
        v.cascade = &c
        v.showDeleteConfirm = true
        v.confirmInput = ""
        return nil
    case key.Matches(m, v.keys.Refresh):
        return v.Init()
    }
    if v.tabs.ActiveTab().ID == "backends" {
        if cmd, handled := v.handleBackendsKey(m); handled {
            return cmd
        }
    }
    return v.tabs.Update(m)
```

Append the new method:

```go
func (v *LoadBalancerDetailsView) handleBackendsKey(m tea.KeyMsg) (tea.Cmd, bool) {
	urls := v.flatGroupURLs()
	if len(urls) == 0 {
		return nil, false
	}
	switch m.String() {
	case "j", "down":
		if v.groupFocus < 0 {
			v.groupFocus = 0
		} else if v.groupFocus < len(urls)-1 {
			v.groupFocus++
		}
		return nil, true
	case "k", "up":
		if v.groupFocus > 0 {
			v.groupFocus--
		}
		return nil, true
	case "esc":
		if v.groupFocus >= 0 {
			v.groupFocus = -1
			return nil, true
		}
	}
	return nil, false
}
```

- [ ] **Step 3: Show the cursor (`▸`) on the focused row**

Modify the `renderBackends` group line. Replace:

```go
b.WriteString(fmt.Sprintf("    Group: %s  %s\n", shortNameURL(be.Group), v.renderHealthBadge(be.Group)))
```

with:

```go
cursor := "   "
focusedURL := ""
urls := v.flatGroupURLs()
if v.groupFocus >= 0 && v.groupFocus < len(urls) {
    focusedURL = urls[v.groupFocus]
}
if be.Group == focusedURL {
    cursor = " ▸ "
}
b.WriteString(fmt.Sprintf("  %sGroup: %s  %s\n", cursor, shortNameURL(be.Group), v.renderHealthBadge(be.Group)))
```

Wait — `flatGroupURLs()` is being called inside a loop body for every backend. Hoist it up.

Replace the outer loop preamble:

```go
for i := range v.backends {
    bs := &v.backends[i]
    b.WriteString(...)
```

with:

```go
urls := v.flatGroupURLs()
focusedURL := ""
if v.groupFocus >= 0 && v.groupFocus < len(urls) {
    focusedURL = urls[v.groupFocus]
}
for i := range v.backends {
    bs := &v.backends[i]
    b.WriteString(...)
```

Then inside the inner loop just use the captured `focusedURL`:

```go
for _, be := range bs.Backends {
    cursor := "   "
    if be.Group == focusedURL {
        cursor = " ▸ "
    }
    b.WriteString(fmt.Sprintf("  %sGroup: %s  %s\n", cursor, shortNameURL(be.Group), v.renderHealthBadge(be.Group)))
    if v.groupExpanded[be.Group] {
        b.WriteString(v.renderHealthExpansion(be.Group))
    }
}
```

- [ ] **Step 4: Write tests for navigation**

Append to `loadbalancer_details_test.go`:

```go
func TestGroupFocusNavigation(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.backendsLoaded = true
	v.backends = []gcp.BackendService{{
		Name: "api-backend",
		Backends: []gcp.Backend{
			{Group: "https://example/zones/z/instanceGroups/g-1"},
			{Group: "https://example/zones/z/instanceGroups/g-2"},
			{Group: "https://example/zones/z/instanceGroups/g-3"},
		},
	}}
	v.tabs.SetActiveTabByID("backends")

	// First 'j' moves from -1 to 0.
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 0, v.groupFocus)

	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, v.groupFocus)

	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // clamps at 2
	assert.Equal(t, 2, v.groupFocus)

	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, v.groupFocus)

	v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, -1, v.groupFocus)
}

func TestRenderBackendsShowsCursor(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.backendsLoaded = true
	v.backends = []gcp.BackendService{{
		Name: "api-backend",
		Backends: []gcp.Backend{
			{Group: "https://example/zones/z/instanceGroups/g-1"},
			{Group: "https://example/zones/z/instanceGroups/g-2"},
		},
	}}
	v.groupFocus = 1
	out := v.renderBackends()
	// The focused row gets the cursor glyph.
	assert.Contains(t, out, "▸Group: g-2")
}
```

Verify `tabs.SetActiveTabByID` exists. If not, check the `tabs` package for an equivalent (e.g. `SetActive(int)` indexed). Adjust the test to use whatever the public API is.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/ui/views/ -run "TestGroupFocusNavigation|TestRenderBackendsShowsCursor" -v`
Expected: both PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_details_test.go
git commit -m "2026-05-13: LB phase 2 — group focus navigation"
```

---

### Task 15: Per-instance expansion (Tab/Enter)

**Files:**
- Modify: `internal/ui/views/loadbalancer_details.go`
- Modify: `internal/ui/views/loadbalancer_details_test.go`

- [ ] **Step 1: Write the failing test**

Append to `loadbalancer_details_test.go`:

```go
func TestExpansionToggle(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.backendsLoaded = true
	v.backends = []gcp.BackendService{{
		Name:     "api-backend",
		Backends: []gcp.Backend{{Group: "https://example/zones/z/instanceGroups/g-1"}},
	}}
	v.tabs.SetActiveTabByID("backends")
	v.groupFocus = 0

	// Enter expands.
	v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, v.groupExpanded["https://example/zones/z/instanceGroups/g-1"])

	// Enter collapses.
	v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, v.groupExpanded["https://example/zones/z/instanceGroups/g-1"])
}

func TestRenderHealthExpansionDrawsTable(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.backendsLoaded = true
	v.backends = []gcp.BackendService{{
		Name:     "api-backend",
		Backends: []gcp.Backend{{Group: "https://example/zones/z/instanceGroups/g-1"}},
	}}
	v.groupHealth["https://example/zones/z/instanceGroups/g-1"] = groupHealthState{
		phase: groupHealthOK,
		statuses: []gcp.InstanceHealth{
			{Instance: "vm-1", HealthState: "HEALTHY", IPAddress: "10.0.0.5"},
			{Instance: "vm-2", HealthState: "UNHEALTHY", IPAddress: "10.0.0.6", FailureReason: "HTTP 503"},
		},
	}
	v.groupExpanded["https://example/zones/z/instanceGroups/g-1"] = true
	out := v.renderBackends()
	assert.Contains(t, out, "vm-1")
	assert.Contains(t, out, "HEALTHY")
	assert.Contains(t, out, "vm-2")
	assert.Contains(t, out, "UNHEALTHY")
	assert.Contains(t, out, "HTTP 503")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/views/ -run "TestExpansionToggle|TestRenderHealthExpansionDrawsTable" -v`
Expected: FAIL.

- [ ] **Step 3: Implement Enter handling in `handleBackendsKey`**

Update `handleBackendsKey` to add Enter/Tab toggling:

```go
func (v *LoadBalancerDetailsView) handleBackendsKey(m tea.KeyMsg) (tea.Cmd, bool) {
	urls := v.flatGroupURLs()
	if len(urls) == 0 {
		return nil, false
	}
	switch m.String() {
	case "j", "down":
		if v.groupFocus < 0 {
			v.groupFocus = 0
		} else if v.groupFocus < len(urls)-1 {
			v.groupFocus++
		}
		return nil, true
	case "k", "up":
		if v.groupFocus > 0 {
			v.groupFocus--
		}
		return nil, true
	case "enter", "tab":
		if v.groupFocus < 0 || v.groupFocus >= len(urls) {
			return nil, false
		}
		url := urls[v.groupFocus]
		v.groupExpanded[url] = !v.groupExpanded[url]
		return nil, true
	case "esc":
		if v.groupFocus >= 0 {
			v.groupFocus = -1
			return nil, true
		}
	}
	return nil, false
}
```

- [ ] **Step 4: Implement `renderHealthExpansion`**

Replace the stubbed `renderHealthExpansion` with:

```go
// renderHealthExpansion draws the per-instance health table for an
// expanded group. Each row shows the instance short name, IP:port,
// state, and (for unhealthy members) the failure reason.
func (v *LoadBalancerDetailsView) renderHealthExpansion(groupURL string) string {
	st, ok := v.groupHealth[groupURL]
	if !ok || st.phase != groupHealthOK {
		return ""
	}
	var b strings.Builder
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	gray := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	for _, s := range st.statuses {
		dot := gray.Render("○")
		state := s.HealthState
		switch s.HealthState {
		case "HEALTHY":
			dot = green.Render("●")
		case "UNHEALTHY":
			dot = red.Render("●")
		case "DRAINING":
			dot = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04")).Render("●")
		}
		addr := s.Instance
		if s.IPAddress != "" {
			if s.Port > 0 {
				addr = fmt.Sprintf("%s (%s:%d)", s.Instance, s.IPAddress, s.Port)
			} else {
				addr = fmt.Sprintf("%s (%s)", s.Instance, s.IPAddress)
			}
		}
		reason := s.FailureReason
		if reason == "" {
			b.WriteString(fmt.Sprintf("        %s %-30s %s\n", dot, addr, state))
		} else {
			b.WriteString(fmt.Sprintf("        %s %-30s %s — %s\n", dot, addr, state, reason))
		}
	}
	return b.String()
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/ui/views/ -run "TestExpansionToggle|TestRenderHealthExpansionDrawsTable" -v`
Expected: both PASS.

- [ ] **Step 6: Run full view-test suite + lint**

Run: `go test ./internal/ui/views/ && make lint`
Expected: all pass, no new lint warnings.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_details_test.go
git commit -m "2026-05-13: LB phase 2 — group expansion + per-instance table"
```

---

## Phase D — Observability sub-view

### Task 16: Create `loadbalancer_observability.go` skeleton

**Files:**
- Create: `internal/ui/views/loadbalancer_observability.go`
- Create: `internal/ui/views/loadbalancer_observability_test.go`

- [ ] **Step 1: Create the file with the struct, chart setup, and a placeholder View**

Create `internal/ui/views/loadbalancer_observability.go`:

```go
package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/metricchart"
)

// Internal messages.
type lbMetricsLoadedMsg struct {
	metrics  *gcp.LBMetrics
	warnings []string // non-fatal per-metric fetch errors
}
type lbMetricsErrorMsg struct{ err error }
type lbObsTickMsg struct{}

// loadBalancerObservability owns the Observability tab's state for a
// single LB: metrics, range selection, auto-refresh, and chart instances.
type loadBalancerObservability struct {
	projectID          string
	forwardingRuleName string
	gcpClient          *gcp.Client

	// metrics
	metrics         *gcp.LBMetrics
	metricsLoading  bool
	metricsError    error
	metricsWarnings []string

	// view state
	timeRange   time.Duration
	autoRefresh bool
	tabActive   bool

	spinner spinner.Model
	width   int

	// charts
	requestCountChart  *metricchart.Chart
	latencyChart       *metricchart.Chart // p50/p95/p99
	errorRateChart     *metricchart.Chart // 4xx/5xx percent
	backendLatChart    *metricchart.Chart
	throughputChart    *metricchart.Chart // in/out
}

func newLoadBalancerObservability(projectID, forwardingRuleName string, gcpClient *gcp.Client) *loadBalancerObservability {
	req := metricchart.New(metricchart.HeightStandard)
	req.SetStatsFormatter(metricchart.FormatCountStats)

	lat := metricchart.New(metricchart.HeightStandard)
	lat.SetStatsFormatter(nil).SetYLabelFormatter(metricchart.LatencyYLabel)

	errc := metricchart.New(metricchart.HeightCompact)
	errc.SetStatsFormatter(nil).SetYLabelFormatter(metricchart.PercentYLabel)
	errc.SetYRange(0, 10)

	be := metricchart.New(metricchart.HeightStandard)
	be.SetStatsFormatter(nil).SetYLabelFormatter(metricchart.LatencyYLabel)

	thr := metricchart.New(metricchart.HeightCompact)
	thr.SetStatsFormatter(nil)

	return &loadBalancerObservability{
		projectID:          projectID,
		forwardingRuleName: forwardingRuleName,
		gcpClient:          gcpClient,
		timeRange:          24 * time.Hour,
		autoRefresh:        true,
		spinner:            components.NewGCPSpinner(),
		requestCountChart:  req,
		latencyChart:       lat,
		errorRateChart:     errc,
		backendLatChart:    be,
		throughputChart:    thr,
	}
}

// Init triggers the first metrics fetch.
func (o *loadBalancerObservability) Init() tea.Cmd {
	o.metricsLoading = true
	o.metricsError = nil
	return tea.Batch(o.spinner.Tick, o.fetchAllMetrics())
}

// View renders the tab. Filled out in later tasks.
func (o *loadBalancerObservability) View() string {
	var b strings.Builder
	titleStyle := lipgloss.NewStyle().Bold(true)
	b.WriteString(titleStyle.Render("Observability"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", max(0, min(o.width-4, 60))))
	b.WriteString("\n\n")
	if o.metricsLoading {
		fmt.Fprintf(&b, "  %s Loading metrics...\n", o.spinner.View())
		return b.String()
	}
	if o.metricsError != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		b.WriteString(errStyle.Render(fmt.Sprintf("  Error loading metrics: %v", o.metricsError)))
		b.WriteString("\n  Press 'r' to retry\n")
		return b.String()
	}
	if o.metrics == nil {
		b.WriteString("  (no metric data)\n")
		return b.String()
	}
	// Charts wired in later tasks; for now emit a placeholder so the
	// tab renders something useful in the skeleton commit.
	b.WriteString("  (charts pending)\n")
	return b.String()
}

// resizeCharts propagates width changes to every chart.
func (o *loadBalancerObservability) resizeCharts() {
	w := o.width - 2 // small indent
	if w < 10 {
		w = 10
	}
	o.requestCountChart.Resize(w)
	o.latencyChart.Resize(w)
	o.errorRateChart.Resize(w)
	o.backendLatChart.Resize(w)
	o.throughputChart.Resize(w)
}

// fetchAllMetrics is the parallel fetch. Implemented in Task 18.
func (o *loadBalancerObservability) fetchAllMetrics() tea.Cmd {
	return func() tea.Msg {
		// Stub until Task 18 wires it.
		return lbMetricsLoadedMsg{metrics: &gcp.LBMetrics{LastFetch: time.Now()}}
	}
}

// StartAutoRefresh marks the tab active. Filled out in Task 24.
func (o *loadBalancerObservability) StartAutoRefresh() tea.Cmd {
	o.tabActive = true
	return nil
}

// StopAutoRefresh marks the tab inactive.
func (o *loadBalancerObservability) StopAutoRefresh() {
	o.tabActive = false
}

// Update routes messages.
func (o *loadBalancerObservability) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case lbMetricsLoadedMsg:
		o.metrics = m.metrics
		o.metricsWarnings = m.warnings
		o.metricsLoading = false
		o.metricsError = nil
		return nil
	case lbMetricsErrorMsg:
		o.metricsError = m.err
		o.metricsLoading = false
		return nil
	case spinner.TickMsg:
		if o.metricsLoading {
			var cmd tea.Cmd
			o.spinner, cmd = o.spinner.Update(m)
			return cmd
		}
	}
	return nil
}

// Compile-time use to avoid unused imports during the skeleton commit.
var _ = context.Background
```

(The `var _ = context.Background` line is removed in Task 18 when the real fetch uses context.)

- [ ] **Step 2: Create the skeleton test file**

Create `internal/ui/views/loadbalancer_observability_test.go`:

```go
package views

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewLoadBalancerObservabilityDefaults(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	assert.Equal(t, "rule-x", obs.forwardingRuleName)
	assert.Equal(t, 24*time.Hour, obs.timeRange)
	assert.True(t, obs.autoRefresh)
	assert.NotNil(t, obs.requestCountChart)
	assert.NotNil(t, obs.latencyChart)
	assert.NotNil(t, obs.errorRateChart)
	assert.NotNil(t, obs.backendLatChart)
	assert.NotNil(t, obs.throughputChart)
}

func TestLoadBalancerObservabilityViewLoading(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	obs.metricsLoading = true
	out := obs.View()
	assert.Contains(t, out, "Loading metrics")
}
```

- [ ] **Step 3: Build and run**

Run: `go build ./... && go test ./internal/ui/views/ -run TestLoadBalancerObservability -v`
Expected: build clean, both tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/views/loadbalancer_observability.go internal/ui/views/loadbalancer_observability_test.go
git commit -m "2026-05-13: LB phase 2 — observability sub-view skeleton"
```

---

### Task 17: Wire the Observability tab into the details view

Add the 4th tab; lazy-init the sub-view on first activation; route `View()`, `Update()`, and `SetSize()`.

**Files:**
- Modify: `internal/ui/views/loadbalancer_details.go`
- Modify: `internal/ui/views/loadbalancer_details_test.go`

- [ ] **Step 1: Add the tab entry and a field on the view**

In `NewLoadBalancerDetailsView`, append the 4th tab:

```go
t := tabs.New([]tabs.Tab{
    {ID: "overview", Label: "Overview"},
    {ID: "routing", Label: "Routing"},
    {ID: "backends", Label: "Backends"},
    {ID: "observability", Label: "Observability"},
})
```

Add a field to the struct:

```go
observability *loadBalancerObservability
```

- [ ] **Step 2: Lazy-init on `tabs.TabChangedMsg`**

In `Update()`, currently `v.tabs.Update(m)` returns a `tea.Cmd` from the tabs component, but the message is unwrapped inside `tabs`. Add a `tabs.TabChangedMsg` case before key handling. Search for `tabs.TabChangedMsg` to confirm — the cloud run service details view uses `case tabs.TabChangedMsg:` in `Update`. Add the same:

Add a new top-level case in the `switch m := msg.(type)` block, after `case lbErrorMsg`:

```go
case tabs.TabChangedMsg:
    if v.tabs.ActiveTab().ID == "observability" {
        if v.observability == nil {
            v.observability = newLoadBalancerObservability(v.projectID, v.name, v.gcpClient)
            v.observability.width = max(1, v.width-4)
            v.observability.resizeCharts()
        }
        if v.observability.metrics == nil || v.observability.metricsLoading {
            return tea.Batch(v.observability.Init(), v.observability.StartAutoRefresh())
        }
        return v.observability.StartAutoRefresh()
    }
    if v.observability != nil {
        v.observability.StopAutoRefresh()
    }
    return nil
```

- [ ] **Step 3: Route observability messages**

After `case lbGroupSkippedMsg:` add a passthrough for messages owned by the observability sub-view:

```go
case lbMetricsLoadedMsg, lbMetricsErrorMsg, lbObsTickMsg:
    if v.observability != nil {
        return v.observability.Update(m)
    }
    return nil
```

- [ ] **Step 4: Forward spinner ticks to the sub-view**

The existing `spinner.TickMsg` handler updates the view's own spinner. Add a second forward to the obs sub-view when it's loading. Locate:

```go
case spinner.TickMsg:
    if !v.fetchState.fwdLoaded {
        var cmd tea.Cmd
        v.spinner, cmd = v.spinner.Update(msg)
        return cmd
    }
    return nil
```

Replace with:

```go
case spinner.TickMsg:
    var cmds []tea.Cmd
    if !v.fetchState.fwdLoaded {
        var cmd tea.Cmd
        v.spinner, cmd = v.spinner.Update(msg)
        if cmd != nil {
            cmds = append(cmds, cmd)
        }
    }
    if v.observability != nil && v.observability.metricsLoading {
        if cmd := v.observability.Update(msg); cmd != nil {
            cmds = append(cmds, cmd)
        }
    }
    if len(cmds) == 0 {
        return nil
    }
    return tea.Batch(cmds...)
```

- [ ] **Step 5: Render the new tab body**

In `View()`, extend the switch:

```go
switch v.tabs.ActiveTab().ID {
case "overview":
    b.WriteString(v.renderOverview())
case "routing":
    b.WriteString(v.renderRouting())
case "backends":
    b.WriteString(v.renderBackends())
case "observability":
    b.WriteString(v.renderObservability())
}
```

Add the method:

```go
func (v *LoadBalancerDetailsView) renderObservability() string {
	if v.observability == nil {
		return "Loading observability..."
	}
	return v.observability.View()
}
```

- [ ] **Step 6: Propagate size**

In `SetSize`:

```go
func (v *LoadBalancerDetailsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.tabs.SetSize(width)
	if v.observability != nil {
		v.observability.width = max(1, width-4)
		v.observability.resizeCharts()
	}
}
```

- [ ] **Step 7: Write a tab-routing test**

Append to `loadbalancer_details_test.go`:

```go
func TestObservabilityTabLazyInit(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.tabs.SetActiveTabByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.observability)
	assert.True(t, v.observability.tabActive)
}

func TestObservabilityTabLeaveStopsRefresh(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.tabs.SetActiveTabByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.observability)

	v.tabs.SetActiveTabByID("overview")
	v.Update(tabs.TabChangedMsg{})
	assert.False(t, v.observability.tabActive)
}
```

Test file needs `"github.com/slayer/gcon/internal/ui/components/tabs"` imported.

- [ ] **Step 8: Run tests**

Run: `go test ./internal/ui/views/ -run "TestObservabilityTab" -v`
Expected: both PASS.

- [ ] **Step 9: Run full view-test suite**

Run: `go test ./internal/ui/views/`
Expected: all pass.

- [ ] **Step 10: Commit**

```bash
git add internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_details_test.go internal/ui/views/loadbalancer_observability.go
git commit -m "2026-05-13: LB phase 2 — wire Observability tab into details view"
```

---

### Task 18: Real `fetchAllMetrics` + request count chart

**Files:**
- Modify: `internal/ui/views/loadbalancer_observability.go`
- Modify: `internal/ui/views/loadbalancer_observability_test.go`

- [ ] **Step 1: Replace the stub `fetchAllMetrics`**

In `loadbalancer_observability.go`, replace the stub:

```go
func (o *loadBalancerObservability) fetchAllMetrics() tea.Cmd {
	return func() tea.Msg {
		// Stub until Task 18 wires it.
		return lbMetricsLoadedMsg{metrics: &gcp.LBMetrics{LastFetch: time.Now()}}
	}
}
```

with:

```go
func (o *loadBalancerObservability) fetchAllMetrics() tea.Cmd {
	if o.gcpClient == nil {
		return func() tea.Msg {
			return lbMetricsErrorMsg{err: fmt.Errorf("monitoring client unavailable")}
		}
	}
	rule := o.forwardingRuleName
	projectID := o.projectID
	duration := o.timeRange
	client := o.gcpClient
	return func() tea.Msg {
		mc, err := client.GetMonitoringClient(projectID)
		if err != nil {
			return lbMetricsErrorMsg{err: fmt.Errorf("init monitoring client: %w", err)}
		}
		ctx := context.Background()
		out := &gcp.LBMetrics{LastFetch: time.Now()}
		var warnings []string

		if rc, err := mc.GetLBRequestCount(ctx, rule, duration); err != nil {
			warnings = append(warnings, fmt.Sprintf("request count: %v", err))
		} else {
			out.RequestCount = rc
		}
		return lbMetricsLoadedMsg{metrics: out, warnings: warnings}
	}
}
```

Remove `var _ = context.Background` from the bottom of the file.

- [ ] **Step 2: Wire data into the request count chart on load**

In `Update()`, in the `lbMetricsLoadedMsg` handler, push data into the chart:

```go
case lbMetricsLoadedMsg:
    o.metrics = m.metrics
    o.metricsWarnings = m.warnings
    o.metricsLoading = false
    o.metricsError = nil
    if o.metrics != nil {
        o.requestCountChart.SetData(o.metrics.RequestCount)
    }
    return nil
```

- [ ] **Step 3: Render the request count chart in `View()`**

Replace the `"  (charts pending)\n"` placeholder block with:

```go
if o.metrics == nil {
    b.WriteString("  (no metric data)\n")
    return b.String()
}
o.renderTimeRangeSelector(&b)
b.WriteString("\n")
o.renderRequestCount(&b)
return b.String()
```

Add the helpers:

```go
func (o *loadBalancerObservability) renderTimeRangeSelector(b *strings.Builder) {
	options := []struct {
		label string
		d     time.Duration
	}{
		{"1h", 1 * time.Hour},
		{"6h", 6 * time.Hour},
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
	}
	active := lipgloss.NewStyle().Foreground(lipgloss.Color("#1A73E8")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString("  ")
	for _, opt := range options {
		if opt.d == o.timeRange {
			b.WriteString(active.Render("[" + opt.label + "]"))
		} else {
			b.WriteString(muted.Render(" " + opt.label + " "))
		}
		b.WriteString("  ")
	}
	state := "OFF"
	if o.autoRefresh {
		state = "ON"
	}
	b.WriteString(muted.Render(fmt.Sprintf("    auto-refresh %s    r refresh", state)))
	b.WriteString("\n")
}

func (o *loadBalancerObservability) renderRequestCount(b *strings.Builder) {
	section := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString(section.Render("Request Count"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if o.metrics != nil && len(o.metrics.RequestCount) > 0 {
		b.WriteString(o.requestCountChart.View())
	} else {
		b.WriteString(muted.Render("  No request data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}
```

- [ ] **Step 4: Test the data wiring**

Append to `loadbalancer_observability_test.go`:

```go
func TestRequestCountChartWiringOnLoad(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	now := time.Now()
	obs.Update(lbMetricsLoadedMsg{
		metrics: &gcp.LBMetrics{
			RequestCount: []gcp.DataPoint{
				{Timestamp: now.Add(-2 * time.Minute), Value: 100},
				{Timestamp: now.Add(-1 * time.Minute), Value: 150},
			},
		},
	})
	out := obs.View()
	assert.Contains(t, out, "Request Count")
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/ui/views/ -run "TestRequestCountChartWiringOnLoad|TestLoadBalancerObservability" -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/loadbalancer_observability.go internal/ui/views/loadbalancer_observability_test.go
git commit -m "2026-05-13: LB phase 2 — fetchAllMetrics + request count chart"
```

---

### Task 19: Latency chart (p50/p95/p99 overlay)

**Files:**
- Modify: `internal/ui/views/loadbalancer_observability.go`
- Modify: `internal/ui/views/loadbalancer_observability_test.go`

- [ ] **Step 1: Extend `fetchAllMetrics` to fetch latencies**

In the fetch goroutine, after the request-count block, add:

```go
if p50, p95, p99, err := mc.GetLBRequestLatencies(ctx, rule, duration); err != nil {
    warnings = append(warnings, fmt.Sprintf("request latencies: %v", err))
} else {
    out.Latency50, out.Latency95, out.Latency99 = p50, p95, p99
}
```

- [ ] **Step 2: Wire data into the latency chart on load**

In `Update()` extend the `lbMetricsLoadedMsg` handler:

```go
case lbMetricsLoadedMsg:
    o.metrics = m.metrics
    o.metricsWarnings = m.warnings
    o.metricsLoading = false
    o.metricsError = nil
    if o.metrics != nil {
        o.requestCountChart.SetData(o.metrics.RequestCount)
        o.latencyChart.SetDataSets([]metricchart.DataSet{
            {Name: "p50", Data: o.metrics.Latency50, Color: "#34A853"},
            {Name: "p95", Data: o.metrics.Latency95, Color: "#FBBC04"},
            {Name: "p99", Data: o.metrics.Latency99, Color: "#EA4335"},
        })
    }
    return nil
```

- [ ] **Step 3: Render the latency chart**

After `o.renderRequestCount(&b)` in `View()`, add `o.renderLatency(&b)`.

Add the method:

```go
func (o *loadBalancerObservability) renderLatency(b *strings.Builder) {
	section := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString(section.Render("Request Latency (p50 / p95 / p99)"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if o.metrics != nil && (len(o.metrics.Latency50) > 0 || len(o.metrics.Latency95) > 0 || len(o.metrics.Latency99) > 0) {
		b.WriteString(o.latencyChart.View())
	} else {
		b.WriteString(muted.Render("  No latency data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}
```

- [ ] **Step 4: Test wiring**

Append:

```go
func TestLatencyChartWiringOnLoad(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	now := time.Now()
	obs.Update(lbMetricsLoadedMsg{
		metrics: &gcp.LBMetrics{
			Latency50: []gcp.DataPoint{{Timestamp: now, Value: 100}},
			Latency95: []gcp.DataPoint{{Timestamp: now, Value: 200}},
			Latency99: []gcp.DataPoint{{Timestamp: now, Value: 500}},
		},
	})
	out := obs.View()
	assert.Contains(t, out, "Request Latency")
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/ui/views/ -run TestLatency -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/loadbalancer_observability.go internal/ui/views/loadbalancer_observability_test.go
git commit -m "2026-05-13: LB phase 2 — latency chart (p50/p95/p99 overlay)"
```

---

### Task 20: Error rate chart (4xx/5xx, computed as percent)

**Files:**
- Modify: `internal/ui/views/loadbalancer_observability.go`
- Modify: `internal/ui/views/loadbalancer_observability_test.go`

Error rate is `RequestCount4xx / RequestCount` and `RequestCount5xx / RequestCount`, point-by-point. Where `RequestCount` is zero, error rate is zero (not NaN).

- [ ] **Step 1: Add the division helper**

Append to `loadbalancer_observability.go`:

```go
// percentRate divides `errs[i]` by `total[i]` for each timestamp where the
// timestamps match (within 1 second tolerance). Result is a percentage
// 0–100. When total is zero, the percentage is zero. Mismatched
// timestamps fall through with zero.
func percentRate(errs, total []gcp.DataPoint) []gcp.DataPoint {
	if len(errs) == 0 || len(total) == 0 {
		return nil
	}
	out := make([]gcp.DataPoint, 0, len(total))
	j := 0
	for i := range total {
		// Advance the errs cursor to the matching (or next-later) timestamp.
		for j < len(errs) && errs[j].Timestamp.Before(total[i].Timestamp.Add(-time.Second)) {
			j++
		}
		var pct float64
		if total[i].Value > 0 && j < len(errs) {
			if errs[j].Timestamp.Sub(total[i].Timestamp).Abs() <= time.Second {
				pct = (errs[j].Value / total[i].Value) * 100
			}
		}
		out = append(out, gcp.DataPoint{Timestamp: total[i].Timestamp, Value: pct})
	}
	return out
}
```

- [ ] **Step 2: Extend `fetchAllMetrics` for code-class series**

Add after the latencies block:

```go
if r4, err := mc.GetLBRequestCountByCodeClass(ctx, rule, "4xx", duration); err != nil {
    warnings = append(warnings, fmt.Sprintf("4xx count: %v", err))
} else {
    out.RequestCount4xx = r4
}
if r5, err := mc.GetLBRequestCountByCodeClass(ctx, rule, "5xx", duration); err != nil {
    warnings = append(warnings, fmt.Sprintf("5xx count: %v", err))
} else {
    out.RequestCount5xx = r5
}
```

- [ ] **Step 3: Wire data into the error chart**

In `Update()` extend the `lbMetricsLoadedMsg` handler to set error-rate series:

```go
err4xx := percentRate(o.metrics.RequestCount4xx, o.metrics.RequestCount)
err5xx := percentRate(o.metrics.RequestCount5xx, o.metrics.RequestCount)
o.errorRateChart.SetDataSets([]metricchart.DataSet{
    {Name: "4xx", Data: err4xx, Color: "#FBBC04"},
    {Name: "5xx", Data: err5xx, Color: "#EA4335"},
})
```

- [ ] **Step 4: Render the error rate section**

Add to `View()`:

```go
o.renderErrorRate(&b)
```

after the latency block. Add the method:

```go
func (o *loadBalancerObservability) renderErrorRate(b *strings.Builder) {
	section := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString(section.Render("Error Rate (4xx / 5xx)"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if o.metrics != nil && (len(o.metrics.RequestCount4xx) > 0 || len(o.metrics.RequestCount5xx) > 0) {
		b.WriteString(o.errorRateChart.View())
	} else {
		b.WriteString(muted.Render("  No error data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}
```

- [ ] **Step 5: Test `percentRate`**

Append:

```go
func TestPercentRateBasic(t *testing.T) {
	now := time.Now()
	total := []gcp.DataPoint{
		{Timestamp: now.Add(-2 * time.Minute), Value: 100},
		{Timestamp: now.Add(-1 * time.Minute), Value: 200},
	}
	errs := []gcp.DataPoint{
		{Timestamp: now.Add(-2 * time.Minute), Value: 5},
		{Timestamp: now.Add(-1 * time.Minute), Value: 10},
	}
	got := percentRate(errs, total)
	require.Len(t, got, 2)
	assert.InDelta(t, 5.0, got[0].Value, 0.001)
	assert.InDelta(t, 5.0, got[1].Value, 0.001)
}

func TestPercentRateZeroTotalIsZero(t *testing.T) {
	now := time.Now()
	total := []gcp.DataPoint{{Timestamp: now, Value: 0}}
	errs := []gcp.DataPoint{{Timestamp: now, Value: 5}}
	got := percentRate(errs, total)
	require.Len(t, got, 1)
	assert.Equal(t, 0.0, got[0].Value)
}

func TestPercentRateMissingErrs(t *testing.T) {
	now := time.Now()
	total := []gcp.DataPoint{{Timestamp: now, Value: 100}}
	got := percentRate(nil, total)
	assert.Nil(t, got)
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/ui/views/ -run "TestPercentRate|TestErrorRate" -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/views/loadbalancer_observability.go internal/ui/views/loadbalancer_observability_test.go
git commit -m "2026-05-13: LB phase 2 — error rate chart (4xx/5xx percent)"
```

---

### Task 21: Backend latency chart

**Files:**
- Modify: `internal/ui/views/loadbalancer_observability.go`

- [ ] **Step 1: Extend `fetchAllMetrics`**

Add after the error code class block:

```go
if p50, p95, p99, err := mc.GetLBBackendLatencies(ctx, rule, duration); err != nil {
    warnings = append(warnings, fmt.Sprintf("backend latencies: %v", err))
} else {
    out.BackendLat50, out.BackendLat95, out.BackendLat99 = p50, p95, p99
}
```

- [ ] **Step 2: Wire data and render**

In the `lbMetricsLoadedMsg` handler:

```go
o.backendLatChart.SetDataSets([]metricchart.DataSet{
    {Name: "p50", Data: o.metrics.BackendLat50, Color: "#34A853"},
    {Name: "p95", Data: o.metrics.BackendLat95, Color: "#FBBC04"},
    {Name: "p99", Data: o.metrics.BackendLat99, Color: "#EA4335"},
})
```

Add `o.renderBackendLatency(&b)` to `View()` after the error rate block:

```go
func (o *loadBalancerObservability) renderBackendLatency(b *strings.Builder) {
	section := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString(section.Render("Backend Latency (origin response time)"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if o.metrics != nil && (len(o.metrics.BackendLat50) > 0 || len(o.metrics.BackendLat95) > 0 || len(o.metrics.BackendLat99) > 0) {
		b.WriteString(o.backendLatChart.View())
	} else {
		b.WriteString(muted.Render("  No backend latency data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}
```

- [ ] **Step 3: Quick smoke test**

Append:

```go
func TestBackendLatencyChartWiring(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	now := time.Now()
	obs.Update(lbMetricsLoadedMsg{
		metrics: &gcp.LBMetrics{
			BackendLat50: []gcp.DataPoint{{Timestamp: now, Value: 30}},
		},
	})
	out := obs.View()
	assert.Contains(t, out, "Backend Latency")
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/views/ -run TestBackendLatencyChartWiring -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/views/loadbalancer_observability.go internal/ui/views/loadbalancer_observability_test.go
git commit -m "2026-05-13: LB phase 2 — backend latency chart"
```

---

### Task 22: Throughput chart (request bytes / response bytes)

**Files:**
- Modify: `internal/ui/views/loadbalancer_observability.go`

- [ ] **Step 1: Extend `fetchAllMetrics`**

Add after the backend latencies block:

```go
if rb, err := mc.GetLBRequestBytes(ctx, rule, duration); err != nil {
    warnings = append(warnings, fmt.Sprintf("request bytes: %v", err))
} else {
    out.RequestBytes = rb
}
if rb, err := mc.GetLBResponseBytes(ctx, rule, duration); err != nil {
    warnings = append(warnings, fmt.Sprintf("response bytes: %v", err))
} else {
    out.ResponseBytes = rb
}
```

- [ ] **Step 2: Wire data and render**

In `lbMetricsLoadedMsg`:

```go
o.throughputChart.SetDataSets([]metricchart.DataSet{
    {Name: "in", Data: o.metrics.RequestBytes, Color: "#1A73E8"},
    {Name: "out", Data: o.metrics.ResponseBytes, Color: "#9334E6"},
})
```

Add `o.renderThroughput(&b)` to `View()`:

```go
func (o *loadBalancerObservability) renderThroughput(b *strings.Builder) {
	section := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	b.WriteString(section.Render("Throughput (bytes in / out)"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("━", max(0, min(o.width-4, 60))))
	b.WriteString("\n")
	if o.metrics != nil && (len(o.metrics.RequestBytes) > 0 || len(o.metrics.ResponseBytes) > 0) {
		b.WriteString(o.throughputChart.View())
	} else {
		b.WriteString(muted.Render("  No throughput data available"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}
```

- [ ] **Step 3: Smoke test**

Append:

```go
func TestThroughputChartWiring(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	now := time.Now()
	obs.Update(lbMetricsLoadedMsg{
		metrics: &gcp.LBMetrics{
			RequestBytes:  []gcp.DataPoint{{Timestamp: now, Value: 1024}},
			ResponseBytes: []gcp.DataPoint{{Timestamp: now, Value: 8192}},
		},
	})
	out := obs.View()
	assert.Contains(t, out, "Throughput")
}
```

- [ ] **Step 4: Run tests + lint**

Run: `go test ./internal/ui/views/ -run "Throughput" -v && make lint`
Expected: PASS, lint clean.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/views/loadbalancer_observability.go internal/ui/views/loadbalancer_observability_test.go
git commit -m "2026-05-13: LB phase 2 — throughput chart"
```

---

### Task 23: Time range selector (`1`–`5`)

**Files:**
- Modify: `internal/ui/views/loadbalancer_observability.go`
- Modify: `internal/ui/views/loadbalancer_details.go`
- Modify: `internal/ui/views/loadbalancer_observability_test.go`

- [ ] **Step 1: Add `handleKey` to the obs sub-view**

Append to `loadbalancer_observability.go`:

```go
// handleKey processes a tea.KeyMsg when the Observability tab content is
// focused. Returns (cmd, true) when it handled the key; (nil, false)
// otherwise — the caller falls through to the tab-strip / view-level
// handlers.
func (o *loadBalancerObservability) handleKey(m tea.KeyMsg) (tea.Cmd, bool) {
	switch m.String() {
	case "1":
		return o.setRange(1 * time.Hour), true
	case "2":
		return o.setRange(6 * time.Hour), true
	case "3":
		return o.setRange(24 * time.Hour), true
	case "4":
		return o.setRange(7 * 24 * time.Hour), true
	case "5":
		return o.setRange(30 * 24 * time.Hour), true
	case "a":
		o.autoRefresh = !o.autoRefresh
		if o.autoRefresh && o.tabActive {
			return o.tickAutoRefresh(), true
		}
		return nil, true
	case "r":
		o.metricsLoading = true
		o.metricsError = nil
		return tea.Batch(o.spinner.Tick, o.fetchAllMetrics()), true
	}
	return nil, false
}

func (o *loadBalancerObservability) setRange(d time.Duration) tea.Cmd {
	if d == o.timeRange {
		return nil
	}
	o.timeRange = d
	o.metricsLoading = true
	return tea.Batch(o.spinner.Tick, o.fetchAllMetrics())
}

// tickAutoRefresh is implemented in Task 24 — stub here.
func (o *loadBalancerObservability) tickAutoRefresh() tea.Cmd { return nil }
```

- [ ] **Step 2: Route Observability tab keys from the details view**

In `LoadBalancerDetailsView.Update()`, before the existing tab-routing fallback, add:

```go
if v.tabs.ActiveTab().ID == "observability" && v.observability != nil {
    if cmd, handled := v.observability.handleKey(m); handled {
        return cmd
    }
}
```

Place this just before:

```go
if v.tabs.ActiveTab().ID == "backends" {
    if cmd, handled := v.handleBackendsKey(m); handled {
        return cmd
    }
}
```

- [ ] **Step 3: Test**

Append to `loadbalancer_observability_test.go`:

```go
func TestObservabilityTimeRangeKeys(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	cases := []struct {
		key  string
		want time.Duration
	}{
		{"1", 1 * time.Hour},
		{"2", 6 * time.Hour},
		{"3", 24 * time.Hour},
		{"4", 7 * 24 * time.Hour},
		{"5", 30 * 24 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			_, handled := obs.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
			assert.True(t, handled)
			assert.Equal(t, tc.want, obs.timeRange)
		})
	}
}

func TestObservabilityAutoRefreshToggle(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	assert.True(t, obs.autoRefresh) // default on
	obs.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.False(t, obs.autoRefresh)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/views/ -run "TestObservabilityTimeRange|TestObservabilityAutoRefreshToggle" -v`
Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/views/loadbalancer_observability.go internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_observability_test.go
git commit -m "2026-05-13: LB phase 2 — time range selector + auto-refresh toggle"
```

---

### Task 24: Auto-refresh tick with stale-tick guard

**Files:**
- Modify: `internal/ui/views/loadbalancer_observability.go`
- Modify: `internal/ui/views/loadbalancer_observability_test.go`

Replace the stub `tickAutoRefresh` from Task 23 with a real implementation, plus the message handler.

- [ ] **Step 1: Implement `tickAutoRefresh` and `lbObsTickMsg` handler**

Replace:

```go
func (o *loadBalancerObservability) tickAutoRefresh() tea.Cmd { return nil }
```

with:

```go
// tickAutoRefresh schedules a single auto-refresh tick. Per the
// stale-tick discipline documented in `.claude/rules/bubble-tea-rendering.md`,
// we guard against ticks delivered after the user navigates away.
func (o *loadBalancerObservability) tickAutoRefresh() tea.Cmd {
	if !o.autoRefresh || !o.tabActive {
		return nil
	}
	return tea.Tick(30*time.Second, func(_ time.Time) tea.Msg {
		return lbObsTickMsg{}
	})
}
```

Update `Update()` with the tick case:

```go
case lbObsTickMsg:
    if !o.autoRefresh || !o.tabActive {
        return nil
    }
    return tea.Batch(o.fetchAllMetrics(), o.tickAutoRefresh())
```

Update `StartAutoRefresh` to actually schedule the first tick:

```go
func (o *loadBalancerObservability) StartAutoRefresh() tea.Cmd {
	o.tabActive = true
	if !o.autoRefresh {
		return nil
	}
	return o.tickAutoRefresh()
}
```

- [ ] **Step 2: Test the stale-tick guard**

Append:

```go
func TestStaleTickIsDropped(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	obs.tabActive = false // simulate user navigated away
	obs.autoRefresh = true
	cmd := obs.Update(lbObsTickMsg{})
	assert.Nil(t, cmd, "tick must produce no command when tab inactive")
}

func TestTickWhenInactiveProducesNoCmd(t *testing.T) {
	obs := newLoadBalancerObservability("p", "rule-x", nil)
	obs.autoRefresh = false
	obs.tabActive = true
	cmd := obs.tickAutoRefresh()
	assert.Nil(t, cmd)
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/ui/views/ -run "TestStaleTickIsDropped|TestTickWhenInactiveProducesNoCmd" -v`
Expected: both PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/views/loadbalancer_observability.go internal/ui/views/loadbalancer_observability_test.go
git commit -m "2026-05-13: LB phase 2 — auto-refresh ticker with stale-tick guard"
```

---

### Task 25: LB-type gating + placeholder

**Files:**
- Modify: `internal/ui/views/loadbalancer_details.go`
- Modify: `internal/ui/views/loadbalancer_details_test.go`

Only HTTP/HTTPS forwarding rules show charts; everything else gets a placeholder body.

- [ ] **Step 1: Add a helper that classifies the rule for observability**

Append to `loadbalancer_details.go`:

```go
// isHTTPSObservabilityCapable returns true when the forwarding rule is an
// HTTP / HTTPS / internal HTTPS LB, which are the types covered by the
// loadbalancing.googleapis.com/https/* metric family.
func isHTTPSObservabilityCapable(r *gcp.ForwardingRule) bool {
	if r == nil {
		return false
	}
	switch r.Type {
	case "HTTPS (external)", "HTTPS (internal)", "HTTP (external)", "HTTP (internal)":
		return true
	}
	return false
}
```

- [ ] **Step 2: Gate observability rendering**

Replace `renderObservability`:

```go
func (v *LoadBalancerDetailsView) renderObservability() string {
	if !isHTTPSObservabilityCapable(v.rule) {
		return v.renderObservabilityPlaceholder()
	}
	if v.observability == nil {
		return "Loading observability..."
	}
	return v.observability.View()
}

func (v *LoadBalancerDetailsView) renderObservabilityPlaceholder() string {
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	kind := "this LB type"
	if v.rule != nil && v.rule.Type != "" {
		kind = v.rule.Type
	}
	var b strings.Builder
	b.WriteString("Observability\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n\n")
	b.WriteString(muted.Render(fmt.Sprintf("  Metrics for %s are not yet supported in gcon.", kind)))
	b.WriteString("\n")
	b.WriteString(muted.Render("  The l3/* metric family for passthrough/proxy Network LBs is on the roadmap."))
	b.WriteString("\n\n")
	b.WriteString(muted.Render("  View metrics in the GCP console:"))
	b.WriteString("\n")
	b.WriteString(muted.Render("    https://console.cloud.google.com/net-services/loadbalancing"))
	b.WriteString("\n")
	return b.String()
}
```

- [ ] **Step 3: Skip lazy-init for unsupported types**

In the `tabs.TabChangedMsg` handler, wrap the obs-init in the type check:

```go
case tabs.TabChangedMsg:
    if v.tabs.ActiveTab().ID == "observability" {
        if !isHTTPSObservabilityCapable(v.rule) {
            return nil
        }
        if v.observability == nil {
            v.observability = newLoadBalancerObservability(v.projectID, v.name, v.gcpClient)
            v.observability.width = max(1, v.width-4)
            v.observability.resizeCharts()
        }
        if v.observability.metrics == nil || v.observability.metricsLoading {
            return tea.Batch(v.observability.Init(), v.observability.StartAutoRefresh())
        }
        return v.observability.StartAutoRefresh()
    }
    if v.observability != nil {
        v.observability.StopAutoRefresh()
    }
    return nil
```

- [ ] **Step 4: Test**

Append to `loadbalancer_details_test.go`:

```go
func TestObservabilityPlaceholderForNetworkLB(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.fwdLoaded = true
	v.rule = &gcp.ForwardingRule{Name: "front", Type: "Network LB (passthrough)"}
	v.tabs.SetActiveTabByID("observability")
	out := v.renderObservability()
	assert.Contains(t, out, "Network LB (passthrough) are not yet supported")
	// Sanity: the sub-view was never created.
	assert.Nil(t, v.observability)
}

func TestObservabilityChartsForHTTPSLB(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.fwdLoaded = true
	v.rule = &gcp.ForwardingRule{Name: "front", Type: "HTTPS (external)"}
	v.tabs.SetActiveTabByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.observability)
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/ui/views/ -run "TestObservabilityPlaceholderForNetworkLB|TestObservabilityChartsForHTTPSLB" -v`
Expected: both PASS.

- [ ] **Step 6: Run full view + GCP suites + lint**

Run: `go test ./internal/ui/views/ && go test ./internal/gcp/ && make lint`
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_details_test.go
git commit -m "2026-05-13: LB phase 2 — LB-type gating + Network LB placeholder"
```

---

## Phase E — Integration + docs

### Task 26: Tie backend-health refresh to the auto-refresh tick

The Observability tab's tick should also re-fire the health fan-out. Health is owned by the parent details view, not the obs sub-view, so the obs tab emits a "tick" the parent acts on too.

**Files:**
- Modify: `internal/ui/views/loadbalancer_observability.go`
- Modify: `internal/ui/views/loadbalancer_details.go`
- Modify: `internal/ui/views/loadbalancer_messages.go`
- Modify: `internal/ui/views/loadbalancer_details_test.go`

- [ ] **Step 1: Add a fan-out tick message**

Append to `loadbalancer_messages.go`:

```go
// lbHealthRefreshMsg is emitted on the auto-refresh tick to re-run the
// backend-health fan-out independently of the metrics refresh.
type lbHealthRefreshMsg struct{}
```

- [ ] **Step 2: Emit `lbHealthRefreshMsg` alongside the metrics fetch on tick**

In `loadbalancer_observability.go`, modify the `lbObsTickMsg` handler:

```go
case lbObsTickMsg:
    if !o.autoRefresh || !o.tabActive {
        return nil
    }
    return tea.Batch(
        o.fetchAllMetrics(),
        o.tickAutoRefresh(),
        func() tea.Msg { return lbHealthRefreshMsg{} },
    )
```

- [ ] **Step 3: Handle the message in the details view**

In `LoadBalancerDetailsView.Update()` add a case:

```go
case lbHealthRefreshMsg:
    return v.fetchAllBackendHealth()
```

- [ ] **Step 4: Test the wiring**

Append to `loadbalancer_details_test.go`:

```go
func TestHealthRefreshOnAutoRefresh(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.backendsLoaded = true
	v.backends = []gcp.BackendService{{
		Name:     "api-backend",
		Backends: []gcp.Backend{{Group: "https://example/zones/z/instanceGroups/g-1"}},
	}}
	// No client → fetchAllBackendHealth returns nil. Just verify the
	// handler returns without panicking and that the message is wired.
	cmd := v.Update(lbHealthRefreshMsg{})
	_ = cmd
	// Existing groupHealth shouldn't have been wiped (state is only
	// rewritten on Init or load).
	assert.NotNil(t, v.groupHealth)
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/ui/views/ -run TestHealthRefreshOnAutoRefresh -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/loadbalancer_observability.go internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_messages.go internal/ui/views/loadbalancer_details_test.go
git commit -m "2026-05-13: LB phase 2 — wire health refresh into auto-refresh tick"
```

---

### Task 27: Documentation

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `.claude/rules/key-bindings.md`

- [ ] **Step 1: Update `CLAUDE.md` implemented-features list**

In `CLAUDE.md`, find the Load Balancers (Phase 1) bullet:

```
- [x] Load Balancers (Phase 1)
```

Add immediately below it:

```
- [x] Load Balancers (Phase 2) — live backend health + Observability tab
  - Backend health: `backendServices.getHealth` per group, inline `● N/M healthy`
    badges on the Backends tab, expand with `Tab`/`Enter` for per-instance
    HEALTHY / UNHEALTHY / DRAINING state and IP:port.
  - Serverless NEGs (Cloud Run / Cloud Functions / App Engine) and Cloud
    Storage backends are auto-detected and skipped with a labeled placeholder.
  - Observability tab (HTTP / HTTPS / internal HTTPS only): request count,
    request latency (p50/p95/p99), error rate (4xx/5xx as a percentage of
    total requests), backend latency, and throughput (bytes in / out).
  - Time-range selector (1h/6h/24h/7d/30d), auto-refresh on a 30 s tick,
    manual `r` refresh.
  - Network LBs (passthrough / proxy / legacy) render an explicit
    placeholder on the Observability tab — `l3/*` metric family covered
    by a follow-up phase.
```

- [ ] **Step 2: Update `.claude/rules/key-bindings.md`**

In `.claude/rules/key-bindings.md`, find the "Load Balancer Details View" section. Replace it with:

```markdown
## Load Balancer Details View

| Key | Action |
|-----|--------|
| `D` | Delete LB (with dependency cascade, type-to-confirm) |
| `r` | Refresh details (cascades into health re-fetch and metrics re-fetch) |
| `Tab` | Switch focus (tabs / content) |
| `h/l` or `1/2/3/4` | Switch tabs (Overview / Routing / Backends / Observability) |
| `Esc` | Cancel delete dialog / Go back |

### Backends tab

| Key | Action |
|-----|--------|
| `j` / `↓` | Move group focus down |
| `k` / `↑` | Move group focus up |
| `Tab` / `Enter` | Toggle per-instance expansion on focused group |
| `Esc` | Exit group focus |

### Observability tab (HTTP/HTTPS LBs only)

| Key | Action |
|-----|--------|
| `1`–`5` | Time range 1h/6h/24h/7d/30d |
| `a` | Toggle auto-refresh (30 s, default on) |
| `r` | Manual refresh metrics + health |
```

- [ ] **Step 3: Update `README.md`**

In `README.md`, locate the feature list section that mentions Load Balancers (search for "Load Balancers (Phase 1)" or similar). Append the Phase 2 bullets in the same form as the existing list. (Use the same wording as Step 1.)

- [ ] **Step 4: Lint markdown if applicable**

Run: `make lint`
Expected: clean.

- [ ] **Step 5: Run full test suite as a final gate**

Run: `go test ./... && make lint`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add README.md CLAUDE.md .claude/rules/key-bindings.md
git commit -m "2026-05-13: LB phase 2 — documentation"
```

---

## Final verification

- [ ] **Run the full test suite once more**

Run: `go test ./...`
Expected: all pass.

- [ ] **Run lint**

Run: `make lint`
Expected: clean.

- [ ] **Smoke test in the running TUI**

Run: `make run`
Manual steps:
1. Navigate to Load balancing.
2. Pick an HTTP(S) LB. Verify:
   - Backends tab shows badges per group; navigate with `j/k`; expand with `Enter`.
   - Observability tab shows five charts within ~5 s.
   - `1`/`2`/`3`/`4`/`5` switches time range and refetches.
   - `a` toggles auto-refresh; the indicator updates.
3. Pick a Network LB (passthrough). Verify:
   - Backends tab still shows config and badges.
   - Observability tab shows the placeholder.

- [ ] **Push branch and open PR**

```bash
git push -u origin 2026-05-13-load-balancers-phase2
gh pr create --title "Load Balancers Phase 2 — live health + observability tab" --body "$(cat <<'EOF'
## Summary

- Adds inline backend health badges and per-instance expansion to the Backends tab.
- Adds an Observability tab (HTTP/HTTPS LBs) with request count, latency, error rate, backend latency, and throughput charts.
- Network LB / TCP-proxy / SSL-proxy LBs render an explicit placeholder; `l3/*` metric coverage is deferred.

## Test plan

- [ ] Backends tab badges render for an HTTP(S) LB and reflect the health state of each group.
- [ ] Serverless NEG (Cloud Run backend) shows the "(no health — …)" label without an API error.
- [ ] Observability tab populates all five charts for an HTTP(S) LB.
- [ ] Time range keys (1-5) refetch with the new duration.
- [ ] Auto-refresh toggle persists across tab switches; ticks delivered while away are dropped.
- [ ] Network LB shows the placeholder body on the Observability tab.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Expected: PR URL printed.

---

## Self-Review Notes

Spec coverage scan:
- Backend health fan-out (spec §Backends tab) → Tasks 9-15.
- Observability charts (spec §Observability tab) → Tasks 16-22.
- Time range + auto-refresh (spec §Auto-refresh and stale-tick discipline) → Tasks 23-24, 26.
- LB-type gating (spec §LB-type gating) → Task 25.
- `getHealth` types + NEG detection (spec §New types) → Tasks 1-3.
- Monitoring helpers (spec §Metric data source) → Tasks 4-8.
- Documentation → Task 27.

Type names match across tasks: `groupHealthState`, `groupHealthPhase`, `groupHealthLoading`/`OK`/`Errored`/`Skipped`, `backendGroupKind`, `lbGroupHealthLoadedMsg`, `lbGroupSkippedMsg`, `lbHealthRefreshMsg`, `loadBalancerObservability`, `lbMetricsLoadedMsg`, `lbObsTickMsg`. Constant names match: `lbMetricRequestCount`, `lbMetricRequestLatencies`, `lbMetricBackendLatencies`, `lbMetricRequestBytes`, `lbMetricResponseBytes`.
