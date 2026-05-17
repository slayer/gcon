package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodePoolFQN_Delete(t *testing.T) {
	// nodePoolFQN composes the full GKE resource path used by every
	// mutation. Regression guard: trailing-slash / missing-zone bugs.
	got := nodePoolFQN("proj", "us-central1", "prod", "default")
	assert.Equal(t, "projects/proj/locations/us-central1/clusters/prod/nodePools/default", got)
}
