package gcp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/compute/v1"
)

func TestRouteFromAPI(t *testing.T) {
	r := &compute.Route{
		Name:        "custom-route-1",
		Description: "Route to on-prem via VPN",
		Network:     "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/prod-vpc",
		DestRange:   "192.168.0.0/16",
		Priority:    100,
		NextHopVpnTunnel: "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/vpnTunnels/tunnel-to-onprem",
		Tags:              []string{"vpn-routed"},
		CreationTimestamp: "2024-06-15T10:00:00.000-07:00",
	}

	route := routeFromAPI(r)

	assert.Equal(t, "custom-route-1", route.Name)
	assert.Equal(t, "Route to on-prem via VPN", route.Description)
	assert.Equal(t, "prod-vpc", route.Network)
	assert.Equal(t, "192.168.0.0/16", route.DestRange)
	assert.Equal(t, int64(100), route.Priority)
	assert.Equal(t, "tunnel-to-onprem", route.NextHop)
	assert.Equal(t, "VPNTunnel", route.NextHopType)
	assert.Equal(t, "Static", route.RouteType)
	assert.Equal(t, []string{"vpn-routed"}, route.Tags)
	assert.Equal(t, "2024-06-15T10:00:00.000-07:00", route.CreatedAt)
}

func TestRouteFromAPI_DefaultGateway(t *testing.T) {
	r := &compute.Route{
		Name:       "default-route-abc123",
		Network:    "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/default",
		DestRange:  "0.0.0.0/0",
		Priority:   1000,
		NextHopGateway: "https://www.googleapis.com/compute/v1/projects/my-project/global/gateways/default-internet-gateway",
	}

	route := routeFromAPI(r)

	assert.Equal(t, "Default internet gateway", route.NextHop)
	assert.Equal(t, "Gateway", route.NextHopType)
	assert.Equal(t, "System", route.RouteType)
}

func TestRouteFromAPI_NoNextHop(t *testing.T) {
	r := &compute.Route{
		Name:      "orphan-route",
		Network:   "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/default",
		DestRange: "10.0.0.0/8",
		Priority:  1000,
	}

	route := routeFromAPI(r)

	assert.Equal(t, "", route.NextHop)
	assert.Equal(t, "", route.NextHopType)
}

func TestRouteFromAPI_NilTags(t *testing.T) {
	r := &compute.Route{
		Name:    "no-tags-route",
		Network: "default",
	}

	route := routeFromAPI(r)

	assert.Nil(t, route.Tags)
}

