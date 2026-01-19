package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/stretchr/testify/assert"
)

func TestNewHeader(t *testing.T) {
	h := NewHeader()

	assert.NotNil(t, h)
	assert.Equal(t, 0, h.width)
	assert.Equal(t, "", h.projectID)
	assert.Equal(t, "", h.category)
	assert.Empty(t, h.resources)
}

func TestHeaderSetSize(t *testing.T) {
	h := NewHeader()

	h.SetSize(100)
	assert.Equal(t, 100, h.width)
	assert.Equal(t, 100, h.Width())
}

func TestHeaderSetProject(t *testing.T) {
	h := NewHeader()

	h.SetProject("my-project-123")
	assert.Equal(t, "my-project-123", h.projectID)
}

func TestHeaderSetCategory(t *testing.T) {
	h := NewHeader()

	h.SetCategory("Compute Engine")
	assert.Equal(t, "Compute Engine", h.category)
}

func TestHeaderSetResources(t *testing.T) {
	h := NewHeader()

	resources := []string{"my-instance", "detail-view"}
	h.SetResources(resources)
	assert.Equal(t, resources, h.resources)
}

func TestHeaderRenderAppName(t *testing.T) {
	h := NewHeader()
	h.SetSize(200)

	appName := h.renderAppName()

	// Check that it contains expected components
	assert.Contains(t, appName, "gcon")
	assert.Contains(t, appName, "Console Platform TUI")

	// Should not be empty
	assert.NotEmpty(t, appName)

	// Should render without errors
	assert.Greater(t, lipgloss.Width(appName), 0)
}

func TestHeaderRenderBreadcrumbs_NoProject(t *testing.T) {
	h := NewHeader()
	h.SetSize(200)

	breadcrumbs := h.renderBreadcrumbs()

	// Should be empty when no project is set
	assert.Empty(t, breadcrumbs)
}

func TestHeaderRenderBreadcrumbs_ProjectOnly(t *testing.T) {
	h := NewHeader()
	h.SetSize(200)
	h.SetProject("my-project-123")

	breadcrumbs := h.renderBreadcrumbs()

	// Should contain project ID
	assert.Contains(t, breadcrumbs, "my-project-123")
	assert.NotEmpty(t, breadcrumbs)
}

func TestHeaderRenderBreadcrumbs_ProjectAndCategory(t *testing.T) {
	h := NewHeader()
	h.SetSize(200)
	h.SetProject("my-project-123")
	h.SetCategory("Compute Engine")

	breadcrumbs := h.renderBreadcrumbs()

	// Should contain both project and category
	assert.Contains(t, breadcrumbs, "my-project-123")
	assert.Contains(t, breadcrumbs, "Compute Engine")

	// Should contain separator
	sep := symbols.HeaderSepRight()
	assert.Contains(t, breadcrumbs, sep)
}

func TestHeaderRenderBreadcrumbs_FullPath(t *testing.T) {
	h := NewHeader()
	h.SetSize(200)
	h.SetProject("my-project-123")
	h.SetCategory("Compute Engine")
	h.SetResources([]string{"my-instance"})

	breadcrumbs := h.renderBreadcrumbs()

	// Should contain all segments
	assert.Contains(t, breadcrumbs, "my-project-123")
	assert.Contains(t, breadcrumbs, "Compute Engine")
	assert.Contains(t, breadcrumbs, "my-instance")

	// Should contain separators
	sep := symbols.HeaderSepRight()
	assert.Contains(t, breadcrumbs, sep)
}

func TestHeaderRenderBreadcrumbs_MultipleResources(t *testing.T) {
	h := NewHeader()
	h.SetSize(200)
	h.SetProject("my-project-123")
	h.SetCategory("Cloud Storage")
	h.SetResources([]string{"my-bucket", "folder1", "folder2"})

	breadcrumbs := h.renderBreadcrumbs()

	// Should contain all segments
	assert.Contains(t, breadcrumbs, "my-project-123")
	assert.Contains(t, breadcrumbs, "Cloud Storage")
	assert.Contains(t, breadcrumbs, "my-bucket")
	assert.Contains(t, breadcrumbs, "folder1")
	assert.Contains(t, breadcrumbs, "folder2")
}

func TestHeaderView_EmptyState(t *testing.T) {
	h := NewHeader()
	h.SetSize(0)

	view := h.View()

	// Should return empty string when width is 0
	assert.Empty(t, view)
}

func TestHeaderView_AppNameOnly(t *testing.T) {
	h := NewHeader()
	h.SetSize(200)

	view := h.View()

	// Should contain app name
	assert.Contains(t, view, "gcon")
	assert.Contains(t, view, "Console Platform TUI")
	assert.NotEmpty(t, view)
}

func TestHeaderView_WithBreadcrumbs(t *testing.T) {
	h := NewHeader()
	h.SetSize(200)
	h.SetProject("my-project-123")
	h.SetCategory("Compute Engine")
	h.SetResources([]string{"my-instance"})

	view := h.View()

	// Should contain both app name and breadcrumbs
	assert.Contains(t, view, "gcon")
	assert.Contains(t, view, "my-project-123")
	assert.Contains(t, view, "Compute Engine")
	assert.Contains(t, view, "my-instance")
}

