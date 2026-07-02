package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildClusterUpdate_LoggingMonitoring(t *testing.T) {
	log := "logging.googleapis.com/kubernetes"
	mon := "none"
	upd := buildClusterUpdate(ClusterEdit{
		LoggingService:    &log,
		MonitoringService: &mon,
	})
	require.NotNil(t, upd)
	assert.Equal(t, log, upd.DesiredLoggingService)
	assert.Equal(t, mon, upd.DesiredMonitoringService)
	assert.Contains(t, upd.ForceSendFields, "DesiredLoggingService")
	assert.Contains(t, upd.ForceSendFields, "DesiredMonitoringService")
}

func TestBuildClusterUpdate_LoggingOnly(t *testing.T) {
	log := "logging.googleapis.com/kubernetes"
	upd := buildClusterUpdate(ClusterEdit{LoggingService: &log})
	require.NotNil(t, upd)
	assert.Equal(t, log, upd.DesiredLoggingService)
	assert.Empty(t, upd.DesiredMonitoringService)
	assert.Contains(t, upd.ForceSendFields, "DesiredLoggingService")
	assert.NotContains(t, upd.ForceSendFields, "DesiredMonitoringService")
}

func TestBuildClusterUpdate_NothingChanged(t *testing.T) {
	upd := buildClusterUpdate(ClusterEdit{})
	require.NotNil(t, upd)
	assert.Empty(t, upd.DesiredLoggingService)
	assert.Empty(t, upd.DesiredMonitoringService)
	assert.Empty(t, upd.ForceSendFields)
}

func TestBuildSetResourceLabelsRequest(t *testing.T) {
	labels := map[string]string{"team": "platform", "env": "prod"}
	edit := ClusterEdit{
		ResourceLabels:            &labels,
		ResourceLabelsFingerprint: "abc123",
	}
	req := buildSetResourceLabelsRequest(edit)
	require.NotNil(t, req)
	assert.Equal(t, "platform", req.ResourceLabels["team"])
	assert.Equal(t, "prod", req.ResourceLabels["env"])
	assert.Equal(t, "abc123", req.LabelFingerprint)
}

func TestBuildSetResourceLabelsRequest_EmptyLabels(t *testing.T) {
	labels := map[string]string{}
	edit := ClusterEdit{
		ResourceLabels:            &labels,
		ResourceLabelsFingerprint: "fp42",
	}
	req := buildSetResourceLabelsRequest(edit)
	require.NotNil(t, req)
	assert.Empty(t, req.ResourceLabels)
	assert.Equal(t, "fp42", req.LabelFingerprint)
}

func TestBuildSetResourceLabelsRequest_NilLabelsReturnsNil(t *testing.T) {
	// Callers branch on nil; the builder must not panic on a zero-value edit.
	req := buildSetResourceLabelsRequest(ClusterEdit{})
	assert.Nil(t, req)
}

func TestBuildNodePoolUpdate_LabelsOnly(t *testing.T) {
	labels := map[string]string{"role": "gpu"}
	req := buildNodePoolUpdate(NodePoolEdit{Labels: &labels})
	require.NotNil(t, req)
	require.NotNil(t, req.Labels)
	assert.Equal(t, "gpu", req.Labels.Labels["role"])
	assert.Nil(t, req.Taints)
	assert.Nil(t, req.Tags)
	assert.Nil(t, req.UpgradeSettings)
}

func TestBuildNodePoolUpdate_TagsOnly(t *testing.T) {
	tags := []string{"http-server", "https-server"}
	req := buildNodePoolUpdate(NodePoolEdit{Tags: &tags})
	require.NotNil(t, req)
	assert.Nil(t, req.Labels)
	assert.Nil(t, req.Taints)
	require.NotNil(t, req.Tags)
	assert.Equal(t, []string{"http-server", "https-server"}, req.Tags.Tags)
}

