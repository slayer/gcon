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
					Image:   "gcr.io/test-proj/backend:latest",
					Command: []string{"/bin/sh", "-c"},
					Args:    []string{"./start.sh"},
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
			VpcAccess: &run.GoogleCloudRunV2VpcAccess{
				Connector: "projects/test-proj/locations/us-east1/connectors/my-connector",
				Egress:    "ALL_TRAFFIC",
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
	assert.Equal(t, "INGRESS_TRAFFIC_ALL", details.IngressRaw)
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

	// Container entrypoint and args
	assert.Equal(t, []string{"/bin/sh", "-c"}, details.Command)
	assert.Equal(t, []string{"./start.sh"}, details.Args)

	// VPC access
	assert.Equal(t, "projects/test-proj/locations/us-east1/connectors/my-connector", details.VPCConnector)
	assert.Equal(t, "All traffic", details.VPCEgress)

	// Env vars: regular + secret ref
	assert.Equal(t, "8080", details.EnvVars["PORT"])
	assert.Equal(t, "(secret ref)", details.EnvVars["DB_PASS"])

	// Labels
	assert.Equal(t, "prod", details.Labels["env"])

	// Traffic: one entry for LATEST
	assert.Len(t, details.Traffic, 1)
	assert.Equal(t, "LATEST", details.Traffic[0].Type)
	assert.Equal(t, int64(100), details.Traffic[0].Percent)

	// RawYAML should be non-empty
	assert.NotEmpty(t, details.RawYAML)
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

func TestFormatVPCEgress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ALL_TRAFFIC", "All traffic"},
		{"PRIVATE_RANGES_ONLY", "Private ranges only"},
		{"UNKNOWN_VALUE", "UNKNOWN_VALUE"},
		{"", ""},
	}

	for _, tt := range tests {
		name := tt.input
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatVPCEgress(tt.input))
		})
	}
}

func TestBuildServicePatch(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	int64Ptr := func(i int64) *int64 { return &i }

	t.Run("empty update produces no mask", func(t *testing.T) {
		update := &CloudRunServiceUpdate{}
		_, maskPaths := buildServicePatch(update)
		assert.Empty(t, maskPaths)
	})

	t.Run("description only", func(t *testing.T) {
		update := &CloudRunServiceUpdate{Description: strPtr("new desc")}
		svc, maskPaths := buildServicePatch(update)
		assert.Contains(t, maskPaths, "description")
		assert.Equal(t, "new desc", svc.Description)
	})

	t.Run("ingress and labels", func(t *testing.T) {
		update := &CloudRunServiceUpdate{
			Ingress: strPtr("INGRESS_TRAFFIC_INTERNAL_ONLY"),
			Labels:  map[string]string{"env": "staging"},
		}
		svc, maskPaths := buildServicePatch(update)
		assert.Contains(t, maskPaths, "ingress")
		assert.Contains(t, maskPaths, "labels")
		assert.Equal(t, "INGRESS_TRAFFIC_INTERNAL_ONLY", svc.Ingress)
		assert.Equal(t, "staging", svc.Labels["env"])
	})

	t.Run("container fields share template.containers mask", func(t *testing.T) {
		update := &CloudRunServiceUpdate{
			Image:  strPtr("gcr.io/p/img:v2"),
			Port:   int64Ptr(9090),
			CPU:    strPtr("4"),
			Memory: strPtr("2Gi"),
		}
		svc, maskPaths := buildServicePatch(update)
		assert.Contains(t, maskPaths, "template.containers")
		assert.NotNil(t, svc.Template)
		assert.Len(t, svc.Template.Containers, 1)
		c := svc.Template.Containers[0]
		assert.Equal(t, "gcr.io/p/img:v2", c.Image)
		assert.Equal(t, int64(9090), c.Ports[0].ContainerPort)
		assert.Equal(t, "4", c.Resources.Limits["cpu"])
		assert.Equal(t, "2Gi", c.Resources.Limits["memory"])
	})

	t.Run("command empty slice clears entrypoint", func(t *testing.T) {
		update := &CloudRunServiceUpdate{
			Image:   strPtr("img:latest"),
			Command: []string{}, // explicitly clear
		}
		svc, _ := buildServicePatch(update)
		c := svc.Template.Containers[0]
		assert.NotNil(t, c.Command)
		assert.Empty(t, c.Command)
		assert.Contains(t, c.ForceSendFields, "Command")
	})

	t.Run("env vars", func(t *testing.T) {
		update := &CloudRunServiceUpdate{
			Image:   strPtr("img:latest"),
			EnvVars: map[string]string{"KEY": "val"},
		}
		svc, _ := buildServicePatch(update)
		c := svc.Template.Containers[0]
		assert.Len(t, c.Env, 1)
		assert.Equal(t, "KEY", c.Env[0].Name)
		assert.Equal(t, "val", c.Env[0].Value)
	})

	t.Run("scaling fields", func(t *testing.T) {
		update := &CloudRunServiceUpdate{
			MinInstances: int64Ptr(2),
			MaxInstances: int64Ptr(50),
		}
		svc, maskPaths := buildServicePatch(update)
		assert.Contains(t, maskPaths, "template.scaling.minInstanceCount")
		assert.Contains(t, maskPaths, "template.scaling.maxInstanceCount")
		assert.Equal(t, int64(2), svc.Template.Scaling.MinInstanceCount)
		assert.Equal(t, int64(50), svc.Template.Scaling.MaxInstanceCount)
	})

	t.Run("concurrency and timeout", func(t *testing.T) {
		update := &CloudRunServiceUpdate{
			Concurrency: int64Ptr(100),
			Timeout:     int64Ptr(600),
		}
		svc, maskPaths := buildServicePatch(update)
		assert.Contains(t, maskPaths, "template.maxInstanceRequestConcurrency")
		assert.Contains(t, maskPaths, "template.timeout")
		assert.Equal(t, int64(100), svc.Template.MaxInstanceRequestConcurrency)
		assert.Equal(t, "600s", svc.Template.Timeout)
	})

	t.Run("service account", func(t *testing.T) {
		update := &CloudRunServiceUpdate{
			ServiceAccount: strPtr("sa@proj.iam.gserviceaccount.com"),
		}
		svc, maskPaths := buildServicePatch(update)
		assert.Contains(t, maskPaths, "template.serviceAccount")
		assert.Equal(t, "sa@proj.iam.gserviceaccount.com", svc.Template.ServiceAccount)
	})

	t.Run("vpc access", func(t *testing.T) {
		update := &CloudRunServiceUpdate{
			VPCConnector: strPtr("projects/p/locations/r/connectors/c"),
			VPCEgress:    strPtr("ALL_TRAFFIC"),
		}
		svc, maskPaths := buildServicePatch(update)
		assert.Contains(t, maskPaths, "template.vpcAccess")
		assert.Equal(t, "projects/p/locations/r/connectors/c", svc.Template.VpcAccess.Connector)
		assert.Equal(t, "ALL_TRAFFIC", svc.Template.VpcAccess.Egress)
	})

	t.Run("all fields combined produce correct mask count", func(t *testing.T) {
		update := &CloudRunServiceUpdate{
			Description:    strPtr("desc"),
			Ingress:        strPtr("INGRESS_TRAFFIC_ALL"),
			Labels:         map[string]string{"k": "v"},
			Image:          strPtr("img"),
			MinInstances:   int64Ptr(1),
			MaxInstances:   int64Ptr(10),
			Concurrency:    int64Ptr(80),
			Timeout:        int64Ptr(300),
			ServiceAccount: strPtr("sa@p.iam.gserviceaccount.com"),
			VPCConnector:   strPtr("conn"),
		}
		_, maskPaths := buildServicePatch(update)
		// description, ingress, labels, template.containers,
		// template.scaling.min, template.scaling.max,
		// template.maxInstanceRequestConcurrency, template.timeout,
		// template.serviceAccount, template.vpcAccess
		assert.Len(t, maskPaths, 10)
	})
}

