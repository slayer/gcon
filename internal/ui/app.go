package ui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/slayer/gcon/internal/ui/layout"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/slayer/gcon/internal/ui/views"
)

// ViewType represents different screens in the application
type ViewType int

const (
	ViewProjects ViewType = iota
	ViewInstances
	ViewInstanceDetails
	ViewDisks
	ViewBuckets
	ViewObjects // Browsing objects within a bucket
	ViewNetworks
	ViewFirewall
	ViewLogs
)

// FocusedPanel indicates which panel has keyboard focus
type FocusedPanel int

const (
	FocusContent FocusedPanel = iota
	FocusSidebar
)

// App is the main application model
type App struct {
	gcpClient *gcp.Client
	styles    Styles
	keys      KeyMap
	help      help.Model
	width     int
	height    int
	layout    *layout.Layout // Tile-based layout manager

	// Current view state
	currentView         ViewType
	projectView         *views.ProjectsView
	instancesView       *views.InstancesView
	instanceDetailsView *views.InstanceDetailsView
	bucketsView         *views.BucketsView
	objectsView         *views.ObjectsView

	// Selected context
	selectedProject  *gcp.Project
	selectedInstance *gcp.Instance
	selectedBucket   *gcp.Bucket

	// UI state
	showHelp bool
	err      error

	// Initial project from config/flag (skip project selector if set)
	initialProjectID string

	// Sidebar navigation (active after project selection)
	sidebar      *sidebar.Sidebar
	focusedPanel FocusedPanel
}

// AppOptions configures the application
type AppOptions struct {
	// InitialProjectID skips project selector and goes directly to this project
	InitialProjectID string
}

// NewApp creates a new application instance
func NewApp(client *gcp.Client, opts AppOptions) *App {
	return &App{
		gcpClient:        client,
		styles:           DefaultStyles(),
		keys:             DefaultKeyMap(),
		help:             help.New(),
		layout:           layout.New(),
		currentView:      ViewProjects,
		projectView:      views.NewProjectsView(client),
		initialProjectID: opts.InitialProjectID,
		sidebar:          sidebar.New(),
		focusedPanel:     FocusContent,
	}
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	// If initial project is set, load it directly instead of showing selector
	if a.initialProjectID != "" {
		return a.loadInitialProject()
	}
	return a.projectView.Init()
}

// loadInitialProject fetches project details and switches to instances view
func (a *App) loadInitialProject() tea.Cmd {
	return func() tea.Msg {
		project, err := a.gcpClient.GetProject(context.Background(), a.initialProjectID)
		if err != nil {
			return InitialProjectErrorMsg{
				Err:       err,
				ProjectID: a.initialProjectID,
			}
		}
		return InitialProjectLoadedMsg{Project: *project}
	}
}

