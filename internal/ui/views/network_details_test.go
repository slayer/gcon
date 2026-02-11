package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetworkDetailsView_NewNetworkDetailsView(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.Equal(t, "test-network", v.networkName)
	assert.True(t, v.loading, "View should start in loading state")
	assert.True(t, v.subnetsLoading, "Subnets should start loading")
}

func TestNetworkDetailsView_RenderLoading(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)

	output := renderLoading(v.spinner, "Loading network details...")

	assert.Contains(t, output, "Loading network details...")
	assert.Contains(t, output, v.spinner.View())
}

func TestNetworkDetailsView_GetNetworkName(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "my-vpc", nil)

	assert.Equal(t, "my-vpc", v.GetNetworkName())
}

func TestNetworkDetailsView_GetComputeClient(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "my-vpc", nil)

	// Should return nil when no client is set
	assert.Nil(t, v.GetComputeClient())
}

func TestFormatSubnetMode(t *testing.T) {
	tests := []struct {
		autoCreate bool
		expected   string
	}{
		{true, "Auto"},
		{false, "Custom"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatSubnetMode(tt.autoCreate))
		})
	}
}

func TestFormatMTU(t *testing.T) {
	tests := []struct {
		name     string
		mtu      int64
		expected string
	}{
		{"default MTU (0)", 0, "Default (1460)"},
		{"custom MTU 1500", 1500, "1500"},
		{"jumbo MTU 8896", 8896, "8896"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatMTU(tt.mtu))
		})
	}
}

func TestFormatSubnetPurpose(t *testing.T) {
	tests := []struct {
		purpose  string
		expected string
	}{
		{"PRIVATE", "Private"},
		{"REGIONAL_MANAGED_PROXY", "Managed Proxy"},
		{"GLOBAL_MANAGED_PROXY", "Global Proxy"},
		{"PRIVATE_SERVICE_CONNECT", "PSC"},
		{"INTERNAL_HTTPS_LOAD_BALANCER", "Internal LB"},
		{"SOMETHING_NEW", "SOMETHING_NEW"},
		{"", "—"},
	}

	for _, tt := range tests {
		t.Run(tt.purpose, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatSubnetPurpose(tt.purpose))
		})
	}
}

func TestFormatYesNo(t *testing.T) {
	assert.Equal(t, "Yes", formatYesNo(true))
	assert.Equal(t, "No", formatYesNo(false))
}

func TestNetworkDetailsView_IsMenuOpen(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)

	assert.False(t, v.IsMenuOpen(), "Menu should start closed")
}
