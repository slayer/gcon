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
