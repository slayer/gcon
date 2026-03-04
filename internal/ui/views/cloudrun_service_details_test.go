package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestCloudRunServiceDetailsView_Init(t *testing.T) {
	view := NewCloudRunServiceDetailsView(
		"test-project",
		"web-api",
		"projects/test-project/locations/us-central1/services/web-api",
		nil, // nil client is OK, Init creates commands that check for nil
		nil, // gcpClient
	)

	assert.True(t, view.detailsLoading)
	assert.True(t, view.revisionsLoading)
	assert.Equal(t, "web-api", view.serviceName)
	assert.Equal(t, "projects/test-project/locations/us-central1/services/web-api", view.fullName)

	cmd := view.Init()
	assert.NotNil(t, cmd)
}

func TestCloudRunServiceDetailsView_TabSwitching(t *testing.T) {
	view := NewCloudRunServiceDetailsView(
		"test-project",
		"web-api",
		"projects/test-project/locations/us-central1/services/web-api",
		nil,
		nil,
	)

	// Verify tabs are set up correctly
	assert.Equal(t, runTabIDDetails, view.tabs.ActiveTab().ID)

	// Tab IDs should correspond to expected values (4 tabs: Details, Revisions, YAML, Observability)
	assert.Equal(t, 4, len(view.tabViewports))
}

func TestCloudRunServiceDetailsView_HasTextInputFocused(t *testing.T) {
	view := NewCloudRunServiceDetailsView(
		"test-project",
		"web-api",
		"projects/test-project/locations/us-central1/services/web-api",
		nil,
		nil,
	)

	// No dialogs shown
	assert.False(t, view.HasTextInputFocused())

	// Traffic dialog active
	view.showTrafficDialog = true
	view.trafficDialog = newTrafficSplitDialog(
		[]gcp.CloudRunRevision{{ShortName: "rev-1"}},
		nil,
	)
	assert.True(t, view.HasTextInputFocused())

	// Reset
	view.showTrafficDialog = false
	view.trafficDialog = nil

	// Delete confirm active
	view.showDeleteConfirm = true
	// Without an actual dialog instance, it returns false
	assert.False(t, view.HasTextInputFocused())
}

func TestCloudRunServiceDetailsView_IsMenuOpen(t *testing.T) {
	view := NewCloudRunServiceDetailsView(
		"test-project",
		"web-api",
		"projects/test-project/locations/us-central1/services/web-api",
		nil,
		nil,
	)

	assert.False(t, view.IsMenuOpen())

	view.menuOpen = true
	assert.True(t, view.IsMenuOpen())

	view.menuOpen = false
	view.showDeleteConfirm = true
	assert.True(t, view.IsMenuOpen())

	view.showDeleteConfirm = false
	view.showTrafficDialog = true
	assert.True(t, view.IsMenuOpen())
}

func TestCloudRunServiceDetailsView_GetServiceName(t *testing.T) {
	view := NewCloudRunServiceDetailsView(
		"test-project",
		"my-service",
		"projects/test-project/locations/us-east1/services/my-service",
		nil,
		nil,
	)

	assert.Equal(t, "my-service", view.GetServiceName())
}

func TestCloudRunServiceDetailsView_SetError(t *testing.T) {
	view := NewCloudRunServiceDetailsView(
		"test-project",
		"web-api",
		"projects/test-project/locations/us-central1/services/web-api",
		nil,
		nil,
	)

	assert.Nil(t, view.detailsErr)

	testErr := assert.AnError
	view.SetError(testErr)
	assert.Equal(t, testErr, view.detailsErr)
}

func TestTrafficSplitDialog_Validate(t *testing.T) {
	revisions := []gcp.CloudRunRevision{
		{ShortName: "rev-1"},
		{ShortName: "rev-2"},
	}

	dialog := newTrafficSplitDialog(revisions, nil)

	// Default is all zeros — should fail validation (not 100)
	err := dialog.validate()
	assert.Contains(t, err, "sum to 100")

	// Set to valid split
	dialog.inputs[0].SetValue("70")
	dialog.inputs[1].SetValue("30")
	err = dialog.validate()
	assert.Empty(t, err)

	// Invalid number
	dialog.inputs[0].SetValue("abc")
	err = dialog.validate()
	assert.Contains(t, err, "Invalid percentage")

	// Out of range
	dialog.inputs[0].SetValue("150")
	dialog.inputs[1].SetValue("0")
	err = dialog.validate()
	assert.Contains(t, err, "0-100")
}