// sidebarActive returns true if sidebar should be shown
func (a *App) sidebarActive() bool {
	return a.selectedProject != nil && a.currentView != ViewProjects
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle back navigation first (before view-specific handlers)
		if key.Matches(msg, a.keys.Back) {
			// If sidebar is focused and drilled down, go back in sidebar
			if a.focusedPanel == FocusSidebar && len(a.sidebar.GetPath()) > 0 {
				a.sidebar.Update(msg)
				return a, nil
			}

			switch a.currentView {
			case ViewInstanceDetails:
				// Go back to instances list
				a.currentView = ViewInstances
				a.instanceDetailsView = nil
				a.selectedInstance = nil
				a.updateSidebarActiveView()
				return a, nil
			case ViewObjects:
				// Check if we can go up a folder, otherwise go back to buckets
				if a.objectsView != nil {
					handled, cmd := a.objectsView.HandleBack()
					if handled {
						return a, cmd
					}
				}
				// Go back to buckets list
				a.currentView = ViewBuckets
				a.objectsView = nil
				a.selectedBucket = nil
				a.updateSidebarActiveView()
				return a, nil
			case ViewInstances, ViewDisks, ViewBuckets, ViewNetworks, ViewFirewall:
				// Go back to projects, clear sidebar state
				a.currentView = ViewProjects
				a.instancesView = nil
				// Close storage client before discarding bucketsView
				if a.bucketsView != nil {
					_ = a.bucketsView.Close()
				}
				a.bucketsView = nil
				a.selectedProject = nil
				a.focusedPanel = FocusContent
				return a, nil
			}
		}

		// Global key handlers
		switch {
		case key.Matches(msg, a.keys.Quit):
			// Clean up resources before quitting
			a.cleanup()
			return a, tea.Quit
		case key.Matches(msg, a.keys.Help):
			a.showHelp = !a.showHelp
			return a, nil
		case key.Matches(msg, a.keys.Tab):
			// Switch focus between sidebar and content
			if a.sidebarActive() {
				a.toggleFocus()
			}
			return a, nil
		case key.Matches(msg, a.keys.ShiftTab):
			// Same as Tab for now (toggle)
			if a.sidebarActive() {
				a.toggleFocus()
			}
			return a, nil
		case key.Matches(msg, a.keys.ToggleSidebar):
			// Toggle sidebar collapsed/expanded
			if a.sidebarActive() {
				a.sidebar.Toggle()
				a.updateViewSizes()
			}
			return a, nil
		}

		// Route to sidebar if focused
		if a.sidebarActive() && a.focusedPanel == FocusSidebar {
			cmd := a.sidebar.Update(msg)
			return a, cmd
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.Width = msg.Width
		// Update layout with new terminal dimensions
		a.layout.SetSize(msg.Width, msg.Height)
		a.updateViewSizes()
		return a, nil

	case ErrorMsg:
		a.err = msg.Err
		return a, nil

	case views.ProjectSelectedMsg:
		project := msg.Project
		a.selectedProject = &project
		// Navigate to instances view with sidebar
		a.currentView = ViewInstances
		a.instancesView = views.NewInstancesView(project.ID)
		a.focusedPanel = FocusContent
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a, a.instancesView.Init()

	case views.InstanceSelectedMsg:
		// Navigate to instance details view
		inst := msg.Instance
		a.selectedInstance = &inst
		a.currentView = ViewInstanceDetails
		// Pass compute client from instances view to avoid re-initialization
		a.instanceDetailsView = views.NewInstanceDetailsView(
			a.selectedProject.ID,
			inst.Zone,
			inst.Name,
			a.instancesView.GetComputeClient(),
		)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a, a.instanceDetailsView.Init()

	case InitialProjectLoadedMsg:
		// Initial project loaded successfully, go directly to instances view
		a.selectedProject = &msg.Project
		a.currentView = ViewInstances
		a.instancesView = views.NewInstancesView(msg.Project.ID)
		a.focusedPanel = FocusContent
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a, a.instancesView.Init()

	case InitialProjectErrorMsg:
		// Failed to load initial project, fall back to selector with error displayed
		a.err = msg.Err
		a.initialProjectID = ""
		return a, a.projectView.Init()

	case sidebar.NavigateMsg:
		// Handle sidebar navigation
		return a, a.handleSidebarNavigation(msg)

	case views.BucketSelectedMsg:
		// Navigate to objects view within the selected bucket
		if a.bucketsView == nil {
			return a, nil
		}
		storageClient := a.bucketsView.GetStorageClient()
		if storageClient == nil {
			return a, nil
		}
		bucket := msg.Bucket
		a.selectedBucket = &bucket
		a.currentView = ViewObjects
		a.objectsView = views.NewObjectsView(bucket.Name, storageClient)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a, a.objectsView.Init()
	}

	// Delegate to current view (only if content is focused)
	var cmd tea.Cmd
	if a.focusedPanel == FocusContent || !a.sidebarActive() {
		switch a.currentView {
		case ViewProjects:
			cmd = a.projectView.Update(msg)
		case ViewInstances:
			if a.instancesView != nil {
				cmd = a.instancesView.Update(msg)
			}
		case ViewInstanceDetails:
			if a.instanceDetailsView != nil {
				cmd = a.instanceDetailsView.Update(msg)
			}
		case ViewBuckets:
			if a.bucketsView != nil {
				cmd = a.bucketsView.Update(msg)
			}
		case ViewObjects:
			if a.objectsView != nil {
				cmd = a.objectsView.Update(msg)
			}
		}
	}

	return a, cmd
}

