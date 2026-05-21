package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

func TestGKEClusterEdit_NoChangesRejected(t *testing.T) {
	details := &gcp.ClusterDetails{
		Cluster:           gcp.Cluster{Name: "prod", Location: "us-central1"},
		LoggingService:    "logging.googleapis.com/kubernetes",
		MonitoringService: "monitoring.googleapis.com/kubernetes",
		MaintenanceDaily:  "03:00",
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.ErrorIs(t, v.err, errClusterEditNoChanges)
	assert.Equal(t, clusterEditStateForm, v.state)
}

func TestGKEClusterEdit_LoggingServiceChangeTransitionsToDiff(t *testing.T) {
	details := &gcp.ClusterDetails{
		Cluster:           gcp.Cluster{Name: "prod"},
		LoggingService:    "logging.googleapis.com/kubernetes",
		MonitoringService: "monitoring.googleapis.com/kubernetes",
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	v.Form.SetData(map[string]any{
		"logging_service": "none",
	})
	cmd := v.handleSubmit()
	assert.Nil(t, cmd, "handleSubmit only transitions state; deploy happens on Enter")
	assert.Equal(t, clusterEditStateDiff, v.state)
	require.NotNil(t, v.pendingBasic)
	require.NotNil(t, v.pendingBasic.LoggingService)
	assert.Equal(t, "none", *v.pendingBasic.LoggingService)
	assert.Nil(t, v.pendingMaintenance)
}

func TestGKEClusterEdit_MaintenanceChangeTransitionsToDiff(t *testing.T) {
	details := &gcp.ClusterDetails{
		Cluster:           gcp.Cluster{Name: "prod"},
		LoggingService:    "logging.googleapis.com/kubernetes",
		MonitoringService: "monitoring.googleapis.com/kubernetes",
		MaintenanceDaily:  "", // initial: no daily window
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	v.Form.SetData(map[string]any{
		"maintenance_kind":        "daily",
		"maintenance_daily_start": "03:00",
	})
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.Equal(t, clusterEditStateDiff, v.state)
	require.NotNil(t, v.pendingMaintenance)
	assert.Equal(t, gcp.MaintenanceKindDaily, v.pendingMaintenance.Kind)
	assert.Equal(t, "03:00", v.pendingMaintenance.Daily)
}

func TestGKEClusterEdit_ConfirmDeployEmitsRequest(t *testing.T) {
	details := &gcp.ClusterDetails{
		Cluster:        gcp.Cluster{Name: "prod"},
		LoggingService: "none",
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	v.Form.SetData(map[string]any{
		"logging_service": "logging.googleapis.com/kubernetes",
	})
	require.Nil(t, v.handleSubmit())
	require.Equal(t, clusterEditStateDiff, v.state)

	cmd := v.confirmDeploy()
	require.NotNil(t, cmd)
	assert.Equal(t, clusterEditStateSaving, v.state)

	batchMsg, ok := cmd().(tea.BatchMsg)
	require.True(t, ok)
	found := false
	for _, c := range batchMsg {
		if c == nil {
			continue
		}
		if req, ok := c().(GKEClusterEditRequestMsg); ok {
			require.NotNil(t, req.Basic)
			require.NotNil(t, req.Basic.LoggingService)
			assert.Equal(t, "logging.googleapis.com/kubernetes", *req.Basic.LoggingService)
			found = true
			break
		}
	}
	assert.True(t, found, "batch must contain GKEClusterEditRequestMsg")
}

func TestGKEClusterEdit_UnknownLoggingNoFalsePositive(t *testing.T) {
	// Regression: a cluster reporting a legacy logging service (e.g.
	// "logging.googleapis.com" without "/kubernetes") must not flag a
	// diff when the user has not touched the dropdown — the form leaves
	// the dropdown at its default ("none") and the baseline is forced to
	// "none" so they match.
	details := &gcp.ClusterDetails{
		Cluster:           gcp.Cluster{Name: "prod"},
		LoggingService:    "logging.googleapis.com",    // legacy, unknown
		MonitoringService: "monitoring.googleapis.com", // legacy, unknown
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.ErrorIs(t, v.err, errClusterEditNoChanges)
	assert.Equal(t, clusterEditStateForm, v.state)
}

func TestGKEClusterEdit_LabelsEditedTransitionsToDiff(t *testing.T) {
	details := &gcp.ClusterDetails{
		Cluster:                   gcp.Cluster{Name: "prod"},
		LoggingService:            "logging.googleapis.com/kubernetes",
		MonitoringService:         "monitoring.googleapis.com/kubernetes",
		ResourceLabels:            map[string]string{"env": "dev"},
		ResourceLabelsFingerprint: "fp1",
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	// Simulate the user editing labels (skip the labeledit UI for this unit test).
	v.editingLabels = map[string]string{"env": "prod"}

	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.Equal(t, clusterEditStateDiff, v.state)
	require.NotNil(t, v.pendingBasic)
	require.NotNil(t, v.pendingBasic.ResourceLabels)
	assert.Equal(t, "prod", (*v.pendingBasic.ResourceLabels)["env"])
	assert.Equal(t, "fp1", v.pendingBasic.ResourceLabelsFingerprint)
}

func TestGKEClusterEdit_LabelsUnchangedNoDiff(t *testing.T) {
	details := &gcp.ClusterDetails{
		Cluster:           gcp.Cluster{Name: "prod"},
		LoggingService:    "logging.googleapis.com/kubernetes",
		MonitoringService: "monitoring.googleapis.com/kubernetes",
		ResourceLabels:    map[string]string{"env": "dev"},
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	// Same map as initial — must not trigger a diff.
	v.editingLabels = map[string]string{"env": "dev"}

	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.ErrorIs(t, v.err, errClusterEditNoChanges)
}

func TestGKEClusterEdit_RecurringMaintenanceChangeTransitionsToDiff(t *testing.T) {
	details := &gcp.ClusterDetails{
		Cluster:           gcp.Cluster{Name: "prod"},
		LoggingService:    "logging.googleapis.com/kubernetes",
		MonitoringService: "monitoring.googleapis.com/kubernetes",
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	v.Form.SetData(map[string]any{
		"maintenance_kind":               "recurring",
		"maintenance_days":               []string{"MO", "WE", "FR"},
		"maintenance_recurring_start":    "03:00",
		"maintenance_recurring_duration": int64(4),
	})
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.Equal(t, clusterEditStateDiff, v.state)
	require.NotNil(t, v.pendingMaintenance)
	assert.Equal(t, gcp.MaintenanceKindRecurring, v.pendingMaintenance.Kind)
	assert.Equal(t, []string{"FR", "MO", "WE"}, v.pendingMaintenance.Days) // sorted
	assert.Equal(t, "03:00", v.pendingMaintenance.Start)
	assert.Equal(t, "4h", v.pendingMaintenance.Duration)
}

func TestGKEClusterEdit_RecurringMaintenanceRejectsEmptyDays(t *testing.T) {
	details := &gcp.ClusterDetails{
		Cluster:           gcp.Cluster{Name: "prod"},
		LoggingService:    "logging.googleapis.com/kubernetes",
		MonitoringService: "monitoring.googleapis.com/kubernetes",
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	v.Form.SetData(map[string]any{
		"maintenance_kind":               "recurring",
		"maintenance_days":               []string{},
		"maintenance_recurring_start":    "03:00",
		"maintenance_recurring_duration": int64(4),
	})
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.ErrorIs(t, v.err, errMaintenanceDaysEmpty)
}

func TestGKEClusterEdit_RecurringRoundTripNoFalsePositive(t *testing.T) {
	// Cluster already has a recurring window matching the form's defaults.
	details := &gcp.ClusterDetails{
		Cluster:           gcp.Cluster{Name: "prod"},
		LoggingService:    "logging.googleapis.com/kubernetes",
		MonitoringService: "monitoring.googleapis.com/kubernetes",
		MaintenanceRecurring: &gcp.RecurringWindow{
			Days: []string{"MO", "WE", "FR"}, Start: "03:00", Duration: "4h",
		},
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	// Form should be pre-populated; submit with no changes must NOT create a diff.
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.ErrorIs(t, v.err, errClusterEditNoChanges)
}

func TestGKEClusterEdit_OpenLabelEditorTransitionsToEditingState(t *testing.T) {
	details := &gcp.ClusterDetails{
		Cluster:        gcp.Cluster{Name: "prod"},
		ResourceLabels: map[string]string{"a": "1"},
	}
	v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
	v.SetSize(120, 40)
	v.openLabelEditor()
	assert.Equal(t, clusterEditStateEditingLabels, v.state)
	require.NotNil(t, v.labelEditor)
	assert.True(t, v.HasTextInputFocused() || !v.labelEditor.IsEditing(), "delegation works in either state")
}
