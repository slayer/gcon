package views

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

var errLBListTest = errors.New("boom")

func TestLoadBalancersView_LoadedMsg_PopulatesRows(t *testing.T) {
	v := NewLoadBalancersView("proj", nil)
	v.SetSize(120, 30)

	rules := []gcp.ForwardingRule{
		{Name: "front", Scope: "global", Type: "HTTPS (external)", IPAddress: "34.1.2.3", PortRange: "443", Target: "https://x/y/targetHttpsProxies/p", SelfLink: "https://x/y/forwardingRules/front"},
		{Name: "back", Scope: "us-central1", Type: "Network LB (passthrough)", IPAddress: "10.0.0.5", PortRange: "80", Target: "https://x/y/backendServices/b", SelfLink: "https://x/y/forwardingRules/back"},
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
	_ = v.Update(loadBalancersErrorMsg{err: errLBListTest})
	out := v.View()
	assert.Contains(t, out, "boom")
}
