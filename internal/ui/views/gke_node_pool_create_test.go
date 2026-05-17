package views

import (
	"testing"

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
	msg := cmd()
	req, ok := msg.(GKENodePoolCreateRequestMsg)
	require.True(t, ok)
	assert.Equal(t, "gpu-pool", req.Pool.Name)
	assert.Equal(t, int64(2), req.Pool.InitialNodeCount)
}
