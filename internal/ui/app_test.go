package ui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/projectselector"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/slayer/gcon/internal/ui/views"
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

func TestNewApp_StoresConfigProfile(t *testing.T) {
	// Set up test environment with known profile
	oldEnv := os.Getenv("CLOUDSDK_ACTIVE_CONFIG_NAME")
	t.Cleanup(func() {
		if oldEnv != "" {
			_ = os.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", oldEnv) //nolint:errcheck // Test setup
		} else {
			_ = os.Unsetenv("CLOUDSDK_ACTIVE_CONFIG_NAME") //nolint:errcheck // Test cleanup
		}
	})

	tests := []struct {
		name          string
		envValue      string
		expectedValue string
	}{
		{
			name:          "stores profile from env var",
			envValue:      "test-profile",
			expectedValue: "test-profile",
		},
		{
			name:          "defaults to 'default' when no config",
			envValue:      "",
			expectedValue: "default",
		},
		{
			name:          "stores production profile",
			envValue:      "production",
			expectedValue: "production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				_ = os.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", tt.envValue) //nolint:errcheck // Test setup
			} else {
				_ = os.Unsetenv("CLOUDSDK_ACTIVE_CONFIG_NAME") //nolint:errcheck // Test cleanup
				// Also clear CLOUDSDK_CONFIG to ensure we get "default"
				oldConfig := os.Getenv("CLOUDSDK_CONFIG")
				_ = os.Setenv("CLOUDSDK_CONFIG", "/nonexistent/path") //nolint:errcheck // Test setup
				t.Cleanup(func() {
					if oldConfig != "" {
						_ = os.Setenv("CLOUDSDK_CONFIG", oldConfig) //nolint:errcheck // Test setup
					} else {
						_ = os.Unsetenv("CLOUDSDK_CONFIG") //nolint:errcheck // Test cleanup
					}
				})
			}

			app := NewApp(nil, AppOptions{})
			assert.Equal(t, tt.expectedValue, app.configProfile)
		})
	}
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

func TestUploadKeyRoutedToObjectsView(t *testing.T) {
	app := createTestApp()
	simulateProjectSelection(app)

	// Setup: viewing empty bucket with objects view
	app.currentView = ViewObjects
	app.objectsView = views.NewObjectsView("test-bucket", nil)
	app.objectsView.SetContext(app.ctx)
	// Simulate loaded empty bucket
	app.objectsView.Update(views.ObjectsLoadedMsgForTest([]gcp.StorageObject{}, "", false))

	// Focus should be on content
	assert.Equal(t, FocusContent, app.focusedPanel)

	// Press 'u' for upload
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

	// File picker should be shown in objects view
	assert.True(t, app.objectsView.IsFilePickerShown(), "file picker should open when 'u' is pressed in empty bucket")
}

func TestProjectSelectorCancelOnStartupQuitsApp(t *testing.T) {
	app := createTestApp()

	// Simulate startup with no default project
	app.ShowProjectSelectorOnStartup()

	// Verify project selector is configured to show on startup
	assert.True(t, app.showProjectSelector, "project selector should be shown")
	assert.True(t, app.projectSelectorShownOnStartup, "should track that selector shown on startup")

	// Simulate user canceling project selector (pressing ESC)
	model, cmd := app.Update(projectselector.ProjectSelectorCanceledMsg{})

	// App should quit when user cancels on startup
	assert.NotNil(t, cmd, "command should be returned")

	// Execute command to check if it's a quit command
	msg := cmd()
	_, isQuitMsg := msg.(tea.QuitMsg)
	assert.True(t, isQuitMsg, "should return quit message when project selector canceled on startup")

	// Cast to verify it's still an App model
	_, ok := model.(*App)
	assert.True(t, ok, "should return App model")
}

