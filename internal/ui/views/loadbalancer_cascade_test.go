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
	otherURLMap := mkURLMap("m2", bs1URL)
	proxies := []gcp.TargetProxy{mkProxy("p1", urlMapURL), mkProxy("p2", otherURLMap.SelfLink)}
	urlMaps := []gcp.URLMap{mkURLMap("m1", bs1URL), otherURLMap}
	backends := []gcp.BackendService{mkBackend("b1")}

	otherFwd := mkFwd("f2", proxies[1].SelfLink)

	c := ComputeCascade(rule, []gcp.ForwardingRule{rule, otherFwd}, proxies, urlMaps, backends)

	for _, it := range c.Delete {
		assert.NotEqual(t, "backendService", it.Kind)
	}
	if assert.Len(t, c.Keep, 1) {
		assert.Equal(t, "b1", c.Keep[0].Name)
	}
}

func TestComputeCascade_SharedProxy_KeptCascadeStopsAtProxy(t *testing.T) {
	proxyURL := "https://example/global/targetHttpsProxies/p1"
	urlMapURL := "https://example/global/urlMaps/m1"
	bs1URL := "https://example/global/backendServices/b1"

	rule := mkFwd("f1", proxyURL)
	otherRuleSameProxy := mkFwd("f2", proxyURL)
	proxies := []gcp.TargetProxy{mkProxy("p1", urlMapURL)}
	urlMaps := []gcp.URLMap{mkURLMap("m1", bs1URL)}
	backends := []gcp.BackendService{mkBackend("b1")}

	c := ComputeCascade(rule, []gcp.ForwardingRule{rule, otherRuleSameProxy}, proxies, urlMaps, backends)

	kinds := []string{}
	for _, it := range c.Delete {
		kinds = append(kinds, it.Kind)
	}
	assert.Equal(t, []string{"forwardingRule"}, kinds, "only the forwarding rule should be deleted; proxy is shared")
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
	assert.Equal(t, []string{"forwardingRule"}, kinds)
}

func TestComputeCascade_CrossRegionNameCollision_StillCascadesProxy(t *testing.T) {
	// Two forwarding rules share a Name ("frontend") in different scopes,
	// pointing at *different* proxies. Deleting one must not be tricked into
	// keeping its proxy alive because of the same-named rule in another scope.
	proxyURL := "https://example/global/targetHttpsProxies/p1"
	urlMapURL := "https://example/global/urlMaps/m1"
	bs1URL := "https://example/global/backendServices/b1"

	rule := mkFwd("frontend", proxyURL)
	rule.Scope = "global"
	rule.SelfLink = "https://example/global/forwardingRules/frontend"

	collider := gcp.ForwardingRule{
		Name:     "frontend",
		Scope:    "us-central1",
		SelfLink: "https://example/regions/us-central1/forwardingRules/frontend",
		Target:   "https://example/regions/us-central1/targetHttpsProxies/p2", // unrelated proxy
	}

	proxies := []gcp.TargetProxy{mkProxy("p1", urlMapURL)}
	urlMaps := []gcp.URLMap{mkURLMap("m1", bs1URL)}
	backends := []gcp.BackendService{mkBackend("b1")}

	c := ComputeCascade(rule, []gcp.ForwardingRule{rule, collider}, proxies, urlMaps, backends)

	kinds := []string{}
	for _, it := range c.Delete {
		kinds = append(kinds, it.Kind)
	}
	assert.Equal(t, []string{"forwardingRule", "targetHttpsProxies", "urlMap", "backendService"}, kinds,
		"name collision in another scope must not block proxy cascade")
}

func TestComputeCascade_MissingReferences_NoPanic(t *testing.T) {
	proxyURL := "https://example/global/targetHttpsProxies/p1"
	urlMapURL := "https://example/global/urlMaps/missing"

	rule := mkFwd("f1", proxyURL)
	proxies := []gcp.TargetProxy{mkProxy("p1", urlMapURL)}

	c := ComputeCascade(rule, []gcp.ForwardingRule{rule}, proxies, nil, nil)

	kinds := []string{}
	for _, it := range c.Delete {
		kinds = append(kinds, it.Kind)
	}
	assert.Equal(t, []string{"forwardingRule", "targetHttpsProxies"}, kinds)
}
