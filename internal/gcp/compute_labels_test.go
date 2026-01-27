package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstanceLabelsFingerprint_EmptyLabels(t *testing.T) {
	lf := &InstanceLabelsFingerprint{
		Labels:      make(map[string]string),
		Fingerprint: "abc123",
	}

	assert.NotNil(t, lf.Labels)
	assert.Empty(t, lf.Labels)
	assert.Equal(t, "abc123", lf.Fingerprint)
}

func TestInstanceLabelsFingerprint_WithLabels(t *testing.T) {
	lf := &InstanceLabelsFingerprint{
		Labels: map[string]string{
			"env":   "prod",
			"team":  "backend",
			"owner": "alice",
		},
		Fingerprint: "xyz789",
	}

	assert.Len(t, lf.Labels, 3)
	assert.Equal(t, "prod", lf.Labels["env"])
	assert.Equal(t, "backend", lf.Labels["team"])
	assert.Equal(t, "alice", lf.Labels["owner"])
	assert.Equal(t, "xyz789", lf.Fingerprint)
}
