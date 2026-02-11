package gcp

import (
	"context"
	"sort"
	"strings"

	"google.golang.org/api/compute/v1"
)

// Network represents a simplified VPC network
type Network struct {
	Name         string
	ID           uint64
	Description  string
	AutoCreate   bool   // Auto mode (auto-creates subnets) vs Custom mode
	RoutingMode  string // REGIONAL or GLOBAL
	SubnetsCount int    // Number of subnets
	CreatedAt    string
}

// NetworkDetails holds detailed information about a VPC network
type NetworkDetails struct {
	Name                  string
	ID                    uint64
	Description           string
	AutoCreateSubnetworks bool
	RoutingMode           string // REGIONAL or GLOBAL
	Mtu                   int64  // 0 means default (1460)
	GatewayIPv4           string
	EnableUlaInternalIpv6 bool
	InternalIpv6Range     string
	Peerings              []NetworkPeering
	SubnetworkCount       int // Derived from SubnetworkURLs length
	CreatedAt             string
	SelfLink              string
}

// NetworkPeering represents a VPC network peering connection
type NetworkPeering struct {
	Name    string
	Network string // Peer network name (extracted from URL)
	State   string // ACTIVE, INACTIVE
}

// Subnet represents a VPC subnetwork
type Subnet struct {
	Name                  string
	Region                string // Extracted from self-link URL
	IPCidrRange           string
	GatewayAddress        string
	Purpose               string // PRIVATE, REGIONAL_MANAGED_PROXY, etc.
	PrivateIPGoogleAccess bool
	EnableFlowLogs        bool
	CreatedAt             string
}

// ListNetworks retrieves all VPC networks in a project
func (c *ComputeClient) ListNetworks(ctx context.Context, projectID string) ([]Network, error) {
	var networks []Network

	req := c.service.Networks.List(projectID)
	err := req.Pages(ctx, func(page *compute.NetworkList) error {
		for _, n := range page.Items {
			networks = append(networks, networkFromAPI(n))
		}
		return nil
	})
	if err != nil {
		return nil, WrapListError(err, "networks", projectID)
	}

	return networks, nil
}

// GetNetworkDetails fetches detailed info for a single VPC network
func (c *ComputeClient) GetNetworkDetails(ctx context.Context, projectID, networkName string) (*NetworkDetails, error) {
	n, err := c.service.Networks.Get(projectID, networkName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "network", networkName)
	}
	return networkDetailsFromAPI(n), nil
}

// ListSubnetsByNetwork retrieves all subnets belonging to a specific network.
// Uses AggregatedList to fetch subnets across all regions, then filters by network.
func (c *ComputeClient) ListSubnetsByNetwork(ctx context.Context, projectID, networkName string) ([]Subnet, error) {
	var subnets []Subnet

	// Build the expected network self-link suffix for filtering
	networkSuffix := "/networks/" + networkName

	req := c.service.Subnetworks.AggregatedList(projectID)
	err := req.Pages(ctx, func(page *compute.SubnetworkAggregatedList) error {
		for _, scopedList := range page.Items {
			if scopedList.Subnetworks == nil {
				continue
			}
			for _, s := range scopedList.Subnetworks {
				// Only include subnets that belong to our target network
				if strings.HasSuffix(s.Network, networkSuffix) {
					subnets = append(subnets, subnetFromAPI(s))
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, WrapListError(err, "subnets", projectID)
	}

	// Sort by region then name for consistent display
	sort.Slice(subnets, func(i, j int) bool {
		if subnets[i].Region != subnets[j].Region {
			return subnets[i].Region < subnets[j].Region
		}
		return subnets[i].Name < subnets[j].Name
	})

	return subnets, nil
}

// networkFromAPI converts a Compute Engine Network to our simplified struct
func networkFromAPI(n *compute.Network) Network {
	routingMode := ""
	if n.RoutingConfig != nil {
		routingMode = n.RoutingConfig.RoutingMode
	}

	return Network{
		Name:         n.Name,
		ID:           n.Id,
		Description:  n.Description,
		AutoCreate:   n.AutoCreateSubnetworks,
		RoutingMode:  routingMode,
		SubnetsCount: len(n.Subnetworks),
		CreatedAt:    n.CreationTimestamp,
	}
}

// networkDetailsFromAPI converts a Compute Engine Network to NetworkDetails
func networkDetailsFromAPI(n *compute.Network) *NetworkDetails {
	routingMode := ""
	if n.RoutingConfig != nil {
		routingMode = n.RoutingConfig.RoutingMode
	}

	peerings := make([]NetworkPeering, len(n.Peerings))
	for i, p := range n.Peerings {
		peerings[i] = NetworkPeering{
			Name:    p.Name,
			Network: extractNameFromURL(p.Network),
			State:   p.State,
		}
	}

	return &NetworkDetails{
		Name:                  n.Name,
		ID:                    n.Id,
		Description:           n.Description,
		AutoCreateSubnetworks: n.AutoCreateSubnetworks,
		RoutingMode:           routingMode,
		Mtu:                   n.Mtu,
		GatewayIPv4:           n.GatewayIPv4,
		EnableUlaInternalIpv6: n.EnableUlaInternalIpv6,
		InternalIpv6Range:     n.InternalIpv6Range,
		Peerings:              peerings,
		SubnetworkCount:       len(n.Subnetworks),
		CreatedAt:             n.CreationTimestamp,
		SelfLink:              n.SelfLink,
	}
}

// subnetFromAPI converts a Compute Engine Subnetwork to our Subnet struct
func subnetFromAPI(s *compute.Subnetwork) Subnet {
	enableFlowLogs := false
	if s.LogConfig != nil {
		enableFlowLogs = s.LogConfig.Enable
	}

	return Subnet{
		Name:                  s.Name,
		Region:                extractRegionFromURL(s.Region),
		IPCidrRange:           s.IpCidrRange,
		GatewayAddress:        s.GatewayAddress,
		Purpose:               s.Purpose,
		PrivateIPGoogleAccess: s.PrivateIpGoogleAccess,
		EnableFlowLogs:        enableFlowLogs,
		CreatedAt:             s.CreationTimestamp,
	}
}

// extractRegionFromURL extracts the region name from a GCP resource URL.
// E.g., ".../regions/us-central1" → "us-central1"
func extractRegionFromURL(url string) string {
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "regions" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	// Fallback: return the last path segment
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

// extractNameFromURL extracts the resource name (last path segment) from a GCP URL.
// E.g., ".../projects/my-project/global/networks/my-network" → "my-network"
func extractNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}