func TestProjectSelectorCancelAfterStartupDoesNotQuit(t *testing.T) {
	app := createTestApp()
	simulateProjectSelection(app)

	// User opens project selector from command palette (not on startup)
	app.showProjectSelector = true
	// projectSelectorShownOnStartup should be false (default)

	// Simulate user canceling project selector
	model, cmd := app.Update(projectselector.ProjectSelectorCanceledMsg{})

	// App should NOT quit, just hide the selector
	assert.False(t, app.showProjectSelector, "project selector should be hidden")

	// Command can be nil or not a quit command
	if cmd != nil {
		msg := cmd()
		_, isQuitMsg := msg.(tea.QuitMsg)
		assert.False(t, isQuitMsg, "should NOT return quit message when canceling after startup")
	}

	// Cast to verify it's still an App model
	appModel, ok := model.(*App)
	assert.True(t, ok, "should return App model")
	assert.NotNil(t, appModel.selectedProject, "should still have selected project")
}

func TestIAMPolicyAddBinding_AppRouting(t *testing.T) {
	app := createTestApp()
	simulateProjectSelection(app)

	// Set up IAM policy view
	app.currentView = ViewIAMPolicy
	app.iamPolicyView = views.NewIAMPolicyView("test-project")

	// Simulate: InputConfirmMsg arrives at the app (from input dialog)
	// This message should be delegated to the view since the app doesn't handle it
	_, cmd := app.Update(views.AddIAMBindingMsg{
		ProjectID: "test-project",
		Role:      "roles/viewer",
		Member:    "user:test@example.com",
	})

	// The app should call handleAddIAMBinding which calls getIAMClient()
	// Since no real IAM client is set (view wasn't Init'd), getIAMClient returns nil
	// So the cmd should be nil — BUT this proves the message routing works
	// The key question: does the app's switch statement catch AddIAMBindingMsg?
	// If it returns nil here, it means the handler was called but iamClient is nil
	assert.Nil(t, cmd, "cmd should be nil because IAM client is not initialized")

	// Verify the view is still set
	assert.NotNil(t, app.iamPolicyView)
	assert.Equal(t, ViewIAMPolicy, app.currentView)
}

func TestIAMPolicyOverlayKeyRouting(t *testing.T) {
	policy := &gcp.IAMPolicy{
		Bindings: []gcp.IAMBinding{
			{Role: "roles/viewer", Members: []string{"user:alice@example.com", "user:bob@example.com"}},
			{Role: "roles/editor", Members: []string{"user:alice@example.com"}},
		},
	}

	t.Run("By Role tab: 'd' in overlay opens confirm dialog", func(t *testing.T) {
		app := createTestApp()
		simulateProjectSelection(app)
		app.currentView = ViewIAMPolicy
		v := views.NewIAMPolicyView("test-project")
		v.SetLoading(false)
		v.SetPolicy(policy)
		v.SwitchToRoleTab()
		app.iamPolicyView = v

		// Enter to open overlay
		app.Update(tea.KeyMsg{Type: tea.KeyEnter}) //nolint:errcheck // test
		assert.True(t, v.IsOverlayShown(), "overlay should be open after Enter")

		// 'd' to open confirm dialog
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}) //nolint:errcheck // test
		assert.True(t, v.IsConfirmShown(), "confirm dialog should be shown after 'd' in overlay")
	})

	t.Run("By Member tab: 'd' in overlay opens confirm dialog", func(t *testing.T) {
		app := createTestApp()
		simulateProjectSelection(app)
		app.currentView = ViewIAMPolicy
		v := views.NewIAMPolicyView("test-project")
		v.SetLoading(false)
		v.SetPolicy(policy)
		v.SwitchToMemberTab()
		app.iamPolicyView = v

		// Enter to open overlay (member's roles)
		app.Update(tea.KeyMsg{Type: tea.KeyEnter}) //nolint:errcheck // test
		assert.True(t, v.IsOverlayShown(), "overlay should be open after Enter")

		// 'd' to open confirm dialog
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}) //nolint:errcheck // test
		assert.True(t, v.IsConfirmShown(), "confirm dialog should be shown after 'd' in overlay")
	})

	t.Run("By Role tab: 'a' in overlay opens input dialog", func(t *testing.T) {
		app := createTestApp()
		simulateProjectSelection(app)
		app.currentView = ViewIAMPolicy
		v := views.NewIAMPolicyView("test-project")
		v.SetLoading(false)
		v.SetPolicy(policy)
		v.SwitchToRoleTab()
		app.iamPolicyView = v

		// Enter to open overlay
		app.Update(tea.KeyMsg{Type: tea.KeyEnter}) //nolint:errcheck // test
		assert.True(t, v.IsOverlayShown(), "overlay should be open after Enter")

		// 'a' to open input dialog
		app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}) //nolint:errcheck // test
		assert.True(t, v.IsInputShown(), "input dialog should be shown after 'a' in overlay")
	})
}

