package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

// ── Task 7: k8s labels editor sub-state ──────────────────────────────────────

func TestGKENodePoolEdit_LabelsEditedTransitionsToDiff(t *testing.T) {
	pool := &gcp.NodePool{Name: "default", Labels: map[string]string{"role": "worker"}}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	v.editingLabels = map[string]string{"role": "gpu"}
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.Equal(t, nodePoolEditStateDiff, v.state)
	require.NotNil(t, v.pendingFields)
	require.NotNil(t, v.pendingFields.Labels)
	assert.Equal(t, "gpu", (*v.pendingFields.Labels)["role"])
}

func TestGKENodePoolEdit_LabelsUnchangedNoDiff(t *testing.T) {
	pool := &gcp.NodePool{Name: "default", Labels: map[string]string{"role": "worker"}}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	v.editingLabels = map[string]string{"role": "worker"} // identical
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.ErrorIs(t, v.err, errNodePoolEditNoChanges)
}

func TestGKENodePoolEdit_OpenLabelEditorUsesK8sValidators(t *testing.T) {
	pool := &gcp.NodePool{Name: "default", Labels: map[string]string{"role": "worker"}}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	v.SetSize(120, 40)
	v.openLabelEditor()
	require.NotNil(t, v.labelEditor)
	assert.Equal(t, nodePoolEditStateEditingLabels, v.state)
	// Smoke test: k8s key regex must accept "kubernetes.io/role" (prefix/name form).
	assert.True(t, k8sLabelKeyPattern.MatchString("kubernetes.io/role"),
		"k8s label key regex should accept DNS-prefixed key")
	// k8s value regex must accept empty (empty-value taints are valid).
	assert.True(t, k8sLabelValuePattern.MatchString(""),
		"k8s label value regex should accept empty string")
}

// ── Task 8: taints editor sub-state ──────────────────────────────────────────

func TestGKENodePoolEdit_TaintsEditedTransitionsToDiff(t *testing.T) {
	pool := &gcp.NodePool{Name: "default", Taints: nil}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	v.editingTaints = []gcp.NodeTaint{
		{Key: "dedicated", Value: "gpu", Effect: "NO_SCHEDULE"},
	}
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.Equal(t, nodePoolEditStateDiff, v.state)
	require.NotNil(t, v.pendingFields)
	require.NotNil(t, v.pendingFields.Taints)
	require.Len(t, *v.pendingFields.Taints, 1)
	assert.Equal(t, "dedicated", (*v.pendingFields.Taints)[0].Key)
}

func TestGKENodePoolEdit_TaintsUnchangedNoDiff(t *testing.T) {
	initial := []gcp.NodeTaint{{Key: "k", Value: "v", Effect: "NO_SCHEDULE"}}
	pool := &gcp.NodePool{Name: "default", Taints: initial}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	v.editingTaints = append([]gcp.NodeTaint(nil), initial...)
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.ErrorIs(t, v.err, errNodePoolEditNoChanges)
}

func TestGKENodePoolEdit_TaintsOrderInsensitive(t *testing.T) {
	pool := &gcp.NodePool{Name: "default", Taints: []gcp.NodeTaint{
		{Key: "a", Value: "1", Effect: "NO_SCHEDULE"},
		{Key: "b", Value: "2", Effect: "NO_EXECUTE"},
	}}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	// Same set, different order — must NOT be a diff.
	v.editingTaints = []gcp.NodeTaint{
		{Key: "b", Value: "2", Effect: "NO_EXECUTE"},
		{Key: "a", Value: "1", Effect: "NO_SCHEDULE"},
	}
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.ErrorIs(t, v.err, errNodePoolEditNoChanges)
}

// Duplicate keys with differing effect/value must compare equal as a multiset.
// Sorting by Key alone would be non-deterministic and could miss real changes.
func TestTaintsEqual_DuplicateKeysFullTupleSort(t *testing.T) {
	a := []gcp.NodeTaint{
		{Key: "dedicated", Value: "gpu", Effect: "NO_SCHEDULE"},
		{Key: "dedicated", Value: "tpu", Effect: "NO_EXECUTE"},
	}
	b := []gcp.NodeTaint{
		{Key: "dedicated", Value: "tpu", Effect: "NO_EXECUTE"},
		{Key: "dedicated", Value: "gpu", Effect: "NO_SCHEDULE"},
	}
	assert.True(t, taintsEqual(a, b), "same multiset different order must compare equal")

	c := []gcp.NodeTaint{
		{Key: "dedicated", Value: "gpu", Effect: "NO_SCHEDULE"},
		{Key: "dedicated", Value: "gpu", Effect: "NO_EXECUTE"}, // different effect
	}
	assert.False(t, taintsEqual(a, c), "different effect on same key must compare unequal")
}