func TestBuildServiceFromConfig(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	int64Ptr := func(i int64) *int64 { return &i }

	config := &CloudRunServiceUpdate{
		Description:    strPtr("My service"),
		Ingress:        strPtr("INGRESS_TRAFFIC_ALL"),
		Labels:         map[string]string{"env": "prod"},
		Image:          strPtr("gcr.io/proj/app:v1"),
		Port:           int64Ptr(8080),
		CPU:            strPtr("1"),
		Memory:         strPtr("512Mi"),
		MinInstances:   int64Ptr(0),
		MaxInstances:   int64Ptr(100),
		Concurrency:    int64Ptr(80),
		Timeout:        int64Ptr(300),
		ServiceAccount: strPtr("sa@proj.iam.gserviceaccount.com"),
	}

	svc := buildServiceFromConfig(config)

	assert.Equal(t, "My service", svc.Description)
	assert.Equal(t, "INGRESS_TRAFFIC_ALL", svc.Ingress)
	assert.Equal(t, "prod", svc.Labels["env"])
	assert.NotNil(t, svc.Template)
	assert.Len(t, svc.Template.Containers, 1)
	assert.Equal(t, "gcr.io/proj/app:v1", svc.Template.Containers[0].Image)
	assert.Equal(t, int64(8080), svc.Template.Containers[0].Ports[0].ContainerPort)
	assert.Equal(t, "1", svc.Template.Containers[0].Resources.Limits["cpu"])
	assert.Equal(t, "512Mi", svc.Template.Containers[0].Resources.Limits["memory"])
	assert.Equal(t, int64(0), svc.Template.Scaling.MinInstanceCount)
	assert.Equal(t, int64(100), svc.Template.Scaling.MaxInstanceCount)
	assert.Equal(t, int64(80), svc.Template.MaxInstanceRequestConcurrency)
	assert.Equal(t, "300s", svc.Template.Timeout)
	assert.Equal(t, "sa@proj.iam.gserviceaccount.com", svc.Template.ServiceAccount)
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