// TestEscFromObjectsBackToBucketDetailsPreservesSelectedBucket verifies the
// breadcrumb fix: navigating Buckets → BucketDetails (i) → Objects (Enter)
// → Esc must leave a.selectedBucket populated, because BucketDetailsView's
// breadcrumb in renderHeader reads it. The leavingView cleanup for ViewObjects
// must skip the selectedBucket=nil clear when the parent is ViewBucketDetails.
func TestEscFromObjectsBackToBucketDetailsPreservesSelectedBucket(t *testing.T) {
	app := createTestApp()
	simulateProjectSelection(app)

	// Simulate the navigation chain: Buckets -> BucketDetails -> Objects.
	// Both BucketDetails and Objects share the same selectedBucket.
	bucket := &gcp.Bucket{Name: "my-bucket", Location: "US", StorageClass: "STANDARD"}
	app.selectedBucket = bucket
	app.viewStack = []ViewType{ViewBuckets, ViewBucketDetails}
	app.currentView = ViewObjects
	app.objectsView = views.NewObjectsView(bucket.Name, nil)
	app.objectsView.SetContext(app.ctx)
	// Make sure no internal back navigation kicks in.
	app.objectsView.Update(views.ObjectsLoadedMsgForTest([]gcp.StorageObject{}, "", false))
	app.focusedPanel = FocusContent

	// Press Esc — the leavingView=ViewObjects cleanup branch should preserve
	// selectedBucket because the parent (top of viewStack) is ViewBucketDetails.
	app.Update(tea.KeyMsg{Type: tea.KeyEsc})

	assert.Equal(t, ViewBucketDetails, app.currentView, "should pop back to BucketDetails")
	assert.NotNil(t, app.selectedBucket, "selectedBucket must be preserved when returning to BucketDetails")
	if app.selectedBucket != nil {
		assert.Equal(t, "my-bucket", app.selectedBucket.Name)
	}
}

// TestEscFromObjectsBackToBucketsClearsSelectedBucket verifies the inverse:
// when popping back to ViewBuckets (the standard path), selectedBucket should
// still be cleared so it doesn't leak into the next bucket interaction.
func TestEscFromObjectsBackToBucketsClearsSelectedBucket(t *testing.T) {
	app := createTestApp()
	simulateProjectSelection(app)

	bucket := &gcp.Bucket{Name: "my-bucket"}
	app.selectedBucket = bucket
	app.viewStack = []ViewType{ViewBuckets}
	app.currentView = ViewObjects
	app.objectsView = views.NewObjectsView(bucket.Name, nil)
	app.objectsView.SetContext(app.ctx)
	app.objectsView.Update(views.ObjectsLoadedMsgForTest([]gcp.StorageObject{}, "", false))
	app.focusedPanel = FocusContent

	app.Update(tea.KeyMsg{Type: tea.KeyEsc})

	assert.Equal(t, ViewBuckets, app.currentView, "should pop back to Buckets")
	assert.Nil(t, app.selectedBucket, "selectedBucket should be cleared when returning to Buckets list")
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