func TestRouteTypeDerivation(t *testing.T) {
	tests := []struct {
		name     string
		route    *compute.Route
		expected string
	}{
		{
			name: "peering route — nextHopPeering set",
			route: &compute.Route{
				Name:           "peering-route-abc",
				NextHopPeering: "peer-connection-1",
			},
			expected: "Peering",
		},
		{
			name: "subnet route — nextHopNetwork set",
			route: &compute.Route{
				Name:           "default-route-subnet-10",
				NextHopNetwork: "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/my-vpc",
			},
			expected: "Subnet",
		},
		{
			name: "system route — default name + default gateway",
			route: &compute.Route{
				Name:           "default-route-abc123def456",
				NextHopGateway: "https://www.googleapis.com/compute/v1/projects/my-project/global/gateways/default-internet-gateway",
			},
			expected: "System",
		},
		{
			name: "static route — custom name + default gateway",
			route: &compute.Route{
				Name:           "my-internet-route",
				NextHopGateway: "https://www.googleapis.com/compute/v1/projects/my-project/global/gateways/default-internet-gateway",
			},
			expected: "Static",
		},
		{
			name: "static route — default name but custom gateway",
			route: &compute.Route{
				Name:           "default-route-custom",
				NextHopGateway: "https://www.googleapis.com/compute/v1/projects/my-project/global/gateways/custom-gateway",
			},
			expected: "Static",
		},
		{
			name: "static route — next hop instance",
			route: &compute.Route{
				Name:            "instance-route",
				NextHopInstance: "https://www.googleapis.com/compute/v1/projects/my-project/zones/us-central1-a/instances/nat-gw",
			},
			expected: "Static",
		},
		{
			name: "static route — next hop IP",
			route: &compute.Route{
				Name:      "ip-route",
				NextHopIp: "10.0.0.5",
			},
			expected: "Static",
		},
		{
			name: "static route — next hop ILB",
			route: &compute.Route{
				Name:      "ilb-route",
				NextHopIlb: "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/forwardingRules/my-ilb",
			},
			expected: "Static",
		},
		{
			name: "static route — next hop VPN tunnel",
			route: &compute.Route{
				Name:             "vpn-route",
				NextHopVpnTunnel: "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/vpnTunnels/tunnel-1",
			},
			expected: "Static",
		},
		{
			name: "static route — next hop interconnect",
			route: &compute.Route{
				Name: "interconnect-route",
				NextHopInterconnectAttachment: "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/interconnectAttachments/attach-1",
			},
			expected: "Static",
		},
		{
			name: "static route — no next hop at all",
			route: &compute.Route{
				Name: "empty-route",
			},
			expected: "Static",
		},
		{
			name: "peering takes priority over subnet when both set",
			route: &compute.Route{
				Name:           "weird-route",
				NextHopPeering: "some-peering",
				NextHopNetwork: "https://example.com/networks/vpc",
			},
			expected: "Peering",
		},
		{
			name: "subnet takes priority over system when both set",
			route: &compute.Route{
				Name:           "default-route-overlap",
				NextHopNetwork: "https://example.com/networks/vpc",
				NextHopGateway: "https://www.googleapis.com/compute/v1/projects/p/global/gateways/default-internet-gateway",
			},
			expected: "Subnet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveRouteType(tt.route)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveNextHop(t *testing.T) {
	tests := []struct {
		name         string
		route        *compute.Route
		wantDisplay  string
		wantHopType  string
	}{
		{
			name: "instance — extracts name from URL",
			route: &compute.Route{
				NextHopInstance: "https://www.googleapis.com/compute/v1/projects/my-project/zones/us-central1-a/instances/nat-gateway",
			},
			wantDisplay: "nat-gateway",
			wantHopType: "Instance",
		},
		{
			name: "IP address — shown directly",
			route: &compute.Route{
				NextHopIp: "10.128.0.5",
			},
			wantDisplay: "10.128.0.5",
			wantHopType: "IP",
		},
		{
			name: "VPN tunnel — extracts name",
			route: &compute.Route{
				NextHopVpnTunnel: "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/vpnTunnels/tunnel-to-aws",
			},
			wantDisplay: "tunnel-to-aws",
			wantHopType: "VPNTunnel",
		},
		{
			name: "interconnect attachment — extracts name",
			route: &compute.Route{
				NextHopInterconnectAttachment: "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-east1/interconnectAttachments/partner-attach",
			},
			wantDisplay: "partner-attach",
			wantHopType: "Interconnect",
		},
		{
			name: "ILB — extracts forwarding rule name",
			route: &compute.Route{
				NextHopIlb: "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/forwardingRules/my-ilb-rule",
			},
			wantDisplay: "my-ilb-rule",
			wantHopType: "ILB",
		},
		{
			name: "default internet gateway — friendly label",
			route: &compute.Route{
				NextHopGateway: "https://www.googleapis.com/compute/v1/projects/my-project/global/gateways/default-internet-gateway",
			},
			wantDisplay: "Default internet gateway",
			wantHopType: "Gateway",
		},
		{
			name: "custom gateway — extracts name",
			route: &compute.Route{
				NextHopGateway: "https://www.googleapis.com/compute/v1/projects/my-project/global/gateways/custom-gw",
			},
			wantDisplay: "custom-gw",
			wantHopType: "Gateway",
		},
		{
			name: "network — extracts name",
			route: &compute.Route{
				NextHopNetwork: "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/prod-vpc",
			},
			wantDisplay: "prod-vpc",
			wantHopType: "Network",
		},
		{
			name: "peering — direct peering name",
			route: &compute.Route{
				NextHopPeering: "peer-connection-1",
			},
			wantDisplay: "peer-connection-1",
			wantHopType: "Peering",
		},
		{
			name:        "no next hop — empty",
			route:       &compute.Route{},
			wantDisplay: "",
			wantHopType: "",
		},
		{
			name: "instance takes priority over IP when both set",
			route: &compute.Route{
				NextHopInstance: "https://example.com/instances/my-vm",
				NextHopIp:       "10.0.0.1",
			},
			wantDisplay: "my-vm",
			wantHopType: "Instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display, hopType := resolveNextHop(tt.route)
			assert.Equal(t, tt.wantDisplay, display)
			assert.Equal(t, tt.wantHopType, hopType)
		})
	}
}

func TestRouteDetailsFromAPI(t *testing.T) {
	r := &compute.Route{
		Name:        "detailed-route",
		Id:          123456789,
		Description: "A fully-populated route",
		Network:     "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/prod-vpc",
		DestRange:   "10.0.0.0/8",
		Priority:    500,
		NextHopIp:   "10.128.0.100",
		Tags:        []string{"internal", "backend"},
		Warnings: []*compute.RouteWarnings{
			{Message: "Route may conflict with another route"},
			{Message: "Next hop instance does not exist"},
		},
		CreationTimestamp: "2024-01-20T15:00:00.000-07:00",
		SelfLink:          "https://www.googleapis.com/compute/v1/projects/my-project/global/routes/detailed-route",
	}

	details := routeDetailsFromAPI(r)

	// Common fields (shared with Route)
	assert.Equal(t, "detailed-route", details.Name)
	assert.Equal(t, "A fully-populated route", details.Description)
	assert.Equal(t, "prod-vpc", details.Network)
	assert.Equal(t, "10.0.0.0/8", details.DestRange)
	assert.Equal(t, int64(500), details.Priority)
	assert.Equal(t, "10.128.0.100", details.NextHop)
	assert.Equal(t, "IP", details.NextHopType)
	assert.Equal(t, "Static", details.RouteType)
	assert.Equal(t, []string{"internal", "backend"}, details.Tags)
	assert.Equal(t, "2024-01-20T15:00:00.000-07:00", details.CreatedAt)

	// Detail-only fields
	assert.Equal(t, uint64(123456789), details.ID)
	assert.Equal(t, "https://www.googleapis.com/compute/v1/projects/my-project/global/routes/detailed-route", details.SelfLink)
	assert.Equal(t, "10.128.0.100", details.NextHopIP)

	// Warnings
	assert.Len(t, details.Warnings, 2)
	assert.Equal(t, "Route may conflict with another route", details.Warnings[0])
	assert.Equal(t, "Next hop instance does not exist", details.Warnings[1])
}

func TestRouteDetailsFromAPI_AllNextHopFields(t *testing.T) {
	// Verify all raw next-hop fields are preserved
	r := &compute.Route{
		Name:                          "all-hops",
		NextHopInstance:               "https://example.com/instances/my-vm",
		NextHopIp:                     "10.0.0.1",
		NextHopVpnTunnel:              "https://example.com/vpnTunnels/my-tunnel",
		NextHopInterconnectAttachment: "https://example.com/interconnectAttachments/my-attach",
		NextHopIlb:                    "https://example.com/forwardingRules/my-ilb",
		NextHopGateway:                "https://example.com/gateways/my-gw",
		NextHopNetwork:                "https://example.com/networks/my-net",
		NextHopPeering:                "my-peering",
	}

	details := routeDetailsFromAPI(r)

	assert.Equal(t, "https://example.com/instances/my-vm", details.NextHopInstance)
	assert.Equal(t, "10.0.0.1", details.NextHopIP)
	assert.Equal(t, "https://example.com/vpnTunnels/my-tunnel", details.NextHopVPNTunnel)
	assert.Equal(t, "https://example.com/interconnectAttachments/my-attach", details.NextHopInterconnectAttachment)
	assert.Equal(t, "https://example.com/forwardingRules/my-ilb", details.NextHopILB)
	assert.Equal(t, "https://example.com/gateways/my-gw", details.NextHopGateway)
	assert.Equal(t, "https://example.com/networks/my-net", details.NextHopNetwork)
	assert.Equal(t, "my-peering", details.NextHopPeering)
}

func TestRouteDetailsFromAPI_NoWarnings(t *testing.T) {
	r := &compute.Route{
		Name:     "clean-route",
		Warnings: nil,
	}

	details := routeDetailsFromAPI(r)

	assert.Nil(t, details.Warnings)
}

func TestRouteDetailsFromAPI_EmptyWarningMessage(t *testing.T) {
	// Warnings with empty messages should be excluded
	r := &compute.Route{
		Name: "partial-warnings",
		Warnings: []*compute.RouteWarnings{
			{Message: "Real warning"},
			{Message: ""},
			nil,
			{Message: "Another warning"},
		},
	}

	details := routeDetailsFromAPI(r)

	assert.Len(t, details.Warnings, 2)
	assert.Equal(t, "Real warning", details.Warnings[0])
	assert.Equal(t, "Another warning", details.Warnings[1])
}

func TestCreateRouteConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   RouteConfig
		wantFunc func(t *testing.T, r *compute.Route)
	}{
		{
			name: "gateway next hop",
			config: RouteConfig{
				Name:         "gw-route",
				Description:  "Via gateway",
				Network:      "my-vpc",
				DestRange:    "0.0.0.0/0",
				Priority:     1000,
				NextHopType:  "gateway",
				NextHopValue: "https://www.googleapis.com/compute/v1/projects/p/global/gateways/default-internet-gateway",
			},
			wantFunc: func(t *testing.T, r *compute.Route) {
				assert.Equal(t, "gw-route", r.Name)
				assert.Equal(t, "Via gateway", r.Description)
				assert.Contains(t, r.Network, "my-vpc")
				assert.Equal(t, "0.0.0.0/0", r.DestRange)
				assert.Equal(t, int64(1000), r.Priority)
				assert.Equal(t, "https://www.googleapis.com/compute/v1/projects/p/global/gateways/default-internet-gateway", r.NextHopGateway)
				assert.Empty(t, r.NextHopInstance)
				assert.Empty(t, r.NextHopIp)
			},
		},
		{
			name: "instance next hop",
			config: RouteConfig{
				Name:         "inst-route",
				Network:      "vpc-1",
				DestRange:    "10.0.0.0/8",
				Priority:     100,
				NextHopType:  "instance",
				NextHopValue: "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/instances/nat-gw",
			},
			wantFunc: func(t *testing.T, r *compute.Route) {
				assert.Equal(t, "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/instances/nat-gw", r.NextHopInstance)
				assert.Empty(t, r.NextHopGateway)
			},
		},
		{
			name: "IP next hop",
			config: RouteConfig{
				Name:         "ip-route",
				Network:      "vpc-1",
				DestRange:    "172.16.0.0/12",
				Priority:     200,
				NextHopType:  "ip",
				NextHopValue: "10.128.0.5",
			},
			wantFunc: func(t *testing.T, r *compute.Route) {
				assert.Equal(t, "10.128.0.5", r.NextHopIp)
				assert.Empty(t, r.NextHopGateway)
				assert.Empty(t, r.NextHopInstance)
			},
		},
		{
			name: "VPN tunnel next hop",
			config: RouteConfig{
				Name:         "vpn-route",
				Network:      "vpc-1",
				DestRange:    "192.168.0.0/16",
				Priority:     100,
				NextHopType:  "vpn-tunnel",
				NextHopValue: "https://example.com/vpnTunnels/my-tunnel",
			},
			wantFunc: func(t *testing.T, r *compute.Route) {
				assert.Equal(t, "https://example.com/vpnTunnels/my-tunnel", r.NextHopVpnTunnel)
				assert.Empty(t, r.NextHopGateway)
			},
		},
		{
			name: "interconnect next hop",
			config: RouteConfig{
				Name:         "interconnect-route",
				Network:      "vpc-1",
				DestRange:    "10.10.0.0/16",
				Priority:     50,
				NextHopType:  "interconnect",
				NextHopValue: "https://example.com/interconnectAttachments/attach-1",
			},
			wantFunc: func(t *testing.T, r *compute.Route) {
				assert.Equal(t, "https://example.com/interconnectAttachments/attach-1", r.NextHopInterconnectAttachment)
				assert.Empty(t, r.NextHopGateway)
			},
		},
		{
			name: "ILB next hop",
			config: RouteConfig{
				Name:         "ilb-route",
				Network:      "vpc-1",
				DestRange:    "10.20.0.0/16",
				Priority:     300,
				NextHopType:  "ilb",
				NextHopValue: "https://example.com/forwardingRules/my-ilb",
			},
			wantFunc: func(t *testing.T, r *compute.Route) {
				assert.Equal(t, "https://example.com/forwardingRules/my-ilb", r.NextHopIlb)
				assert.Empty(t, r.NextHopGateway)
			},
		},
		{
			name: "with tags",
			config: RouteConfig{
				Name:         "tagged-route",
				Network:      "vpc-1",
				DestRange:    "10.0.0.0/8",
				Priority:     1000,
				Tags:         []string{"web-server", "backend"},
				NextHopType:  "ip",
				NextHopValue: "10.0.0.1",
			},
			wantFunc: func(t *testing.T, r *compute.Route) {
				assert.Equal(t, []string{"web-server", "backend"}, r.Tags)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build the route struct the same way CreateRoute does internally
			route := &compute.Route{
				Name:        tt.config.Name,
				Description: tt.config.Description,
				Network:     fmt.Sprintf("projects/%s/global/networks/%s", "test-project", tt.config.Network),
				DestRange:   tt.config.DestRange,
				Priority:    tt.config.Priority,
				Tags:        tt.config.Tags,
			}

			switch tt.config.NextHopType {
			case "gateway":
				route.NextHopGateway = tt.config.NextHopValue
			case "instance":
				route.NextHopInstance = tt.config.NextHopValue
			case "ip":
				route.NextHopIp = tt.config.NextHopValue
			case "vpn-tunnel":
				route.NextHopVpnTunnel = tt.config.NextHopValue
			case "interconnect":
				route.NextHopInterconnectAttachment = tt.config.NextHopValue
			case "ilb":
				route.NextHopIlb = tt.config.NextHopValue
			}

			tt.wantFunc(t, route)
		})
	}
}

