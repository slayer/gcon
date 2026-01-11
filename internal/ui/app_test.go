package ui

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/stretchr/testify/assert"
)

// createTestApp creates an App instance for testing without GCP client
func createTestApp() *App {
	app := NewApp(nil, AppOptions{})
	app.width = 120
	app.height = 40
	return app
}

// simulateProjectSelection sets up app state as if a project was selected
func simulateProjectSelection(app *App) {
	app.selectedProject = &gcp.Project{
		ID:   "test-project",
		Name: "Test Project",
	}
	app.currentView = ViewInstances
}

func TestNewApp(t *testing.T) {
	app := createTestApp()

	assert.NotNil(t, app)
	assert.NotNil(t, app.sidebar)
	assert.Equal(t, ViewProjects, app.currentView)
	assert.Equal(t, FocusContent, app.focusedPanel)
}

func TestToggleFocus(t *testing.T) {
	app := createTestApp()
	simulateProjectSelection(app)

	// Initially focused on content
	assert.Equal(t, FocusContent, app.focusedPanel)
	assert.False(t, app.sidebar.IsFocused())

	// Toggle to sidebar
	app.toggleFocus()
	assert.Equal(t, FocusSidebar, app.focusedPanel)
	assert.True(t, app.sidebar.IsFocused())

	// Toggle back to content
	app.toggleFocus()
	assert.Equal(t, FocusContent, app.focusedPanel)
	assert.False(t, app.sidebar.IsFocused())
}

func TestSidebarActive(t *testing.T) {
	app := createTestApp()

	// No project selected - sidebar should not be active
	assert.False(t, app.sidebarActive())

	// Project selected but still on projects view
	app.selectedProject = &gcp.Project{ID: "test"}
	app.currentView = ViewProjects
	assert.False(t, app.sidebarActive())

	// Project selected and on instances view - sidebar should be active
	app.currentView = ViewInstances
	assert.True(t, app.sidebarActive())

	// Also active on other resource views
	app.currentView = ViewDisks
	assert.True(t, app.sidebarActive())

	app.currentView = ViewBuckets
	assert.True(t, app.sidebarActive())
}

func TestUpdateSidebarActiveView(t *testing.T) {
	app := createTestApp()
	simulateProjectSelection(app)

	tests := []struct {
		viewType        ViewType
		expectedSidebar sidebar.ViewType
	}{
		{ViewInstances, sidebar.ViewInstances},
		{ViewInstanceDetails, sidebar.ViewInstances}, // Details maps to Instances
		{ViewDisks, sidebar.ViewDisks},
		{ViewBuckets, sidebar.ViewBuckets},
		{ViewNetworks, sidebar.ViewNetworks},
		{ViewFirewall, sidebar.ViewFirewall},
	}

	for _, tt := range tests {
		t.Run(tt.viewType.String(), func(t *testing.T) {
			app.currentView = tt.viewType
			app.updateSidebarActiveView()
			// We can't directly check sidebar.activeView as it's private,
			// but we can verify no panic occurs and the function completes
		})
	}
}

func TestUpdateViewSizes(t *testing.T) {
	app := createTestApp()
	app.width = 120
	app.height = 40

	// Without project selection (no sidebar)
	app.updateViewSizes()
	// projectView should get full width
	// No way to check view sizes directly, but verify no panic

	// With project selection (sidebar active)
	simulateProjectSelection(app)
	app.updateViewSizes()

	// Sidebar should have its size set
	// Content area should be reduced by sidebar width
}

func TestHandleSidebarNavigation(t *testing.T) {
	app := createTestApp()
	simulateProjectSelection(app)

	// Start with sidebar focused
	app.focusedPanel = FocusSidebar
	app.sidebar.SetFocused(true)

	tests := []struct {
		name         string
		navMsg       sidebar.NavigateMsg
		expectedView ViewType
	}{
		{
			name:         "navigate to disks",
			navMsg:       sidebar.NavigateMsg{ViewType: sidebar.ViewDisks, ItemID: "disks"},
			expectedView: ViewDisks,
		},
		{
			name:         "navigate to buckets",
			navMsg:       sidebar.NavigateMsg{ViewType: sidebar.ViewBuckets, ItemID: "buckets"},
			expectedView: ViewBuckets,
		},
		{
			name:         "navigate to networks",
			navMsg:       sidebar.NavigateMsg{ViewType: sidebar.ViewNetworks, ItemID: "networks"},
			expectedView: ViewNetworks,
		},
		{
			name:         "navigate to firewall",
			navMsg:       sidebar.NavigateMsg{ViewType: sidebar.ViewFirewall, ItemID: "firewall"},
			expectedView: ViewFirewall,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset focus to sidebar before each test
			app.focusedPanel = FocusSidebar
			app.sidebar.SetFocused(true)

			app.handleSidebarNavigation(tt.navMsg)

			assert.Equal(t, tt.expectedView, app.currentView)
			// Focus should switch back to content after navigation
			assert.Equal(t, FocusContent, app.focusedPanel)
			assert.False(t, app.sidebar.IsFocused())
		})
	}
}

func TestHandleSidebarNavigationToInstances(t *testing.T) {
	app := createTestApp()
	simulateProjectSelection(app)

	// Navigate away from instances first
	app.currentView = ViewDisks
	app.focusedPanel = FocusSidebar
	app.sidebar.SetFocused(true)

	// Navigate back to instances
	cmd := app.handleSidebarNavigation(sidebar.NavigateMsg{
		ViewType: sidebar.ViewInstances,
		ItemID:   "vm-instances",
	})

	assert.Equal(t, ViewInstances, app.currentView)
	assert.Equal(t, FocusContent, app.focusedPanel)
	// When instancesView is nil, a new one is created and Init() is returned
	assert.NotNil(t, cmd)
}

func TestHandleSidebarNavigationNoChangeWhenAlreadyOnView(t *testing.T) {
	app := createTestApp()
	simulateProjectSelection(app)

	// Already on instances view
	app.currentView = ViewInstances

	// Try to navigate to instances again
	app.handleSidebarNavigation(sidebar.NavigateMsg{
		ViewType: sidebar.ViewInstances,
		ItemID:   "vm-instances",
	})

	// Should stay on instances
	assert.Equal(t, ViewInstances, app.currentView)
}

// ViewType.String helper for test names
func (v ViewType) String() string {
	switch v {
	case ViewProjects:
		return "ViewProjects"
	case ViewInstances:
		return "ViewInstances"
	case ViewInstanceDetails:
		return "ViewInstanceDetails"
	case ViewDisks:
		return "ViewDisks"
	case ViewBuckets:
		return "ViewBuckets"
	case ViewNetworks:
		return "ViewNetworks"
	case ViewFirewall:
		return "ViewFirewall"
	case ViewLogs:
		return "ViewLogs"
	default:
		return "Unknown"
	}
}
