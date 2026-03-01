package gcp

import (
	"testing"

	run "google.golang.org/api/run/v2"

	"github.com/stretchr/testify/assert"
)

func TestExtractShortName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full service name",
			input:    "projects/my-project/locations/us-central1/services/my-svc",
			expected: "my-svc",
		},
		{
			name:     "full revision name",
			input:    "projects/my-project/locations/us-central1/services/my-svc/revisions/my-svc-00001-abc",
			expected: "my-svc-00001-abc",
		},
		{
			name:     "simple name",
			input:    "my-svc",
			expected: "my-svc",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractShortName(tt.input))
		})
	}
}

func TestExtractRegion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full service name",
			input:    "projects/my-project/locations/us-central1/services/my-svc",
			expected: "us-central1",
		},
		{
			name:     "europe region",
			input:    "projects/p/locations/europe-west1/services/svc",
			expected: "europe-west1",
		},
		{
			name:     "no locations segment",
			input:    "projects/p/services/svc",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractRegion(tt.input))
		})
	}
}

func TestDeriveStatus(t *testing.T) {
	tests := []struct {
		name     string
		svc      *run.GoogleCloudRunV2Service
		expected string
	}{
		{
			name: "ready service",
			svc: &run.GoogleCloudRunV2Service{
				TerminalCondition: &run.GoogleCloudRunV2Condition{
					Type:  "Ready",
					State: "CONDITION_SUCCEEDED",
				},
			},
			expected: "Ready",
		},
		{
			name: "failed service with reason",
			svc: &run.GoogleCloudRunV2Service{
				TerminalCondition: &run.GoogleCloudRunV2Condition{
					Type:   "Ready",
					State:  "CONDITION_FAILED",
					Reason: "CONTAINER_MISSING",
				},
			},
			expected: "CONTAINER_MISSING",
		},
		{
			name: "failed service without reason",
			svc: &run.GoogleCloudRunV2Service{
				TerminalCondition: &run.GoogleCloudRunV2Condition{
					Type:  "Ready",
					State: "CONDITION_FAILED",
				},
			},
			expected: "Failed",
		},
		{
			name: "reconciling service",
			svc: &run.GoogleCloudRunV2Service{
				Reconciling: true,
			},
			expected: "Deploying",
		},
		{
			name:     "unknown status",
			svc:      &run.GoogleCloudRunV2Service{},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, deriveStatus(tt.svc))
		})
	}
}

func TestCloudRunServiceFromAPI(t *testing.T) {
	svc := &run.GoogleCloudRunV2Service{
		Name:                "projects/test-proj/locations/us-central1/services/web-api",
		Uri:                 "https://web-api-abc123.run.app",
		LatestReadyRevision: "projects/test-proj/locations/us-central1/services/web-api/revisions/web-api-00005-xyz",
		UpdateTime:          "2025-01-15T10:30:00.123456Z",
		TerminalCondition: &run.GoogleCloudRunV2Condition{
			Type:  "Ready",
			State: "CONDITION_SUCCEEDED",
		},
	}

	result := cloudRunServiceFromAPI(svc)

	assert.Equal(t, "web-api", result.Name)
	assert.Equal(t, "projects/test-proj/locations/us-central1/services/web-api", result.FullName)
	assert.Equal(t, "us-central1", result.Region)
	assert.Equal(t, "https://web-api-abc123.run.app", result.URL)
	assert.Equal(t, "web-api-00005-xyz", result.LatestRevision)
	assert.Equal(t, "Ready", result.Status)
	assert.Equal(t, "2025-01-15 10:30:00", result.UpdatedAt)
}

