package gcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/api/compute/v1"
)

// Route represents a simplified VPC route for list views
type Route struct {
	Name        string
	Description string
	Network     string // Short name extracted from URL
	DestRange   string // CIDR destination range
	Priority    int64
	NextHop     string // Resolved display string (e.g., instance name, IP)
	NextHopType string // Gateway, Instance, IP, VPNTunnel, Interconnect, ILB
	RouteType   string // Static, Subnet, Peering, System
	Tags        []string
	CreatedAt   string
}

// RouteDetails holds comprehensive route information for the details view
type RouteDetails struct {
	Name        string
	Description string
	Network     string // Short name extracted from URL
	DestRange   string
	Priority    int64
	NextHop     string
	NextHopType string
	RouteType   string
	Tags        []string
	CreatedAt   string

	ID       uint64
	SelfLink string

	// Raw next-hop fields from the API
	NextHopInstance                string
	NextHopIP                     string
	NextHopVPNTunnel              string
	NextHopInterconnectAttachment string
	NextHopILB                    string
	NextHopGateway                string
	NextHopNetwork                string
	NextHopPeering                string

	Warnings []string
}

// RouteConfig holds parameters for creating a new route
type RouteConfig struct {
	Name         string
	Description  string
	Network      string // Network name
	DestRange    string // CIDR
	Priority     int64
	Tags         []string
	NextHopType  string // gateway, instance, ip, vpn-tunnel, interconnect, ilb
	NextHopValue string // Corresponding value for the type
}

// ListRoutes retrieves all routes in a project, sorted by network then priority
func (c *ComputeClient) ListRoutes(ctx context.Context, projectID string) ([]Route, error) {
	var routes []Route

	req := c.service.Routes.List(projectID)
	err := req.Pages(ctx, func(page *compute.RouteList) error {
		for _, r := range page.Items {
			routes = append(routes, routeFromAPI(r))
		}
		return nil
	})
	if err != nil {
		return nil, WrapListError(err, "routes", projectID)
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Network != routes[j].Network {
			return routes[i].Network < routes[j].Network
		}
		return routes[i].Priority < routes[j].Priority
	})

	return routes, nil
}

// ListRoutesByNetwork retrieves routes filtered to a specific network
func (c *ComputeClient) ListRoutesByNetwork(ctx context.Context, projectID, networkName string) ([]Route, error) {
	all, err := c.ListRoutes(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var filtered []Route
	for i := range all {
		if all[i].Network == networkName {
			filtered = append(filtered, all[i])
		}
	}

	return filtered, nil
}

// GetRouteDetails fetches detailed info for a single route
func (c *ComputeClient) GetRouteDetails(ctx context.Context, projectID, routeName string) (*RouteDetails, error) {
	r, err := c.service.Routes.Get(projectID, routeName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "route", routeName)
	}
	return routeDetailsFromAPI(r), nil
}

// CreateRoute creates a new static route
func (c *ComputeClient) CreateRoute(ctx context.Context, projectID string, config RouteConfig) error {
	route := &compute.Route{
		Name:        config.Name,
		Description: config.Description,
		Network:     fmt.Sprintf("projects/%s/global/networks/%s", projectID, config.Network),
		DestRange:   config.DestRange,
		Priority:    config.Priority,
		Tags:        config.Tags,
	}

	// Map the next-hop type to the correct API field
	switch config.NextHopType {
	case "gateway":
		route.NextHopGateway = config.NextHopValue
	case "instance":
		route.NextHopInstance = config.NextHopValue
	case "ip":
		route.NextHopIp = config.NextHopValue
	case "vpn-tunnel":
		route.NextHopVpnTunnel = config.NextHopValue
	case "interconnect":
		route.NextHopInterconnectAttachment = config.NextHopValue
	case "ilb":
		route.NextHopIlb = config.NextHopValue
	}

	_, err := c.service.Routes.Insert(projectID, route).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "create route", config.Name)
	}
	return nil
}

// DeleteRoute deletes a route
func (c *ComputeClient) DeleteRoute(ctx context.Context, projectID, routeName string) error {
	_, err := c.service.Routes.Delete(projectID, routeName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "delete route", routeName)
	}
	return nil
}

