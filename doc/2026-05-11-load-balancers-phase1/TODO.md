# Load Balancers — Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land inventory + details + delete-with-dependency-cascade for GCP Load Balancers in a new "Network Services" sidebar section.

**Architecture:** Forwarding-rule-centric listing aggregated across global + regional scopes. Details view resolves the proxy → URL map → backend services chain lazily via parallel `tea.Cmd`s. Delete cascade computed by a pure `ComputeCascade` function and shown to the user (with shared-resource "Keep" rows) before any API call.

**Tech Stack:** Go 1.26, Bubble Tea, lipgloss, `google.golang.org/api/compute/v1`, testify.

**Spec:** `doc/2026-05-11-load-balancers-phase1/Design.md`.

**Module path:** `github.com/slayer/gcon`.

**Project rules:** never include `Co-Authored-By` in commit messages (see `~/.claude/CLAUDE.md`). Use `mise exec -- go test ...` and `mise exec -- golangci-lint run ...` if the binaries aren't on `$PATH`.

---

## File Structure

### New files

| File | Purpose |
|---|---|
| `internal/gcp/loadbalancers.go` | Domain structs (`ForwardingRule`, `TargetProxy`, `URLMap`, `BackendService`, `HealthCheck`, `SSLCertificate`) + methods on `*ComputeClient`: read (List/Get for each kind), list-all helpers used by cascade sharing checks, and delete methods. Pure data layer; no UI deps. |
| `internal/gcp/loadbalancers_test.go` | Table-driven tests for the type-derivation helper. |
| `internal/ui/views/loadbalancer_cascade.go` | Pure `ComputeCascade` function + `Cascade`, `CascadeItem`, `CascadeKept` types. |
| `internal/ui/views/loadbalancer_cascade_test.go` | Table-driven cascade tests. |
| `internal/ui/views/loadbalancer_messages.go` | `LoadBalancerSelectedMsg`, `LoadBalancerDeleteRequestMsg`, `LoadBalancerDeletedMsg`. |
| `internal/ui/views/loadbalancers.go` | `LoadBalancersView` — list view with table, sort/filter, Enter → details. |
| `internal/ui/views/loadbalancers_test.go` | List view tests (load, sort, filter, navigation). |
| `internal/ui/views/loadbalancer_details.go` | `LoadBalancerDetailsView` — three tabs (Overview / Routing / Backends), parallel fetch state machine, delete trigger. |
| `internal/ui/views/loadbalancer_details_test.go` | Detail view tests. |

### Modified files

| File | Reason |
|---|---|
| `internal/ui/components/sidebar/menu.go` | New "Network Services" category with "Load balancing" leaf. |
| `internal/ui/components/commandpalette/commands.go` | New `ViewLoadBalancers` constant + nav command entry. |
| `internal/ui/app.go` | View routing for `ViewLoadBalancers` / `ViewLoadBalancerDetails`, view-instance fields, `LoadBalancerSelectedMsg` / `LoadBalancerDeleteRequestMsg` / `LoadBalancerDeletedMsg` handlers, `clearAllViews` entry. |
| `internal/ui/app_render.go` | Render cases for the two new views. |
| `internal/ui/app_navigation.go` | Navigation handlers, sidebar guards, sidebar active-view mapping. |
| `README.md`, `CLAUDE.md`, `.claude/rules/key-bindings.md` | Documentation. |

---

## Task 1: Type-derivation helper (`internal/gcp/loadbalancers.go`)

**Files:**
- Create: `internal/gcp/loadbalancers.go` (initial skeleton, just types + helper)
- Create: `internal/gcp/loadbalancers_test.go` (initial)

- [ ] **Step 1: Write the failing tests**

Create `internal/gcp/loadbalancers_test.go`:

```go
package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeriveLoadBalancerType(t *testing.T) {
	tests := []struct {
		name   string
		target string
		scheme string
		proto  string
		want   string
	}{
		{
			name:   "external https managed",
			target: "https://www.googleapis.com/compute/v1/projects/p/global/targetHttpsProxies/x",
			scheme: "EXTERNAL_MANAGED",
			want:   "HTTPS (external)",
		},
		{
			name:   "external http managed",
			target: "https://www.googleapis.com/compute/v1/projects/p/global/targetHttpProxies/x",
			scheme: "EXTERNAL_MANAGED",
			want:   "HTTP (external)",
		},
		{
			name:   "internal https managed",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/targetHttpsProxies/x",
			scheme: "INTERNAL_MANAGED",
			want:   "HTTPS (internal)",
		},
		{
			name:   "tcp proxy external",
			target: "https://www.googleapis.com/compute/v1/projects/p/global/targetTcpProxies/x",
			scheme: "EXTERNAL_MANAGED",
			want:   "TCP proxy (external)",
		},
		{
			name:   "ssl proxy external",
			target: "https://www.googleapis.com/compute/v1/projects/p/global/targetSslProxies/x",
			scheme: "EXTERNAL_MANAGED",
			want:   "SSL proxy (external)",
		},
		{
			name:   "network LB proxy",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/backendServices/x",
			scheme: "EXTERNAL_MANAGED",
			proto:  "TCP",
			want:   "Network LB (proxy)",
		},
		{
			name:   "network LB passthrough",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/backendServices/x",
			scheme: "EXTERNAL",
			proto:  "TCP",
			want:   "Network LB (passthrough)",
		},
		{
			name:   "internal passthrough network LB",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/backendServices/x",
			scheme: "INTERNAL",
			proto:  "TCP",
			want:   "Network LB (passthrough)",
		},
		{
			name:   "legacy target pool",
			target: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/targetPools/x",
			scheme: "EXTERNAL",
			want:   "Network LB (legacy)",
		},
		{
			name:   "unknown target kind falls back to raw segment",
			target: "https://www.googleapis.com/compute/v1/projects/p/global/targetGrpcProxies/x",
			scheme: "EXTERNAL_MANAGED",
			want:   "targetGrpcProxies",
		},
		{
			name:   "empty target",
			target: "",
			scheme: "EXTERNAL",
			want:   "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveLoadBalancerType(tc.target, tc.scheme, tc.proto)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestTargetKind(t *testing.T) {
	got := targetKind("https://www.googleapis.com/compute/v1/projects/p/global/targetHttpsProxies/my-proxy")
	assert.Equal(t, "targetHttpsProxies", got)

	got = targetKind("https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/backendServices/my-bes")
	assert.Equal(t, "backendServices", got)

	got = targetKind("")
	assert.Equal(t, "", got)
}

func TestShortName(t *testing.T) {
	assert.Equal(t, "my-proxy", shortName("https://www.googleapis.com/compute/v1/projects/p/global/targetHttpsProxies/my-proxy"))
	assert.Equal(t, "", shortName(""))
	assert.Equal(t, "x", shortName("x"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go test ./internal/gcp/ -run TestDeriveLoadBalancerType -v`
Expected: FAIL — `DeriveLoadBalancerType`, `targetKind`, `shortName` undefined.

- [ ] **Step 3: Implement the helper file**

Create `internal/gcp/loadbalancers.go`:

```go
package gcp

import "strings"

// DeriveLoadBalancerType returns a human-readable load-balancer type label
// derived from a forwarding rule's Target URL, LoadBalancingScheme, and
// IPProtocol. See doc/2026-05-11-load-balancers-phase1/Design.md for the
// full mapping table.
func DeriveLoadBalancerType(target, scheme, proto string) string {
	kind := targetKind(target)
	if kind == "" {
		return ""
	}
	managed := strings.HasSuffix(scheme, "_MANAGED")

	switch kind {
	case "targetHttpsProxies":
		if scheme == "INTERNAL_MANAGED" {
			return "HTTPS (internal)"
		}
		return "HTTPS (external)"
	case "targetHttpProxies":
		if scheme == "INTERNAL_MANAGED" {
			return "HTTP (internal)"
		}
		return "HTTP (external)"
	case "targetTcpProxies":
		return "TCP proxy (external)"
	case "targetSslProxies":
		return "SSL proxy (external)"
	case "backendServices":
		if managed {
			return "Network LB (proxy)"
		}
		return "Network LB (passthrough)"
	case "targetPools":
		return "Network LB (legacy)"
	default:
		return kind
	}
}

// targetKind returns the resource kind segment from a Compute API self-link
// (e.g. "targetHttpsProxies" from ".../global/targetHttpsProxies/foo").
// Returns "" for empty input.
func targetKind(url string) string {
	if url == "" {
		return ""
	}
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2]
}

// shortName returns the last URL segment.
func shortName(url string) string {
	if url == "" {
		return ""
	}
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go test ./internal/gcp/ -run "TestDeriveLoadBalancerType|TestTargetKind|TestShortName" -v`
Expected: PASS for all subtests.

- [ ] **Step 5: Lint**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- golangci-lint run ./internal/gcp/...`
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add internal/gcp/loadbalancers.go internal/gcp/loadbalancers_test.go
git commit -m "2026-05-11: LB phase 1 — type-derivation helper"
```

---

## Task 2: GCP read methods — forwarding rules + proxies

**Files:**
- Modify: `internal/gcp/loadbalancers.go` (append structs + methods)

This task lands the structs for the resources the details view will fetch, plus the methods to list forwarding rules and fetch a single forwarding rule / target proxy.

- [ ] **Step 1: Append the structs and methods**

Append to `internal/gcp/loadbalancers.go`:

```go
import (
	"context"
	"fmt"

	"google.golang.org/api/compute/v1"
)
```

(Merge with the existing single-import `strings` declaration so the file has one import block.)

Append the structs:

