package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestCloudRunServiceToRow(t *testing.T) {
	svc := gcp.CloudRunService{
		Name:           "web-api",
		FullName:       "projects/p/locations/us-central1/services/web-api",
		Region:         "us-central1",
		URL:            "https://web-api-abc.run.app",
		LatestRevision: "web-api-00005-xyz",
		Status:         "Ready",
		UpdatedAt:      "2025-01-15 10:30:00",
	}

	row := cloudRunServiceToRow(svc)

	assert.Equal(t, "web-api", row.Data[0])
	assert.Equal(t, "us-central1", row.Data[1])
	// Status column has colored symbol, just check it's not empty
	assert.NotEmpty(t, row.Data[2])
	assert.Equal(t, "https://web-api-abc.run.app", row.Data[3])
	assert.Equal(t, "web-api-00005-xyz", row.Data[4])
	assert.Equal(t, "2025-01-15 10:30:00", row.Data[5])
	assert.Equal(t, "projects/p/locations/us-central1/services/web-api", row.ID)
	assert.Contains(t, row.FilterValue, "web-api")
	assert.Contains(t, row.FilterValue, "us-central1")
}

func TestCloudRunServicesView_Init(t *testing.T) {
	view := NewCloudRunServicesView("test-project")

	assert.True(t, view.loading)
	assert.Nil(t, view.runClient)
	assert.Equal(t, "test-project", view.projectID)

	// Init should return a command
	cmd := view.Init()
	assert.NotNil(t, cmd)
}

func TestCloudRunServicesView_InitIdempotent(t *testing.T) {
	view := NewCloudRunServicesView("test-project")

	// First init
	view.Init()
	assert.True(t, view.loading)

	// Simulate data loaded
	view.loading = false
	view.services = []gcp.CloudRunService{
		{Name: "svc-1", FullName: "projects/p/locations/us/services/svc-1"},
	}

	// Second init should reset loading back to true
	assert.False(t, view.loading)
	cmd := view.Init()
	assert.True(t, view.loading)
	assert.NotNil(t, cmd)
}

func TestCloudRunServicesView_HasTextInputFocused(t *testing.T) {
	view := NewCloudRunServicesView("test-project")

	// No dialog shown
	assert.False(t, view.HasTextInputFocused())

	// Table filtering is not active by default
	assert.False(t, view.table.HasTextInputFocused())
}

func TestCloudRunServicesView_IsMenuOpen(t *testing.T) {
	view := NewCloudRunServicesView("test-project")

	assert.False(t, view.IsMenuOpen())

	view.menuOpen = true
	assert.True(t, view.IsMenuOpen())

	view.menuOpen = false
	view.showDeleteConfirm = true
	assert.True(t, view.IsMenuOpen())
}

func TestCloudRunServicesView_FindServiceByFullName(t *testing.T) {
	view := NewCloudRunServicesView("test-project")
	view.services = []gcp.CloudRunService{
		{Name: "svc-1", FullName: "projects/p/locations/us/services/svc-1"},
		{Name: "svc-2", FullName: "projects/p/locations/eu/services/svc-2"},
	}

	svc, ok := view.findServiceByFullName("projects/p/locations/us/services/svc-1")
	assert.True(t, ok)
	assert.Equal(t, "svc-1", svc.Name)

	_, ok = view.findServiceByFullName("projects/p/locations/us/services/nonexistent")
	assert.False(t, ok)
}

func TestCloudRunServicesView_GetCloudRunClient(t *testing.T) {
	view := NewCloudRunServicesView("test-project")

	// Client is nil before init
	assert.Nil(t, view.GetCloudRunClient())
}
