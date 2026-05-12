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
