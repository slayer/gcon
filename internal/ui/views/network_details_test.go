package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestNetworkDetailsView_NewNetworkDetailsView(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.Equal(t, "test-network", v.networkName)
	assert.True(t, v.loading, "View should start in loading state")
	assert.True(t, v.subnetsLoading, "Subnets should start loading")
	assert.True(t, v.routesLoading, "Routes should start loading")
	assert.NotNil(t, v.routeLinks, "Route links should be initialized")
	assert.Len(t, v.tabViewports, 3, "Should have 3 tab viewports (details, subnets, routes)")
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

func TestNetworkDetailsView_RoutesLoadedMsg(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)

	// Simulate routes loaded
	routes := []gcp.Route{
		{Name: "default-route", DestRange: "0.0.0.0/0", Priority: 1000, NextHop: "default-internet-gateway", RouteType: "Static"},
		{Name: "subnet-route", DestRange: "10.0.0.0/24", Priority: 0, NextHop: "test-network", RouteType: "Subnet"},
	}
	v.Update(networkRoutesLoadedMsg{routes: routes})

	assert.False(t, v.routesLoading, "Routes loading should be false after load")
	assert.Nil(t, v.routesErr, "No error expected")
	assert.Len(t, v.routes, 2, "Should have 2 routes")
	assert.True(t, v.routeLinks.HasItems(), "Route links should be populated")
	assert.Equal(t, 2, v.routeLinks.Count(), "Should have 2 route links")
}

func TestNetworkDetailsView_RoutesErrorMsg(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)

	v.Update(networkRoutesErrorMsg{err: assert.AnError})

	assert.False(t, v.routesLoading, "Routes loading should be false after error")
	assert.Equal(t, assert.AnError, v.routesErr, "Error should be set")
	assert.Nil(t, v.routes, "Routes should be nil on error")
}

func TestNetworkDetailsView_RenderRoutesTab(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)
	v.width = 120
	v.details = &gcp.NetworkDetails{Name: "test-network"}
	v.routesLoading = false
	v.routes = []gcp.Route{
		{Name: "default-route", DestRange: "0.0.0.0/0", Priority: 1000, NextHop: "default-gw", RouteType: "Static"},
	}
	v.populateRouteLinks()

	output := v.renderRoutesTab()
	assert.Contains(t, output, "test-network")
	assert.Contains(t, output, "default-route")
	assert.Contains(t, output, "0.0.0.0/0")
	assert.Contains(t, output, "Static")
}

func TestNetworkDetailsView_RenderRoutesTabEmpty(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)
	v.width = 120
	v.details = &gcp.NetworkDetails{Name: "test-network"}
	v.routesLoading = false
	v.routes = nil

	output := v.renderRoutesTab()
	assert.Contains(t, output, "No routes found")
}

func TestNetworkDetailsView_RenderRoutesTabLoading(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)
	v.width = 120
	v.details = &gcp.NetworkDetails{Name: "test-network"}
	v.routesLoading = true

	output := v.renderRoutesTab()
	assert.Contains(t, output, "Loading routes...")
}

func TestNetworkDetailsView_RenderRoutesTabError(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)
	v.width = 120
	v.details = &gcp.NetworkDetails{Name: "test-network"}
	v.routesLoading = false
	v.routesErr = assert.AnError

	output := v.renderRoutesTab()
	assert.Contains(t, output, "Error loading routes:")
	assert.Contains(t, output, "Press 'r' to retry")
}

func TestNetworkDetailsView_GetRegionLabelRoute(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)

	// Switch to Routes tab
	v.tabs.SetActiveByID(networkTabIDRoutes)

	// Enable route links region (normally done when routes load) and set focus
	v.focusMgr.EnableRegion(networkRegionIDRouteLinks)
	v.focusMgr.SetActive(networkRegionIDRouteLinks)

	label := v.getRegionLabel()
	assert.Equal(t, "route", label)
}

func TestNetworkDetailsView_GetRegionLabelSubnet(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)

	// Stay on Subnets tab (default is Details, switch to Subnets)
	v.tabs.SetActiveByID(networkTabIDSubnets)

	// Enable subnet links region and set focus
	v.focusMgr.EnableRegion(networkRegionIDLinks)
	v.focusMgr.SetActive(networkRegionIDLinks)

	label := v.getRegionLabel()
	assert.Equal(t, "subnet", label)
}

func TestNetworkDetailsView_IsMenuOpen(t *testing.T) {
	v := NewNetworkDetailsView("test-project", "test-network", nil)

	assert.False(t, v.IsMenuOpen(), "Menu should start closed")
}