func TestTrafficSplitDialog_Result(t *testing.T) {
	revisions := []gcp.CloudRunRevision{
		{ShortName: "rev-1"},
		{ShortName: "rev-2"},
	}

	dialog := newTrafficSplitDialog(revisions, nil)
	dialog.inputs[0].SetValue("80")
	dialog.inputs[1].SetValue("20")
	dialog.submitted = true

	targets := dialog.Result()
	assert.Len(t, targets, 2)
	assert.Equal(t, "rev-1", targets[0].RevisionName)
	assert.Equal(t, int64(80), targets[0].Percent)
	assert.Equal(t, "REVISION", targets[0].Type)
	assert.Equal(t, "rev-2", targets[1].RevisionName)
	assert.Equal(t, int64(20), targets[1].Percent)
	assert.Equal(t, "REVISION", targets[1].Type)
}

func TestTrafficSplitDialog_Result_SkipsZero(t *testing.T) {
	revisions := []gcp.CloudRunRevision{
		{ShortName: "rev-1"},
		{ShortName: "rev-2"},
		{ShortName: "rev-3"},
	}

	dialog := newTrafficSplitDialog(revisions, nil)
	dialog.inputs[0].SetValue("100")
	dialog.inputs[1].SetValue("0")
	dialog.inputs[2].SetValue("0")
	dialog.submitted = true

	targets := dialog.Result()
	assert.Len(t, targets, 1)
	assert.Equal(t, "rev-1", targets[0].RevisionName)
	assert.Equal(t, int64(100), targets[0].Percent)
}

func TestTrafficSplitDialog_PreservesLatestType(t *testing.T) {
	revisions := []gcp.CloudRunRevision{
		{ShortName: "rev-2"},
		{ShortName: "rev-1"},
	}
	currentTraffic := []gcp.CloudRunTrafficTarget{
		{RevisionName: "(latest)", Percent: 100, Type: "LATEST"},
	}

	dialog := newTrafficSplitDialog(revisions, currentTraffic)

	// First entry should be "(latest)" with 100%
	assert.Equal(t, 3, len(dialog.inputs)) // 1 LATEST + 2 revisions
	assert.Equal(t, "100", dialog.inputs[0].Value())

	// Edit: split 60/40 between latest and rev-1
	dialog.inputs[0].SetValue("60")
	dialog.inputs[1].SetValue("0")
	dialog.inputs[2].SetValue("40")
	dialog.submitted = true

	targets := dialog.Result()
	assert.Len(t, targets, 2)
	// LATEST type preserved
	assert.Equal(t, "LATEST", targets[0].Type)
	assert.Equal(t, int64(60), targets[0].Percent)
	assert.Empty(t, targets[0].RevisionName) // LATEST doesn't need revision name
	// REVISION type for concrete revision
	assert.Equal(t, "REVISION", targets[1].Type)
	assert.Equal(t, "rev-1", targets[1].RevisionName)
	assert.Equal(t, int64(40), targets[1].Percent)
}

func TestTrafficSplitDialog_PreservesTag(t *testing.T) {
	revisions := []gcp.CloudRunRevision{
		{ShortName: "rev-1"},
	}
	currentTraffic := []gcp.CloudRunTrafficTarget{
		{RevisionName: "rev-1", Percent: 100, Type: "REVISION", Tag: "blue"},
	}

	dialog := newTrafficSplitDialog(revisions, currentTraffic)
	dialog.submitted = true

	targets := dialog.Result()
	assert.Len(t, targets, 1)
	assert.Equal(t, "blue", targets[0].Tag)
	assert.Equal(t, "REVISION", targets[0].Type)
}

func TestTrafficSplitDialog_ResultNilWhenCanceled(t *testing.T) {
	revisions := []gcp.CloudRunRevision{
		{ShortName: "rev-1"},
	}

	dialog := newTrafficSplitDialog(revisions, nil)
	dialog.canceled = true

	assert.Nil(t, dialog.Result())
}
