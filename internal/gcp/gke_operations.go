package gcp

import (
	"context"
	"fmt"
	"time"

	container "google.golang.org/api/container/v1"
)

// Operation is the GKE long-running-operation projection. Mutation calls
// return one of these; the UI polls via GetOperation until Status=="DONE".
type Operation struct {
	Name          string
	Type          string
	Status        string
	Target        string
	StatusMessage string
	Detail        string
	StartTime     time.Time
	EndTime       time.Time
}

// GetOperation fetches the current state of an op. `name` is the short
// operation name returned by the mutation that created it (e.g.
// "operation-XXXXX"). `location` is the zone or region of the cluster
// the op targets.
func (c *ContainerClient) GetOperation(ctx context.Context, projectID, location, name string) (Operation, error) {
	fqn := fmt.Sprintf("projects/%s/locations/%s/operations/%s", projectID, location, name)
	raw, err := c.service.Projects.Locations.Operations.Get(fqn).Context(ctx).Do()
	if err != nil {
		return Operation{}, fmt.Errorf("get operation %s: %w", name, err)
	}
	return projectOperation(raw), nil
}

func projectOperation(raw *container.Operation) Operation {
	op := Operation{
		Name:          raw.Name,
		Type:          raw.OperationType,
		Status:        raw.Status,
		Target:        raw.TargetLink,
		StatusMessage: raw.StatusMessage,
		Detail:        raw.Detail,
	}
	if raw.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, raw.StartTime); err == nil {
			op.StartTime = t
		}
	}
	if raw.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, raw.EndTime); err == nil {
			op.EndTime = t
		}
	}
	return op
}