func TestBuildNodePoolUpdate_TaintsAndUpgradeSettings(t *testing.T) {
	taints := []NodeTaint{
		{Key: "dedicated", Value: "gpu", Effect: "NO_SCHEDULE"},
	}
	us := UpgradeSettings{MaxSurge: 2, MaxUnavailable: 0, Strategy: "SURGE"}
	req := buildNodePoolUpdate(NodePoolEdit{Taints: &taints, UpgradeSettings: &us})
	require.NotNil(t, req)
	require.NotNil(t, req.Taints)
	require.Len(t, req.Taints.Taints, 1)
	assert.Equal(t, "dedicated", req.Taints.Taints[0].Key)
	assert.Equal(t, "gpu", req.Taints.Taints[0].Value)
	assert.Equal(t, "NO_SCHEDULE", req.Taints.Taints[0].Effect)
	require.NotNil(t, req.UpgradeSettings)
	assert.Equal(t, int64(2), req.UpgradeSettings.MaxSurge)
	assert.Equal(t, int64(0), req.UpgradeSettings.MaxUnavailable)
	assert.Equal(t, "SURGE", req.UpgradeSettings.Strategy)
	// MaxUnavailable=0 must be sent via ForceSendFields so the zero is not omitted.
	assert.Contains(t, req.UpgradeSettings.ForceSendFields, "MaxUnavailable")
	assert.Contains(t, req.UpgradeSettings.ForceSendFields, "MaxSurge")
}

func TestBuildNodePoolUpdate_MultipleTaints(t *testing.T) {
	taints := []NodeTaint{
		{Key: "dedicated", Value: "gpu", Effect: "NO_SCHEDULE"},
		{Key: "memory", Value: "high", Effect: "PREFER_NO_SCHEDULE"},
		{Key: "critical", Value: "", Effect: "NO_EXECUTE"},
	}
	req := buildNodePoolUpdate(NodePoolEdit{Taints: &taints})
	require.NotNil(t, req.Taints)
	require.Len(t, req.Taints.Taints, 3)
	assert.Equal(t, "PREFER_NO_SCHEDULE", req.Taints.Taints[1].Effect)
	assert.Equal(t, "NO_EXECUTE", req.Taints.Taints[2].Effect)
}

func TestBuildNodePoolUpdate_NothingSet(t *testing.T) {
	req := buildNodePoolUpdate(NodePoolEdit{})
	require.NotNil(t, req)
	assert.Nil(t, req.Labels)
	assert.Nil(t, req.Taints)
	assert.Nil(t, req.Tags)
	assert.Nil(t, req.UpgradeSettings)
}

func TestBuildSetNodePoolManagementRequest_BothTrue(t *testing.T) {
	req := buildSetNodePoolManagementRequest(NodePoolManagement{AutoUpgrade: true, AutoRepair: true})
	require.NotNil(t, req)
	require.NotNil(t, req.Management)
	assert.True(t, req.Management.AutoUpgrade)
	assert.True(t, req.Management.AutoRepair)
	assert.Contains(t, req.Management.ForceSendFields, "AutoUpgrade")
	assert.Contains(t, req.Management.ForceSendFields, "AutoRepair")
}

func TestBuildSetNodePoolManagementRequest_BothFalseForceSend(t *testing.T) {
	req := buildSetNodePoolManagementRequest(NodePoolManagement{AutoUpgrade: false, AutoRepair: false})
	require.NotNil(t, req)
	require.NotNil(t, req.Management)
	assert.False(t, req.Management.AutoUpgrade)
	assert.False(t, req.Management.AutoRepair)
	// Both zero-value bools must be included via ForceSendFields so the API
	// sees explicit false rather than omitted/default.
	assert.Contains(t, req.Management.ForceSendFields, "AutoUpgrade")
	assert.Contains(t, req.Management.ForceSendFields, "AutoRepair")
}

func TestBuildSetNodePoolManagementRequest_Mixed(t *testing.T) {
	req := buildSetNodePoolManagementRequest(NodePoolManagement{AutoUpgrade: true, AutoRepair: false})
	require.NotNil(t, req.Management)
	assert.True(t, req.Management.AutoUpgrade)
	assert.False(t, req.Management.AutoRepair)
	assert.Contains(t, req.Management.ForceSendFields, "AutoUpgrade")
	assert.Contains(t, req.Management.ForceSendFields, "AutoRepair")
}

func TestBuildSetMaintenancePolicyRequest_None(t *testing.T) {
	req := buildSetMaintenancePolicyRequest(MaintenanceWindow{Kind: MaintenanceKindNone})
	require.NotNil(t, req)
	require.NotNil(t, req.MaintenancePolicy)
	require.NotNil(t, req.MaintenancePolicy.Window)
	assert.Nil(t, req.MaintenancePolicy.Window.DailyMaintenanceWindow)
}

