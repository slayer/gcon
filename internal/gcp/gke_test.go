package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/container/v1"
)

func TestLocationType(t *testing.T) {
	cases := map[string]string{
		"us-central1-a":  "zone",
		"us-central1-b":  "zone",
		"europe-west2-c": "zone",
		"us-central1":    "region",
		"europe-west2":   "region",
		"":               "region", // empty defaults to region; harmless since no API call hits this path
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, locationType(in))
		})
	}
}

func TestConvertCluster_AutopilotRegional(t *testing.T) {
	raw := &container.Cluster{
		Name:                 "prod",
		Location:             "us-central1",
		Status:               "RUNNING",
		CurrentMasterVersion: "1.30.5-gke.1014001",
		CurrentNodeVersion:   "1.30.5-gke.1014001",
		CurrentNodeCount:     12,
		Network:              "default",
		Subnetwork:           "default-uscent1",
		Endpoint:             "34.123.45.67",
		CreateTime:           "2025-08-12T14:03:00Z",
		Autopilot:            &container.Autopilot{Enabled: true},
		ReleaseChannel:       &container.ReleaseChannel{Channel: "REGULAR"},
		PrivateClusterConfig: &container.PrivateClusterConfig{EnablePrivateNodes: true},
	}
	got := convertCluster(raw)

	assert.Equal(t, "prod", got.Name)
	assert.Equal(t, "us-central1", got.Location)
	assert.Equal(t, "region", got.LocationType)
	assert.Equal(t, "AUTOPILOT", got.Mode)
	assert.Equal(t, "RUNNING", got.Status)
	assert.Equal(t, "1.30.5-gke.1014001", got.MasterVersion)
	assert.Equal(t, "1.30.5-gke.1014001", got.NodeVersion)
	assert.Equal(t, 12, got.NodeCount)
	assert.Equal(t, "default", got.Network)
	assert.Equal(t, "default-uscent1", got.Subnetwork)
	assert.Equal(t, "REGULAR", got.ReleaseChannel)
	assert.Equal(t, "34.123.45.67", got.Endpoint)
	assert.True(t, got.PrivateCluster)
	assert.Equal(t, "2025-08-12T14:03:00Z", got.CreatedAt)
}

func TestConvertCluster_StandardZonal_MinimalFields(t *testing.T) {
	raw := &container.Cluster{
		Name:     "dev",
		Location: "us-central1-a",
		Status:   "RUNNING",
		// No Autopilot block → STANDARD
		// No ReleaseChannel block → empty
		// No PrivateClusterConfig → false
	}
	got := convertCluster(raw)

	assert.Equal(t, "STANDARD", got.Mode)
	assert.Equal(t, "zone", got.LocationType)
	assert.Equal(t, "", got.ReleaseChannel)
	assert.False(t, got.PrivateCluster)
}

func TestConvertNodePool_AutoscalingOn(t *testing.T) {
	raw := &container.NodePool{
		Name:             "default",
		InitialNodeCount: 3,
		Locations:        []string{"us-central1-a", "us-central1-b"},
		Version:          "1.30.5-gke.1014001",
		Status:           "RUNNING",
		Config: &container.NodeConfig{
			MachineType: "e2-medium",
			DiskSizeGb:  100,
			DiskType:    "pd-balanced",
		},
		Autoscaling: &container.NodePoolAutoscaling{
			Enabled:      true,
			MinNodeCount: 1,
			MaxNodeCount: 10,
		},
		Management: &container.NodeManagement{AutoUpgrade: true, AutoRepair: true},
	}
	got := convertNodePool(raw)

	assert.Equal(t, "default", got.Name)
	assert.Equal(t, "e2-medium", got.MachineType)
	assert.Equal(t, int64(100), got.DiskSizeGB)
	assert.Equal(t, "pd-balanced", got.DiskType)
	assert.Equal(t, 3, got.NodeCount)
	assert.True(t, got.AutoscalingOn)
	assert.Equal(t, 1, got.AutoscalingMin)
	assert.Equal(t, 10, got.AutoscalingMax)
	assert.Equal(t, "1.30.5-gke.1014001", got.NodeVersion)
	assert.Equal(t, "RUNNING", got.Status)
	assert.True(t, got.AutoUpgrade)
	assert.True(t, got.AutoRepair)
	assert.Equal(t, []string{"us-central1-a", "us-central1-b"}, got.Locations)
}

func TestConvertNodePool_AutoscalingOff_NoManagement(t *testing.T) {
	raw := &container.NodePool{
		Name:             "gpu-pool",
		InitialNodeCount: 2,
		Status:           "RUNNING",
		Config:           &container.NodeConfig{MachineType: "g2-standard-4"},
		// No Autoscaling, no Management
	}
	got := convertNodePool(raw)

	assert.False(t, got.AutoscalingOn)
	assert.Equal(t, 0, got.AutoscalingMin)
	assert.Equal(t, 0, got.AutoscalingMax)
	assert.False(t, got.AutoUpgrade)
	assert.False(t, got.AutoRepair)
}

func TestDatabaseEncryption(t *testing.T) {
	cases := []struct {
		name    string
		in      *container.Cluster
		wantOn  bool
		wantKey string
	}{
		{"nil", &container.Cluster{}, false, ""},
		{"decrypted-state", &container.Cluster{DatabaseEncryption: &container.DatabaseEncryption{State: "DECRYPTED"}}, false, ""},
		{"encrypted-no-key", &container.Cluster{DatabaseEncryption: &container.DatabaseEncryption{State: "ENCRYPTED"}}, true, ""},
		{"encrypted-with-key", &container.Cluster{DatabaseEncryption: &container.DatabaseEncryption{State: "ENCRYPTED", KeyName: "projects/p/locations/global/keyRings/r/cryptoKeys/my-key"}}, true, "projects/p/locations/global/keyRings/r/cryptoKeys/my-key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotOn, gotKey := databaseEncryption(tc.in)
			assert.Equal(t, tc.wantOn, gotOn)
			assert.Equal(t, tc.wantKey, gotKey)
		})
	}
}

func TestUniformNodeVersion(t *testing.T) {
	assert.True(t, uniformNodeVersion(nil))
	assert.True(t, uniformNodeVersion([]NodePool{{NodeVersion: "1.30"}}))
	assert.True(t, uniformNodeVersion([]NodePool{{NodeVersion: "1.30"}, {NodeVersion: "1.30"}}))
	assert.False(t, uniformNodeVersion([]NodePool{{NodeVersion: "1.30"}, {NodeVersion: "1.29"}}))
}