// cleanup releases resources held by views
func (a *App) cleanup() {
	if a.bucketsView != nil {
		_ = a.bucketsView.Close() // Best-effort cleanup on exit
	}
}

// toggleFocus switches focus between sidebar and content
func (a *App) toggleFocus() {
	if a.focusedPanel == FocusContent {
		a.focusedPanel = FocusSidebar
		a.sidebar.SetFocused(true)
	} else {
		a.focusedPanel = FocusContent
		a.sidebar.SetFocused(false)
	}
}

// updateViewSizes recalculates sizes for all views using the layout manager
func (a *App) updateViewSizes() {
	// Update layout with sidebar state
	if a.sidebarActive() {
		a.layout.SetSidebarWidth(a.sidebar.Width())
		a.layout.SetSidebarActive(true)
	} else {
		a.layout.SetSidebarActive(false)
	}

	// Get dimensions from layout - layout handles all calculations
	contentWidth := a.layout.ContentWidth()
	contentHeight := a.layout.ContentHeight()

	// Sidebar uses content height directly
	if a.sidebarActive() {
		a.sidebar.SetSize(contentHeight)
	}

	// Projects view uses full width (no sidebar)
	a.projectView.SetSize(a.width, contentHeight)

	// Other views use content area (respecting sidebar)
	if a.instancesView != nil {
		a.instancesView.SetSize(contentWidth, contentHeight)
	}
	if a.instanceDetailsView != nil {
		a.instanceDetailsView.SetSize(contentWidth, contentHeight)
	}
	if a.bucketsView != nil {
		a.bucketsView.SetSize(contentWidth, contentHeight)
	}
	if a.objectsView != nil {
		a.objectsView.SetSize(contentWidth, contentHeight)
	}
}

// updateSidebarActiveView sets the active view highlight in sidebar
func (a *App) updateSidebarActiveView() {
	switch a.currentView {
	case ViewInstances, ViewInstanceDetails:
		a.sidebar.SetActiveView(sidebar.ViewInstances)
	case ViewDisks:
		a.sidebar.SetActiveView(sidebar.ViewDisks)
	case ViewBuckets, ViewObjects:
		a.sidebar.SetActiveView(sidebar.ViewBuckets)
	case ViewNetworks:
		a.sidebar.SetActiveView(sidebar.ViewNetworks)
	case ViewFirewall:
		a.sidebar.SetActiveView(sidebar.ViewFirewall)
	}
}

// handleSidebarNavigation processes sidebar navigation messages
func (a *App) handleSidebarNavigation(msg sidebar.NavigateMsg) tea.Cmd {
	var cmd tea.Cmd

	// Map sidebar ViewType to app ViewType and navigate
	switch msg.ViewType {
	case sidebar.ViewInstances:
		if a.currentView != ViewInstances && a.currentView != ViewInstanceDetails {
			a.currentView = ViewInstances
			a.instanceDetailsView = nil
			a.selectedInstance = nil
			if a.instancesView == nil {
				a.instancesView = views.NewInstancesView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.instancesView.Init()
			}
		}
	case sidebar.ViewDisks:
		a.currentView = ViewDisks
		// Placeholder - view not implemented yet
	case sidebar.ViewBuckets:
		if a.currentView != ViewBuckets && a.currentView != ViewObjects {
			a.currentView = ViewBuckets
			a.objectsView = nil
			a.selectedBucket = nil
			if a.bucketsView == nil {
				a.bucketsView = views.NewBucketsView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.bucketsView.Init()
			}
		}
	case sidebar.ViewNetworks:
		a.currentView = ViewNetworks
		// Placeholder - view not implemented yet
	case sidebar.ViewFirewall:
		a.currentView = ViewFirewall
		// Placeholder - view not implemented yet
	}

	a.updateSidebarActiveView()
	// Switch focus back to content after navigation
	a.focusedPanel = FocusContent
	a.sidebar.SetFocused(false)

	return cmd
}

