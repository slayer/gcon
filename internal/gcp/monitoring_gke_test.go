package gcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGKEFilter_ClusterCPU(t *testing.T) {
	f := gkeClusterFilter("prod", "us-central1", "kubernetes.io/cluster/cpu/allocatable_utilization")
	assert.Contains(t, f, `resource.type = "k8s_cluster"`)
	assert.Contains(t, f, `resource.labels.cluster_name = "prod"`)
	assert.Contains(t, f, `resource.labels.location = "us-central1"`)
	assert.Contains(t, f, `metric.type = "kubernetes.io/cluster/cpu/allocatable_utilization"`)
	// Regression: GCP rejects OR between resource.type values.
	assert.False(t, strings.Contains(f, " OR "), "filter must not contain OR between resource types")
}

func TestGKEFilter_NodeNetwork(t *testing.T) {
	f := gkeNodeFilter("prod", "us-central1", "kubernetes.io/node/network/received_bytes_count")
	assert.Contains(t, f, `resource.type = "k8s_node"`)
	assert.Contains(t, f, `resource.labels.cluster_name = "prod"`)
	assert.Contains(t, f, `metric.type = "kubernetes.io/node/network/received_bytes_count"`)
}
