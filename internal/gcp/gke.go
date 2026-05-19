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

	// Fields used for edit form pre-population (Phase 2c).
	ResourceLabels            map[string]string // billing/org labels on the cluster
	ResourceLabelsFingerprint string            // required when patching labels to avoid 409 (from LabelFingerprint)
	LoggingService            string            // e.g. "logging.googleapis.com/kubernetes" or "none"
	MonitoringService         string
	MaintenanceDaily          string // "HH:MM" UTC; "" when no daily window is set
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
	// InstanceGroupUrls is the list of MIG URLs backing this pool,
	// one per zone the pool spans. Used by the Nodes sub-view to fan
	// out ListManagedInstances calls.
	InstanceGroupUrls []string

	// Fields used for edit form pre-population (Phase 2c).
	Labels          map[string]string // k8s labels on node config (NOT the pool's resource labels)
	Taints          []NodeTaint       // node taints from node config; nil when none
	Tags            []string          // network tags from node config; nil when none
	UpgradeSettings *UpgradeSettings  // nil when not set by the API
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
		Name:              p.Name,
		NodeCount:         int(p.InitialNodeCount),
		NodeVersion:       p.Version,
		Status:            p.Status,
		Locations:         p.Locations,
		InstanceGroupUrls: p.InstanceGroupUrls,
	}
	if p.Config != nil {
		out.MachineType = p.Config.MachineType
		out.DiskSizeGB = p.Config.DiskSizeGb
		out.DiskType = p.Config.DiskType
		out.Labels = p.Config.Labels
		out.Tags = p.Config.Tags
		if len(p.Config.Taints) > 0 {
			out.Taints = make([]NodeTaint, 0, len(p.Config.Taints))
			for _, t := range p.Config.Taints {
				out.Taints = append(out.Taints, NodeTaint{
					Key:    t.Key,
					Value:  t.Value,
					Effect: t.Effect,
				})
			}
		}
	}
	if p.UpgradeSettings != nil {
		out.UpgradeSettings = &UpgradeSettings{
			MaxSurge:       p.UpgradeSettings.MaxSurge,
			MaxUnavailable: p.UpgradeSettings.MaxUnavailable,
			Strategy:       p.UpgradeSettings.Strategy,
		}
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

// GetCluster fetches a single cluster and projects it to ClusterDetails.
// Node pools come from the response's NodePools field — no separate list call.
func (c *ContainerClient) GetCluster(ctx context.Context, projectID, location, name string) (*ClusterDetails, error) {
	fqn := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, name)
	raw, err := c.service.Projects.Locations.Clusters.Get(fqn).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get cluster %s/%s: %w", location, name, err)
	}
	out := &ClusterDetails{
		Cluster:                   convertCluster(raw),
		ClusterIPv4CIDR:           raw.ClusterIpv4Cidr,
		Addons:                    convertAddons(raw.AddonsConfig),
		WorkloadIdentityPool:      workloadIdentityPool(raw),
		MasterAuthorizedNetworks:  authorizedNetworks(raw),
		ResourceLabels:            raw.ResourceLabels,
		ResourceLabelsFingerprint: raw.LabelFingerprint,
		LoggingService:            raw.LoggingService,
		MonitoringService:         raw.MonitoringService,
		MaintenanceDaily:          dailyMaintenanceStart(raw.MaintenancePolicy),
	}
	out.DatabaseEncrypted, out.DatabaseKMSKey = databaseEncryption(raw)
	if raw.IpAllocationPolicy != nil {
		out.ServicesIPv4CIDR = raw.IpAllocationPolicy.ServicesIpv4CidrBlock
	}
	for _, np := range raw.NodePools {
		out.NodePools = append(out.NodePools, convertNodePool(np))
	}
	// The list-view convertCluster set NodeVersion from CurrentNodeVersion and
	// NodeVersionsUniform=true. Override authoritatively from the per-pool data.
	if len(out.NodePools) > 0 {
		out.NodeVersion = out.NodePools[0].NodeVersion
		out.NodeVersionsUniform = uniformNodeVersion(out.NodePools)
	}
	return out, nil
}

func convertAddons(a *container.AddonsConfig) AddonsSummary {
	if a == nil {
		return AddonsSummary{}
	}
	return AddonsSummary{
		HTTPLoadBalancing: a.HttpLoadBalancing != nil && !a.HttpLoadBalancing.Disabled,
		NetworkPolicy:     a.NetworkPolicyConfig != nil && !a.NetworkPolicyConfig.Disabled,
		PersistentDiskCSI: a.GcePersistentDiskCsiDriverConfig != nil && a.GcePersistentDiskCsiDriverConfig.Enabled,
		DNSCache:          a.DnsCacheConfig != nil && a.DnsCacheConfig.Enabled,
	}
}

