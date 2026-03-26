package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/compute/v1"
)

func TestExtractRegionFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "standard region URL",
			url:      "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1",
			expected: "us-central1",
		},
		{
			name:     "subnetwork self-link with region",
			url:      "https://www.googleapis.com/compute/v1/projects/my-project/regions/europe-west1/subnetworks/my-subnet",
			expected: "europe-west1",
		},
		{
			name:     "just region name",
			url:      "us-east1",
			expected: "us-east1",
		},
		{
			name:     "empty string",
			url:      "",
			expected: "",
		},
		{
			name:     "no regions segment",
			url:      "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/default",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRegionFromURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractNameFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "network URL",
			url:      "https://www.googleapis.com/compute/v1/projects/peer-project/global/networks/peer-network",
			expected: "peer-network",
		},
		{
			name:     "simple name",
			url:      "my-network",
			expected: "my-network",
		},
		{
			name:     "empty string",
			url:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNameFromURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNetworkDetailsFromAPI(t *testing.T) {
	n := &compute.Network{
		Name:                  "test-network",
		Id:                    12345,
		Description:           "A test network",
		AutoCreateSubnetworks: true,
		RoutingConfig: &compute.NetworkRoutingConfig{
			RoutingMode: "GLOBAL",
		},
		Mtu:                   1500,
		GatewayIPv4:           "10.128.0.1",
		EnableUlaInternalIpv6: true,
		InternalIpv6Range:     "fd20::/48",
		Peerings: []*compute.NetworkPeering{
			{
				Name:    "peer-1",
				Network: "https://www.googleapis.com/compute/v1/projects/other-project/global/networks/other-vpc",
				State:   "ACTIVE",
			},
		},
		Subnetworks:       []string{"subnet-a", "subnet-b", "subnet-c"},
		CreationTimestamp: "2024-01-15T10:00:00.000-07:00",
		SelfLink:          "https://www.googleapis.com/compute/v1/projects/test-project/global/networks/test-network",
	}

	details := networkDetailsFromAPI(n)

	assert.Equal(t, "test-network", details.Name)
	assert.Equal(t, uint64(12345), details.ID)
	assert.Equal(t, "A test network", details.Description)
	assert.True(t, details.AutoCreateSubnetworks)
	assert.Equal(t, "GLOBAL", details.RoutingMode)
	assert.Equal(t, int64(1500), details.Mtu)
	assert.Equal(t, "10.128.0.1", details.GatewayIPv4)
	assert.True(t, details.EnableUlaInternalIpv6)
	assert.Equal(t, "fd20::/48", details.InternalIpv6Range)
	assert.Equal(t, 3, details.SubnetworkCount)

	// Peering
	assert.Len(t, details.Peerings, 1)
	assert.Equal(t, "peer-1", details.Peerings[0].Name)
	assert.Equal(t, "other-vpc", details.Peerings[0].Network)
	assert.Equal(t, "ACTIVE", details.Peerings[0].State)
}

func TestNetworkDetailsFromAPI_NilRoutingConfig(t *testing.T) {
	n := &compute.Network{
		Name:          "minimal-network",
		RoutingConfig: nil,
	}

	details := networkDetailsFromAPI(n)

	assert.Equal(t, "", details.RoutingMode)
}

func TestSubnetFromAPI(t *testing.T) {
	s := &compute.Subnetwork{
		Name:                  "my-subnet",
		Network:               "https://www.googleapis.com/compute/v1/projects/test-project/global/networks/my-vpc",
		Region:                "https://www.googleapis.com/compute/v1/projects/test-project/regions/us-central1",
		IpCidrRange:           "10.0.0.0/24",
		GatewayAddress:        "10.0.0.1",
		Purpose:               "PRIVATE",
		PrivateIpGoogleAccess: true,
		LogConfig: &compute.SubnetworkLogConfig{
			Enable: true,
		},
		CreationTimestamp: "2024-06-01T12:00:00.000-07:00",
	}

	subnet := subnetFromAPI(s)

	assert.Equal(t, "my-subnet", subnet.Name)
	assert.Equal(t, "my-vpc", subnet.Network)
	assert.Equal(t, "us-central1", subnet.Region)
	assert.Equal(t, "10.0.0.0/24", subnet.IPCidrRange)
	assert.Equal(t, "10.0.0.1", subnet.GatewayAddress)
	assert.Equal(t, "PRIVATE", subnet.Purpose)
	assert.True(t, subnet.PrivateIPGoogleAccess)
	assert.True(t, subnet.EnableFlowLogs)
}

func TestSubnetFromAPI_IncludesNetwork(t *testing.T) {
	tests := []struct {
		name        string
		networkURL  string
		expectedNet string
	}{
		{
			name:        "full network URL",
			networkURL:  "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/production-vpc",
			expectedNet: "production-vpc",
		},
		{
			name:        "short network name",
			networkURL:  "default",
			expectedNet: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &compute.Subnetwork{
				Name:    "test-subnet",
				Network: tt.networkURL,
			}
			subnet := subnetFromAPI(s)
			assert.Equal(t, tt.expectedNet, subnet.Network)
		})
	}
}

