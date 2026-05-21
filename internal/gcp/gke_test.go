package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestServerConfigProjection(t *testing.T) {
	raw := &container.ServerConfig{
		DefaultClusterVersion: "1.30.5-gke.1014001",
		ValidMasterVersions:   []string{"1.31.1-gke.1", "1.30.5-gke.1014001"},
		ValidNodeVersions:     []string{"1.31.1-gke.1", "1.30.5-gke.1014001"},
	}
	cfg := projectServerConfig(raw)
	assert.Equal(t, "1.30.5-gke.1014001", cfg.DefaultClusterVersion)
	assert.Len(t, cfg.ValidMasterVersions, 2)
	assert.Equal(t, "1.31.1-gke.1", cfg.ValidMasterVersions[0])
}

// ── Phase 2c: edit-form pre-population projection tests ──────────────────────

func buildTestClusterDetails(raw *container.Cluster) *ClusterDetails {
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
	if rw := maintenanceRecurringPolicy(raw); rw != nil {
		out.MaintenanceRecurring = rw
	}
	for _, np := range raw.NodePools {
		out.NodePools = append(out.NodePools, convertNodePool(np))
	}
	return out
}

func TestConvertCluster_EditFieldsPopulated(t *testing.T) {
	raw := &container.Cluster{
		Name:             "prod",
		ResourceLabels:   map[string]string{"team": "platform"},
		LabelFingerprint: "fp-abc",
		LoggingService:   "logging.googleapis.com/kubernetes",
		MonitoringService: "none",
		MaintenancePolicy: &container.MaintenancePolicy{
			Window: &container.MaintenanceWindow{
				DailyMaintenanceWindow: &container.DailyMaintenanceWindow{StartTime: "03:00"},
			},
		},
	}
	details := buildTestClusterDetails(raw)
	assert.Equal(t, "platform", details.ResourceLabels["team"])
	assert.Equal(t, "fp-abc", details.ResourceLabelsFingerprint)
	assert.Equal(t, "logging.googleapis.com/kubernetes", details.LoggingService)
	assert.Equal(t, "none", details.MonitoringService)
	assert.Equal(t, "03:00", details.MaintenanceDaily)
}

func TestConvertCluster_NoMaintenanceWindow(t *testing.T) {
	raw := &container.Cluster{Name: "prod"} // no maintenance policy
	details := buildTestClusterDetails(raw)
	assert.Empty(t, details.MaintenanceDaily)
}

func TestDailyMaintenanceStart(t *testing.T) {
	cases := []struct {
		name   string
		policy *container.MaintenancePolicy
		want   string
	}{
		{"nil policy", nil, ""},
		{"nil window", &container.MaintenancePolicy{}, ""},
		{"nil daily window", &container.MaintenancePolicy{Window: &container.MaintenanceWindow{}}, ""},
		{"recurring only", &container.MaintenancePolicy{Window: &container.MaintenanceWindow{
			RecurringWindow: &container.RecurringTimeWindow{},
		}}, ""},
		{"daily window set", &container.MaintenancePolicy{Window: &container.MaintenanceWindow{
			DailyMaintenanceWindow: &container.DailyMaintenanceWindow{StartTime: "06:00"},
		}}, "06:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dailyMaintenanceStart(tc.policy))
		})
	}
}

func TestConvertNodePool_EditFieldsPopulated(t *testing.T) {
	raw := &container.NodePool{
		Name: "gpu",
		Config: &container.NodeConfig{
			Labels: map[string]string{"role": "gpu"},
			Taints: []*container.NodeTaint{
				{Key: "dedicated", Value: "gpu", Effect: "NO_SCHEDULE"},
			},
			Tags: []string{"http-server"},
		},
		UpgradeSettings: &container.UpgradeSettings{MaxSurge: 2, MaxUnavailable: 0, Strategy: "SURGE"},
	}
	pool := convertNodePool(raw)
	assert.Equal(t, "gpu", pool.Labels["role"])
	require.Len(t, pool.Taints, 1)
	assert.Equal(t, "dedicated", pool.Taints[0].Key)
	assert.Equal(t, "gpu", pool.Taints[0].Value)
	assert.Equal(t, "NO_SCHEDULE", pool.Taints[0].Effect)
	assert.Equal(t, []string{"http-server"}, pool.Tags)
	require.NotNil(t, pool.UpgradeSettings)
	assert.Equal(t, int64(2), pool.UpgradeSettings.MaxSurge)
	assert.Equal(t, int64(0), pool.UpgradeSettings.MaxUnavailable)
	assert.Equal(t, "SURGE", pool.UpgradeSettings.Strategy)
}

func TestConvertNodePool_MultipleTagsAndTaints(t *testing.T) {
	raw := &container.NodePool{
		Name: "multi",
		Config: &container.NodeConfig{
			Tags: []string{"tag-a", "tag-b"},
			Taints: []*container.NodeTaint{
				{Key: "k1", Value: "v1", Effect: "NO_SCHEDULE"},
				{Key: "k2", Value: "v2", Effect: "NO_EXECUTE"},
			},
		},
	}
	pool := convertNodePool(raw)
	assert.Equal(t, []string{"tag-a", "tag-b"}, pool.Tags)
	require.Len(t, pool.Taints, 2)
	assert.Equal(t, "NO_EXECUTE", pool.Taints[1].Effect)
	assert.Nil(t, pool.UpgradeSettings)
}