```go
// ForwardingRule is the list-view summary of a Compute forwarding rule.
type ForwardingRule struct {
	Name                string
	SelfLink            string
	Scope               string // "global" or region
	Type                string // derived via DeriveLoadBalancerType
	LoadBalancingScheme string
	IPAddress           string
	IPProtocol          string
	PortRange           string
	Ports               []string
	Target              string // full URL
	Network             string // short name; empty for external
	Subnetwork          string // short name; empty for external
	Labels              map[string]string
	CreatedAt           string
}

// TargetProxy is a normalized view over the various proxy kinds
// (targetHttpProxy, targetHttpsProxy, targetTcpProxy, targetSslProxy).
type TargetProxy struct {
	Name            string
	Kind            string // targetKind() of SelfLink
	SelfLink        string
	Scope           string
	URLMap          string   // full URL, empty for non-HTTP proxies
	SSLCertificates []string // full URLs
	SSLPolicy       string
	QUICOverride    string
	Service         string // backend service URL for SSL/TCP proxies
}

// ListForwardingRules returns every forwarding rule in the project,
// aggregating regional and global results into one flat slice.
func (c *ComputeClient) ListForwardingRules(ctx context.Context, projectID string) ([]ForwardingRule, error) {
	var rules []ForwardingRule

	// Regional via aggregated list.
	aggReq := c.service.ForwardingRules.AggregatedList(projectID)
	if err := aggReq.Pages(ctx, func(page *compute.ForwardingRuleAggregatedList) error {
		for scope, scopedList := range page.Items {
			if scopedList.ForwardingRules == nil {
				continue
			}
			// scope keys look like "regions/us-central1" or "global".
			region := scope
			if i := strings.Index(scope, "/"); i >= 0 {
				region = scope[i+1:]
			}
			for _, fr := range scopedList.ForwardingRules {
				rules = append(rules, convertForwardingRule(fr, region))
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("aggregated list forwarding rules: %w", err)
	}

	// Global.
	globReq := c.service.GlobalForwardingRules.List(projectID)
	if err := globReq.Pages(ctx, func(page *compute.ForwardingRuleList) error {
		for _, fr := range page.Items {
			rules = append(rules, convertForwardingRule(fr, "global"))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list global forwarding rules: %w", err)
	}

	return rules, nil
}

// GetForwardingRule fetches a single forwarding rule by name + scope.
// scope is either "global" or a region name.
func (c *ComputeClient) GetForwardingRule(ctx context.Context, projectID, scope, name string) (*ForwardingRule, error) {
	if scope == "global" {
		fr, err := c.service.GlobalForwardingRules.Get(projectID, name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("get global forwarding rule %s: %w", name, err)
		}
		r := convertForwardingRule(fr, "global")
		return &r, nil
	}
	fr, err := c.service.ForwardingRules.Get(projectID, scope, name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get forwarding rule %s/%s: %w", scope, name, err)
	}
	r := convertForwardingRule(fr, scope)
	return &r, nil
}

func convertForwardingRule(fr *compute.ForwardingRule, scope string) ForwardingRule {
	return ForwardingRule{
		Name:                fr.Name,
		SelfLink:            fr.SelfLink,
		Scope:               scope,
		Type:                DeriveLoadBalancerType(fr.Target, fr.LoadBalancingScheme, fr.IPProtocol),
		LoadBalancingScheme: fr.LoadBalancingScheme,
		IPAddress:           fr.IPAddress,
		IPProtocol:          fr.IPProtocol,
		PortRange:           fr.PortRange,
		Ports:               fr.Ports,
		Target:              fr.Target,
		Network:             shortName(fr.Network),
		Subnetwork:          shortName(fr.Subnetwork),
		Labels:              fr.Labels,
		CreatedAt:           fr.CreationTimestamp,
	}
}

// GetTargetProxy fetches a target proxy. The kind (one of targetHttpProxies /
// targetHttpsProxies / targetTcpProxies / targetSslProxies) is inferred from
// the URL via targetKind. scope is "global" or a region; the proxy kinds
// supported by Phase 1 are all global today, but the parameter is kept for
// future regional internal HTTPS proxies.
func (c *ComputeClient) GetTargetProxy(ctx context.Context, projectID, scope, proxyURL string) (*TargetProxy, error) {
	name := shortName(proxyURL)
	kind := targetKind(proxyURL)

	tp := &TargetProxy{Name: name, Kind: kind, SelfLink: proxyURL, Scope: scope}

	switch kind {
	case "targetHttpsProxies":
		if scope == "global" {
			p, err := c.service.TargetHttpsProxies.Get(projectID, name).Context(ctx).Do()
			if err != nil {
				return nil, fmt.Errorf("get target https proxy %s: %w", name, err)
			}
			tp.URLMap = p.UrlMap
			tp.SSLCertificates = p.SslCertificates
			tp.SSLPolicy = p.SslPolicy
			tp.QUICOverride = p.QuicOverride
		} else {
			p, err := c.service.RegionTargetHttpsProxies.Get(projectID, scope, name).Context(ctx).Do()
			if err != nil {
				return nil, fmt.Errorf("get regional target https proxy %s/%s: %w", scope, name, err)
			}
			tp.URLMap = p.UrlMap
			tp.SSLCertificates = p.SslCertificates
		}
	case "targetHttpProxies":
		if scope == "global" {
			p, err := c.service.TargetHttpProxies.Get(projectID, name).Context(ctx).Do()
			if err != nil {
				return nil, fmt.Errorf("get target http proxy %s: %w", name, err)
			}
			tp.URLMap = p.UrlMap
		} else {
			p, err := c.service.RegionTargetHttpProxies.Get(projectID, scope, name).Context(ctx).Do()
			if err != nil {
				return nil, fmt.Errorf("get regional target http proxy %s/%s: %w", scope, name, err)
			}
			tp.URLMap = p.UrlMap
		}
	case "targetTcpProxies":
		p, err := c.service.TargetTcpProxies.Get(projectID, name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("get target tcp proxy %s: %w", name, err)
		}
		tp.Service = p.Service
	case "targetSslProxies":
		p, err := c.service.TargetSslProxies.Get(projectID, name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("get target ssl proxy %s: %w", name, err)
		}
		tp.Service = p.Service
		tp.SSLCertificates = p.SslCertificates
	default:
		return nil, fmt.Errorf("unsupported target proxy kind: %s", kind)
	}

	return tp, nil
}
```

- [ ] **Step 2: Verify build**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go build ./internal/gcp/...`
Expected: clean build.

- [ ] **Step 3: Verify existing tests still pass**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go test ./internal/gcp/ -v`
Expected: Task 1 tests still PASS.

- [ ] **Step 4: Lint**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- golangci-lint run ./internal/gcp/...`
Expected: 0 issues.

- [ ] **Step 5: Commit**

```bash
git add internal/gcp/loadbalancers.go
git commit -m "2026-05-11: LB phase 1 — forwarding rule + target proxy read methods"
```

---

## Task 3: GCP read methods — URL maps, backend services, health checks, SSL certs

**Files:**
- Modify: `internal/gcp/loadbalancers.go` (append remaining structs + methods)

- [ ] **Step 1: Append remaining read APIs**

Append to `internal/gcp/loadbalancers.go`:

```go
// URLMap is a normalized view over compute.UrlMap / compute.RegionUrlMap.
type URLMap struct {
	Name           string
	SelfLink       string
	Scope          string
	DefaultService string // full URL
	HostRules      []HostRule
	PathMatchers   []PathMatcher
}

// HostRule maps a set of hostnames to a path matcher.
type HostRule struct {
	Hosts       []string
	PathMatcher string
}

// PathMatcher groups path rules with a default service.
type PathMatcher struct {
	Name           string
	DefaultService string
	PathRules      []PathRule
}

// PathRule maps a set of paths to a backend service.
type PathRule struct {
	Paths   []string
	Service string
}

// BackendService is the load-balanced backend pool.
type BackendService struct {
	Name                string
	SelfLink            string
	Scope               string
	Protocol            string
	TimeoutSec          int64
	SessionAffinity     string
	LocalityLBPolicy    string
	HealthChecks        []string // full URLs
	Backends            []Backend
	LoadBalancingScheme string
}

// Backend is one member of a backend service.
type Backend struct {
	Group              string // full URL
	BalancingMode      string
	CapacityScaler     float64
	MaxRate            int64
	MaxRatePerInstance float64
	MaxConnections     int64
}

// HealthCheck is a normalized view of compute.HealthCheck / compute.RegionHealthCheck.
type HealthCheck struct {
	Name               string
	SelfLink           string
	Scope              string
	Type               string // "HTTP", "HTTPS", "TCP", "SSL", "HTTP2", "GRPC"
	Port               int64
	CheckIntervalSec   int64
	TimeoutSec         int64
	HealthyThreshold   int64
	UnhealthyThreshold int64
}

// SSLCertificate is metadata for an SSL certificate. No private key material.
type SSLCertificate struct {
	Name       string
	SelfLink   string
	Scope      string
	Type       string // "MANAGED" or "SELF_MANAGED"
	Status     string
	ExpireTime string
}

// GetURLMap fetches a URL map. scope is "global" or a region.
func (c *ComputeClient) GetURLMap(ctx context.Context, projectID, scope, mapURL string) (*URLMap, error) {
	name := shortName(mapURL)
	if scope == "global" {
		um, err := c.service.UrlMaps.Get(projectID, name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("get url map %s: %w", name, err)
		}
		return convertURLMap(um, "global"), nil
	}
	um, err := c.service.RegionUrlMaps.Get(projectID, scope, name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get regional url map %s/%s: %w", scope, name, err)
	}
	return convertURLMap((*compute.UrlMap)(um), scope), nil
}

func convertURLMap(um *compute.UrlMap, scope string) *URLMap {
	hostRules := make([]HostRule, 0, len(um.HostRules))
	for _, hr := range um.HostRules {
		hostRules = append(hostRules, HostRule{Hosts: hr.Hosts, PathMatcher: hr.PathMatcher})
	}
	matchers := make([]PathMatcher, 0, len(um.PathMatchers))
	for _, pm := range um.PathMatchers {
		rules := make([]PathRule, 0, len(pm.PathRules))
		for _, pr := range pm.PathRules {
			rules = append(rules, PathRule{Paths: pr.Paths, Service: pr.Service})
		}
		matchers = append(matchers, PathMatcher{
			Name:           pm.Name,
			DefaultService: pm.DefaultService,
			PathRules:      rules,
		})
	}
	return &URLMap{
		Name:           um.Name,
		SelfLink:       um.SelfLink,
		Scope:          scope,
		DefaultService: um.DefaultService,
		HostRules:      hostRules,
		PathMatchers:   matchers,
	}
}

// GetBackendService fetches a backend service.
func (c *ComputeClient) GetBackendService(ctx context.Context, projectID, scope, bsURL string) (*BackendService, error) {
	name := shortName(bsURL)
	if scope == "global" {
		bs, err := c.service.BackendServices.Get(projectID, name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("get backend service %s: %w", name, err)
		}
		return convertBackendService(bs, "global"), nil
	}
	bs, err := c.service.RegionBackendServices.Get(projectID, scope, name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get regional backend service %s/%s: %w", scope, name, err)
	}
	return convertBackendService(bs, scope), nil
}

func convertBackendService(bs *compute.BackendService, scope string) *BackendService {
	backends := make([]Backend, 0, len(bs.Backends))
	for _, b := range bs.Backends {
		backends = append(backends, Backend{
			Group:              b.Group,
			BalancingMode:      b.BalancingMode,
			CapacityScaler:     b.CapacityScaler,
			MaxRate:            b.MaxRate,
			MaxRatePerInstance: b.MaxRatePerInstance,
			MaxConnections:     b.MaxConnections,
		})
	}
	return &BackendService{
		Name:                bs.Name,
		SelfLink:            bs.SelfLink,
		Scope:               scope,
		Protocol:            bs.Protocol,
		TimeoutSec:          bs.TimeoutSec,
		SessionAffinity:     bs.SessionAffinity,
		LocalityLBPolicy:    bs.LocalityLbPolicy,
		HealthChecks:        bs.HealthChecks,
		Backends:            backends,
		LoadBalancingScheme: bs.LoadBalancingScheme,
	}
}

// GetHealthCheck fetches a health check.
func (c *ComputeClient) GetHealthCheck(ctx context.Context, projectID, scope, hcURL string) (*HealthCheck, error) {
	name := shortName(hcURL)
	if scope == "global" {
		hc, err := c.service.HealthChecks.Get(projectID, name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("get health check %s: %w", name, err)
		}
		return convertHealthCheck(hc, "global"), nil
	}
	hc, err := c.service.RegionHealthChecks.Get(projectID, scope, name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get regional health check %s/%s: %w", scope, name, err)
	}
	return convertHealthCheck(hc, scope), nil
}

func convertHealthCheck(hc *compute.HealthCheck, scope string) *HealthCheck {
	port := hc.Port
	if port == 0 {
		// Modern health checks use type-specific port fields.
		switch hc.Type {
		case "HTTP":
			if hc.HttpHealthCheck != nil {
				port = hc.HttpHealthCheck.Port
			}
		case "HTTPS":
			if hc.HttpsHealthCheck != nil {
				port = hc.HttpsHealthCheck.Port
			}
		case "TCP":
			if hc.TcpHealthCheck != nil {
				port = hc.TcpHealthCheck.Port
			}
		case "SSL":
			if hc.SslHealthCheck != nil {
				port = hc.SslHealthCheck.Port
			}
		case "HTTP2":
			if hc.Http2HealthCheck != nil {
				port = hc.Http2HealthCheck.Port
			}
		case "GRPC":
			if hc.GrpcHealthCheck != nil {
				port = hc.GrpcHealthCheck.Port
			}
		}
	}
	return &HealthCheck{
		Name:               hc.Name,
		SelfLink:           hc.SelfLink,
		Scope:              scope,
		Type:               hc.Type,
		Port:               port,
		CheckIntervalSec:   hc.CheckIntervalSec,
		TimeoutSec:         hc.TimeoutSec,
		HealthyThreshold:   hc.HealthyThreshold,
		UnhealthyThreshold: hc.UnhealthyThreshold,
	}
}