func TestHeaderTerminalWidth(t *testing.T) {
	h := NewHeader()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "plain text",
			input:    "hello",
			expected: 5,
		},
		{
			name:     "with cloud symbol",
			input:    "☁ text",
			expected: lipgloss.Width("☁ text") + 1, // Cloud symbol counts as 2-wide
		},
		{
			name:     "with powerline arrow",
			input:    "\ue0b0text",
			expected: lipgloss.Width("\ue0b0text") + 1, // Solid arrow counts as 2-wide
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width := h.terminalWidth(tt.input)
			assert.Equal(t, tt.expected, width)
		})
	}
}

func TestHeaderTruncateToWidth(t *testing.T) {
	h := NewHeader()

	tests := []struct {
		name     string
		input    string
		maxWidth int
	}{
		{
			name:     "no truncation needed",
			input:    "hello",
			maxWidth: 10,
		},
		{
			name:     "truncate simple text",
			input:    "hello world",
			maxWidth: 5,
		},
		{
			name:     "truncate with wide symbols",
			input:    "☁ hello world",
			maxWidth: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.truncateToWidth(tt.input, tt.maxWidth)
			actualWidth := h.terminalWidth(result)

			// Result should not exceed maxWidth
			assert.LessOrEqual(t, actualWidth, tt.maxWidth)

			// If input was shorter, result should match input
			inputWidth := h.terminalWidth(tt.input)
			if inputWidth <= tt.maxWidth {
				assert.Equal(t, tt.input, result)
			}
		})
	}
}

func TestHeaderASCIIMode(t *testing.T) {
	// Set ASCII mode
	symbols.SetASCIIMode(true)
	defer symbols.SetASCIIMode(false)

	h := NewHeader()
	h.SetSize(200)
	h.SetProject("my-project")
	h.SetCategory("Compute Engine")

	breadcrumbs := h.renderBreadcrumbs()

	// Should use ASCII separator instead of powerline arrow
	assert.Contains(t, breadcrumbs, ">")
	assert.NotContains(t, breadcrumbs, "\ue0b0") // Solid arrow
	assert.NotContains(t, breadcrumbs, "\ue0b1") // Thin arrow
}

func TestHeaderWidthHandling(t *testing.T) {
	h := NewHeader()

	tests := []struct {
		name   string
		width  int
		verify func(t *testing.T, view string, width int)
	}{
		{
			name:  "narrow width",
			width: 50,
			verify: func(t *testing.T, view string, width int) {
				actualWidth := h.terminalWidth(view)
				assert.LessOrEqual(t, actualWidth, width+5, "view should fit within width with small margin")
			},
		},
		{
			name:  "normal width",
			width: 120,
			verify: func(t *testing.T, view string, width int) {
				assert.NotEmpty(t, view)
				assert.Contains(t, view, "gcon")
			},
		},
		{
			name:  "wide width",
			width: 200,
			verify: func(t *testing.T, view string, width int) {
				assert.NotEmpty(t, view)
				assert.Contains(t, view, "gcon")
				assert.Contains(t, view, "Console Platform TUI")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.SetSize(tt.width)
			h.SetProject("my-project")
			h.SetCategory("Compute Engine")
			h.SetResources([]string{"my-instance"})

			view := h.View()
			tt.verify(t, view, tt.width)
		})
	}
}

func TestHeaderEmptyResources(t *testing.T) {
	h := NewHeader()
	h.SetSize(200)
	h.SetProject("my-project")
	h.SetCategory("Compute Engine")
	h.SetResources([]string{""}) // Empty resource

	breadcrumbs := h.renderBreadcrumbs()

	// Empty resources should be skipped
	assert.Contains(t, breadcrumbs, "my-project")
	assert.Contains(t, breadcrumbs, "Compute Engine")

	// Should have separator between project and category, plus closing separator
	sepCount := strings.Count(breadcrumbs, symbols.HeaderSepRight())
	assert.Equal(t, 2, sepCount, "should have separator between segments and closing separator")
}

func TestHeaderGoogleRainbowColors(t *testing.T) {
	h := NewHeader()
	h.SetSize(200)

	appName := h.renderAppName()

	// Verify that styles are initialized (can't test actual colors in terminal)
	assert.NotEmpty(t, appName)

	// Verify Google colors are properly initialized
	assert.NotEmpty(t, h.styles.GoogleColors.G.Render("test"))
	assert.NotEmpty(t, h.styles.GoogleColors.O1.Render("test"))
	assert.NotEmpty(t, h.styles.GoogleColors.O2.Render("test"))
	assert.NotEmpty(t, h.styles.GoogleColors.G2.Render("test"))
	assert.NotEmpty(t, h.styles.GoogleColors.L.Render("test"))
	assert.NotEmpty(t, h.styles.GoogleColors.E.Render("test"))
}

func TestHeaderBreadcrumbStyles(t *testing.T) {
	styles := DefaultHeaderStyles()

	// Verify breadcrumb styles are initialized
	assert.NotEmpty(t, styles.BreadcrumbProject.Render("test"))
	assert.NotEmpty(t, styles.BreadcrumbCategory.Render("test"))
	assert.NotEmpty(t, styles.BreadcrumbResource.Render("test"))
	assert.NotEmpty(t, styles.BreadcrumbSeparator.Render("test"))
}
