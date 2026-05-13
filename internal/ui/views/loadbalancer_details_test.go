package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

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