// GetSSLCertificate fetches an SSL certificate (metadata only).
func (c *ComputeClient) GetSSLCertificate(ctx context.Context, projectID, scope, certURL string) (*SSLCertificate, error) {
	name := shortName(certURL)
	if scope == "global" {
		sc, err := c.service.SslCertificates.Get(projectID, name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("get ssl certificate %s: %w", name, err)
		}
		out := &SSLCertificate{
			Name:     sc.Name,
			SelfLink: sc.SelfLink,
			Scope:    "global",
			Type:     sc.Type,
			ExpireTime: sc.ExpireTime,
		}
		if sc.Managed != nil {
			out.Status = sc.Managed.Status
		}
		return out, nil
	}
	sc, err := c.service.RegionSslCertificates.Get(projectID, scope, name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get regional ssl certificate %s/%s: %w", scope, name, err)
	}
	out := &SSLCertificate{
		Name:     sc.Name,
		SelfLink: sc.SelfLink,
		Scope:    scope,
		Type:     sc.Type,
		ExpireTime: sc.ExpireTime,
	}
	if sc.Managed != nil {
		out.Status = sc.Managed.Status
	}
	return out, nil
}
```

- [ ] **Step 2: Build + lint**

```bash
cd /home/vlad/dev/my/gcon && mise exec -- go build ./internal/gcp/... && mise exec -- golangci-lint run ./internal/gcp/...
```

Expected: 0 errors, 0 issues.

If any field name doesn't match the actual `google.golang.org/api/compute/v1` types (the API has evolved), adjust to the correct name reported by the compiler. Common gotchas: capitalization of acronyms (`Url` vs `URL`), `PortName` vs `Port`, etc.

- [ ] **Step 3: Commit**

```bash
git add internal/gcp/loadbalancers.go
git commit -m "2026-05-11: LB phase 1 — URL map / backend service / health check / SSL cert reads"
```

---

## Task 4: ComputeCascade pure function

**Files:**
- Create: `internal/ui/views/loadbalancer_cascade.go`
- Create: `internal/ui/views/loadbalancer_cascade_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/views/loadbalancer_cascade_test.go`:

```go
package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func mkProxy(name, urlMap string) gcp.TargetProxy {
	return gcp.TargetProxy{
		Name:     name,
		Kind:     "targetHttpsProxies",
		SelfLink: "https://example/global/targetHttpsProxies/" + name,
		Scope:    "global",
		URLMap:   urlMap,
	}
}

func mkURLMap(name, defaultService string, extraServices ...string) gcp.URLMap {
	rules := []gcp.PathRule{}
	for _, s := range extraServices {
		rules = append(rules, gcp.PathRule{Paths: []string{"/*"}, Service: s})
	}
	return gcp.URLMap{
		Name:           name,
		SelfLink:       "https://example/global/urlMaps/" + name,
		Scope:          "global",
		DefaultService: defaultService,
		PathMatchers: []gcp.PathMatcher{
			{Name: "default", DefaultService: defaultService, PathRules: rules},
		},
	}
}

func mkBackend(name string, healthChecks ...string) gcp.BackendService {
	return gcp.BackendService{
		Name:         name,
		SelfLink:     "https://example/global/backendServices/" + name,
		Scope:        "global",
		HealthChecks: healthChecks,
	}
}

func mkFwd(name, target string) gcp.ForwardingRule {
	return gcp.ForwardingRule{
		Name:     name,
		SelfLink: "https://example/global/forwardingRules/" + name,
		Scope:    "global",
		Target:   target,
	}
}

func TestComputeCascade_HTTPSDedicated_DeletesEverything(t *testing.T) {
	proxyURL := "https://example/global/targetHttpsProxies/p1"
	urlMapURL := "https://example/global/urlMaps/m1"
	bs1URL := "https://example/global/backendServices/b1"
	hc1URL := "https://example/global/healthChecks/h1"

	rule := mkFwd("f1", proxyURL)
	proxies := []gcp.TargetProxy{mkProxy("p1", urlMapURL)}
	urlMaps := []gcp.URLMap{mkURLMap("m1", bs1URL)}
	backends := []gcp.BackendService{mkBackend("b1", hc1URL)}

	c := ComputeCascade(rule, []gcp.ForwardingRule{rule}, proxies, urlMaps, backends)

	assert.Len(t, c.Keep, 0)
	kinds := []string{}
	for _, it := range c.Delete {
		kinds = append(kinds, it.Kind)
	}
	assert.Equal(t, []string{"forwardingRule", "targetHttpsProxies", "urlMap", "backendService", "healthCheck"}, kinds)
}

func TestComputeCascade_SharedBackend_Kept(t *testing.T) {
	proxyURL := "https://example/global/targetHttpsProxies/p1"
	urlMapURL := "https://example/global/urlMaps/m1"
	bs1URL := "https://example/global/backendServices/b1"

	rule := mkFwd("f1", proxyURL)
	// Another URL map (referenced by a different proxy) also uses b1.
	otherURLMap := mkURLMap("m2", bs1URL)
	proxies := []gcp.TargetProxy{mkProxy("p1", urlMapURL), mkProxy("p2", otherURLMap.SelfLink)}
	urlMaps := []gcp.URLMap{mkURLMap("m1", bs1URL), otherURLMap}
	backends := []gcp.BackendService{mkBackend("b1")}

	// Another forwarding rule pointing at the other proxy so that other proxy
	// itself is "live", which keeps url-map m2 live, which keeps b1 live.
	otherFwd := mkFwd("f2", proxies[1].SelfLink)

	c := ComputeCascade(rule, []gcp.ForwardingRule{rule, otherFwd}, proxies, urlMaps, backends)

	// b1 must be in Keep, not Delete.
	for _, it := range c.Delete {
		assert.NotEqual(t, "backendService", it.Kind)
	}
	require := assert.Len(t, c.Keep, 1)
	if require {
		assert.Equal(t, "b1", c.Keep[0].Name)
	}
}

func TestComputeCascade_SharedProxy_KeptCascadeStopsAtProxy(t *testing.T) {
	proxyURL := "https://example/global/targetHttpsProxies/p1"
	urlMapURL := "https://example/global/urlMaps/m1"
	bs1URL := "https://example/global/backendServices/b1"

	rule := mkFwd("f1", proxyURL)
	otherRuleSameProxy := mkFwd("f2", proxyURL) // shares the proxy
	proxies := []gcp.TargetProxy{mkProxy("p1", urlMapURL)}
	urlMaps := []gcp.URLMap{mkURLMap("m1", bs1URL)}
	backends := []gcp.BackendService{mkBackend("b1")}

	c := ComputeCascade(rule, []gcp.ForwardingRule{rule, otherRuleSameProxy}, proxies, urlMaps, backends)

	kinds := []string{}
	for _, it := range c.Delete {
		kinds = append(kinds, it.Kind)
	}
	assert.Equal(t, []string{"forwardingRule"}, kinds, "only the forwarding rule should be deleted; proxy is shared")
	// Proxy must appear in Keep with a reason mentioning the other rule.
	assert.Len(t, c.Keep, 1)
	assert.Equal(t, "p1", c.Keep[0].Name)
	assert.NotEmpty(t, c.Keep[0].KeptBecause)
}

func TestComputeCascade_NetworkLB_DirectBackend(t *testing.T) {
	bs1URL := "https://example/regions/us-central1/backendServices/b1"
	hc1URL := "https://example/global/healthChecks/h1"

	rule := mkFwd("f1", bs1URL)
	rule.Scope = "us-central1"
	rule.Target = bs1URL

	backends := []gcp.BackendService{{
		Name:         "b1",
		SelfLink:     bs1URL,
		Scope:        "us-central1",
		HealthChecks: []string{hc1URL},
	}}

	c := ComputeCascade(rule, []gcp.ForwardingRule{rule}, nil, nil, backends)

	kinds := []string{}
	for _, it := range c.Delete {
		kinds = append(kinds, it.Kind)
	}
	assert.Equal(t, []string{"forwardingRule", "backendService", "healthCheck"}, kinds)
}

func TestComputeCascade_LegacyTargetPool(t *testing.T) {
	poolURL := "https://example/regions/us-central1/targetPools/tp1"
	rule := mkFwd("f1", poolURL)
	rule.Scope = "us-central1"

	c := ComputeCascade(rule, []gcp.ForwardingRule{rule}, nil, nil, nil)

	kinds := []string{}
	for _, it := range c.Delete {
		kinds = append(kinds, it.Kind)
	}
	// Phase 1: cascade stops at the forwarding rule for legacy target pools.
	// The targetPool itself is kept (user must delete manually).
	assert.Equal(t, []string{"forwardingRule"}, kinds)
}