// View implements tea.Model
func (a *App) View() string {
	// Header with breadcrumb navigation (always show, even before window size is known)
	header := a.renderHeader()

	if a.width == 0 {
		// Before window size is known, show header with loading message
		return header + "\n\n  Loading..."
	}

	// Get dimensions from layout for consistent rendering
	_, headerHeight := a.layout.HeaderSize()
	contentHeight := a.layout.ContentHeight()
	_, footerHeight := a.layout.FooterSize()

	debugLog("=== View() called ===")
	debugLog("Terminal: width=%d, height=%d", a.width, a.height)
	debugLog("Layout: header=%d, content=%d, footer=%d, total=%d",
		headerHeight, contentHeight, footerHeight, headerHeight+contentHeight+footerHeight)

	// Main content area (with or without sidebar)
	var content string
	if a.sidebarActive() {
		content = a.renderWithSidebar()
	} else {
		content = a.renderCurrentView()
	}

	debugLogView("Raw header", header)
	debugLogView("Raw content", content)

	// Error display
	if a.err != nil {
		content += "\n" + a.styles.Error.Render("Error: "+a.err.Error())
	}

	// Use lipgloss.Place for positioning, then truncate at line boundaries.
	// Line-based truncation is ANSI-safe since escape sequences don't span lines.
	// This avoids the fragmentation issues that MaxHeight() causes in native terminals.
	//
	// Calculate safe width for each component individually.
	// lipgloss.Place() pads based on lipgloss.Width(), but terminals may render
	// some emojis wider than lipgloss measures. We reduce the target width
	// by the emoji count in each component to compensate.
	footer := a.renderFooter()

	// Each component gets its own safe width calculation
	headerSafeWidth := SafeWidth(a.width, header)
	contentSafeWidth := SafeWidth(a.width, content)
	footerSafeWidth := SafeWidth(a.width, footer)

	placedHeader := lipgloss.Place(headerSafeWidth, headerHeight, lipgloss.Left, lipgloss.Top, header)
	placedContent := lipgloss.Place(contentSafeWidth, contentHeight, lipgloss.Left, lipgloss.Top, content)
	placedFooter := lipgloss.Place(footerSafeWidth, footerHeight, lipgloss.Left, lipgloss.Top, footer)

	debugLog("SafeWidths: header=%d, content=%d, footer=%d (terminal=%d)",
		headerSafeWidth, contentSafeWidth, footerSafeWidth, a.width)
	debugLog("EmojiCounts: header=%d, content=%d, footer=%d",
		countWideEmojis(header), countWideEmojis(content), countWideEmojis(footer))
	debugLogView("Placed header", placedHeader)
	debugLogView("Placed content", placedContent)
	debugLogView("Placed footer", placedFooter)

	styledHeader := truncateToHeight(placedHeader, headerHeight)
	styledContent := truncateToHeight(placedContent, contentHeight)
	styledFooter := truncateToHeight(placedFooter, footerHeight)

	// Compose final layout with guaranteed heights
	result := lipgloss.JoinVertical(
		lipgloss.Left,
		styledHeader,
		styledContent,
		styledFooter,
	)

	// Log width analysis
	resultLines := strings.Split(result, "\n")
	debugLog("Final result: %d lines", len(resultLines))
	for i, line := range resultLines {
		w := lipgloss.Width(line)
		tw := TerminalWidth(line)
		if tw > a.width {
			debugLog("⚠️ Line %d exceeds width: lipgloss=%d, terminal=%d > %d", i, w, tw, a.width)
		}
	}
	debugLog("")

	return result
}

