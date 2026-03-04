package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func TestFormatEnvVarsForEdit(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected string
	}{
		{
			name:     "empty",
			input:    nil,
			expected: "",
		},
		{
			name:     "single plain var",
			input:    map[string]string{"PORT": "8080"},
			expected: "PORT=8080",
		},
		{
			name:     "secret refs excluded",
			input:    map[string]string{"PORT": "8080", "DB_PASS": "(secret ref)"},
			expected: "PORT=8080",
		},
		{
			name:     "sorted by key",
			input:    map[string]string{"ZZZ": "last", "AAA": "first"},
			expected: "AAA=first\nZZZ=last",
		},
		{
			name:     "only secret refs yields empty",
			input:    map[string]string{"SECRET1": "(secret ref)", "SECRET2": "(secret ref)"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatEnvVarsForEdit(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractRegionFromFullName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard service name",
			input:    "projects/my-proj/locations/us-central1/services/my-svc",
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
			name:     "empty",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractRegionFromFullName(tt.input))
		})
	}
}

func TestCloudRunEditView_BuildUpdate(t *testing.T) {
	v := NewCloudRunEditView("my-project", "my-svc", "projects/my-project/locations/us-central1/services/my-svc", nil, false)
	v.buildForm()
	v.form.SetData(map[string]any{
		"description":    "Updated service",
		"image":          "gcr.io/proj/app:v2",
		"port":           int64(9090),
		"command":        "/bin/sh -c",
		"args":           "./run.sh --debug",
		"env_vars":       "PORT=9090\nMODE=prod",
		"cpu":            "2",
		"memory":         "1Gi",
		"timeout":        int64(600),
		"min_instances":   int64(1),
		"max_instances":   int64(50),
		"concurrency":    int64(100),
		"ingress":        "INGRESS_TRAFFIC_INTERNAL_ONLY",
		"vpc_connector":  "projects/p/locations/r/connectors/c",
		"vpc_egress":     "ALL_TRAFFIC",
		"service_account": "sa@proj.iam.gserviceaccount.com",
	})

	update := v.buildUpdate()

	assert.Equal(t, "Updated service", *update.Description)
	assert.Equal(t, "gcr.io/proj/app:v2", *update.Image)
	assert.Equal(t, int64(9090), *update.Port)
	assert.Equal(t, []string{"/bin/sh", "-c"}, update.Command)
	assert.Equal(t, []string{"./run.sh", "--debug"}, update.Args)
	assert.Equal(t, "2", *update.CPU)
	assert.Equal(t, "1Gi", *update.Memory)
	assert.Equal(t, int64(600), *update.Timeout)
	assert.Equal(t, int64(1), *update.MinInstances)
	assert.Equal(t, int64(50), *update.MaxInstances)
	assert.Equal(t, int64(100), *update.Concurrency)
	assert.Equal(t, "INGRESS_TRAFFIC_INTERNAL_ONLY", *update.Ingress)
	assert.Equal(t, "projects/p/locations/r/connectors/c", *update.VPCConnector)
	assert.Equal(t, "ALL_TRAFFIC", *update.VPCEgress)
	assert.Equal(t, "sa@proj.iam.gserviceaccount.com", *update.ServiceAccount)
	assert.Equal(t, "9090", update.EnvVars["PORT"])
	assert.Equal(t, "prod", update.EnvVars["MODE"])
}

func TestCloudRunEditView_BuildUpdateEmptyCommand(t *testing.T) {
	v := NewCloudRunEditView("proj", "svc", "full/name", nil, false)
	v.buildForm()
	v.form.SetData(map[string]any{
		"description":    "",
		"image":          "img:latest",
		"port":           int64(8080),
		"command":        "",
		"args":           "",
		"env_vars":       "",
		"cpu":            "1",
		"memory":         "512Mi",
		"timeout":        int64(300),
		"min_instances":   int64(0),
		"max_instances":   int64(100),
		"concurrency":    int64(80),
		"ingress":        "INGRESS_TRAFFIC_ALL",
		"vpc_connector":  "",
		"vpc_egress":     "",
		"service_account": "",
	})

	update := v.buildUpdate()

	// Empty command/args should produce empty slices (clear entrypoint)
	assert.Empty(t, update.Command)
	assert.Empty(t, update.Args)
	// Empty env_vars should produce empty map
	assert.Empty(t, update.EnvVars)
	// Empty vpc_connector should be nil (no change) to avoid conflicting with NetworkInterfaces
	assert.Nil(t, update.VPCConnector)
}