func TestResolveGatewayDisplay(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "default internet gateway",
			url:      "https://www.googleapis.com/compute/v1/projects/my-project/global/gateways/default-internet-gateway",
			expected: "Default internet gateway",
		},
		{
			name:     "custom gateway",
			url:      "https://www.googleapis.com/compute/v1/projects/my-project/global/gateways/my-custom-gateway",
			expected: "my-custom-gateway",
		},
		{
			name:     "bare name",
			url:      "default-internet-gateway",
			expected: "default-internet-gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveGatewayDisplay(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSystemRoute(t *testing.T) {
	tests := []struct {
		name     string
		route    *compute.Route
		expected bool
	}{
		{
			name: "system route — default name + default gateway",
			route: &compute.Route{
				Name:           "default-route-abc123",
				NextHopGateway: "https://www.googleapis.com/compute/v1/projects/p/global/gateways/default-internet-gateway",
			},
			expected: true,
		},
		{
			name: "not system — custom name",
			route: &compute.Route{
				Name:           "my-custom-route",
				NextHopGateway: "https://www.googleapis.com/compute/v1/projects/p/global/gateways/default-internet-gateway",
			},
			expected: false,
		},
		{
			name: "not system — custom gateway",
			route: &compute.Route{
				Name:           "default-route-xyz",
				NextHopGateway: "https://www.googleapis.com/compute/v1/projects/p/global/gateways/custom-gw",
			},
			expected: false,
		},
		{
			name: "not system — no gateway",
			route: &compute.Route{
				Name: "default-route-noop",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSystemRoute(tt.route)
			assert.Equal(t, tt.expected, result)
		})
	}
}