func TestGKENodePoolEdit_NoChangesRejected(t *testing.T) {
	pool := &gcp.NodePool{
		Name:        "default",
		AutoUpgrade: true,
		AutoRepair:  true,
		UpgradeSettings: &gcp.UpgradeSettings{
			MaxSurge:       1,
			MaxUnavailable: 0,
			Strategy:       "SURGE",
		},
	}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.ErrorIs(t, v.err, errNodePoolEditNoChanges)
	assert.Equal(t, nodePoolEditStateForm, v.state)
}

func TestGKENodePoolEdit_ManagementToggleEmits(t *testing.T) {
	pool := &gcp.NodePool{
		Name:        "default",
		AutoUpgrade: true,
		AutoRepair:  true,
	}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	v.Form.SetData(map[string]any{
		"auto_upgrade": false,
	})
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.Equal(t, nodePoolEditStateDiff, v.state)
	require.NotNil(t, v.pendingManagement)
	assert.False(t, v.pendingManagement.AutoUpgrade)
	assert.True(t, v.pendingManagement.AutoRepair)
	assert.Nil(t, v.pendingFields, "fields not dirty")
}

func TestGKENodePoolEdit_UpgradeSettingsChangeTransitionsToDiff(t *testing.T) {
	pool := &gcp.NodePool{
		Name: "default",
		UpgradeSettings: &gcp.UpgradeSettings{
			MaxSurge:       1,
			MaxUnavailable: 0,
			Strategy:       "SURGE",
		},
	}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	v.Form.SetData(map[string]any{
		"max_surge": int64(3),
	})
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.Equal(t, nodePoolEditStateDiff, v.state)
	require.NotNil(t, v.pendingFields)
	require.NotNil(t, v.pendingFields.UpgradeSettings)
	assert.Equal(t, int64(3), v.pendingFields.UpgradeSettings.MaxSurge)
	assert.Equal(t, "SURGE", v.pendingFields.UpgradeSettings.Strategy)
}

func TestGKENodePoolEdit_ConfirmDeployEmitsRequest(t *testing.T) {
	pool := &gcp.NodePool{
		Name:        "default",
		AutoUpgrade: true,
	}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	v.Form.SetData(map[string]any{"auto_upgrade": false})
	require.Nil(t, v.handleSubmit())
	require.Equal(t, nodePoolEditStateDiff, v.state)
	cmd := v.confirmDeploy()
	require.NotNil(t, cmd)
	assert.Equal(t, nodePoolEditStateSaving, v.state)
	batchMsg, ok := cmd().(tea.BatchMsg)
	require.True(t, ok)
	found := false
	for _, c := range batchMsg {
		if c == nil {
			continue
		}
		if req, ok := c().(GKENodePoolEditRequestMsg); ok {
			require.NotNil(t, req.Management)
			assert.False(t, req.Management.AutoUpgrade)
			found = true
			break
		}
	}
	assert.True(t, found)
}


func TestGKENodePoolEdit_UnknownStrategyNoFalsePositive(t *testing.T) {
	// Regression: a pool reporting an unknown strategy (e.g. "SHORT_LIVED")
	// must not silently flag a diff when the user has not touched the
	// dropdown — the baseline is forced to SURGE so the form value equals
	// the baseline.
	pool := &gcp.NodePool{
		Name:        "default",
		AutoUpgrade: true,
		AutoRepair:  true,
		UpgradeSettings: &gcp.UpgradeSettings{
			MaxSurge:       1,
			MaxUnavailable: 0,
			Strategy:       "SHORT_LIVED",
		},
	}
	v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
	cmd := v.handleSubmit()
	assert.Nil(t, cmd)
	assert.ErrorIs(t, v.err, errNodePoolEditNoChanges)
	assert.Equal(t, nodePoolEditStateForm, v.state)
}
