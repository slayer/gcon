package gcp

import (
	"time"

	"google.golang.org/api/container/v1"
)

// Cluster is the list-view projection of a GKE cluster.
type Cluster struct {
	Name           string
	Location       string // "us-central1-a" or "us-central1"
	LocationType   string // "zone" | "region"
	Mode           string // "AUTOPILOT" | "STANDARD"
	Status         string // PROVISIONING / RUNNING / RECONCILING / STOPPING / ERROR / DEGRADED
	MasterVersion  string
	NodeVersion    string // "(varies)" when non-uniform across pools
	NodeCount      int    // sum across node pools
	Network        string
	Subnetwork     string
	ReleaseChannel string // RAPID / REGULAR / STABLE / "" (unspecified)
	Endpoint       string
	PrivateCluster bool
	CreatedAt      time.Time
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
	DatabaseEncryption       string // "ENCRYPTED (key: name)" | "DECRYPTED"
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