// renderHeader creates the header with breadcrumb
func (a *App) renderHeader() string {
	header := a.styles.Title.Render(symbols.Cloud() + " gcon")
	if a.selectedProject != nil {
		header += a.styles.Muted.Render(" • " + a.selectedProject.ID)

		// Show current category from sidebar if drilled down
		if category := a.sidebar.GetCurrentCategory(); category != "" {
			header += a.styles.Muted.Render(" • " + category)
		} else {
			// Show category based on current view
			switch a.currentView {
			case ViewInstances, ViewInstanceDetails, ViewDisks:
				header += a.styles.Muted.Render(" • Compute Engine")
			case ViewBuckets, ViewObjects:
				header += a.styles.Muted.Render(" • Cloud Storage")
			case ViewNetworks, ViewFirewall:
				header += a.styles.Muted.Render(" • VPC Network")
			}
		}

		if a.currentView == ViewInstanceDetails && a.selectedInstance != nil {
			header += a.styles.Muted.Render(" • " + a.selectedInstance.Name)
		}

		// Show bucket name and path when browsing objects
		if a.currentView == ViewObjects && a.objectsView != nil {
			header += a.styles.Muted.Render(" • " + a.objectsView.GetBucketName())
			if path := a.objectsView.GetCurrentPath(); path != "" {
				header += a.styles.Muted.Render(" / " + path)
			}
		}
	}
	return header
}

// renderWithSidebar creates the two-panel layout with guaranteed matching heights
func (a *App) renderWithSidebar() string {
	// Sidebar already has its own styling (border, width, height) applied internally
	sidebarView := a.sidebar.View()
	contentView := a.renderCurrentView()

	// Calculate safe width accounting for emojis in sidebar and content.
	// Use SafeWidth to automatically detect emojis and reduce width accordingly.
	combined := sidebarView + contentView
	mainWidth := SafeWidth(a.layout.ContentWidth(), combined)
	mainStyle := lipgloss.NewStyle().Width(mainWidth)
	styledContent := mainStyle.Render(contentView)

	// Join horizontally - parent View() will enforce overall height
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarView,
		styledContent,
	)
}

// renderCurrentView renders the content area based on current view
func (a *App) renderCurrentView() string {
	switch a.currentView {
	case ViewProjects:
		return a.projectView.View()
	case ViewInstances:
		if a.instancesView != nil {
			return a.instancesView.View()
		}
	case ViewInstanceDetails:
		if a.instanceDetailsView != nil {
			return a.instanceDetailsView.View()
		}
	case ViewDisks:
		return a.renderPlaceholder("Disks")
	case ViewBuckets:
		if a.bucketsView != nil {
			return a.bucketsView.View()
		}
	case ViewObjects:
		if a.objectsView != nil {
			return a.objectsView.View()
		}
	case ViewNetworks:
		return a.renderPlaceholder("VPC Networks")
	case ViewFirewall:
		return a.renderPlaceholder("Firewall Rules")
	}
	return "View not implemented"
}

// renderPlaceholder renders a placeholder for unimplemented views
func (a *App) renderPlaceholder(name string) string {
	return a.styles.Muted.Render("\n  " + name + " view - not implemented yet\n\n  Use sidebar to navigate to VM instances.")
}

// renderFooter creates the help footer
func (a *App) renderFooter() string {
	if a.showHelp {
		return a.help.View(a.keys)
	}

	helpText := "? help • q quit"
	if a.currentView != ViewProjects {
		helpText = "esc back • " + helpText
	}
	if a.sidebarActive() {
		helpText = "tab focus • [ sidebar • " + helpText
	}
	return a.styles.Help.Render(helpText)
}

// truncateToHeight truncates content to exactly maxLines by splitting on newlines.
// This is ANSI-safe because escape sequences don't span line boundaries.
func truncateToHeight(content string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}

	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}

	// Take only the first maxLines lines
	return strings.Join(lines[:maxLines], "\n")
}
