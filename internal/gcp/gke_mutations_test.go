package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodePoolFQN_Delete(t *testing.T) {
	// nodePoolFQN composes the full GKE resource path used by every
	// mutation. Regression guard: trailing-slash / missing-zone bugs.
	got := nodePoolFQN("proj", "us-central1", "prod", "default")
	assert.Equal(t, "projects/proj/locations/us-central1/clusters/prod/nodePools/default", got)
}

func TestClusterFQN(t *testing.T) {
	// Regression guard: cluster FQN segment order/casing.
	got := clusterFQN("proj", "us-central1", "prod")
	assert.Equal(t, "projects/proj/locations/us-central1/clusters/prod", got)
}

func TestSetNodePoolSizeRequest_ZeroCount(t *testing.T) {
	// Scale-to-zero requires ForceSendFields on NodeCount (per the
	// gcp-api-gotchas.md "ForceSendFields for Create operations" rule
	// — Go's omitempty would drop a 0).
	req := buildSetNodePoolSizeRequest(0)
	assert.Equal(t, int64(0), req.NodeCount)
	assert.Contains(t, req.ForceSendFields, "NodeCount")
}

func TestUpdateNodePoolAutoscalingRequest_DisabledZeroes(t *testing.T) {
	// Disabling autoscale must send Enabled=false even though it's
	// the Go zero value — same ForceSendFields requirement.
	req := buildUpdateNodePoolAutoscalingRequest(false, 0, 0)
	require.NotNil(t, req.Autoscaling)
	assert.False(t, req.Autoscaling.Enabled)
	assert.Contains(t, req.Autoscaling.ForceSendFields, "Enabled")
}