// routeFromAPI converts a Compute Engine Route to our simplified Route struct
func routeFromAPI(r *compute.Route) Route {
	nextHop, nextHopType := resolveNextHop(r)
	routeType := deriveRouteType(r)

	return Route{
		Name:        r.Name,
		Description: r.Description,
		Network:     extractNameFromURL(r.Network),
		DestRange:   r.DestRange,
		Priority:    r.Priority,
		NextHop:     nextHop,
		NextHopType: nextHopType,
		RouteType:   routeType,
		Tags:        r.Tags,
		CreatedAt:   r.CreationTimestamp,
	}
}

// routeDetailsFromAPI converts a Compute Engine Route to our RouteDetails struct
func routeDetailsFromAPI(r *compute.Route) *RouteDetails {
	nextHop, nextHopType := resolveNextHop(r)
	routeType := deriveRouteType(r)

	var warnings []string
	for _, w := range r.Warnings {
		if w != nil && w.Message != "" {
			warnings = append(warnings, w.Message)
		}
	}

	return &RouteDetails{
		Name:        r.Name,
		Description: r.Description,
		Network:     extractNameFromURL(r.Network),
		DestRange:   r.DestRange,
		Priority:    r.Priority,
		NextHop:     nextHop,
		NextHopType: nextHopType,
		RouteType:   routeType,
		Tags:        r.Tags,
		CreatedAt:   r.CreationTimestamp,

		ID:       r.Id,
		SelfLink: r.SelfLink,

		NextHopInstance:                r.NextHopInstance,
		NextHopIP:                     r.NextHopIp,
		NextHopVPNTunnel:              r.NextHopVpnTunnel,
		NextHopInterconnectAttachment: r.NextHopInterconnectAttachment,
		NextHopILB:                    r.NextHopIlb,
		NextHopGateway:                r.NextHopGateway,
		NextHopNetwork:                r.NextHopNetwork,
		NextHopPeering:                r.NextHopPeering,

		Warnings: warnings,
	}
}

// deriveRouteType infers the route type since GCP doesn't expose it directly.
// Order matters: peering and subnet checks must come before the system/static fallback.
func deriveRouteType(r *compute.Route) string {
	if r.NextHopPeering != "" {
		return "Peering"
	}
	if r.NextHopNetwork != "" {
		// Auto-created subnet routes have nextHopNetwork set
		return "Subnet"
	}
	if isSystemRoute(r) {
		return "System"
	}
	return "Static"
}

// isSystemRoute checks if a route is a GCP-managed system route.
// System routes have names starting with "default-route-" and use the default internet gateway.
func isSystemRoute(r *compute.Route) bool {
	if !strings.HasPrefix(r.Name, "default-route-") {
		return false
	}
	return strings.HasSuffix(r.NextHopGateway, "/global/gateways/default-internet-gateway")
}

// resolveNextHop returns a human-readable next-hop display string and its type.
// Checks each possible next-hop field in order of specificity.
func resolveNextHop(r *compute.Route) (display, hopType string) {
	switch {
	case r.NextHopInstance != "":
		return extractNameFromURL(r.NextHopInstance), "Instance"
	case r.NextHopIp != "":
		return r.NextHopIp, "IP"
	case r.NextHopVpnTunnel != "":
		return extractNameFromURL(r.NextHopVpnTunnel), "VPNTunnel"
	case r.NextHopInterconnectAttachment != "":
		return extractNameFromURL(r.NextHopInterconnectAttachment), "Interconnect"
	case r.NextHopIlb != "":
		return extractNameFromURL(r.NextHopIlb), "ILB"
	case r.NextHopGateway != "":
		return resolveGatewayDisplay(r.NextHopGateway), "Gateway"
	case r.NextHopNetwork != "":
		return extractNameFromURL(r.NextHopNetwork), "Network"
	case r.NextHopPeering != "":
		return r.NextHopPeering, "Peering"
	default:
		return "", ""
	}
}

// resolveGatewayDisplay returns a friendly display name for gateway URLs.
// The default internet gateway gets a readable label; others show the extracted name.
func resolveGatewayDisplay(gatewayURL string) string {
	if strings.HasSuffix(gatewayURL, "/global/gateways/default-internet-gateway") {
		return "Default internet gateway"
	}
	return extractNameFromURL(gatewayURL)
}
