package views

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/tabs"
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

func TestRenderHealthSummaryAbbreviatesAboveFive(t *testing.T) {
	statuses := make([]gcp.InstanceHealth, 7)
	for i := range statuses {
		statuses[i] = gcp.InstanceHealth{Instance: "vm", HealthState: "HEALTHY"}
	}
	statuses[6].HealthState = "UNHEALTHY"
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	out := v.renderHealthSummary(statuses)
	assert.Contains(t, out, "6/7 healthy")
	// At >5 members the per-instance dot row is suppressed — only one
	// summary dot precedes the count.
	assert.NotContains(t, out, "○")
}

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
	v.tabs.SetActiveByID("backends")

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
	assert.Contains(t, out, "▸")
	assert.Contains(t, out, "Group: g-2")
}

func TestExpansionToggle(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.backendsLoaded = true
	v.backends = []gcp.BackendService{{
		Name:     "api-backend",
		Backends: []gcp.Backend{{Group: "https://example/zones/z/instanceGroups/g-1"}},
	}}
	v.tabs.SetActiveByID("backends")
	v.groupFocus = 0

	v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, v.groupExpanded["https://example/zones/z/instanceGroups/g-1"])

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

func TestObservabilityTabLazyInit(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.rule = &gcp.ForwardingRule{Name: "front", Type: "HTTPS (external)"}
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.observability)
	assert.True(t, v.observability.tabActive)
}

func TestObservabilityTabLeaveStopsRefresh(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.rule = &gcp.ForwardingRule{Name: "front", Type: "HTTPS (external)"}
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.observability)

	v.tabs.SetActiveByID("overview")
	v.Update(tabs.TabChangedMsg{})
	assert.False(t, v.observability.tabActive)
}

// Regression: switching to the Observability tab BEFORE the forwarding
// rule has loaded must still create the sub-view once the rule arrives.
// Without the lbFwdLoadedMsg hand-off, v.observability stays nil forever
// and (a) the body is stuck on "Loading observability…" and (b) key "1"
// falls through to tabs.Update and switches tabs instead of changing the
// time range.
func TestObservabilityLazyInitAfterRuleLoadsLate(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.Nil(t, v.observability, "rule is nil → sub-view gated off")

	v.Update(lbFwdLoadedMsg{rule: &gcp.ForwardingRule{Name: "front", Type: "HTTPS (external)"}})
	require.NotNil(t, v.observability, "obs sub-view created once rule loads on the active obs tab")

	// And pressing "1" now reaches the obs key handler instead of the
	// tab-strip's digit-as-tab-switcher path.
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	assert.Equal(t, 1*time.Hour, v.observability.timeRange)
	assert.Equal(t, "observability", v.tabs.ActiveTab().ID, "tab must not have switched")
}

// Tab change to obs before rule loads: ensure no sub-view created (so the
// next handler gets a clean slate).
func TestObservabilityTabClickedWithNilRuleDoesNotCreateSubview(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	assert.Nil(t, v.observability)
}

func TestObservabilityPlaceholderForNetworkLB(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.fwdLoaded = true
	v.rule = &gcp.ForwardingRule{Name: "front", Type: "Network LB (passthrough)"}
	v.tabs.SetActiveByID("observability")
	out := v.renderObservability()
	assert.Contains(t, out, "Network LB (passthrough) are not yet supported")
	assert.Nil(t, v.observability)
}

func TestObservabilityChartsForHTTPSLB(t *testing.T) {
	v := NewLoadBalancerDetailsView("proj", "global", "front", nil, nil)
	v.fetchState.fwdLoaded = true
	v.rule = &gcp.ForwardingRule{Name: "front", Type: "HTTPS (external)"}
	v.tabs.SetActiveByID("observability")
	v.Update(tabs.TabChangedMsg{})
	require.NotNil(t, v.observability)
}

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
