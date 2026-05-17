package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	container "google.golang.org/api/container/v1"
)

func TestProjectOperation(t *testing.T) {
	raw := &container.Operation{
		Name:          "operation-1234-abcd",
		OperationType: "UPGRADE_MASTER",
		Status:        "RUNNING",
		TargetLink:    "https://container.googleapis.com/v1/projects/p/locations/us-central1/clusters/prod",
		StatusMessage: "Upgrading control plane",
		StartTime:     "2026-05-17T10:00:00Z",
		EndTime:       "",
		Detail:        "Phase: PRE_UPGRADE",
	}
	op := projectOperation(raw)
	assert.Equal(t, "operation-1234-abcd", op.Name)
	assert.Equal(t, "UPGRADE_MASTER", op.Type)
	assert.Equal(t, "RUNNING", op.Status)
	assert.Equal(t, "Phase: PRE_UPGRADE", op.Detail)
	assert.False(t, op.StartTime.IsZero())
	assert.True(t, op.EndTime.IsZero())
}

func TestProjectOperation_DoneSetsEndTime(t *testing.T) {
	raw := &container.Operation{
		Name:    "op-done",
		Status:  "DONE",
		EndTime: "2026-05-17T10:05:00Z",
	}
	op := projectOperation(raw)
	assert.Equal(t, "DONE", op.Status)
	assert.False(t, op.EndTime.IsZero())
}