func TestBuildSetMaintenancePolicyRequest_Daily(t *testing.T) {
	req := buildSetMaintenancePolicyRequest(MaintenanceWindow{Kind: MaintenanceKindDaily, Daily: "03:00"})
	require.NotNil(t, req.MaintenancePolicy.Window.DailyMaintenanceWindow)
	assert.Equal(t, "03:00", req.MaintenancePolicy.Window.DailyMaintenanceWindow.StartTime)
}

func TestUpdateClusterBasic_RejectsEmptyEdit(t *testing.T) {
	// Empty edit short-circuits before any API call — no live client needed.
	c := &ContainerClient{}
	_, err := c.UpdateClusterBasic(t.Context(), "p", "us-central1", "prod", ClusterEdit{})
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoClusterEditFields)
}

func TestUpdateClusterBasic_RejectsMixedEdit(t *testing.T) {
	// Both labels and services set in one call must be rejected so the
	// app-layer sequential runner splits them into separate steps.
	labels := map[string]string{"team": "platform"}
	log := "none"
	c := &ContainerClient{}
	_, err := c.UpdateClusterBasic(t.Context(), "p", "us-central1", "prod", ClusterEdit{
		ResourceLabels: &labels,
		LoggingService: &log,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, errMixedClusterEdit)
}

func TestUpdateNodePoolFields_RejectsEmptyEdit(t *testing.T) {
	c := &ContainerClient{}
	_, err := c.UpdateNodePoolFields(t.Context(), "p", "us-central1", "prod", "default", NodePoolEdit{})
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoNodePoolEditFields)
}

func TestBuildSetMaintenancePolicyRequest_Recurring(t *testing.T) {
	req := buildSetMaintenancePolicyRequest(MaintenanceWindow{
		Kind:     MaintenanceKindRecurring,
		Days:     []string{"MO", "WE", "FR"},
		Start:    "03:00",
		Duration: "4h",
	})
	require.NotNil(t, req.MaintenancePolicy)
	require.NotNil(t, req.MaintenancePolicy.Window)
	require.NotNil(t, req.MaintenancePolicy.Window.RecurringWindow)
	rw := req.MaintenancePolicy.Window.RecurringWindow
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO,WE,FR", rw.Recurrence)
	require.NotNil(t, rw.Window)
	// Start at 03:00 on baseline date (2026-01-04, Sunday)
	assert.Equal(t, "2026-01-04T03:00:00Z", rw.Window.StartTime)
	// End at 07:00 (start + 4h)
	assert.Equal(t, "2026-01-04T07:00:00Z", rw.Window.EndTime)
}

func TestBuildSetMaintenancePolicyRequest_RecurringSingleDay(t *testing.T) {
	req := buildSetMaintenancePolicyRequest(MaintenanceWindow{
		Kind:     MaintenanceKindRecurring,
		Days:     []string{"SU"},
		Start:    "02:00",
		Duration: "1h",
	})
	rw := req.MaintenancePolicy.Window.RecurringWindow
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=SU", rw.Recurrence)
	assert.Equal(t, "2026-01-04T02:00:00Z", rw.Window.StartTime)
	assert.Equal(t, "2026-01-04T03:00:00Z", rw.Window.EndTime)
}

func TestBuildSetMaintenancePolicyRequest_RecurringInvalidStartFallsBackToMidnight(t *testing.T) {
	req := buildSetMaintenancePolicyRequest(MaintenanceWindow{
		Kind:     MaintenanceKindRecurring,
		Days:     []string{"MO"},
		Start:    "bad-format",
		Duration: "4h",
	})
	rw := req.MaintenancePolicy.Window.RecurringWindow
	assert.Equal(t, "2026-01-04T00:00:00Z", rw.Window.StartTime)
	assert.Equal(t, "2026-01-04T04:00:00Z", rw.Window.EndTime)
}

func TestParseHHMM(t *testing.T) {
	h, m, ok := parseHHMM("03:00")
	require.True(t, ok)
	assert.Equal(t, 3, h)
	assert.Equal(t, 0, m)

	_, _, ok = parseHHMM("bad")
	assert.False(t, ok)

	_, _, ok = parseHHMM("25:00") // out of range
	assert.False(t, ok)
}

func TestParseHoursOr(t *testing.T) {
	assert.Equal(t, 4, parseHoursOr("4h", 1))
	assert.Equal(t, 12, parseHoursOr("12h", 1))
	assert.Equal(t, 1, parseHoursOr("bad", 1))
	assert.Equal(t, 1, parseHoursOr("0h", 1))  // out of range → fallback
	assert.Equal(t, 1, parseHoursOr("24h", 1)) // out of range → fallback
}
