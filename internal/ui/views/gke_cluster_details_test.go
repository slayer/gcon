package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func gkeDetailsFixture(mode string) *GKEClusterDetailsView {
	v := NewGKEClusterDetailsView("proj", "us-central1", "prod", nil, nil)
	v.SetSize(160, 40)
	v.loading = false
	v.details = &gcp.ClusterDetails{
		Cluster: gcp.Cluster{
			Name:                "prod",
			Location:            "us-central1",
			LocationType:        "region",
			Mode:                mode,
			Status:              "RUNNING",
			MasterVersion:       "1.30.5-gke.1014001",
			NodeVersion:         "1.30.5-gke.1014001",
			NodeVersionsUniform: true,
			Network:             "default",
			Subnetwork:          "default-uscent1",
			ReleaseChannel:      "REGULAR",
			Endpoint:            "34.123.45.67",
			PrivateCluster:      false,
			CreatedAt:           "2025-08-12T14:03:00Z",
		},
		ClusterIPv4CIDR:      "10.4.0.0/14",
		ServicesIPv4CIDR:     "10.8.0.0/20",
		Addons:               gcp.AddonsSummary{HTTPLoadBalancing: true, PersistentDiskCSI: true},
		WorkloadIdentityPool: "prod.svc.id.goog",
		DatabaseEncrypted:    true,
		DatabaseKMSKey:       "projects/p/locations/global/keyRings/r/cryptoKeys/my-key",
		NodePools: []gcp.NodePool{
			{Name: "default", MachineType: "e2-medium", NodeCount: 3, AutoscalingOn: true, AutoscalingMin: 1, AutoscalingMax: 10, NodeVersion: "1.30.5-gke.1014001", Status: "RUNNING", AutoUpgrade: true, AutoRepair: true},
		},
	}
	return v
}

func TestGKEClusterDetails_OverviewRendersStandard(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.tabs.SetActiveByID("overview")
	out := v.View()
	assert.Contains(t, out, "Standard")
	assert.Contains(t, out, "us-central1 (regional)")
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "default-uscent1")
	assert.Contains(t, out, "REGULAR")
	assert.Contains(t, out, "prod.svc.id.goog")
	assert.Contains(t, out, "ENCRYPTED (key: my-key)")
	assert.Contains(t, out, "HTTP load balancing: Enabled")
	assert.Contains(t, out, "Network policy: Disabled")
}

func TestGKEClusterDetails_OverviewRendersAutopilot(t *testing.T) {
	v := gkeDetailsFixture("AUTOPILOT")
	v.tabs.SetActiveByID("overview")
	out := v.View()
	assert.Contains(t, out, "Autopilot")
}

func TestGKEClusterDetails_NodePoolsRendersStandard(t *testing.T) {
	v := gkeDetailsFixture("STANDARD")
	v.tabs.SetActiveByID("nodepools")
	v.refreshNodePoolsTable()
	out := v.View()
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "e2-medium")
	assert.Contains(t, out, "on (1–10)")
	assert.Contains(t, out, "1.30.5-gke.1014001")
}

func TestGKEClusterDetails_NodePoolsRendersAutopilotSuffix(t *testing.T) {
	v := gkeDetailsFixture("AUTOPILOT")
	// Inject a system-managed pool
	v.details.NodePools = []gcp.NodePool{
		{Name: "default-pool", MachineType: "e2-medium", NodeCount: 1, Status: "RUNNING"},
	}
	v.tabs.SetActiveByID("nodepools")
	v.refreshNodePoolsTable()
	out := v.View()
	assert.Contains(t, out, "default-pool [managed by Autopilot]")
	assert.Contains(t, out, "—") // autoscale / version cells
}
