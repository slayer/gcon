package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

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
