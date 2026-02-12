package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirewallDetailsView_New(t *testing.T) {
	v := NewFirewallDetailsView("test-project", "allow-ssh", nil)

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.Equal(t, "allow-ssh", v.ruleName)
	assert.True(t, v.loading, "View should start in loading state")
	assert.NotNil(t, v.tabs, "Tabs should be initialized")
	assert.NotNil(t, v.focusMgr, "Focus manager should be initialized")
}

func TestFirewallDetailsView_Tabs(t *testing.T) {
	v := NewFirewallDetailsView("test-project", "allow-ssh", nil)

	// Verify two tabs exist
	assert.Equal(t, 2, v.tabs.Count())

	// First tab (default active) should be Details
	assert.Equal(t, "Details", v.tabs.ActiveTab().Label)

	// Switch to second tab and verify it's Rules
	v.tabs.SetActive(1)
	assert.Equal(t, "Rules", v.tabs.ActiveTab().Label)
}

func TestFirewallDetailsView_GetRuleName(t *testing.T) {
	v := NewFirewallDetailsView("test-project", "deny-egress", nil)

	assert.Equal(t, "deny-egress", v.GetRuleName())
}

func TestFirewallDetailsView_GetComputeClient(t *testing.T) {
	v := NewFirewallDetailsView("test-project", "allow-ssh", nil)

	// Should return nil when no client is set
	assert.Nil(t, v.GetComputeClient())
}

func TestFirewallDetailsView_IsMenuOpen(t *testing.T) {
	v := NewFirewallDetailsView("test-project", "allow-ssh", nil)

	assert.False(t, v.IsMenuOpen(), "Menu should start closed")
}

func TestFirewallDetailsView_HasTextInputFocused(t *testing.T) {
	v := NewFirewallDetailsView("test-project", "allow-ssh", nil)

	// No delete confirm dialog active by default
	assert.False(t, v.HasTextInputFocused())
}
