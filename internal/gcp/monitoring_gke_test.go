package gcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGKEFilter_NodeCPU(t *testing.T) {
	// CPU and memory utilization come from per-node metrics, reduced
	// across the cluster — there is no `kubernetes.io/cluster/*` family.
	f := gkeNodeFilter("prod", "us-central1", "kubernetes.io/node/cpu/allocatable_utilization")
	assert.Contains(t, f, `resource.type = "k8s_node"`)
	assert.Contains(t, f, `resource.labels.cluster_name = "prod"`)
	assert.Contains(t, f, `resource.labels.location = "us-central1"`)
	assert.Contains(t, f, `metric.type = "kubernetes.io/node/cpu/allocatable_utilization"`)
	// Regression: GCP rejects OR between resource.type values.
	assert.False(t, strings.Contains(f, " OR "), "filter must not contain OR between resource types")
}

func TestGKEFilter_NodeNetwork(t *testing.T) {
	f := gkeNodeFilter("prod", "us-central1", "kubernetes.io/node/network/received_bytes_count")
	assert.Contains(t, f, `resource.type = "k8s_node"`)
	assert.Contains(t, f, `resource.labels.cluster_name = "prod"`)
	assert.Contains(t, f, `metric.type = "kubernetes.io/node/network/received_bytes_count"`)
}

func TestGKEFilter_PodCount(t *testing.T) {
	// Pod count uses k8s_pod resource type — REDUCE_COUNT counts unique
	// pod series.
	f := gkePodFilter("prod", "us-central1", "kubernetes.io/pod/network/received_bytes_count")
	assert.Contains(t, f, `resource.type = "k8s_pod"`)
	assert.Contains(t, f, `resource.labels.cluster_name = "prod"`)
	assert.Contains(t, f, `metric.type = "kubernetes.io/pod/network/received_bytes_count"`)
	assert.False(t, strings.Contains(f, " OR "), "filter must not contain OR between resource types")
}