func workloadIdentityPool(c *container.Cluster) string {
	if c.WorkloadIdentityConfig == nil {
		return ""
	}
	return c.WorkloadIdentityConfig.WorkloadPool
}

func authorizedNetworks(c *container.Cluster) []string {
	if c.MasterAuthorizedNetworksConfig == nil || !c.MasterAuthorizedNetworksConfig.Enabled {
		return nil
	}
	out := make([]string, 0, len(c.MasterAuthorizedNetworksConfig.CidrBlocks))
	for _, b := range c.MasterAuthorizedNetworksConfig.CidrBlocks {
		out = append(out, b.CidrBlock)
	}
	return out
}

// dailyMaintenanceStart returns the "HH:MM" UTC start time of the cluster's
// daily maintenance window, or "" when no daily window is configured (absent
// policy, absent window, absent daily window, or recurring-window-only config).
func dailyMaintenanceStart(p *container.MaintenancePolicy) string {
	if p == nil || p.Window == nil || p.Window.DailyMaintenanceWindow == nil {
		return ""
	}
	return p.Window.DailyMaintenanceWindow.StartTime
}

// databaseEncryption returns whether the cluster has DB encryption enabled and
// (if so) the full KMS key resource URI. Empty key URI when not encrypted.
func databaseEncryption(c *container.Cluster) (encrypted bool, keyName string) {
	if c.DatabaseEncryption == nil || c.DatabaseEncryption.State != "ENCRYPTED" {
		return false, ""
	}
	return true, c.DatabaseEncryption.KeyName
}

// ListNodePools returns the node pools for a single cluster. Phase 2 uses
// this for per-tab refresh; Phase 1 reads pools from GetCluster instead.
func (c *ContainerClient) ListNodePools(ctx context.Context, projectID, location, clusterName string) ([]NodePool, error) {
	parent := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, clusterName)
	resp, err := c.service.Projects.Locations.Clusters.NodePools.List(parent).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list node pools %s/%s: %w", location, clusterName, err)
	}
	out := make([]NodePool, 0, len(resp.NodePools))
	for _, raw := range resp.NodePools {
		out = append(out, convertNodePool(raw))
	}
	return out, nil
}

// DeleteCluster kicks off a cluster delete. Returns when the API accepts
// the request; the resulting Operation runs server-side and is not polled.
func (c *ContainerClient) DeleteCluster(ctx context.Context, projectID, location, name string) error {
	fqn := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, name)
	if _, err := c.service.Projects.Locations.Clusters.Delete(fqn).Context(ctx).Do(); err != nil {
		return fmt.Errorf("delete cluster %s/%s: %w", location, name, err)
	}
	return nil
}

func uniformNodeVersion(pools []NodePool) bool {
	if len(pools) <= 1 {
		return true
	}
	v := pools[0].NodeVersion
	for i := 1; i < len(pools); i++ {
		if pools[i].NodeVersion != v {
			return false
		}
	}
	return true
}

// GKENode is a cluster-aware view of a MIG instance. The Pool field is
// set by the caller in views/gke_nodes.go from loop context (we iterate
// each NodePool's InstanceGroupUrls), not parsed from the MIG name —
// pool names can contain hyphens, making MIG-name parsing ambiguous.
type GKENode struct {
	MIGInstance
	Pool string
}

// ServerConfig is the subset of container.ServerConfig the upgrade
// picker reads. ValidMasterVersions / ValidNodeVersions are sorted
// newest-first by GCP and we keep that order.
type ServerConfig struct {
	DefaultClusterVersion string
	ValidMasterVersions   []string
	ValidNodeVersions     []string
}

func (c *ContainerClient) GetServerConfig(ctx context.Context, projectID, location string) (ServerConfig, error) {
	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, location)
	raw, err := c.service.Projects.Locations.GetServerConfig(parent).Context(ctx).Do()
	if err != nil {
		return ServerConfig{}, fmt.Errorf("get server config for %s: %w", location, err)
	}
	return projectServerConfig(raw), nil
}

func projectServerConfig(raw *container.ServerConfig) ServerConfig {
	return ServerConfig{
		DefaultClusterVersion: raw.DefaultClusterVersion,
		ValidMasterVersions:   append([]string{}, raw.ValidMasterVersions...),
		ValidNodeVersions:     append([]string{}, raw.ValidNodeVersions...),
	}
}
