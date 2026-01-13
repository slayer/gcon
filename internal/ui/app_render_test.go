package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/views"
	"github.com/stretchr/testify/assert"
)

func TestApp_RenderingHeightConsistency(t *testing.T) {
	app := createTestApp()
	app.width = 120
	app.height = 40

	// Simulate project selection to activate sidebar
	simulateProjectSelection(app)

	// Create instances view (in loading state)
	app.instancesView = views.NewInstancesView("test-project")

	// Force layout size update
	app.layout.SetSize(app.width, app.height)
	app.updateViewSizes()

	t.Logf("App dimensions: width=%d, height=%d", app.width, app.height)
	t.Logf("Content height from layout: %d", app.layout.ContentHeight())

	// With teatile-based layout, height enforcement happens in View() method
	// using MaxHeight() to truncate content that exceeds the allocated space.
	// The full view should have exactly the terminal height.
	fullView := app.View()
	viewHeight := lipgloss.Height(fullView)
	t.Logf("Full app view height: %d", viewHeight)

	assert.Equal(t, app.height, viewHeight,
		"Full app view should have exactly app.height lines")
}

func TestApp_InstanceDetailsRenderingHeight(t *testing.T) {
	app := createTestApp()
	app.width = 120
	app.height = 40

	// Simulate project selection
	app.selectedProject = &gcp.Project{ID: "test-project", Name: "Test"}
	app.currentView = ViewInstanceDetails

	// Create instance details view (will be in loading state)
	app.instanceDetailsView = views.NewInstanceDetailsView("test-project", "us-central1-a", "test-vm", nil)
	app.layout.SetSize(app.width, app.height)
	app.updateViewSizes()

	// With teatile-based layout, height enforcement happens at View() level
	// using MaxHeight() to truncate any content overflow.
	fullView := app.View()
	viewHeight := lipgloss.Height(fullView)

	t.Logf("Full view height: %d", viewHeight)
	t.Logf("Expected height: %d", app.height)

	assert.Equal(t, app.height, viewHeight,
		"Full app view should have exactly app.height lines")
}

func TestApp_LayoutDimensionsMatch(t *testing.T) {
	app := createTestApp()
	app.width = 120
	app.height = 40

	// Simulate project selection to activate sidebar
	simulateProjectSelection(app)
	app.instancesView = views.NewInstancesView("test-project")

	// Update layout
	app.layout.SetSize(app.width, app.height)
	app.updateViewSizes()

	// Test that layout calculates correct dimensions
	_, headerH := app.layout.HeaderSize()
	contentH := app.layout.ContentHeight()
	_, footerH := app.layout.FooterSize()

	t.Logf("Header height: %d", headerH)
	t.Logf("Content height: %d", contentH)
	t.Logf("Footer height: %d", footerH)

	// Heights should sum to total height
	assert.Equal(t, app.height, headerH+contentH+footerH,
		"Header + Content + Footer heights should equal total height")

	// Sidebar and main should have same height
	_, sidebarH := app.layout.SidebarSize()
	_, mainH := app.layout.MainSize()
	assert.Equal(t, sidebarH, mainH,
		"Sidebar and main content tiles should have equal heights")
}

func TestApp_FullViewConsistentHeight(t *testing.T) {
	app := createTestApp()
	app.width = 120
	app.height = 40

	// Simulate project selection
	simulateProjectSelection(app)
	app.instancesView = views.NewInstancesView("test-project")

	// Update layout
	app.layout.SetSize(app.width, app.height)
	app.updateViewSizes()

	// Get full app view
	fullView := app.View()

	// Verify the view actually renders with proper height using lipgloss.Height()
	// lipgloss.Height() counts the number of lines in the string
	viewHeight := lipgloss.Height(fullView)
	t.Logf("View height (lipgloss.Height): %d", viewHeight)
	t.Logf("Expected height: %d", app.height)

	assert.Equal(t, app.height, viewHeight,
		"Full app view should have exactly app.height lines")
}
