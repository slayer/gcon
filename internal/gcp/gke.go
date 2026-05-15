package gcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/api/container/v1"
	"google.golang.org/api/option"
)

// locationType returns "zone" for fully-qualified zones (us-central1-a) and
// "region" otherwise. A GKE location with two or more "-" segments is a
// zone; with one or zero segments it is a region.
func locationType(location string) string {
	if strings.Count(location, "-") >= 2 {
		return "zone"
	}
	return "region"
}

// Cluster is the list-view projection of a GKE cluster.
type Cluster struct {
	Name                string
	Location            string // "us-central1-a" or "us-central1"
	LocationType        string // "zone" | "region"
	Mode                string // "AUTOPILOT" | "STANDARD"
	Status              string // PROVISIONING / RUNNING / RECONCILING / STOPPING / ERROR / DEGRADED
	MasterVersion       string
	NodeVersion         string // version of the first pool ("" if no pools)
	NodeVersionsUniform bool   // true when all pools share NodeVersion
	NodeCount           int    // sum across node pools
	Network             string
	Subnetwork          string
	ReleaseChannel      string // RAPID / REGULAR / STABLE / "" (unspecified)
	Endpoint            string
	PrivateCluster      bool
	CreatedAt           string // raw CreationTimestamp pass-through (RFC3339)
}

// ClusterDetails is the full projection used by the details view.
type ClusterDetails struct {
	Cluster
	NodePools                []NodePool
	Addons                   AddonsSummary
	ClusterIPv4CIDR          string
	ServicesIPv4CIDR         string
	WorkloadIdentityPool     string // "" when disabled
	MasterAuthorizedNetworks []string
	DatabaseEncrypted        bool
	DatabaseKMSKey           string // full key URI; empty when not encrypted
}

// AddonsSummary captures the four addons surfaced in Phase 1.
type AddonsSummary struct {
	HTTPLoadBalancing bool
	NetworkPolicy     bool
	PersistentDiskCSI bool
	DNSCache          bool
}

// NodePool is the per-pool projection used by the Node Pools tab.
type NodePool struct {
	Name           string
	MachineType    string
	DiskSizeGB     int64
	DiskType       string
	NodeCount      int
	AutoscalingMin int
	AutoscalingMax int
	AutoscalingOn  bool
	NodeVersion    string
	Status         string
	AutoUpgrade    bool
	AutoRepair     bool
	Locations      []string // zones the pool spans
}

// ContainerClient wraps the GKE container API.
type ContainerClient struct {
	service *container.Service
}

// convertCluster maps the raw container.Cluster into our list-view Cluster.
// Mode is derived from the Autopilot block, never from the raw API in the UI.
// The list view doesn't have authoritative per-pool versions, so
// NodeVersionsUniform defaults to true (overridden in GetCluster).
func convertCluster(c *container.Cluster) Cluster {
	mode := "STANDARD"
	if c.Autopilot != nil && c.Autopilot.Enabled {
		mode = "AUTOPILOT"
	}
	releaseChannel := ""
	if c.ReleaseChannel != nil {
		releaseChannel = c.ReleaseChannel.Channel
	}
	private := false
	if c.PrivateClusterConfig != nil {
		private = c.PrivateClusterConfig.EnablePrivateNodes
	}
	return Cluster{
		Name:                c.Name,
		Location:            c.Location,
		LocationType:        locationType(c.Location),
		Mode:                mode,
		Status:              c.Status,
		MasterVersion:       c.CurrentMasterVersion,
		NodeVersion:         c.CurrentNodeVersion,
		NodeVersionsUniform: true,
		NodeCount:           int(c.CurrentNodeCount),
		Network:             c.Network,
		Subnetwork:          c.Subnetwork,
		ReleaseChannel:      releaseChannel,
		Endpoint:            c.Endpoint,
		PrivateCluster:      private,
		CreatedAt:           c.CreateTime,
	}
}

// convertNodePool maps a raw container.NodePool into our per-pool projection.
// AutoscalingOn is only true when the Autoscaling block is present and
// explicitly enabled; min/max stay zero when autoscaling is off. AutoUpgrade
// and AutoRepair default to false when the Management block is absent.
func convertNodePool(p *container.NodePool) NodePool {
	out := NodePool{
		Name:        p.Name,
		NodeCount:   int(p.InitialNodeCount),
		NodeVersion: p.Version,
		Status:      p.Status,
		Locations:   p.Locations,
	}
	if p.Config != nil {
		out.MachineType = p.Config.MachineType
		out.DiskSizeGB = p.Config.DiskSizeGb
		out.DiskType = p.Config.DiskType
	}
	if p.Autoscaling != nil && p.Autoscaling.Enabled {
		out.AutoscalingOn = true
		out.AutoscalingMin = int(p.Autoscaling.MinNodeCount)
		out.AutoscalingMax = int(p.Autoscaling.MaxNodeCount)
	}
	if p.Management != nil {
		out.AutoUpgrade = p.Management.AutoUpgrade
		out.AutoRepair = p.Management.AutoRepair
	}
	return out
}

// NewContainerClient creates a GKE container API client using ADC.
func NewContainerClient(ctx context.Context) (*ContainerClient, error) {
	svc, err := container.NewService(ctx, option.WithScopes(container.CloudPlatformScope))
	if err != nil {
		return nil, fmt.Errorf("create container client: %w", err)
	}
	return &ContainerClient{service: svc}, nil
}

// ListClusters returns all clusters across all locations for the project.
// Location "-" is the GKE wildcard for "any location".
func (c *ContainerClient) ListClusters(ctx context.Context, projectID string) ([]Cluster, error) {
	parent := fmt.Sprintf("projects/%s/locations/-", projectID)
	resp, err := c.service.Projects.Locations.Clusters.List(parent).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}
	out := make([]Cluster, 0, len(resp.Clusters))
	for _, raw := range resp.Clusters {
		out = append(out, convertCluster(raw))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
