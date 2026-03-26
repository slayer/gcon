package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/views"
)

// Force TrueColor so ANSI output is deterministic across CI and local runs.
func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// setupAppWithSize creates a test app and sends WindowSizeMsg to trigger layout.
// Pins CLOUDSDK_ACTIVE_CONFIG_NAME so the config-profile badge is deterministic.
func setupAppWithSize(t *testing.T, width, height int) *App {
	t.Helper()
	t.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", "test-profile")
	app := createTestApp()
	model, _ := app.Update(tea.WindowSizeMsg{Width: width, Height: height})
	result, ok := model.(*App)
	if !ok {
		t.Fatal("Update returned unexpected model type")
	}
	return result
}

func TestGolden_App_InitialState(t *testing.T) {
	// App starts on the projects view in loading state (no data injected).
	// Tests the full layout: header + content + footer.
	app := setupAppWithSize(t, 120, 40)

	golden.RequireEqual(t, []byte(app.View()))
}

func TestGolden_App_WithSidebar_Loading(t *testing.T) {
	// After project selection, sidebar appears and instances view is in loading state.
	// Tests the full layout: header + sidebar + content + footer.
	app := setupAppWithSize(t, 120, 40)
	simulateProjectSelection(app)
	app.instancesView = views.NewInstancesView("test-project")
	app.layout.SetSize(app.width, app.height)
	app.updateViewSizes()

	golden.RequireEqual(t, []byte(app.View()))
}

func TestGolden_App_WithSidebar_InstancesLoaded(t *testing.T) {
	// Full layout with loaded instances data in the content area.
	app := setupAppWithSize(t, 120, 40)
	simulateProjectSelection(app)
	app.instancesView = views.NewInstancesView("test-project")
	app.layout.SetSize(app.width, app.height)
	app.updateViewSizes()

	// Inject loaded data via the exported test helper
	app.instancesView.Update(views.InstancesLoadedMsgForTest(testAppInstances()))

	golden.RequireEqual(t, []byte(app.View()))
}

func TestGolden_App_HelpVisible(t *testing.T) {
	// When help is toggled, the footer shows key bindings instead of status bar.
	app := setupAppWithSize(t, 120, 40)
	simulateProjectSelection(app)
	app.instancesView = views.NewInstancesView("test-project")
	app.layout.SetSize(app.width, app.height)
	app.updateViewSizes()

	app.showHelp = true

	golden.RequireEqual(t, []byte(app.View()))
}

func TestGolden_App_NarrowTerminal(t *testing.T) {
	// Verify layout degrades gracefully at narrow widths.
	app := setupAppWithSize(t, 80, 24)
	simulateProjectSelection(app)
	app.instancesView = views.NewInstancesView("test-project")
	app.layout.SetSize(app.width, app.height)
	app.updateViewSizes()

	golden.RequireEqual(t, []byte(app.View()))
}

// testAppInstances returns a fixed set of instances for app-level golden tests.
func testAppInstances() []gcp.Instance {
	return []gcp.Instance{
		{
			Name:        "web-server-1",
			Zone:        "us-central1-a",
			MachineType: "e2-medium",
			Status:      "RUNNING",
			InternalIP:  "10.128.0.2",
			ExternalIP:  "35.192.0.1",
			CreatedAt:   "2024-01-15T10:30:00.000-07:00",
		},
		{
			Name:        "db-server-1",
			Zone:        "us-central1-b",
			MachineType: "n2-standard-4",
			Status:      "RUNNING",
			InternalIP:  "10.128.0.3",
			ExternalIP:  "",
			CreatedAt:   "2024-02-20T14:00:00.000-07:00",
		},
		{
			Name:        "dev-instance",
			Zone:        "us-east1-b",
			MachineType: "e2-micro",
			Status:      "TERMINATED",
			InternalIP:  "10.142.0.5",
			ExternalIP:  "",
			CreatedAt:   "2024-03-10T08:15:00.000-07:00",
		},
	}
}
