package gcp

import (
	"context"

	"google.golang.org/api/compute/v1"
)

// InstanceLabelsFingerprint holds labels and their fingerprint for optimistic locking
type InstanceLabelsFingerprint struct {
	Labels      map[string]string
	Fingerprint string
}

// GetInstanceLabelsFingerprint returns current labels and fingerprint for an instance.
// The fingerprint is used for optimistic locking when updating labels.
func (c *ComputeClient) GetInstanceLabelsFingerprint(ctx context.Context, projectID, zone, instanceName string) (*InstanceLabelsFingerprint, error) {
	inst, err := c.service.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "instance labels", instanceName)
	}

	result := &InstanceLabelsFingerprint{
		Labels:      make(map[string]string),
		Fingerprint: inst.LabelFingerprint,
	}

	// Copy labels if present
	if inst.Labels != nil {
		for k, v := range inst.Labels {
			result.Labels[k] = v
		}
	}

	return result, nil
}

// SetInstanceLabels updates instance labels using the fingerprint for optimistic locking.
// Returns an error if the fingerprint has changed (concurrent modification).
func (c *ComputeClient) SetInstanceLabels(ctx context.Context, projectID, zone, instanceName string, labels map[string]string, fingerprint string) error {
	req := &compute.InstancesSetLabelsRequest{
		LabelFingerprint: fingerprint,
		Labels:           labels,
	}

	_, err := c.service.Instances.SetLabels(projectID, zone, instanceName, req).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "set instance labels", instanceName)
	}

	return nil
}
