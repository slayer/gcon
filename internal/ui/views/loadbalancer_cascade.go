package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/slayer/gcon/internal/gcp"
)

// CascadeItem is a resource scheduled for deletion.
type CascadeItem struct {
	Kind  string
	Name  string
	Scope string
	URL   string
}

// CascadeKept is a resource the cascade would have liked to delete but kept
// because something else still references it.
type CascadeKept struct {
	Kind        string
	Name        string
	Scope       string
	KeptBecause []string
}

// Cascade is the result of computing what a single-LB delete will remove.
type Cascade struct {
	Delete []CascadeItem
	Keep   []CascadeKept
}

// ComputeCascade returns the set of resources that should be deleted to
// remove the given forwarding rule, plus the set of resources that would
// have been deleted but are shared with another LB.
//
// The function is pure — no API calls. The caller is responsible for
// providing the full project-wide inventory of forwarding rules, proxies,
// URL maps, and backend services.
func ComputeCascade(
	rule gcp.ForwardingRule,
	allFwdRules []gcp.ForwardingRule,
	allProxies []gcp.TargetProxy,
	allURLMaps []gcp.URLMap,
	allBackends []gcp.BackendService,
) Cascade {
	var c Cascade

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
		c = cascadeDirectBackend(c, rule.Target, rule.SelfLink, allFwdRules, allURLMaps, allBackends)
	case "targetPools":
		// Legacy: cascade stops at the forwarding rule in Phase 1.
	}

	return c
}

func cascadeProxy(
	c Cascade,
	rule gcp.ForwardingRule,
	allFwdRules []gcp.ForwardingRule,
	allProxies []gcp.TargetProxy,
	allURLMaps []gcp.URLMap,
	allBackends []gcp.BackendService,
	kind string,
) Cascade {
	proxyURL := rule.Target
	proxy := findProxy(allProxies, proxyURL)

	otherUsers := otherForwardingRulesUsing(allFwdRules, rule.SelfLink, proxyURL)
	if len(otherUsers) > 0 {
		c.Keep = append(c.Keep, CascadeKept{
			Kind:        kind,
			Name:        shortNameURL(proxyURL),
			Scope:       scopeFromURL(proxyURL),
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

	if proxy == nil {
		return c
	}

	if proxy.URLMap != "" {
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
		// Only cascade into the URL map if we have its data — otherwise
		// we can't walk its backends and adding it to Delete blind could
		// reference a non-existent resource.
		um := findURLMap(allURLMaps, urlMapURL)
		if um == nil {
			return c
		}
		c.Delete = append(c.Delete, CascadeItem{
			Kind:  "urlMap",
			Name:  shortNameURL(urlMapURL),
			Scope: scopeFromURL(urlMapURL),
			URL:   urlMapURL,
		})
		c = cascadeBackendsFromURLMap(c, *um, urlMapURL, allURLMaps, allFwdRules, allBackends)
	} else if proxy.Service != "" {
		// TCP/SSL proxies point directly at a backend service.
		c = cascadeBackend(c, proxy.Service, allFwdRules, allURLMaps, allBackends)
	}

	return c
}

func cascadeBackendsFromURLMap(
	c Cascade,
	um gcp.URLMap,
	urlMapURL string,
	allURLMaps []gcp.URLMap,
	allFwdRules []gcp.ForwardingRule,
	allBackends []gcp.BackendService,
) Cascade {
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

	// Sort backend URLs for deterministic cascade order — the confirm dialog
	// and the delete sequence must be stable across runs.
	beURLs := make([]string, 0, len(beSet))
	for u := range beSet {
		beURLs = append(beURLs, u)
	}
	sort.Strings(beURLs)

	for _, beURL := range beURLs {
		otherMaps := urlMapsReferencingExcept(allURLMaps, beURL, urlMapURL)
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

		bs := findBackend(allBackends, beURL)
		if bs != nil {
			c = cascadeHealthChecks(c, bs.HealthChecks, beURL, allBackends)
		}
	}
	return c
}

func cascadeBackend(
	c Cascade,
	beURL string,
	allFwdRules []gcp.ForwardingRule,
	allURLMaps []gcp.URLMap,
	allBackends []gcp.BackendService,
) Cascade {
	otherMaps := urlMapsReferencing(allURLMaps, beURL)
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

// cascadeDirectBackend handles a forwarding rule whose target is a backend
// service (Network LB case).
func cascadeDirectBackend(
	c Cascade,
	beURL string,
	ourFwdSelfLink string,
	allFwdRules []gcp.ForwardingRule,
	allURLMaps []gcp.URLMap,
	allBackends []gcp.BackendService,
) Cascade {
	otherMaps := urlMapsReferencing(allURLMaps, beURL)
	others := []string{}
	for i := range allFwdRules {
		fr := &allFwdRules[i]
		if fr.SelfLink == ourFwdSelfLink {
			continue
		}
		if fr.Target == beURL {
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
		others := []string{}
		for i := range allBackends {
			bs := &allBackends[i]
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

// otherForwardingRulesUsing returns the names of forwarding rules (other than
// the one identified by excludeSelfLink) that target the given URL.
// SelfLink is globally unique; Name is not (it can collide across regions).
func otherForwardingRulesUsing(all []gcp.ForwardingRule, excludeSelfLink, target string) []string {
	out := []string{}
	for i := range all {
		fr := &all[i]
		if fr.SelfLink == excludeSelfLink {
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
	for i := range all {
		p := &all[i]
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
	for i := range all {
		fr := &all[i]
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

// targetKindFromURL extracts the second-to-last segment of a Compute self-link.
func targetKindFromURL(url string) string {
	last := strings.LastIndex(url, "/")
	if last < 0 {
		return ""
	}
	prev := strings.LastIndex(url[:last], "/")
	if prev < 0 {
		return ""
	}
	return url[prev+1 : last]
}

func shortNameURL(url string) string {
	idx := strings.LastIndex(url, "/")
	if idx < 0 {
		return url
	}
	return url[idx+1:]
}

// scopeFromURL returns "global" or a region name based on the URL shape:
//
//	".../global/<kind>/<name>"        → "global"
//	".../regions/<r>/<kind>/<name>"   → "<r>"
func scopeFromURL(url string) string {
	if strings.Contains(url, "/global/") {
		return "global"
	}
	if idx := strings.Index(url, "/regions/"); idx >= 0 {
		rest := url[idx+len("/regions/"):]
		end := strings.Index(rest, "/")
		if end < 0 {
			return rest
		}
		return rest[:end]
	}
	return ""
}