func TestSubnetFromAPI_EmptyNetwork(t *testing.T) {
	s := &compute.Subnetwork{
		Name:    "orphan-subnet",
		Network: "",
	}
	subnet := subnetFromAPI(s)
	assert.Equal(t, "", subnet.Network)
}

func TestSubnetFromAPI_NilLogConfig(t *testing.T) {
	s := &compute.Subnetwork{
		Name:      "no-logs-subnet",
		Region:    "https://www.googleapis.com/compute/v1/projects/test-project/regions/europe-west1",
		LogConfig: nil,
	}

	subnet := subnetFromAPI(s)

	assert.False(t, subnet.EnableFlowLogs)
	assert.Equal(t, "europe-west1", subnet.Region)
}

func TestSubnetDetailsFromAPI(t *testing.T) {
	s := &compute.Subnetwork{
		Id:          98765,
		Name:        "prod-subnet",
		Description: "Production subnet with flow logs",
		State:       "READY",
		Region:      "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1",
		Network:     "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/prod-vpc",
		IpCidrRange: "10.10.0.0/20",
		GatewayAddress:        "10.10.0.1",
		Purpose:               "PRIVATE",
		StackType:             "IPV4_IPV6",
		Ipv6AccessType:        "INTERNAL",
		Ipv6CidrRange:         "fd20:abc:123::/48",
		PrivateIpGoogleAccess: true,
		LogConfig: &compute.SubnetworkLogConfig{
			Enable:              true,
			AggregationInterval: "INTERVAL_5_SEC",
			FlowSampling:        0.5,
			Metadata:            "INCLUDE_ALL_METADATA",
			FilterExpr:          "true",
		},
		SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
			{
				RangeName:   "pods",
				IpCidrRange: "10.20.0.0/16",
			},
			{
				RangeName:   "services",
				IpCidrRange: "10.30.0.0/20",
			},
		},
		CreationTimestamp: "2024-03-15T08:30:00.000-07:00",
		SelfLink:          "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/subnetworks/prod-subnet",
	}

	details := subnetDetailsFromAPI(s)

	// Basic fields
	assert.Equal(t, uint64(98765), details.ID)
	assert.Equal(t, "prod-subnet", details.Name)
	assert.Equal(t, "Production subnet with flow logs", details.Description)
	assert.Equal(t, "READY", details.Status)
	assert.Equal(t, "us-central1", details.Region)
	assert.Equal(t, "prod-vpc", details.Network)
	assert.Equal(t, "10.10.0.0/20", details.IPCidrRange)
	assert.Equal(t, "10.10.0.1", details.GatewayAddress)
	assert.Equal(t, "PRIVATE", details.Purpose)
	assert.Equal(t, "IPV4_IPV6", details.StackType)
	assert.Equal(t, "INTERNAL", details.IPv6AccessType)
	assert.Equal(t, "fd20:abc:123::/48", details.IPv6CidrRange)
	assert.True(t, details.PrivateIPGoogleAccess)
	assert.Equal(t, "2024-03-15T08:30:00.000-07:00", details.CreatedAt)
	assert.Equal(t, "https://www.googleapis.com/compute/v1/projects/my-project/regions/us-central1/subnetworks/prod-subnet", details.SelfLink)

	// Flow logs
	assert.True(t, details.EnableFlowLogs)
	assert.Equal(t, "INTERVAL_5_SEC", details.FlowLogConfig.AggregationInterval)
	assert.Equal(t, 0.5, details.FlowLogConfig.FlowSampling)
	assert.Equal(t, "INCLUDE_ALL_METADATA", details.FlowLogConfig.Metadata)
	assert.Equal(t, "true", details.FlowLogConfig.FilterExpr)

	// Secondary ranges
	assert.Len(t, details.SecondaryIPRanges, 2)
	assert.Equal(t, "pods", details.SecondaryIPRanges[0].Name)
	assert.Equal(t, "10.20.0.0/16", details.SecondaryIPRanges[0].CidrRange)
	assert.Equal(t, "services", details.SecondaryIPRanges[1].Name)
	assert.Equal(t, "10.30.0.0/20", details.SecondaryIPRanges[1].CidrRange)
}

func TestSubnetDetailsFromAPI_NilLogConfig(t *testing.T) {
	s := &compute.Subnetwork{
		Name:      "no-logs-subnet",
		State:     "READY",
		Region:    "https://www.googleapis.com/compute/v1/projects/test-project/regions/europe-west1",
		Network:   "https://www.googleapis.com/compute/v1/projects/test-project/global/networks/default",
		LogConfig: nil,
	}

	details := subnetDetailsFromAPI(s)

	assert.False(t, details.EnableFlowLogs)
	assert.Equal(t, FlowLogConfig{}, details.FlowLogConfig)
}

func TestSubnetDetailsFromAPI_NoSecondaryRanges(t *testing.T) {
	s := &compute.Subnetwork{
		Name:              "simple-subnet",
		State:             "READY",
		Region:            "https://www.googleapis.com/compute/v1/projects/test-project/regions/us-east1",
		Network:           "https://www.googleapis.com/compute/v1/projects/test-project/global/networks/default",
		SecondaryIpRanges: nil,
	}

	details := subnetDetailsFromAPI(s)

	// nil SecondaryIpRanges should produce nil (not a populated slice)
	assert.Nil(t, details.SecondaryIPRanges)
}
