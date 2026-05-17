package gcp

import (
	"context"
	"fmt"
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