func TestComputeCascade_MissingReferences_NoPanic(t *testing.T) {
	// URL map referenced by the proxy but not in the input list — must not panic.
	proxyURL := "https://example/global/targetHttpsProxies/p1"
	urlMapURL := "https://example/global/urlMaps/missing"

	rule := mkFwd("f1", proxyURL)
	proxies := []gcp.TargetProxy{mkProxy("p1", urlMapURL)}

	c := ComputeCascade(rule, []gcp.ForwardingRule{rule}, proxies, nil, nil)

	kinds := []string{}
	for _, it := range c.Delete {
		kinds = append(kinds, it.Kind)
	}
	// Forwarding rule + proxy are cascaded; URL map / backends absent (no panic).
	assert.Equal(t, []string{"forwardingRule", "targetHttpsProxies"}, kinds)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go test ./internal/ui/views/ -run TestComputeCascade -v`
Expected: FAIL — `ComputeCascade` undefined.

- [ ] **Step 3: Implement `ComputeCascade`**

Create `internal/ui/views/loadbalancer_cascade.go`:

```go
package views

import (
	"fmt"

	"github.com/slayer/gcon/internal/gcp"
)

// CascadeItem is a resource scheduled for deletion.
type CascadeItem struct {
	Kind  string // "forwardingRule" | "targetHttpProxies" | "targetHttpsProxies" | "targetTcpProxies" | "targetSslProxies" | "urlMap" | "backendService" | "healthCheck"
	Name  string
	Scope string // "global" or region
	URL   string // self-link, used by the executor to sequence deletes
}

// CascadeKept is a resource the cascade would have liked to delete but kept
// because something else still references it.
type CascadeKept struct {
	Kind        string
	Name        string
	Scope       string
	KeptBecause []string // human-readable references
}

// Cascade is the result of computing what a single-LB delete will remove.
type Cascade struct {
	Delete []CascadeItem // ordered: root first, leaves last
	Keep   []CascadeKept
}

// ComputeCascade returns the set of resources that should be deleted to
// remove the given forwarding rule, plus the set of resources that would
// have been deleted but are shared with another LB.
//
// The function is pure — no API calls. The caller is responsible for
// providing the full project-wide inventory of forwarding rules, proxies,
// URL maps, and backend services.
func ComputeCascade(rule gcp.ForwardingRule, allFwdRules []gcp.ForwardingRule, allProxies []gcp.TargetProxy, allURLMaps []gcp.URLMap, allBackends []gcp.BackendService) Cascade {
	var c Cascade

	// Step 1: always delete the forwarding rule.
	c.Delete = append(c.Delete, CascadeItem{
		Kind:  "forwardingRule",
		Name:  rule.Name,
		Scope: rule.Scope,
		URL:   rule.SelfLink,
	})

	if rule.Target == "" {
		return c
	}

	kind := targetKindFromURL(rule.Target)
	switch kind {
	case "targetHttpProxies", "targetHttpsProxies", "targetTcpProxies", "targetSslProxies":
		c = cascadeProxy(c, rule, allFwdRules, allProxies, allURLMaps, allBackends, kind)
	case "backendServices":
		c = cascadeDirectBackend(c, rule.Target, allFwdRules, allURLMaps, allBackends)
	case "targetPools":
		// Legacy: cascade stops at the forwarding rule in Phase 1.
	}

	return c
}

// cascadeProxy adds proxy/urlMap/backends to the cascade as appropriate.
func cascadeProxy(c Cascade, rule gcp.ForwardingRule, allFwdRules []gcp.ForwardingRule, allProxies []gcp.TargetProxy, allURLMaps []gcp.URLMap, allBackends []gcp.BackendService, kind string) Cascade {
	proxyURL := rule.Target
	proxy := findProxy(allProxies, proxyURL)

	// Proxy shared by another forwarding rule?
	otherUsers := otherForwardingRulesUsing(allFwdRules, rule.Name, proxyURL)
	if len(otherUsers) > 0 {
		name := shortNameURL(proxyURL)
		scope := scopeFromURL(proxyURL)
		c.Keep = append(c.Keep, CascadeKept{
			Kind:  kind,
			Name:  name,
			Scope: scope,
			KeptBecause: forwardingRuleReasons(otherUsers),
		})
		return c
	}
	c.Delete = append(c.Delete, CascadeItem{
		Kind:  kind,
		Name:  shortNameURL(proxyURL),
		Scope: scopeFromURL(proxyURL),
		URL:   proxyURL,
	})

	// HTTP/HTTPS proxies have a URL map.
	if proxy != nil && proxy.URLMap != "" {
		urlMapURL := proxy.URLMap
		otherProxies := otherProxiesUsing(allProxies, proxyURL, urlMapURL)
		if len(otherProxies) > 0 {
			c.Keep = append(c.Keep, CascadeKept{
				Kind:        "urlMap",
				Name:        shortNameURL(urlMapURL),
				Scope:       scopeFromURL(urlMapURL),
				KeptBecause: proxyReasons(otherProxies),
			})
			return c
		}
		c.Delete = append(c.Delete, CascadeItem{
			Kind:  "urlMap",
			Name:  shortNameURL(urlMapURL),
			Scope: scopeFromURL(urlMapURL),
			URL:   urlMapURL,
		})

		// Backend services from the URL map.
		um := findURLMap(allURLMaps, urlMapURL)
		if um != nil {
			c = cascadeBackendsFromURLMap(c, *um, urlMapURL, allURLMaps, allFwdRules, allBackends)
		}
	} else if proxy != nil && proxy.Service != "" {
		// TCP/SSL proxies point directly at a backend service.
		c = cascadeBackend(c, proxy.Service, []string{"targetProxy:" + proxy.Name}, allFwdRules, allURLMaps, allBackends, ourURLMapName(urlMapsReferencing(allURLMaps, proxy.Service)))
	}

	return c
}

func cascadeBackendsFromURLMap(c Cascade, um gcp.URLMap, urlMapURL string, allURLMaps []gcp.URLMap, allFwdRules []gcp.ForwardingRule, allBackends []gcp.BackendService) Cascade {
	// Collect every backend service URL referenced by this URL map.
	beSet := map[string]struct{}{}
	if um.DefaultService != "" {
		beSet[um.DefaultService] = struct{}{}
	}
	for _, pm := range um.PathMatchers {
		if pm.DefaultService != "" {
			beSet[pm.DefaultService] = struct{}{}
		}
		for _, pr := range pm.PathRules {
			if pr.Service != "" {
				beSet[pr.Service] = struct{}{}
			}
		}
	}

	for beURL := range beSet {
		// Used by any *other* URL map (not the one we're cascading)?
		otherMaps := urlMapsReferencingExcept(allURLMaps, beURL, urlMapURL)
		// Used directly by another forwarding rule (Network LB)?
		otherFwds := forwardingRulesUsingBackend(allFwdRules, beURL)
		reasons := []string{}
		for _, m := range otherMaps {
			reasons = append(reasons, "url-map:"+m)
		}
		for _, fr := range otherFwds {
			reasons = append(reasons, "forwarding-rule:"+fr)
		}
		if len(reasons) > 0 {
			c.Keep = append(c.Keep, CascadeKept{
				Kind:        "backendService",
				Name:        shortNameURL(beURL),
				Scope:       scopeFromURL(beURL),
				KeptBecause: reasons,
			})
			continue
		}
		c.Delete = append(c.Delete, CascadeItem{
			Kind:  "backendService",
			Name:  shortNameURL(beURL),
			Scope: scopeFromURL(beURL),
			URL:   beURL,
		})

		// Health checks from this backend service.
		bs := findBackend(allBackends, beURL)
		if bs != nil {
			c = cascadeHealthChecks(c, bs.HealthChecks, beURL, allBackends)
		}
	}
	return c
}

// cascadeBackend cascades a single backend service URL (used by TCP/SSL
// proxies and by direct-backend Network LBs).
func cascadeBackend(c Cascade, beURL string, reasonsForExclusion []string, allFwdRules []gcp.ForwardingRule, allURLMaps []gcp.URLMap, allBackends []gcp.BackendService, _ string) Cascade {
	otherMaps := urlMapsReferencing(allURLMaps, beURL)
	otherFwds := forwardingRulesUsingBackend(allFwdRules, beURL)
	// Exclude self-references — for direct backend, our forwarding rule will
	// be in otherFwds; remove ourselves later via caller's responsibility.
	reasons := []string{}
	for _, m := range otherMaps {
		reasons = append(reasons, "url-map:"+m)
	}
	for _, fr := range otherFwds {
		reasons = append(reasons, "forwarding-rule:"+fr)
	}
	if len(reasons) > 0 {
		c.Keep = append(c.Keep, CascadeKept{
			Kind:        "backendService",
			Name:        shortNameURL(beURL),
			Scope:       scopeFromURL(beURL),
			KeptBecause: reasons,
		})
		return c
	}
	c.Delete = append(c.Delete, CascadeItem{
		Kind:  "backendService",
		Name:  shortNameURL(beURL),
		Scope: scopeFromURL(beURL),
		URL:   beURL,
	})
	bs := findBackend(allBackends, beURL)
	if bs != nil {
		c = cascadeHealthChecks(c, bs.HealthChecks, beURL, allBackends)
	}
	_ = reasonsForExclusion
	return c
}

// cascadeDirectBackend handles a forwarding rule whose target is a backend
// service (Network LB case).
func cascadeDirectBackend(c Cascade, beURL string, allFwdRules []gcp.ForwardingRule, allURLMaps []gcp.URLMap, allBackends []gcp.BackendService) Cascade {
	otherMaps := urlMapsReferencing(allURLMaps, beURL)
	// Other forwarding rules (besides ours) using this backend directly?
	others := []string{}
	for _, fr := range allFwdRules {
		if fr.Target == beURL && fr.SelfLink != "" && c.Delete[0].URL != fr.SelfLink {
			others = append(others, fr.Name)
		}
	}
	reasons := []string{}
	for _, m := range otherMaps {
		reasons = append(reasons, "url-map:"+m)
	}
	for _, fr := range others {
		reasons = append(reasons, "forwarding-rule:"+fr)
	}
	if len(reasons) > 0 {
		c.Keep = append(c.Keep, CascadeKept{
			Kind:        "backendService",
			Name:        shortNameURL(beURL),
			Scope:       scopeFromURL(beURL),
			KeptBecause: reasons,
		})
		return c
	}
	c.Delete = append(c.Delete, CascadeItem{
		Kind:  "backendService",
		Name:  shortNameURL(beURL),
		Scope: scopeFromURL(beURL),
		URL:   beURL,
	})
	bs := findBackend(allBackends, beURL)
	if bs != nil {
		c = cascadeHealthChecks(c, bs.HealthChecks, beURL, allBackends)
	}
	return c
}

func cascadeHealthChecks(c Cascade, hcURLs []string, ownerBackend string, allBackends []gcp.BackendService) Cascade {
	for _, hcURL := range hcURLs {
		// Used by any other backend service?
		others := []string{}
		for _, bs := range allBackends {
			if bs.SelfLink == ownerBackend {
				continue
			}
			for _, h := range bs.HealthChecks {
				if h == hcURL {
					others = append(others, bs.Name)
					break
				}
			}
		}
		if len(others) > 0 {
			reasons := []string{}
			for _, o := range others {
				reasons = append(reasons, "backend-service:"+o)
			}
			c.Keep = append(c.Keep, CascadeKept{
				Kind:        "healthCheck",
				Name:        shortNameURL(hcURL),
				Scope:       scopeFromURL(hcURL),
				KeptBecause: reasons,
			})
			continue
		}
		c.Delete = append(c.Delete, CascadeItem{
			Kind:  "healthCheck",
			Name:  shortNameURL(hcURL),
			Scope: scopeFromURL(hcURL),
			URL:   hcURL,
		})
	}
	return c
}

// --- pure helpers ---

func findProxy(ps []gcp.TargetProxy, url string) *gcp.TargetProxy {
	for i := range ps {
		if ps[i].SelfLink == url {
			return &ps[i]
		}
	}
	return nil
}

func findURLMap(ms []gcp.URLMap, url string) *gcp.URLMap {
	for i := range ms {
		if ms[i].SelfLink == url {
			return &ms[i]
		}
	}
	return nil
}

func findBackend(bs []gcp.BackendService, url string) *gcp.BackendService {
	for i := range bs {
		if bs[i].SelfLink == url {
			return &bs[i]
		}
	}
	return nil
}

func otherForwardingRulesUsing(all []gcp.ForwardingRule, excludeName, target string) []string {
	out := []string{}
	for _, fr := range all {
		if fr.Name == excludeName {
			continue
		}
		if fr.Target == target {
			out = append(out, fr.Name)
		}
	}
	return out
}

func otherProxiesUsing(all []gcp.TargetProxy, excludeProxyURL, urlMapURL string) []string {
	out := []string{}
	for _, p := range all {
		if p.SelfLink == excludeProxyURL {
			continue
		}
		if p.URLMap == urlMapURL {
			out = append(out, p.Name)
		}
	}
	return out
}

func urlMapsReferencing(maps []gcp.URLMap, beURL string) []string {
	return urlMapsReferencingExcept(maps, beURL, "")
}

func urlMapsReferencingExcept(maps []gcp.URLMap, beURL, exceptURLMapURL string) []string {
	out := []string{}
	for _, m := range maps {
		if m.SelfLink == exceptURLMapURL {
			continue
		}
		if m.DefaultService == beURL {
			out = append(out, m.Name)
			continue
		}
		referenced := false
		for _, pm := range m.PathMatchers {
			if pm.DefaultService == beURL {
				referenced = true
				break
			}
			for _, pr := range pm.PathRules {
				if pr.Service == beURL {
					referenced = true
					break
				}
			}
			if referenced {
				break
			}
		}
		if referenced {
			out = append(out, m.Name)
		}
	}
	return out
}

func forwardingRulesUsingBackend(all []gcp.ForwardingRule, beURL string) []string {
	out := []string{}
	for _, fr := range all {
		if fr.Target == beURL {
			out = append(out, fr.Name)
		}
	}
	return out
}

func forwardingRuleReasons(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = fmt.Sprintf("forwarding-rule: %s", n)
	}
	return out
}

func proxyReasons(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = fmt.Sprintf("proxy: %s", n)
	}
	return out
}

// ourURLMapName is a no-op helper kept for future use.
func ourURLMapName(maps []string) string {
	if len(maps) > 0 {
		return maps[0]
	}
	return ""
}

// targetKindFromURL extracts the second-to-last segment of a Compute self-link.
func targetKindFromURL(url string) string {
	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == '/' {
			// Find the previous '/'.
			for j := i - 1; j >= 0; j-- {
				if url[j] == '/' {
					return url[j+1 : i]
				}
			}
			return ""
		}
	}
	return ""
}

// shortNameURL is the last segment.
func shortNameURL(url string) string {
	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == '/' {
			return url[i+1:]
		}
	}
	return url
}

