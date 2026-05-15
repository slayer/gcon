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