func TestCloudRunEditView_PopulateFormFromOriginal(t *testing.T) {
	v := NewCloudRunEditView("proj", "my-svc", "projects/p/locations/us-central1/services/my-svc", nil, false)
	v.original = &gcp.CloudRunServiceDetails{
		Name:           "my-svc",
		Description:    "Test service",
		ContainerImage: "gcr.io/proj/img:v1",
		ContainerPort:  8080,
		Command:        []string{"/bin/sh", "-c"},
		Args:           []string{"./start.sh"},
		CPU:            "2",
		Memory:         "1Gi",
		TimeoutSeconds: 300,
		MinInstances:   1,
		MaxInstances:   10,
		Concurrency:    80,
		IngressRaw:     "INGRESS_TRAFFIC_ALL",
		VPCConnector:   "projects/p/locations/r/connectors/c",
		VPCEgress:      "All traffic",
		VPCEgressRaw:   "ALL_TRAFFIC",
		ServiceAccount: "sa@proj.iam.gserviceaccount.com",
		EnvVars: map[string]string{
			"PORT":    "8080",
			"DB_PASS": "(secret ref)",
		},
	}

	v.buildForm()
	v.populateFormFromOriginal()

	data := v.form.GetData()

	assert.Equal(t, "Test service", data["description"])
	assert.Equal(t, "gcr.io/proj/img:v1", data["image"])
	assert.Equal(t, "/bin/sh -c", data["command"])
	assert.Equal(t, "./start.sh", data["args"])
	assert.Equal(t, "2", data["cpu"])
	assert.Equal(t, "1Gi", data["memory"])
	assert.Equal(t, "INGRESS_TRAFFIC_ALL", data["ingress"])
	assert.Equal(t, "ALL_TRAFFIC", data["vpc_egress"])
	assert.Equal(t, "projects/p/locations/r/connectors/c", data["vpc_connector"])
	assert.Equal(t, "sa@proj.iam.gserviceaccount.com", data["service_account"])

	// Env vars should exclude secret refs
	envText, ok := data["env_vars"].(string)
	assert.True(t, ok)
	assert.Contains(t, envText, "PORT=8080")
	assert.NotContains(t, envText, "DB_PASS")
}

func TestCloudRunEditView_IsCreate(t *testing.T) {
	editView := NewCloudRunEditView("proj", "svc", "full/name", nil, false)
	assert.False(t, editView.IsCreate())

	createView := NewCloudRunEditView("proj", "", "", nil, true)
	assert.True(t, createView.IsCreate())
}

func TestCloudRunEditView_HasTextInputFocused(t *testing.T) {
	v := NewCloudRunEditView("proj", "", "", nil, true)
	// Before form init, should return false
	assert.False(t, v.HasTextInputFocused())

	// After initialization, the first text input should be focused
	v.Init()
	assert.True(t, v.HasTextInputFocused())
}

func TestCloudRunEditView_SetError(t *testing.T) {
	v := NewCloudRunEditView("proj", "svc", "full/name", nil, false)
	v.state = cloudRunEditStateSaving

	testErr := assert.AnError
	v.SetError(testErr)

	assert.Equal(t, cloudRunEditStateForm, v.state)
	assert.Equal(t, testErr, v.err)
}

func TestCloudRunEditView_ViewStates(t *testing.T) {
	t.Run("loading state", func(t *testing.T) {
		v := NewCloudRunEditView("proj", "svc", "full/name", nil, false)
		v.state = cloudRunEditStateLoading
		view := v.View()
		assert.Contains(t, view, "Loading service configuration")
	})

	t.Run("saving state edit", func(t *testing.T) {
		v := NewCloudRunEditView("proj", "svc", "full/name", nil, false)
		v.state = cloudRunEditStateSaving
		view := v.View()
		assert.Contains(t, view, "Updating Cloud Run service")
	})

	t.Run("saving state create", func(t *testing.T) {
		v := NewCloudRunEditView("proj", "", "", nil, true)
		v.state = cloudRunEditStateSaving
		view := v.View()
		assert.Contains(t, view, "Creating Cloud Run service")
	})

	t.Run("error state without form", func(t *testing.T) {
		v := NewCloudRunEditView("proj", "svc", "full/name", nil, false)
		v.state = cloudRunEditStateForm
		v.form = nil
		v.err = assert.AnError
		view := v.View()
		assert.Contains(t, view, "assert.AnError")
	})
}

func TestCloudRunRegions(t *testing.T) {
	regions := cloudRunRegions()
	assert.True(t, len(regions) > 10, "should have many regions")
	assert.Contains(t, regions, "us-central1")
	assert.Contains(t, regions, "europe-west1")
	assert.Contains(t, regions, "asia-east1")
}
