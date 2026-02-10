package gcp

import (
	"context"

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
