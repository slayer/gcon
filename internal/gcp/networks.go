package gcp

import (
	"context"
	"fmt"
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
	Network               string // Network name, extracted from network URL
	Region                string // Extracted from self-link URL
	IPCidrRange           string
	GatewayAddress        string
	Purpose               string // PRIVATE, REGIONAL_MANAGED_PROXY, etc.
	PrivateIPGoogleAccess bool
	EnableFlowLogs        bool
	CreatedAt             string
}

// SubnetDetails holds comprehensive subnet information for the details view
type SubnetDetails struct {
	ID                    uint64
	Name                  string
	Description           string
	Status                string // READY, etc.
	Region                string
	Network               string // Network name extracted from URL
	IPCidrRange           string
	GatewayAddress        string
	Purpose               string
	StackType             string // IPV4_ONLY, IPV4_IPV6
	IPv6AccessType        string
	IPv6CidrRange         string
	PrivateIPGoogleAccess bool
	EnableFlowLogs        bool
	FlowLogConfig         FlowLogConfig
	SecondaryIPRanges     []SecondaryRange
	CreatedAt             string
	SelfLink              string
}

// FlowLogConfig holds VPC flow log configuration details
type FlowLogConfig struct {
	AggregationInterval string
	FlowSampling        float64
	Metadata            string
	FilterExpr          string
}

// SecondaryRange represents a secondary IP range in a subnet
type SecondaryRange struct {
	Name      string
	CidrRange string
}

// SubnetCreateConfig holds configuration for creating a new subnet
type SubnetCreateConfig struct {
	Name                string
	Description         string
	Network             string // Network name
	Region              string
	CIDRRange           string
	Purpose             string // PRIVATE, REGIONAL_MANAGED_PROXY, INTERNAL_HTTPS_LOAD_BALANCER
	StackType           string // IPV4_ONLY, IPV4_IPV6
	PrivateGoogleAccess bool
	EnableFlowLogs      bool
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

// ListAllSubnets retrieves all subnets across all regions and networks.
func (c *ComputeClient) ListAllSubnets(ctx context.Context, projectID string) ([]Subnet, error) {
	var subnets []Subnet

	req := c.service.Subnetworks.AggregatedList(projectID)
	err := req.Pages(ctx, func(page *compute.SubnetworkAggregatedList) error {
		for _, scopedList := range page.Items {
			if scopedList.Subnetworks == nil {
				continue
			}
			for _, s := range scopedList.Subnetworks {
				subnets = append(subnets, subnetFromAPI(s))
			}
		}
		return nil
	})
	if err != nil {
		return nil, WrapListError(err, "subnets", projectID)
	}

	sort.Slice(subnets, func(i, j int) bool {
		if subnets[i].Region != subnets[j].Region {
			return subnets[i].Region < subnets[j].Region
		}
		return subnets[i].Name < subnets[j].Name
	})

	return subnets, nil
}

// GetSubnetDetails fetches detailed information for a single subnet
func (c *ComputeClient) GetSubnetDetails(ctx context.Context, projectID, region, subnetName string) (*SubnetDetails, error) {
	s, err := c.service.Subnetworks.Get(projectID, region, subnetName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "subnet", subnetName)
	}
	return subnetDetailsFromAPI(s), nil
}

// CreateSubnet creates a new subnet in the specified region
func (c *ComputeClient) CreateSubnet(ctx context.Context, projectID string, config SubnetCreateConfig) error {
	subnet := &compute.Subnetwork{
		Name:                  config.Name,
		Description:           config.Description,
		Network:               fmt.Sprintf("projects/%s/global/networks/%s", projectID, config.Network),
		IpCidrRange:           config.CIDRRange,
		Purpose:               config.Purpose,
		StackType:             config.StackType,
		PrivateIpGoogleAccess: config.PrivateGoogleAccess,
	}

	if config.EnableFlowLogs {
		subnet.LogConfig = &compute.SubnetworkLogConfig{
			Enable: true,
		}
	}

	_, err := c.service.Subnetworks.Insert(projectID, config.Region, subnet).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "create subnet", config.Name)
	}
	return nil
}

// DeleteSubnet deletes a subnet in the specified region
func (c *ComputeClient) DeleteSubnet(ctx context.Context, projectID, region, subnetName string) error {
	_, err := c.service.Subnetworks.Delete(projectID, region, subnetName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "delete subnet", subnetName)
	}
	return nil
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
		Network:               extractNameFromURL(s.Network),
		Region:                extractRegionFromURL(s.Region),
		IPCidrRange:           s.IpCidrRange,
		GatewayAddress:        s.GatewayAddress,
		Purpose:               s.Purpose,
		PrivateIPGoogleAccess: s.PrivateIpGoogleAccess,
		EnableFlowLogs:        enableFlowLogs,
		CreatedAt:             s.CreationTimestamp,
	}
}

// subnetDetailsFromAPI converts a Compute Engine Subnetwork to our SubnetDetails struct
func subnetDetailsFromAPI(s *compute.Subnetwork) *SubnetDetails {
	enableFlowLogs := false
	var flowLogCfg FlowLogConfig
	if s.LogConfig != nil {
		enableFlowLogs = s.LogConfig.Enable
		flowLogCfg = FlowLogConfig{
			AggregationInterval: s.LogConfig.AggregationInterval,
			FlowSampling:        s.LogConfig.FlowSampling,
			Metadata:            s.LogConfig.Metadata,
			FilterExpr:          s.LogConfig.FilterExpr,
		}
	}

	var secondaryRanges []SecondaryRange
	for _, r := range s.SecondaryIpRanges {
		secondaryRanges = append(secondaryRanges, SecondaryRange{
			Name:      r.RangeName,
			CidrRange: r.IpCidrRange,
		})
	}

	return &SubnetDetails{
		ID:                    s.Id,
		Name:                  s.Name,
		Description:           s.Description,
		Status:                s.State,
		Region:                extractRegionFromURL(s.Region),
		Network:               extractNameFromURL(s.Network),
		IPCidrRange:           s.IpCidrRange,
		GatewayAddress:        s.GatewayAddress,
		Purpose:               s.Purpose,
		StackType:             s.StackType,
		IPv6AccessType:        s.Ipv6AccessType,
		IPv6CidrRange:         s.Ipv6CidrRange,
		PrivateIPGoogleAccess: s.PrivateIpGoogleAccess,
		EnableFlowLogs:        enableFlowLogs,
		FlowLogConfig:         flowLogCfg,
		SecondaryIPRanges:     secondaryRanges,
		CreatedAt:             s.CreationTimestamp,
		SelfLink:              s.SelfLink,
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
