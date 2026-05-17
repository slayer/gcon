package gcp

import (
	"context"
	"fmt"

	container "google.golang.org/api/container/v1"
)

// nodePoolFQN composes the full resource path GKE uses for node-pool
// operations: projects/X/locations/Y/clusters/Z/nodePools/W.
func nodePoolFQN(projectID, location, clusterName, poolName string) string {
	return fmt.Sprintf("projects/%s/locations/%s/clusters/%s/nodePools/%s",
		projectID, location, clusterName, poolName)
}

//nolint:unused // used by Task 4 (UpgradeControlPlane)
func clusterFQN(projectID, location, clusterName string) string {
	return fmt.Sprintf("projects/%s/locations/%s/clusters/%s",
		projectID, location, clusterName)
}

// DeleteNodePool kicks a pool deletion. Returns the Operation projection
// so the caller can poll it.
func (c *ContainerClient) DeleteNodePool(ctx context.Context, projectID, location, clusterName, poolName string) (Operation, error) {
	raw, err := c.service.Projects.Locations.Clusters.NodePools.
		Delete(nodePoolFQN(projectID, location, clusterName, poolName)).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("delete node pool %s: %w", poolName, err)
	}
	return projectOperation(raw), nil
}

// SetNodePoolSize sets the current node count. Operation completes when
// the pool reaches the target. Pass count=0 to scale the pool to zero
// (the pool stays defined; instances drain).
func (c *ContainerClient) SetNodePoolSize(ctx context.Context, projectID, location, clusterName, poolName string, count int64) (Operation, error) {
	req := buildSetNodePoolSizeRequest(count)
	raw, err := c.service.Projects.Locations.Clusters.NodePools.
		SetSize(nodePoolFQN(projectID, location, clusterName, poolName), req).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("resize node pool %s to %d: %w", poolName, count, err)
	}
	return projectOperation(raw), nil
}

func buildSetNodePoolSizeRequest(count int64) *container.SetNodePoolSizeRequest {
	return &container.SetNodePoolSizeRequest{
		NodeCount:       count,
		ForceSendFields: []string{"NodeCount"},
	}
}

// UpdateNodePoolAutoscaling toggles autoscale on/off and sets the
// min/max bounds in one call. Pass enabled=false to disable.
func (c *ContainerClient) UpdateNodePoolAutoscaling(ctx context.Context, projectID, location, clusterName, poolName string, enabled bool, minCount, maxCount int64) (Operation, error) {
	req := buildUpdateNodePoolAutoscalingRequest(enabled, minCount, maxCount)
	raw, err := c.service.Projects.Locations.Clusters.NodePools.
		SetAutoscaling(nodePoolFQN(projectID, location, clusterName, poolName), req).
		Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("update autoscaling on %s: %w", poolName, err)
	}
	return projectOperation(raw), nil
}

func buildUpdateNodePoolAutoscalingRequest(enabled bool, minCount, maxCount int64) *container.SetNodePoolAutoscalingRequest {
	as := &container.NodePoolAutoscaling{
		Enabled:         enabled,
		MinNodeCount:    minCount,
		MaxNodeCount:    maxCount,
		ForceSendFields: []string{"Enabled", "MinNodeCount", "MaxNodeCount"},
	}
	return &container.SetNodePoolAutoscalingRequest{
		Autoscaling: as,
	}
}