// scopeFromURL returns "global" or a region name based on the URL shape:
//   ".../global/<kind>/<name>"   → "global"
//   ".../regions/<r>/<kind>/<name>" → "<r>"
func scopeFromURL(url string) string {
	// Find "/global/" or "/regions/<x>/".
	if idx := indexOf(url, "/global/"); idx >= 0 {
		return "global"
	}
	if idx := indexOf(url, "/regions/"); idx >= 0 {
		rest := url[idx+len("/regions/"):]
		end := indexOf(rest, "/")
		if end < 0 {
			return rest
		}
		return rest[:end]
	}
	return ""
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go test ./internal/ui/views/ -run TestComputeCascade -v`
Expected: all 6 PASS.

If a test fails, inspect which one — the most likely cause is a self-link / scope-parsing edge case. Don't change the test expectations to fit a buggy implementation; fix the implementation.

- [ ] **Step 5: Lint**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- golangci-lint run ./internal/ui/views/...`
Expected: 0 issues. If the linter flags unused helper functions, remove them (the placeholder `ourURLMapName` may be flagged; remove it if so).

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/loadbalancer_cascade.go internal/ui/views/loadbalancer_cascade_test.go
git commit -m "2026-05-11: LB phase 1 — ComputeCascade pure function"
```

---

## Task 5: GCP delete methods + list-all helpers for sharing checks

**Files:**
- Modify: `internal/gcp/loadbalancers.go` (append delete + list-all methods)

These are the API calls used by the cascade flow: list-all queries to populate the cascade input, and delete queries to execute the cascade.

- [ ] **Step 1: Append the delete + list-all methods**

Append to `internal/gcp/loadbalancers.go`:

```go
// ListAllProxies returns every target proxy (http/https/tcp/ssl) in the project,
// global + regional. Phase 1 only consults this for cascade sharing checks.
func (c *ComputeClient) ListAllProxies(ctx context.Context, projectID string) ([]TargetProxy, error) {
	var out []TargetProxy

	// Global https proxies.
	if err := c.service.TargetHttpsProxies.List(projectID).Pages(ctx, func(page *compute.TargetHttpsProxyList) error {
		for _, p := range page.Items {
			out = append(out, TargetProxy{
				Name: p.Name, Kind: "targetHttpsProxies", SelfLink: p.SelfLink, Scope: "global",
				URLMap: p.UrlMap, SSLCertificates: p.SslCertificates,
			})
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list target https proxies: %w", err)
	}

	// Global http proxies.
	if err := c.service.TargetHttpProxies.List(projectID).Pages(ctx, func(page *compute.TargetHttpProxyList) error {
		for _, p := range page.Items {
			out = append(out, TargetProxy{
				Name: p.Name, Kind: "targetHttpProxies", SelfLink: p.SelfLink, Scope: "global",
				URLMap: p.UrlMap,
			})
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list target http proxies: %w", err)
	}

	// Global tcp proxies.
	if err := c.service.TargetTcpProxies.List(projectID).Pages(ctx, func(page *compute.TargetTcpProxyList) error {
		for _, p := range page.Items {
			out = append(out, TargetProxy{
				Name: p.Name, Kind: "targetTcpProxies", SelfLink: p.SelfLink, Scope: "global",
				Service: p.Service,
			})
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list target tcp proxies: %w", err)
	}

	// Global ssl proxies.
	if err := c.service.TargetSslProxies.List(projectID).Pages(ctx, func(page *compute.TargetSslProxyList) error {
		for _, p := range page.Items {
			out = append(out, TargetProxy{
				Name: p.Name, Kind: "targetSslProxies", SelfLink: p.SelfLink, Scope: "global",
				Service: p.Service, SSLCertificates: p.SslCertificates,
			})
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list target ssl proxies: %w", err)
	}

	return out, nil
}

// ListAllURLMaps returns every URL map (global + regional).
func (c *ComputeClient) ListAllURLMaps(ctx context.Context, projectID string) ([]URLMap, error) {
	var out []URLMap

	// Global.
	if err := c.service.UrlMaps.List(projectID).Pages(ctx, func(page *compute.UrlMapList) error {
		for _, um := range page.Items {
			out = append(out, *convertURLMap(um, "global"))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list url maps: %w", err)
	}

	// Regional aggregated.
	if err := c.service.UrlMaps.AggregatedList(projectID).Pages(ctx, func(page *compute.UrlMapsAggregatedList) error {
		for scope, scoped := range page.Items {
			if scoped.UrlMaps == nil {
				continue
			}
			region := scope
			if i := strings.Index(scope, "/"); i >= 0 {
				region = scope[i+1:]
			}
			if region == "global" {
				continue // already collected above
			}
			for _, um := range scoped.UrlMaps {
				out = append(out, *convertURLMap(um, region))
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("aggregated list url maps: %w", err)
	}

	return out, nil
}

// ListAllBackendServices returns every backend service in the project.
func (c *ComputeClient) ListAllBackendServices(ctx context.Context, projectID string) ([]BackendService, error) {
	var out []BackendService

	if err := c.service.BackendServices.AggregatedList(projectID).Pages(ctx, func(page *compute.BackendServiceAggregatedList) error {
		for scope, scoped := range page.Items {
			if scoped.BackendServices == nil {
				continue
			}
			region := scope
			if i := strings.Index(scope, "/"); i >= 0 {
				region = scope[i+1:]
			}
			for _, bs := range scoped.BackendServices {
				out = append(out, *convertBackendService(bs, region))
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("aggregated list backend services: %w", err)
	}

	return out, nil
}

// DeleteForwardingRule deletes a forwarding rule.
func (c *ComputeClient) DeleteForwardingRule(ctx context.Context, projectID, scope, name string) error {
	if scope == "global" {
		_, err := c.service.GlobalForwardingRules.Delete(projectID, name).Context(ctx).Do()
		return err
	}
	_, err := c.service.ForwardingRules.Delete(projectID, scope, name).Context(ctx).Do()
	return err
}

// DeleteTargetProxy deletes a proxy of the given kind.
func (c *ComputeClient) DeleteTargetProxy(ctx context.Context, projectID, scope, kind, name string) error {
	switch kind {
	case "targetHttpsProxies":
		if scope == "global" {
			_, err := c.service.TargetHttpsProxies.Delete(projectID, name).Context(ctx).Do()
			return err
		}
		_, err := c.service.RegionTargetHttpsProxies.Delete(projectID, scope, name).Context(ctx).Do()
		return err
	case "targetHttpProxies":
		if scope == "global" {
			_, err := c.service.TargetHttpProxies.Delete(projectID, name).Context(ctx).Do()
			return err
		}
		_, err := c.service.RegionTargetHttpProxies.Delete(projectID, scope, name).Context(ctx).Do()
		return err
	case "targetTcpProxies":
		_, err := c.service.TargetTcpProxies.Delete(projectID, name).Context(ctx).Do()
		return err
	case "targetSslProxies":
		_, err := c.service.TargetSslProxies.Delete(projectID, name).Context(ctx).Do()
		return err
	}
	return fmt.Errorf("unsupported proxy kind: %s", kind)
}

// DeleteURLMap deletes a URL map.
func (c *ComputeClient) DeleteURLMap(ctx context.Context, projectID, scope, name string) error {
	if scope == "global" {
		_, err := c.service.UrlMaps.Delete(projectID, name).Context(ctx).Do()
		return err
	}
	_, err := c.service.RegionUrlMaps.Delete(projectID, scope, name).Context(ctx).Do()
	return err
}

// DeleteBackendService deletes a backend service.
func (c *ComputeClient) DeleteBackendService(ctx context.Context, projectID, scope, name string) error {
	if scope == "global" {
		_, err := c.service.BackendServices.Delete(projectID, name).Context(ctx).Do()
		return err
	}
	_, err := c.service.RegionBackendServices.Delete(projectID, scope, name).Context(ctx).Do()
	return err
}

// DeleteHealthCheck deletes a health check.
func (c *ComputeClient) DeleteHealthCheck(ctx context.Context, projectID, scope, name string) error {
	if scope == "global" {
		_, err := c.service.HealthChecks.Delete(projectID, name).Context(ctx).Do()
		return err
	}
	_, err := c.service.RegionHealthChecks.Delete(projectID, scope, name).Context(ctx).Do()
	return err
}
```

- [ ] **Step 2: Build + lint**

```bash
cd /home/vlad/dev/my/gcon && mise exec -- go build ./internal/gcp/... && mise exec -- golangci-lint run ./internal/gcp/...
```

If any compute-API method name doesn't exist (e.g. `UrlMaps.AggregatedList` — some kinds only have `.List`), check the godoc for the actual surface and adapt. For URL maps the aggregated list exists; for HTTP/HTTPS proxies there's only `.List`.

- [ ] **Step 3: Commit**

```bash
git add internal/gcp/loadbalancers.go
git commit -m "2026-05-11: LB phase 1 — list-all + delete methods"
```

---

## Task 6: Message types

**Files:**
- Create: `internal/ui/views/loadbalancer_messages.go`

- [ ] **Step 1: Create the file**

Create `internal/ui/views/loadbalancer_messages.go`:

```go
package views

// LoadBalancerSelectedMsg is emitted by the list view when the user presses
// Enter on a row. The app routes it to ViewLoadBalancerDetails.
type LoadBalancerSelectedMsg struct {
	SelfLink string
	Scope    string
	Name     string
}

// LoadBalancerDeleteRequestMsg is emitted by the details view when the user
// confirms a delete. The app runs the cascade against the GCP client and
// replies with LoadBalancerDeletedMsg.
type LoadBalancerDeleteRequestMsg struct {
	Cascade Cascade
}

// LoadBalancerDeletedMsg is emitted by the app after the delete cascade
// completes (or partially completes). Errs maps each failed CascadeItem
// (keyed by its URL) to the error that occurred.
type LoadBalancerDeletedMsg struct {
	Errs map[string]error
}
```

- [ ] **Step 2: Verify build**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go build ./internal/ui/views/...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/loadbalancer_messages.go
git commit -m "2026-05-11: LB phase 1 — message types"
```

---

## Task 7: Sidebar "Network Services" category + command palette + view enum

**Files:**
- Modify: `internal/ui/components/sidebar/menu.go`
- Modify: `internal/ui/components/commandpalette/commands.go`
- Modify: `internal/ui/components/sidebar/sidebar.go` (or wherever `SidebarView` constants live)

This task adds the navigation surfaces. The views themselves don't exist yet — they'll be added in Task 8 and 9 — but the constants and routing skeleton land here.

- [ ] **Step 1: Add the new sidebar category**

In `internal/ui/components/sidebar/menu.go`, find the existing `VPC Network` category block (around line 122). After it (and before the next category), insert:

```go
		{
			ID:     "network-services",
			Label:  "Network Services",
			Icon:   IconNetwork, // reuse the existing network icon for now; a dedicated icon can be added later
			Hotkey: 'N',
			Type:   MenuItemCategory,
			Children: []MenuItem{
				{ID: "load-balancers", Label: "Load balancing", Icon: IconNetwork, Hotkey: 'l', Type: MenuItemLeaf, ViewType: ViewLoadBalancers},
			},
		},
```

(If `ViewLoadBalancers` doesn't yet exist as a constant in the sidebar package, add it to the `SidebarView` (or equivalent) enum first. Grep for `ViewNetworks` or `ViewSubnets` to find the location.)

- [ ] **Step 2: Add the command palette navigation entry**

In `internal/ui/components/commandpalette/commands.go`, find the existing nav commands (e.g. `nav:networks`). Add the `ViewLoadBalancers` enum constant if missing, an icon constant if missing, and the nav command:

```go
{
    ID:       "nav:load-balancers",
    Label:    "Networking: Load balancing",
    Icon:     IconNetwork, // or a new icon if you want to disambiguate
    Type:     CommandTypeNavigation,
    ViewType: ViewLoadBalancers,
    Enabled:  true,
},
```

Add the `ViewLoadBalancers` enum value in the same file's iota block (sequence must match `app.go`'s `ViewType` enum — keep them aligned; see step 3).

- [ ] **Step 3: Add the App-level `ViewType` constants**

In `internal/ui/app.go`, find the `ViewType` enum (look for `ViewProjects`, `ViewNetworks`, etc.). Add two new entries at the end:

```go
ViewLoadBalancers
ViewLoadBalancerDetails
```

And confirm the corresponding constants exist (and have the same name) in `internal/ui/components/sidebar/sidebar.go` and `internal/ui/components/commandpalette/commands.go`. The three enums are kept in sync by convention.

- [ ] **Step 4: Verify the build**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go build ./...`

If the app's `renderCurrentView` switch warns about un-handled cases or any view-routing function complains, that's expected — those will be filled in by later tasks. If the build fails because `ViewLoadBalancers` is referenced but not yet declared in one of the three places, declare it there.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/components/sidebar/ internal/ui/components/commandpalette/ internal/ui/app.go
git commit -m "2026-05-11: LB phase 1 — Network Services sidebar category + view enum"
```

---

## Task 8: `LoadBalancersView` (list)

**Files:**
- Create: `internal/ui/views/loadbalancers.go`
- Create: `internal/ui/views/loadbalancers_test.go`

This view follows the `firewalls.go` / `networks.go` pattern: embed `TableClickDelegate`, async load via `tea.Cmd`, sort/filter via the standard mechanisms, Enter emits `LoadBalancerSelectedMsg`.

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/views/loadbalancers_test.go`:

```go
package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

func TestLoadBalancersView_LoadedMsg_PopulatesRows(t *testing.T) {
	v := NewLoadBalancersView("proj", nil)
	v.SetSize(120, 30)

	rules := []gcp.ForwardingRule{
		{Name: "front", Scope: "global", Type: "HTTPS (external)", IPAddress: "34.1.2.3", PortRange: "443", Target: "https://x/y/targetHttpsProxies/p"},
		{Name: "back", Scope: "us-central1", Type: "Network LB (passthrough)", IPAddress: "10.0.0.5", PortRange: "80", Target: "https://x/y/backendServices/b"},
	}
	_ = v.Update(loadBalancersLoadedMsg{rules: rules})
	out := v.View()
	assert.Contains(t, out, "front")
	assert.Contains(t, out, "back")
	assert.Contains(t, out, "34.1.2.3")
}

func TestLoadBalancersView_EnterEmitsSelectedMsg(t *testing.T) {
	v := NewLoadBalancersView("proj", nil)
	v.SetSize(120, 30)
	_ = v.Update(loadBalancersLoadedMsg{rules: []gcp.ForwardingRule{
		{Name: "front", Scope: "global", SelfLink: "https://example/global/forwardingRules/front"},
	}})
	v.table.SetCursor(0)

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	sel, ok := msg.(LoadBalancerSelectedMsg)
	require.True(t, ok, "expected LoadBalancerSelectedMsg, got %T", msg)
	assert.Equal(t, "front", sel.Name)
	assert.Equal(t, "global", sel.Scope)
}

func TestLoadBalancersView_ErrorRendered(t *testing.T) {
	v := NewLoadBalancersView("proj", nil)
	v.SetSize(120, 30)
	_ = v.Update(loadBalancersErrorMsg{err: assertErr("boom")})
	out := v.View()
	assert.Contains(t, out, "boom")
}

// sentinel error helper to satisfy err113.
var errLBListTest = stringErr("boom")

type stringErr string

func (s stringErr) Error() string { return string(s) }

func assertErr(s string) error { return stringErr(s) }
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go test ./internal/ui/views/ -run TestLoadBalancersView -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement the view**

Create `internal/ui/views/loadbalancers.go`:

```go
package views

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	uictx "github.com/slayer/gcon/internal/ui/context"
)

// LoadBalancersView is the project-wide list of forwarding rules.
type LoadBalancersView struct {
	TableClickDelegate

	projectID    string
	client       *gcp.ComputeClient
	spinner      components.Spinner
	loading      bool
	err          error
	rules        []gcp.ForwardingRule
	table        table.Model
	width, height int
	keys         loadBalancersKeyMap
}

type loadBalancersKeyMap struct {
	Select  key.Binding
	Refresh key.Binding
}

func defaultLoadBalancersKeyMap() loadBalancersKeyMap {
	return loadBalancersKeyMap{
		Select:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}

// NewLoadBalancersView constructs the list view.
func NewLoadBalancersView(projectID string, client *gcp.ComputeClient) *LoadBalancersView {
	cols := []table.Column{
		{Title: "Name", Width: 24},
		{Title: "Type", Width: 24},
		{Title: "Scope", Width: 16},
		{Title: "IP", Width: 18},
		{Title: "Ports", Width: 14},
		{Title: "Backend", Width: 24},
	}
	t := table.New(cols, "Load balancers")

	v := &LoadBalancersView{
		projectID: projectID,
		client:    client,
		spinner:   components.NewGCPSpinner(),
		loading:   true,
		table:     t,
		keys:      defaultLoadBalancersKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init kicks off the initial load.
func (v *LoadBalancersView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	return tea.Batch(v.spinner.Tick, v.load())
}

func (v *LoadBalancersView) load() tea.Cmd {
	if v.client == nil {
		return func() tea.Msg {
			return loadBalancersErrorMsg{err: fmt.Errorf("compute client not initialized")}
		}
	}
	return func() tea.Msg {
		rules, err := v.client.ListForwardingRules(context.Background(), v.projectID)
		if err != nil {
			return loadBalancersErrorMsg{err: err}
		}
		return loadBalancersLoadedMsg{rules: rules}
	}
}

// SetSize records dimensions and resizes the table.
func (v *LoadBalancersView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.table.SetSize(width, height-3)
}

func (v *LoadBalancersView) SetContext(ctx *uictx.ProgramContext) {
	v.SetSize(ctx.ContentWidth, ctx.ContentHeight)
}

// Update routes messages.
func (v *LoadBalancersView) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case loadBalancersLoadedMsg:
		v.loading = false
		v.err = nil
		v.rules = m.rules
		v.refreshTable()
		return nil

	case loadBalancersErrorMsg:
		v.loading = false
		v.err = m.err
		return nil

	case tea.KeyMsg:
		switch {
		case key.Matches(m, v.keys.Select):
			row := v.table.SelectedRow()
			if row == nil {
				return nil
			}
			rule := v.findRule(row.ID)
			if rule == nil {
				return nil
			}
			selected := *rule
			return func() tea.Msg {
				return LoadBalancerSelectedMsg{
					SelfLink: selected.SelfLink,
					Scope:    selected.Scope,
					Name:     selected.Name,
				}
			}
		case key.Matches(m, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.load())
		}
	}

	cmd, _ := v.table.Update(msg)
	return cmd
}

// View renders the view.
func (v *LoadBalancersView) View() string {
	if v.loading {
		return renderLoading(v.spinner, "Loading load balancers...")
	}
	if v.err != nil {
		return components.RenderError(v.err)
	}
	return v.table.View()
}

func (v *LoadBalancersView) refreshTable() {
	rows := make([]table.Row, 0, len(v.rules))
	for _, r := range v.rules {
		ports := r.PortRange
		if ports == "" && len(r.Ports) > 0 {
			ports = joinStrings(r.Ports, ",")
		}
		rows = append(rows, table.Row{
			ID:   r.SelfLink,
			Data: []string{r.Name, r.Type, r.Scope, r.IPAddress, ports, shortName(r.Target)},
		})
	}
	v.table.SetRows(rows)
}

func (v *LoadBalancersView) findRule(selfLink string) *gcp.ForwardingRule {
	for i := range v.rules {
		if v.rules[i].SelfLink == selfLink {
			return &v.rules[i]
		}
	}
	return nil
}

// joinStrings is a tiny strings.Join wrapper so callers don't need to import
// "strings" for one call site.
func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// shortName extracts the last URL segment.
func shortName(url string) string {
	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == '/' {
			return url[i+1:]
		}
	}
	return url
}

// Internal messages.
type loadBalancersLoadedMsg struct {
	rules []gcp.ForwardingRule
}

type loadBalancersErrorMsg struct {
	err error
}
```

If `shortName` collides with a function of the same name already in the `views` package (it may have been declared in the cascade file as `shortNameURL`), reuse the existing one and remove the duplicate. The test file's expectations stay the same.

- [ ] **Step 4: Run tests**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go test ./internal/ui/views/ -run TestLoadBalancersView -v`
Expected: 3 PASS.

- [ ] **Step 5: Lint**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- golangci-lint run ./internal/ui/views/...`
Expected: 0 issues. (Fix any flagged: dynamic errors via sentinel, etc.)

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/loadbalancers.go internal/ui/views/loadbalancers_test.go
git commit -m "2026-05-11: LB phase 1 — LoadBalancersView (list)"
```

---

## Task 9: `LoadBalancerDetailsView` — tabs + parallel fetch

**Files:**
- Create: `internal/ui/views/loadbalancer_details.go`
- Create: `internal/ui/views/loadbalancer_details_test.go`

This is the largest single task. The details view fetches the proxy chain in parallel, renders three tabs (Overview / Routing / Backends), and triggers the delete cascade on `D`.

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/views/loadbalancer_details_test.go`:

```go
package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func TestLoadBalancerDetailsView_Overview_RendersForwardingRule(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil)
	v.SetSize(120, 30)
	v.rule = &gcp.ForwardingRule{
		Name:                "front",
		Scope:               "global",
		Type:                "HTTPS (external)",
		IPAddress:           "34.1.2.3",
		PortRange:           "443",
		LoadBalancingScheme: "EXTERNAL_MANAGED",
	}
	v.fetchState.fwdLoaded = true
	out := v.View()
	assert.Contains(t, out, "front")
	assert.Contains(t, out, "34.1.2.3")
	assert.Contains(t, out, "HTTPS (external)")
}

func TestLoadBalancerDetailsView_Routing_RendersURLMap(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil)
	v.SetSize(120, 30)
	v.rule = &gcp.ForwardingRule{Name: "front", Scope: "global", Type: "HTTPS (external)"}
	v.urlMap = &gcp.URLMap{
		Name:           "m1",
		DefaultService: "https://x/global/backendServices/default-be",
		HostRules: []gcp.HostRule{
			{Hosts: []string{"*.example.com"}, PathMatcher: "default"},
		},
		PathMatchers: []gcp.PathMatcher{
			{Name: "default", DefaultService: "https://x/global/backendServices/default-be",
				PathRules: []gcp.PathRule{
					{Paths: []string{"/api/*"}, Service: "https://x/global/backendServices/api-be"},
				}},
		},
	}
	v.fetchState.fwdLoaded = true
	v.fetchState.urlMapLoaded = true
	v.tabs.SetActive("routing")
	out := v.View()
	assert.Contains(t, out, "*.example.com")
	assert.Contains(t, out, "/api/*")
	assert.Contains(t, out, "api-be")
}

func TestLoadBalancerDetailsView_Backends_RendersBackendList(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil)
	v.SetSize(120, 30)
	v.rule = &gcp.ForwardingRule{Name: "front", Scope: "global", Type: "HTTPS (external)"}
	v.backends = []gcp.BackendService{
		{
			Name:     "default-be",
			Protocol: "HTTPS",
			Backends: []gcp.Backend{
				{Group: "https://x/zones/us-central1-a/instanceGroups/g1", BalancingMode: "UTILIZATION", CapacityScaler: 1.0},
			},
		},
	}
	v.fetchState.fwdLoaded = true
	v.fetchState.backendsLoaded = true
	v.tabs.SetActive("backends")
	out := v.View()
	assert.Contains(t, out, "default-be")
	assert.Contains(t, out, "g1")
}

func TestLoadBalancerDetailsView_DKey_OpensConfirmDialog(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil)
	v.SetSize(120, 30)
	v.rule = &gcp.ForwardingRule{
		Name:     "front",
		Scope:    "global",
		SelfLink: "https://x/global/forwardingRules/front",
	}
	v.fetchState.fwdLoaded = true
	v.fetchState.sharingChecksLoaded = true // pretend cascade preconditions met
	// Pre-populate inventories used by ComputeCascade.
	v.allFwdRules = []gcp.ForwardingRule{*v.rule}
	// (No proxies / backends so cascade is just the forwarding rule.)

	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	assert.True(t, v.showDeleteConfirm, "D must open the delete-confirm overlay")
	assert.NotNil(t, v.cascade, "cascade must be computed before showing the dialog")
	assert.Len(t, v.cascade.Delete, 1, "cascade includes the forwarding rule")
}
```

Note: this test file relies on a couple of types declared by the implementation — `fetchState`, `showDeleteConfirm`, `cascade`. They are inspected as field accesses in tests; the implementation must keep those names.

- [ ] **Step 2: Run tests to verify failure**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go test ./internal/ui/views/ -run TestLoadBalancerDetailsView -v`
Expected: FAIL — undefined types and methods.

- [ ] **Step 3: Implement the details view**

Create `internal/ui/views/loadbalancer_details.go`:

```go
package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	uictx "github.com/slayer/gcon/internal/ui/context"
)

// LoadBalancerDetailsView is the multi-tab inspector for a single forwarding rule.
type LoadBalancerDetailsView struct {
	projectID string
	scope     string
	name      string
	client    *gcp.ComputeClient

	tabs    *tabs.Tabs
	spinner components.Spinner
	width   int
	height  int

	// Fetched state — set asynchronously.
	rule     *gcp.ForwardingRule
	proxy    *gcp.TargetProxy
	urlMap   *gcp.URLMap
	backends []gcp.BackendService
	checks   []gcp.HealthCheck
	certs    []gcp.SSLCertificate

	// Cascade-preconditions inventory.
	allFwdRules []gcp.ForwardingRule
	allProxies  []gcp.TargetProxy
	allURLMaps  []gcp.URLMap
	allBackends []gcp.BackendService

	fetchState fetchState
	err        error

	// Delete state.
	cascade           *Cascade
	showDeleteConfirm bool
	confirmInput      string
	deleting          bool
	deleteErrs        map[string]error

	keys loadBalancerDetailsKeyMap
}

type fetchState struct {
	fwdLoaded           bool
	proxyLoaded         bool
	urlMapLoaded        bool
	backendsLoaded      bool
	checksLoaded        bool
	certsLoaded         bool
	sharingChecksLoaded bool
}

type loadBalancerDetailsKeyMap struct {
	Refresh key.Binding
	Delete  key.Binding
	Back    key.Binding
}

func defaultLoadBalancerDetailsKeyMap() loadBalancerDetailsKeyMap {
	return loadBalancerDetailsKeyMap{
		Refresh: key.NewBinding(key.WithKeys("r")),
		Delete:  key.NewBinding(key.WithKeys("D")),
		Back:    key.NewBinding(key.WithKeys("esc")),
	}
}

// NewLoadBalancerDetailsView constructs the view.
func NewLoadBalancerDetailsView(projectID, scope, name string, client *gcp.ComputeClient) *LoadBalancerDetailsView {
	t := tabs.New([]tabs.Tab{
		{ID: "overview", Label: "Overview"},
		{ID: "routing", Label: "Routing"},
		{ID: "backends", Label: "Backends"},
	})
	return &LoadBalancerDetailsView{
		projectID: projectID,
		scope:     scope,
		name:      name,
		client:    client,
		tabs:      t,
		spinner:   components.NewGCPSpinner(),
		keys:      defaultLoadBalancerDetailsKeyMap(),
	}
}

// Init begins the parallel fetch.
func (v *LoadBalancerDetailsView) Init() tea.Cmd {
	v.fetchState = fetchState{}
	v.err = nil
	v.cascade = nil
	v.showDeleteConfirm = false
	v.deleteErrs = nil
	return tea.Batch(v.spinner.Tick, v.fetchFwd(), v.fetchSharingInventory())
}

// SetSize records dimensions.
func (v *LoadBalancerDetailsView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetContext mirrors other views.
func (v *LoadBalancerDetailsView) SetContext(ctx *uictx.ProgramContext) {
	v.SetSize(ctx.ContentWidth, ctx.ContentHeight)
}

// Update routes messages.
func (v *LoadBalancerDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case lbFwdLoadedMsg:
		v.rule = m.rule
		v.fetchState.fwdLoaded = true
		// Now that we have the rule, kick off the proxy/urlmap/backend chain.
		return v.fetchChainCmds()
	case lbProxyLoadedMsg:
		v.proxy = m.proxy
		v.fetchState.proxyLoaded = true
		if m.proxy != nil && m.proxy.URLMap != "" {
			return v.fetchURLMap(m.proxy.URLMap)
		}
		if m.proxy != nil && m.proxy.Service != "" {
			return v.fetchBackends([]string{m.proxy.Service})
		}
		return nil
	case lbURLMapLoadedMsg:
		v.urlMap = m.urlMap
		v.fetchState.urlMapLoaded = true
		if m.urlMap == nil {
			return nil
		}
		return v.fetchBackends(collectBackendURLs(*m.urlMap))
	case lbBackendsLoadedMsg:
		v.backends = m.services
		v.fetchState.backendsLoaded = true
		// Aggregate unique health checks.
		seen := map[string]struct{}{}
		hcs := []string{}
		for _, bs := range m.services {
			for _, h := range bs.HealthChecks {
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
	case lbHealthChecksLoadedMsg:
		v.checks = m.checks
		v.fetchState.checksLoaded = true
		return nil
	case lbSharingLoadedMsg:
		v.allFwdRules = m.fwdRules
		v.allProxies = m.proxies
		v.allURLMaps = m.urlMaps
		v.allBackends = m.backends
		v.fetchState.sharingChecksLoaded = true
		return nil
	case lbErrorMsg:
		v.err = m.err
		return nil

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
			return nil
		case key.Matches(m, v.keys.Refresh):
			return v.Init()
		}
		// Forward to tabs for h/l/1/2/3 navigation.
		v.tabs.Update(m)
		return nil
	}
	return nil
}

func (v *LoadBalancerDetailsView) handleConfirmKey(m tea.KeyMsg) tea.Cmd {
	switch m.String() {
	case "esc":
		v.showDeleteConfirm = false
		v.cascade = nil
		v.confirmInput = ""
		return nil
	case "enter":
		if v.confirmInput == v.name && v.cascade != nil {
			v.deleting = true
			v.showDeleteConfirm = false
			cascade := *v.cascade
			return func() tea.Msg {
				return LoadBalancerDeleteRequestMsg{Cascade: cascade}
			}
		}
	case "backspace":
		if len(v.confirmInput) > 0 {
			v.confirmInput = v.confirmInput[:len(v.confirmInput)-1]
		}
	default:
		if len(m.Runes) > 0 {
			v.confirmInput += string(m.Runes)
		}
	}
	return nil
}

// View renders the view.
func (v *LoadBalancerDetailsView) View() string {
	if v.err != nil {
		return components.RenderError(v.err)
	}
	if !v.fetchState.fwdLoaded {
		return renderLoading(v.spinner, "Loading load balancer...")
	}

	var b strings.Builder
	b.WriteString(v.tabs.View())
	b.WriteString("\n\n")

	switch v.tabs.ActiveTab().ID {
	case "overview":
		b.WriteString(v.renderOverview())
	case "routing":
		b.WriteString(v.renderRouting())
	case "backends":
		b.WriteString(v.renderBackends())
	}

	if v.showDeleteConfirm && v.cascade != nil {
		b.WriteString("\n\n" + v.renderConfirmDialog())
	}
	return b.String()
}

func (v *LoadBalancerDetailsView) renderOverview() string {
	r := v.rule
	if r == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Name:    %s\n", r.Name))
	b.WriteString(fmt.Sprintf("Type:    %s\n", r.Type))
	b.WriteString(fmt.Sprintf("Scope:   %s\n", r.Scope))
	b.WriteString(fmt.Sprintf("IP:      %s\n", r.IPAddress))
	ports := r.PortRange
	if ports == "" {
		ports = strings.Join(r.Ports, ",")
	}
	b.WriteString(fmt.Sprintf("Ports:   %s\n", ports))
	b.WriteString(fmt.Sprintf("Scheme:  %s\n", r.LoadBalancingScheme))
	if r.Network != "" {
		b.WriteString(fmt.Sprintf("Network: %s\n", r.Network))
	}
	if r.Subnetwork != "" {
		b.WriteString(fmt.Sprintf("Subnet:  %s\n", r.Subnetwork))
	}
	if v.proxy != nil {
		b.WriteString(fmt.Sprintf("\nProxy:   %s (%s)\n", v.proxy.Name, v.proxy.Kind))
		if v.proxy.SSLPolicy != "" {
			b.WriteString(fmt.Sprintf("  SSL policy: %s\n", v.proxy.SSLPolicy))
		}
	}
	for _, c := range v.certs {
		b.WriteString(fmt.Sprintf("Cert:    %s (%s) status=%s\n", c.Name, c.Type, c.Status))
	}
	return b.String()
}

func (v *LoadBalancerDetailsView) renderRouting() string {
	if v.urlMap == nil {
		return "(no URL map — Network LB or pending)"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Default service: %s\n\n", shortName(v.urlMap.DefaultService)))
	for _, hr := range v.urlMap.HostRules {
		b.WriteString(fmt.Sprintf("Hosts: %s → matcher: %s\n", strings.Join(hr.Hosts, ", "), hr.PathMatcher))
	}
	for _, pm := range v.urlMap.PathMatchers {
		b.WriteString(fmt.Sprintf("\nMatcher %s (default → %s)\n", pm.Name, shortName(pm.DefaultService)))
		for _, pr := range pm.PathRules {
			b.WriteString(fmt.Sprintf("  %s → %s\n", strings.Join(pr.Paths, ", "), shortName(pr.Service)))
		}
	}
	return b.String()
}

func (v *LoadBalancerDetailsView) renderBackends() string {
	if !v.fetchState.backendsLoaded {
		return "Loading backends..."
	}
	if len(v.backends) == 0 {
		return "(no backends)"
	}
	var b strings.Builder
	for _, bs := range v.backends {
		b.WriteString(fmt.Sprintf("Backend service: %s\n", bs.Name))
		b.WriteString(fmt.Sprintf("  Protocol: %s  Timeout: %ds  Affinity: %s\n", bs.Protocol, bs.TimeoutSec, bs.SessionAffinity))
		for _, be := range bs.Backends {
			b.WriteString(fmt.Sprintf("    Group: %s  Mode: %s  Cap: %.2f\n", shortName(be.Group), be.BalancingMode, be.CapacityScaler))
		}
		for _, hcURL := range bs.HealthChecks {
			b.WriteString(fmt.Sprintf("    Health check: %s\n", shortName(hcURL)))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (v *LoadBalancerDetailsView) renderConfirmDialog() string {
	if v.cascade == nil {
		return ""
	}
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Bold(true)
	var b strings.Builder
	b.WriteString(red.Render(fmt.Sprintf("Delete load balancer: %s", v.name)))
	b.WriteString("\n\nWill delete:\n")
	for _, it := range v.cascade.Delete {
		b.WriteString(fmt.Sprintf("  %s: %s (%s)\n", it.Kind, it.Name, it.Scope))
	}
	if len(v.cascade.Keep) > 0 {
		b.WriteString("\nWill keep (still in use):\n")
		for _, k := range v.cascade.Keep {
			b.WriteString(fmt.Sprintf("  %s: %s — %s\n", k.Kind, k.Name, strings.Join(k.KeptBecause, ", ")))
		}
	}
	b.WriteString(fmt.Sprintf("\nType the LB name to confirm: %s_\n", v.confirmInput))
	b.WriteString("\n[Enter] Delete  [Esc] Cancel\n")
	return b.String()
}

// --- async fetch helpers ---

func (v *LoadBalancerDetailsView) fetchFwd() tea.Cmd {
	if v.client == nil {
		return func() tea.Msg { return lbErrorMsg{err: fmt.Errorf("compute client not initialized")} }
	}
	return func() tea.Msg {
		fr, err := v.client.GetForwardingRule(context.Background(), v.projectID, v.scope, v.name)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		return lbFwdLoadedMsg{rule: fr}
	}
}

func (v *LoadBalancerDetailsView) fetchChainCmds() tea.Cmd {
	if v.rule == nil || v.rule.Target == "" {
		return nil
	}
	kind := targetKindFromURL(v.rule.Target)
	switch kind {
	case "targetHttpProxies", "targetHttpsProxies", "targetTcpProxies", "targetSslProxies":
		return v.fetchProxy(v.rule.Target)
	case "backendServices":
		return v.fetchBackends([]string{v.rule.Target})
	}
	return nil
}

func (v *LoadBalancerDetailsView) fetchProxy(url string) tea.Cmd {
	scope := scopeFromURL(url)
	if v.client == nil {
		return func() tea.Msg { return lbErrorMsg{err: fmt.Errorf("compute client not initialized")} }
	}
	return func() tea.Msg {
		tp, err := v.client.GetTargetProxy(context.Background(), v.projectID, scope, url)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		return lbProxyLoadedMsg{proxy: tp}
	}
}

func (v *LoadBalancerDetailsView) fetchURLMap(url string) tea.Cmd {
	scope := scopeFromURL(url)
	if v.client == nil {
		return nil
	}
	return func() tea.Msg {
		um, err := v.client.GetURLMap(context.Background(), v.projectID, scope, url)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		return lbURLMapLoadedMsg{urlMap: um}
	}
}

func (v *LoadBalancerDetailsView) fetchBackends(urls []string) tea.Cmd {
	if v.client == nil || len(urls) == 0 {
		return nil
	}
	return func() tea.Msg {
		var services []gcp.BackendService
		for _, u := range urls {
			bs, err := v.client.GetBackendService(context.Background(), v.projectID, scopeFromURL(u), u)
			if err != nil {
				return lbErrorMsg{err: err}
			}
			services = append(services, *bs)
		}
		return lbBackendsLoadedMsg{services: services}
	}
}

func (v *LoadBalancerDetailsView) fetchHealthChecks(urls []string) tea.Cmd {
	if v.client == nil || len(urls) == 0 {
		return nil
	}
	return func() tea.Msg {
		var checks []gcp.HealthCheck
		for _, u := range urls {
			hc, err := v.client.GetHealthCheck(context.Background(), v.projectID, scopeFromURL(u), u)
			if err != nil {
				return lbErrorMsg{err: err}
			}
			checks = append(checks, *hc)
		}
		return lbHealthChecksLoadedMsg{checks: checks}
	}
}

func (v *LoadBalancerDetailsView) fetchSharingInventory() tea.Cmd {
	if v.client == nil {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		fwds, err := v.client.ListForwardingRules(ctx, v.projectID)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		proxies, err := v.client.ListAllProxies(ctx, v.projectID)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		urlMaps, err := v.client.ListAllURLMaps(ctx, v.projectID)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		backends, err := v.client.ListAllBackendServices(ctx, v.projectID)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		return lbSharingLoadedMsg{fwdRules: fwds, proxies: proxies, urlMaps: urlMaps, backends: backends}
	}
}

func collectBackendURLs(um gcp.URLMap) []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(um.DefaultService)
	for _, pm := range um.PathMatchers {
		add(pm.DefaultService)
		for _, pr := range pm.PathRules {
			add(pr.Service)
		}
	}
	return out
}

// Internal messages.
type lbFwdLoadedMsg struct{ rule *gcp.ForwardingRule }
type lbProxyLoadedMsg struct{ proxy *gcp.TargetProxy }
type lbURLMapLoadedMsg struct{ urlMap *gcp.URLMap }
type lbBackendsLoadedMsg struct{ services []gcp.BackendService }
type lbHealthChecksLoadedMsg struct{ checks []gcp.HealthCheck }
type lbSharingLoadedMsg struct {
	fwdRules []gcp.ForwardingRule
	proxies  []gcp.TargetProxy
	urlMaps  []gcp.URLMap
	backends []gcp.BackendService
}
type lbErrorMsg struct{ err error }
```

- [ ] **Step 4: Run tests**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go test ./internal/ui/views/ -run TestLoadBalancerDetailsView -v`

The 4 named tests must pass. Other tests in the package must still pass.

If a test fails because of a function signature mismatch (e.g., `tabs.SetActive` doesn't exist), inspect the tabs component and adapt — common alternatives are `tabs.Activate("routing")` or `tabs.SetActiveByID("routing")`.

- [ ] **Step 5: Lint**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- golangci-lint run ./internal/ui/views/...`
Expected: 0 issues. Replace any dynamic `fmt.Errorf("...")` calls flagged by `err113` with sentinels following the pattern from `internal/ssh/`.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/loadbalancer_details.go internal/ui/views/loadbalancer_details_test.go
git commit -m "2026-05-11: LB phase 1 — LoadBalancerDetailsView (tabs + parallel fetch + delete trigger)"
```

---

## Task 10: App-level routing + delete execution

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_render.go`
- Modify: `internal/ui/app_navigation.go`

This task connects the new views into the app:
1. Adds view-instance fields and renders them.
2. Routes `LoadBalancerSelectedMsg` → push to `ViewLoadBalancerDetails`.
3. Routes `LoadBalancerDeleteRequestMsg` → run the cascade, emit `LoadBalancerDeletedMsg`.
4. Routes `LoadBalancerDeletedMsg` → pop back to list, refresh.
5. Sidebar navigation handlers.
6. `clearAllViews` cleanup.

- [ ] **Step 1: Add view-instance fields to `App`**

In `internal/ui/app.go`, find the `App` struct. Add:

```go
loadBalancersView        *views.LoadBalancersView
loadBalancerDetailsView  *views.LoadBalancerDetailsView
```

- [ ] **Step 2: Add render cases**

In `internal/ui/app_render.go` (or wherever `renderCurrentView` lives), find the switch. Add:

```go
case ViewLoadBalancers:
    if a.loadBalancersView != nil {
        return a.loadBalancersView.View()
    }
case ViewLoadBalancerDetails:
    if a.loadBalancerDetailsView != nil {
        return a.loadBalancerDetailsView.View()
    }
```

- [ ] **Step 3: Add navigation handlers**

In `internal/ui/app_navigation.go`, look for how `ViewNetworks` is initialized when the user clicks the sidebar entry. Add the equivalent for load balancers:

```go
case sidebar.ViewLoadBalancers:
    if a.currentView != ViewLoadBalancers && a.currentView != ViewLoadBalancerDetails {
        a.pushView(ViewLoadBalancers)
        a.loadBalancersView = views.NewLoadBalancersView(a.selectedProject.ID, a.computeClient)
        a.updateViewSizes()
        return a, a.loadBalancersView.Init()
    }
```

(Use whatever the project's sidebar-active-view → app-view glue is. Grep `case sidebar.ViewNetworks` for the existing pattern.)

- [ ] **Step 4: Add message handlers in `Update`**

In `internal/ui/app.go`'s `(*App).Update`, add cases:

```go
case views.LoadBalancerSelectedMsg:
    a.pushView(ViewLoadBalancerDetails)
    a.loadBalancerDetailsView = views.NewLoadBalancerDetailsView(
        a.selectedProject.ID, msg.Scope, msg.Name, a.computeClient,
    )
    a.updateViewSizes()
    return a, a.loadBalancerDetailsView.Init()

case views.LoadBalancerDeleteRequestMsg:
    return a, a.executeLoadBalancerDelete(msg.Cascade)

case views.LoadBalancerDeletedMsg:
    // Pop details, refresh list.
    a.popView()
    if a.loadBalancersView != nil {
        return a, a.loadBalancersView.Init()
    }
    return a, nil
```

- [ ] **Step 5: Add the delete-execution helper**

Append to `internal/ui/app.go`:

```go
// executeLoadBalancerDelete runs the cascade in dependency order:
// forwarding rule → proxy → url map → backend services (parallel) → health checks (parallel).
// Returns LoadBalancerDeletedMsg with a per-URL error map; map is empty on full success.
func (a *App) executeLoadBalancerDelete(c views.Cascade) tea.Cmd {
	client := a.computeClient
	projectID := a.selectedProject.ID
	return func() tea.Msg {
		ctx := context.Background()
		errs := map[string]error{}

		// Partition cascade items by kind for ordered execution.
		var fwd, proxy, urlMap views.CascadeItem
		var backends, checks []views.CascadeItem
		for _, it := range c.Delete {
			switch it.Kind {
			case "forwardingRule":
				fwd = it
			case "targetHttpProxies", "targetHttpsProxies", "targetTcpProxies", "targetSslProxies":
				proxy = it
			case "urlMap":
				urlMap = it
			case "backendService":
				backends = append(backends, it)
			case "healthCheck":
				checks = append(checks, it)
			}
		}

		// Step 1: forwarding rule.
		if fwd.URL != "" {
			if err := client.DeleteForwardingRule(ctx, projectID, fwd.Scope, fwd.Name); err != nil {
				errs[fwd.URL] = err
				return views.LoadBalancerDeletedMsg{Errs: errs}
			}
		}
		// Step 2: proxy.
		if proxy.URL != "" {
			if err := client.DeleteTargetProxy(ctx, projectID, proxy.Scope, proxy.Kind, proxy.Name); err != nil {
				errs[proxy.URL] = err
				return views.LoadBalancerDeletedMsg{Errs: errs}
			}
		}
		// Step 3: url map.
		if urlMap.URL != "" {
			if err := client.DeleteURLMap(ctx, projectID, urlMap.Scope, urlMap.Name); err != nil {
				errs[urlMap.URL] = err
				return views.LoadBalancerDeletedMsg{Errs: errs}
			}
		}
		// Step 4: backends (parallel).
		for _, it := range backends {
			if err := client.DeleteBackendService(ctx, projectID, it.Scope, it.Name); err != nil {
				errs[it.URL] = err
			}
		}
		// Step 5: health checks (parallel).
		for _, it := range checks {
			if err := client.DeleteHealthCheck(ctx, projectID, it.Scope, it.Name); err != nil {
				errs[it.URL] = err
			}
		}
		return views.LoadBalancerDeletedMsg{Errs: errs}
	}
}
```

Note: Phase 1 runs backends and health checks sequentially inside the goroutine; "parallel" within those steps is a future optimization (would use `errgroup`).

- [ ] **Step 6: Update `clearAllViews`**

In `internal/ui/app_navigation.go`, find `clearAllViews` and add:

```go
a.loadBalancersView = nil
a.loadBalancerDetailsView = nil
```

- [ ] **Step 7: Add sidebar guards**

In whatever function maps `a.currentView` → sidebar active-view, add cases for `ViewLoadBalancers` and `ViewLoadBalancerDetails` mapping to `sidebar.ViewLoadBalancers`.

- [ ] **Step 8: Build + test + lint**

```bash
cd /home/vlad/dev/my/gcon && mise exec -- go build ./... && mise exec -- go test ./... && mise exec -- golangci-lint run ./...
```

Expected: clean build, all tests pass, 0 lint issues.

- [ ] **Step 9: Commit**

```bash
git add internal/ui/app.go internal/ui/app_render.go internal/ui/app_navigation.go
git commit -m "2026-05-11: LB phase 1 — app routing + delete execution"
```

---

## Task 11: Documentation

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `.claude/rules/key-bindings.md`

- [ ] **Step 1: Update `.claude/rules/key-bindings.md`**

Add a new section after the existing networking-area sections:

```markdown
## Load Balancers View

| Key | Action |
|-----|--------|
| `Enter` | View load balancer details |
| `S` | Open sort menu |
| `/` | Filter load balancers |
| `r` | Refresh list |
| `Esc` | Go back |

## Load Balancer Details View

| Key | Action |
|-----|--------|
| `D` | Delete LB (with dependency cascade) |
| `r` | Refresh details |
| `Tab` | Switch focus (tabs / content) |
| `h/l` or `1/2/3` | Switch tabs (Overview / Routing / Backends) |
| `Esc` | Go back |
```

- [ ] **Step 2: Update `CLAUDE.md`**

Under "Implemented Features", before the "Planned Features" header, add:

```markdown
- [x] Load Balancers (Phase 1)
  - List forwarding rules across global + all regions, with type derivation
    (HTTPS external / internal, HTTP, TCP/SSL proxy, Network LB proxy/passthrough/legacy)
  - Details view with three tabs: Overview, Routing (URL map host/path → backend),
    Backends (instance groups / NEGs, health checks, balancing mode)
  - Delete with dependency cascade: walks proxy → URL map → backend services →
    health checks, skipping shared resources; type-to-confirm dialog lists every
    resource that will be deleted and every shared one that will be kept
```

- [ ] **Step 3: Update `README.md`**

Find the Compute Engine or Networking feature section. Add a Load Balancing bullet pointing readers at the new view; one short paragraph is enough.

- [ ] **Step 4: Commit**

```bash
git add README.md CLAUDE.md .claude/rules/key-bindings.md
git commit -m "2026-05-11: LB phase 1 — documentation"
```

---

## Task 12: Final validation + PR

**Files:** none modified; verification only.

- [ ] **Step 1: Full test suite**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go test ./...`
Expected: all green.

- [ ] **Step 2: Lint**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- golangci-lint run ./...`
Expected: 0 issues.

- [ ] **Step 3: Build**

Run: `cd /home/vlad/dev/my/gcon && mise exec -- go build ./...`
Expected: clean.

- [ ] **Step 4: Push the branch**

```bash
git push -u origin 2026-05-11-load-balancers-phase1
```

- [ ] **Step 5: Open PR**

```bash
gh pr create --title "Load Balancers Phase 1 — inventory, details, delete cascade" --body "$(cat <<'EOF'
## Summary

- New "Network Services" sidebar category (sibling to "VPC Network"), with "Load balancing" as the first leaf.
- Inventory view: lists every forwarding rule in the project across global + all regions, with derived type column.
- Details view (three tabs: Overview / Routing / Backends) that resolves the proxy → URL map → backend services chain in parallel.
- Delete with dependency cascade: walks the dependency graph, distinguishes shared resources, type-to-confirm dialog showing exactly what will be deleted and what will be kept.

## Test plan

- [x] `ComputeCascade` table-driven tests (dedicated / shared backend / shared proxy / Network LB direct / legacy target pool / missing references)
- [x] Type derivation table-driven tests
- [x] List view tests (load → render → Enter emits SelectedMsg)
- [x] Details view tests (Overview / Routing / Backends rendering + D opens confirm)
- [x] `make test` green
- [x] `make lint` clean
- [ ] Manual: navigate to Network Services → Load balancing in a real project, drill into an LB, attempt a delete with shared backend → verify "Will keep" section

Spec: \`doc/2026-05-11-load-balancers-phase1/Design.md\`
Plan: \`doc/2026-05-11-load-balancers-phase1/TODO.md\`
EOF
)"
```

---

## Self-review checklist

Run through before handoff:

1. **Spec coverage** — every section of Design.md maps to one or more tasks:
   - List view (Section 2 of spec) → Task 8
   - Details view tabs (Section 3) → Task 9
   - Delete cascade (Section 4) → Tasks 4 (pure fn), 9 (UI), 10 (execution)
   - Architecture / files (Section 5) → Tasks 1, 2, 3, 5, 6, 7, 8, 9, 10
   - Sidebar entry → Task 7
   - Testing strategy → embedded in each task's TDD steps

2. **No placeholders** — every step has concrete code or commands.

3. **Type consistency** — `ComputeCascade`, `Cascade`, `CascadeItem`, `CascadeKept`, `LoadBalancerSelectedMsg`, `LoadBalancerDeleteRequestMsg`, `LoadBalancerDeletedMsg`, `ForwardingRule`, `TargetProxy`, `URLMap`, `BackendService`, `HealthCheck`, `SSLCertificate`, `DeriveLoadBalancerType`, `targetKind`, `shortName`, `scopeFromURL`, all referenced consistently across tasks.

4. **TDD where it makes sense** — pure-function tasks (1, 4) and view tasks (8, 9) lead with failing tests; mechanical wiring tasks (5, 6, 7, 10, 11) use build/lint as the green signal.