func TestCloudRunServiceDetailsFromAPI(t *testing.T) {
	svc := &run.GoogleCloudRunV2Service{
		Name:                "projects/test-proj/locations/us-east1/services/backend",
		Uri:                 "https://backend-abc.run.app",
		LatestReadyRevision: "projects/test-proj/locations/us-east1/services/backend/revisions/backend-00001-abc",
		Description:         "Backend service",
		Ingress:             "INGRESS_TRAFFIC_ALL",
		CreateTime:          "2025-01-01T00:00:00Z",
		UpdateTime:          "2025-01-15T12:00:00Z",
		Creator:             "user@example.com",
		LastModifier:        "admin@example.com",
		Labels:              map[string]string{"env": "prod"},
		TerminalCondition: &run.GoogleCloudRunV2Condition{
			Type:    "Ready",
			State:   "CONDITION_SUCCEEDED",
			Message: "Service is ready",
		},
		Template: &run.GoogleCloudRunV2RevisionTemplate{
			ServiceAccount:                "sa@test-proj.iam.gserviceaccount.com",
			MaxInstanceRequestConcurrency: 80,
			Timeout:                       "300s",
			Scaling: &run.GoogleCloudRunV2RevisionScaling{
				MinInstanceCount: 1,
				MaxInstanceCount: 10,
			},
			Containers: []*run.GoogleCloudRunV2Container{
				{
					Image: "gcr.io/test-proj/backend:latest",
					Ports: []*run.GoogleCloudRunV2ContainerPort{
						{ContainerPort: 8080},
					},
					Resources: &run.GoogleCloudRunV2ResourceRequirements{
						Limits: map[string]string{
							"cpu":    "2",
							"memory": "512Mi",
						},
					},
					Env: []*run.GoogleCloudRunV2EnvVar{
						{Name: "PORT", Value: "8080"},
						{Name: "DB_PASS", ValueSource: &run.GoogleCloudRunV2EnvVarSource{}},
					},
				},
			},
		},
		Traffic: []*run.GoogleCloudRunV2TrafficTarget{
			{
				Type:    "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST",
				Percent: 100,
			},
		},
	}

	details := cloudRunServiceDetailsFromAPI(svc)

	assert.Equal(t, "backend", details.Name)
	assert.Equal(t, "us-east1", details.Region)
	assert.Equal(t, "Ready", details.Status)
	assert.Equal(t, "Service is ready", details.StatusMessage)
	assert.Equal(t, "Backend service", details.Description)
	assert.Equal(t, "All", details.Ingress)
	assert.Equal(t, "sa@test-proj.iam.gserviceaccount.com", details.ServiceAccount)
	assert.Equal(t, "gcr.io/test-proj/backend:latest", details.ContainerImage)
	assert.Equal(t, int64(8080), details.ContainerPort)
	assert.Equal(t, "2", details.CPU)
	assert.Equal(t, "512Mi", details.Memory)
	assert.Equal(t, int64(1), details.MinInstances)
	assert.Equal(t, int64(10), details.MaxInstances)
	assert.Equal(t, int64(80), details.Concurrency)
	assert.Equal(t, int64(300), details.TimeoutSeconds)
	assert.Equal(t, "user@example.com", details.Creator)
	assert.Equal(t, "admin@example.com", details.LastModifier)

	// Env vars: regular + secret ref
	assert.Equal(t, "8080", details.EnvVars["PORT"])
	assert.Equal(t, "(secret ref)", details.EnvVars["DB_PASS"])

	// Labels
	assert.Equal(t, "prod", details.Labels["env"])

	// Traffic: one entry for LATEST
	assert.Len(t, details.Traffic, 1)
	assert.Equal(t, "LATEST", details.Traffic[0].Type)
	assert.Equal(t, int64(100), details.Traffic[0].Percent)

	// RawJSON should be non-empty
	assert.NotEmpty(t, details.RawJSON)
}

func TestDeriveRevisionStatus(t *testing.T) {
	tests := []struct {
		name     string
		rev      *run.GoogleCloudRunV2Revision
		expected string
	}{
		{
			name: "ready revision",
			rev: &run.GoogleCloudRunV2Revision{
				Conditions: []*run.GoogleCloudRunV2Condition{
					{Type: "Ready", State: "CONDITION_SUCCEEDED"},
				},
			},
			expected: "Ready",
		},
		{
			name: "failed revision",
			rev: &run.GoogleCloudRunV2Revision{
				Conditions: []*run.GoogleCloudRunV2Condition{
					{Type: "Ready", State: "CONDITION_FAILED"},
				},
			},
			expected: "Failed",
		},
		{
			name: "pending revision",
			rev: &run.GoogleCloudRunV2Revision{
				Conditions: []*run.GoogleCloudRunV2Condition{
					{Type: "Active", State: "CONDITION_SUCCEEDED"},
				},
			},
			expected: "Pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, deriveRevisionStatus(tt.rev))
		})
	}
}

func TestFormatIngress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"INGRESS_TRAFFIC_ALL", "All"},
		{"INGRESS_TRAFFIC_INTERNAL_ONLY", "Internal only"},
		{"INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER", "Internal + Load Balancer"},
		{"INGRESS_TRAFFIC_NONE", "None"},
		{"UNKNOWN_VALUE", "UNKNOWN_VALUE"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatIngress(tt.input))
		})
	}
}

func TestFormatCloudRunTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "RFC3339 with nanos",
			input:    "2025-01-15T10:30:00.123456Z",
			expected: "2025-01-15 10:30:00",
		},
		{
			name:     "RFC3339 without nanos",
			input:    "2025-01-15T10:30:00Z",
			expected: "2025-01-15 10:30:00",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid format passes through",
			input:    "not-a-date",
			expected: "not-a-date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatCloudRunTime(tt.input))
		})
	}
}
