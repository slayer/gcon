package gcp

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/compute/v1"
)

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
	return convertURLMap(um, scope), nil
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
	var port int64
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
			Name:       sc.Name,
			SelfLink:   sc.SelfLink,
			Scope:      "global",
			Type:       sc.Type,
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
		Name:       sc.Name,
		SelfLink:   sc.SelfLink,
		Scope:      scope,
		Type:       sc.Type,
		ExpireTime: sc.ExpireTime,
	}
	if sc.Managed != nil {
		out.Status = sc.Managed.Status
	}
	return out, nil
}

var errUnsupportedProxyKind = fmt.Errorf("unsupported target proxy kind")

// GetTargetProxy fetches a target proxy. The kind (one of targetHttpProxies /
// targetHttpsProxies / targetTcpProxies / targetSslProxies) is inferred from
// the URL via targetKind. scope is "global" or a region.
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
		return nil, fmt.Errorf("%w: %s", errUnsupportedProxyKind, kind)
	}

	return tp, nil
}
