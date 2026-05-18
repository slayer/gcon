package views

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGKENodePoolCreate_FormHasRequiredSections(t *testing.T) {
	v := NewGKENodePoolCreateView("proj", "us-central1", "prod", nil)
	require.NotNil(t, v.Form)
	want := []string{"Basic", "Autoscaling", "Lifecycle"}
	got := []string{}
	for _, s := range v.Form.Sections() {
		got = append(got, s.Title)
	}
	for _, w := range want {
		assert.Contains(t, got, w, "form must include the %s section", w)
	}
}

func TestGKENodePoolCreate_SubmitEmitsRequest(t *testing.T) {
	v := NewGKENodePoolCreateView("proj", "us-central1", "prod", nil)
	v.Form.SetData(map[string]any{
		"name":          "gpu-pool",
		"initial_count": int64(2),
	})
	cmd := v.handleSubmit()
	require.NotNil(t, cmd)

	// handleSubmit returns tea.Batch(spinnerTick, requestCmd) — unwrap to find the request msg.
	batchMsg, ok := cmd().(tea.BatchMsg)
	require.True(t, ok, "expected tea.BatchMsg")

	var req GKENodePoolCreateRequestMsg
	found := false
	for _, c := range batchMsg {
		if c == nil {
			continue
		}
		if r, ok := c().(GKENodePoolCreateRequestMsg); ok {
			req = r
			found = true
			break
		}
	}
	require.True(t, found, "batch must contain a GKENodePoolCreateRequestMsg")
	assert.Equal(t, "gpu-pool", req.Pool.Name)
	assert.Equal(t, int64(2), req.Pool.InitialNodeCount)
}

func TestGKENodePoolCreate_SubmitRejectsInvertedAutoscaleRange(t *testing.T) {
	v := NewGKENodePoolCreateView("proj", "us-central1", "prod", nil)
	v.Form.SetData(map[string]any{
		"name":              "gpu-pool",
		"initial_count":     int64(5),
		"autoscale_enabled": true,
		"min_nodes":         int64(8),
		"max_nodes":         int64(3),
	})
	cmd := v.handleSubmit()
	assert.Nil(t, cmd, "submit must be blocked when min > max")
	assert.NotNil(t, v.Err)
	assert.True(t, errors.Is(v.Err, errNodePoolAutoscaleRangeInverted),
		"error must wrap errNodePoolAutoscaleRangeInverted, got %v", v.Err)
}