func TestConvertNodePool_NoEditFields(t *testing.T) {
	raw := &container.NodePool{Name: "default"} // no Config, no UpgradeSettings
	pool := convertNodePool(raw)
	assert.Nil(t, pool.Labels)
	assert.Nil(t, pool.Taints)
	assert.Nil(t, pool.Tags)
	assert.Nil(t, pool.UpgradeSettings)
}

// ── Phase 2d: recurring window projection tests ──────────────────────────────

func TestConvertCluster_RecurringWindow(t *testing.T) {
	raw := &container.Cluster{
		Name: "prod",
		MaintenancePolicy: &container.MaintenancePolicy{
			Window: &container.MaintenanceWindow{
				RecurringWindow: &container.RecurringTimeWindow{
					Window: &container.TimeWindow{
						StartTime: "2026-01-05T03:00:00Z",
						EndTime:   "2026-01-05T07:00:00Z",
					},
					Recurrence: "FREQ=WEEKLY;BYDAY=MO,WE,FR",
				},
			},
		},
	}
	details := buildTestClusterDetails(raw)
	require.NotNil(t, details.MaintenanceRecurring)
	assert.Equal(t, []string{"MO", "WE", "FR"}, details.MaintenanceRecurring.Days)
	assert.Equal(t, "03:00", details.MaintenanceRecurring.Start)
	assert.Equal(t, "4h", details.MaintenanceRecurring.Duration)
}

func TestConvertCluster_RecurringSingleDay(t *testing.T) {
	raw := &container.Cluster{
		MaintenancePolicy: &container.MaintenancePolicy{
			Window: &container.MaintenanceWindow{
				RecurringWindow: &container.RecurringTimeWindow{
					Window: &container.TimeWindow{
						StartTime: "2026-01-04T02:00:00Z",
						EndTime:   "2026-01-04T03:00:00Z",
					},
					Recurrence: "FREQ=WEEKLY;BYDAY=SU",
				},
			},
		},
	}
	details := buildTestClusterDetails(raw)
	require.NotNil(t, details.MaintenanceRecurring)
	assert.Equal(t, []string{"SU"}, details.MaintenanceRecurring.Days)
	assert.Equal(t, "02:00", details.MaintenanceRecurring.Start)
	assert.Equal(t, "1h", details.MaintenanceRecurring.Duration)
}

func TestConvertCluster_RecurringUnsupportedRRULE(t *testing.T) {
	raw := &container.Cluster{
		MaintenancePolicy: &container.MaintenancePolicy{
			Window: &container.MaintenanceWindow{
				RecurringWindow: &container.RecurringTimeWindow{
					Window: &container.TimeWindow{StartTime: "2026-01-04T03:00:00Z", EndTime: "2026-01-04T07:00:00Z"},
					Recurrence: "FREQ=MONTHLY;BYMONTHDAY=1",
				},
			},
		},
	}
	details := buildTestClusterDetails(raw)
	assert.Nil(t, details.MaintenanceRecurring, "monthly RRULE must not parse")
}

func TestConvertCluster_NoRecurringWindow(t *testing.T) {
	raw := &container.Cluster{Name: "prod"} // no maintenance policy
	details := buildTestClusterDetails(raw)
	assert.Nil(t, details.MaintenanceRecurring)
}

func TestParseWeeklyByday(t *testing.T) {
	assert.Equal(t, []string{"MO", "WE", "FR"}, parseWeeklyByday("FREQ=WEEKLY;BYDAY=MO,WE,FR"))
	assert.Equal(t, []string{"SU"}, parseWeeklyByday("BYDAY=SU;FREQ=WEEKLY")) // order flexible
	assert.Nil(t, parseWeeklyByday("FREQ=MONTHLY;BYDAY=MO"))
	assert.Nil(t, parseWeeklyByday("FREQ=WEEKLY;BYDAY=ZZ"))
	assert.Nil(t, parseWeeklyByday("FREQ=WEEKLY")) // no BYDAY
	assert.Nil(t, parseWeeklyByday(""))
}

func TestParseRecurringWindowTimes(t *testing.T) {
	start, dur := parseRecurringWindowTimes("2026-01-04T03:00:00Z", "2026-01-04T07:00:00Z")
	assert.Equal(t, "03:00", start)
	assert.Equal(t, "4h", dur)

	start, dur = parseRecurringWindowTimes("bad", "2026-01-04T07:00:00Z")
	assert.Empty(t, start)
	assert.Empty(t, dur)

	start, _ = parseRecurringWindowTimes("2026-01-04T07:00:00Z", "2026-01-04T03:00:00Z") // end before start
	assert.Empty(t, start)
}
