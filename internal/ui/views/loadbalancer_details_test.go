package views

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

func TestLoadBalancerDetailsView_Overview_RendersForwardingRule(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
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
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
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
	v.tabs.SetActiveByID("routing")
	out := v.View()
	assert.Contains(t, out, "*.example.com")
	assert.Contains(t, out, "/api/*")
	assert.Contains(t, out, "api-be")
}

func TestLoadBalancerDetailsView_Backends_RendersBackendList(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
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
	v.tabs.SetActiveByID("backends")
	out := v.View()
	assert.Contains(t, out, "default-be")
	assert.Contains(t, out, "g1")
}

func TestLoadBalancerDetailsView_DKey_OpensConfirmDialog(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.SetSize(120, 30)
	v.rule = &gcp.ForwardingRule{
		Name:     "front",
		Scope:    "global",
		SelfLink: "https://x/global/forwardingRules/front",
	}
	v.fetchState.fwdLoaded = true
	v.fetchState.sharingChecksLoaded = true
	v.allFwdRules = []gcp.ForwardingRule{*v.rule}

	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	assert.True(t, v.showDeleteConfirm, "D must open the delete-confirm overlay")
	assert.NotNil(t, v.cascade, "cascade must be computed before showing the dialog")
	assert.Len(t, v.cascade.Delete, 1, "cascade includes the forwarding rule")
}

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

func TestGroupScope(t *testing.T) {
	cases := map[string]string{
		"https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/instanceGroups/ig-prod":   "us-central1-a",
		"https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/networkEndpointGroups/neg": "us-central1",
		"https://www.googleapis.com/compute/v1/projects/p/global/networkEndpointGroups/global-neg":      "global",
		"": "",
	}
	for url, want := range cases {
		t.Run(url, func(t *testing.T) {
			assert.Equal(t, want, groupScope(url))
		})
	}
}

var errTestBoom = errors.New("boom")

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

	v.Update(lbGroupHealthErrorMsg{groupURL: "ig-2", err: errTestBoom})
	st = v.groupHealth["ig-2"]
	assert.Equal(t, groupHealthErrored, st.phase)
	assert.EqualError(t, st.err, "boom")

	v.Update(lbGroupSkippedMsg{groupURL: "neg-1", reason: "serverless NEG"})
	st = v.groupHealth["neg-1"]
	assert.Equal(t, groupHealthSkipped, st.phase)
	assert.Equal(t, "serverless NEG", st.reason)
}

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
