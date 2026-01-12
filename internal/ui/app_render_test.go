package ui

import (
	"strings"
	"testing"

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

	// Force size update
	app.updateViewSizes()

	t.Logf("App dimensions: width=%d, height=%d", app.width, app.height)
	t.Logf("Content height (height-4): %d", app.height-4)

	// Check sidebar height
	sidebarView := app.sidebar.View()
	sidebarLines := strings.Count(sidebarView, "\n")
	t.Logf("Sidebar lines: %d", sidebarLines)

	// Check what renderCurrentView outputs
	contentView := app.renderCurrentView()
	contentLines := strings.Count(contentView, "\n")
	t.Logf("Content view lines: %d", contentLines)

	// Check the combined sidebar+content
	combinedView := app.renderWithSidebar()
	combinedLines := strings.Count(combinedView, "\n")
	t.Logf("Combined (sidebar+content) lines: %d", combinedLines)

	// Check full app view
	fullView := app.View()
	fullLines := strings.Count(fullView, "\n")
	t.Logf("Full app view lines: %d", fullLines)

	// Log the actual outputs for debugging
	t.Logf("\n=== Sidebar View ===\n%s", sidebarView)
	t.Logf("\n=== Content View ===\n%s", contentView)

	// Sidebar and content should have same newline count for proper horizontal join
	// (lipgloss Height(n) outputs n-1 newlines)
	assert.Equal(t, sidebarLines, contentLines,
		"Sidebar and content should have same newline count for proper horizontal join")
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
	app.updateViewSizes()

	// Check heights
	sidebarView := app.sidebar.View()
	sidebarLines := strings.Count(sidebarView, "\n")

	contentView := app.renderCurrentView()
	contentLines := strings.Count(contentView, "\n")

	t.Logf("Sidebar lines: %d", sidebarLines)
	t.Logf("Content (instance details loading) lines: %d", contentLines)

	assert.Equal(t, sidebarLines, contentLines,
		"Sidebar and instance details loading view should have same line count")
}
