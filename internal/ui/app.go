package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/config"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/commandpalette"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/layout"
	"github.com/slayer/gcon/internal/ui/views"
)

// ViewType represents different screens in the application
type ViewType int

const (
	ViewNone ViewType = -1 // Sentinel value for unset/invalid view

	ViewProjects ViewType = iota
	ViewInstances
	ViewInstanceDetails
	ViewMetadata
	ViewProjectMetadata
	ViewDisks
	ViewDiskDetails
	ViewSnapshots
	ViewSnapshotDetails
	ViewImages
	ViewImageDetails
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
	ctx       *context.ProgramContext // Shared context for all views
	styles    Styles
	keys      KeyMap
	help      help.Model
	width     int
	height    int
	layout    *layout.Layout // Tile-based layout manager

	// Current view state
	currentView         ViewType
	viewStack           []ViewType // For back navigation
	projectView         *views.ProjectsView
	instancesView       *views.InstancesView
	instanceDetailsView *views.InstanceDetailsView
	metadataView        *views.InstanceMetadataView
	projectMetadataView *views.ProjectMetadataView
	disksView           *views.DisksView
	diskDetailsView     *views.DiskDetailsView
	snapshotsView       *views.SnapshotsView
	snapshotDetailsView *views.SnapshotDetailsView
	imagesView          *views.ImagesView
	imageDetailsView    *views.ImageDetailsView
	bucketsView         *views.BucketsView
	objectsView         *views.ObjectsView

	// Selected context
	selectedProject  *gcp.Project
	selectedInstance *gcp.Instance
	selectedDisk     *gcp.Disk
	selectedSnapshot *gcp.Snapshot
	selectedImage    *gcp.Image
	selectedBucket   *gcp.Bucket

	// UI state
	showHelp              bool
	err                   error
	loadingInitialProject bool // True while loading project from config/flag

	// Initial project from config/flag (skip project selector if set)
	initialProjectID string

	// Sidebar navigation (active after project selection)
	sidebar      *sidebar.Sidebar
	focusedPanel FocusedPanel

	// Command palette
	commandPalette     *commandpalette.CommandPalette
	showCommandPalette bool
	recentTracker      *commandpalette.RecentTracker

	// Footer
	footer *components.Footer

	// Authenticated identity (email of user or service account)
	authenticatedIdentity string
	identityType          config.IdentityType

	// GCloud configuration profile name
	configProfile string
}

// AppOptions configures the application
type AppOptions struct {
	// InitialProjectID skips project selector and goes directly to this project
	InitialProjectID string
}

// NewApp creates a new application instance
func NewApp(client *gcp.Client, opts AppOptions) *App {
	ctx := context.New()

	// Get authenticated identity and type if client is available
	var authenticatedIdentity string
	var identityType config.IdentityType
	if client != nil {
		authenticatedIdentity = client.GetAuthenticatedIdentity()
		identityType = client.GetIdentityType()
	}

	// Get gcloud config profile
	configProfile := config.ResolveActiveConfigName()

	a := &App{
		gcpClient:             client,
		ctx:                   ctx,
		styles:                DefaultStyles(),
		keys:                  DefaultKeyMap(),
		help:                  help.New(),
		layout:                layout.New(),
		currentView:           ViewProjects,
		viewStack:             []ViewType{},
		projectView:           views.NewProjectsView(client),
		initialProjectID:      opts.InitialProjectID,
		sidebar:               sidebar.New(),
		focusedPanel:          FocusContent,
		commandPalette:        commandpalette.New(),
		recentTracker:         commandpalette.NewRecentTracker(),
		footer:                components.NewFooter(),
		authenticatedIdentity: authenticatedIdentity,
		identityType:          identityType,
		configProfile:         configProfile,
	}

	// Set up the StartTask callback for async operation tracking
	ctx.StartTask = a.startTask

	return a
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	// If initial project is set, load it directly instead of showing selector
	if a.initialProjectID != "" {
		a.loadingInitialProject = true
		return a.loadInitialProject()
	}
	return a.projectView.Init()
}

// loadInitialProject switches directly to instances view using the configured project ID.
// Trade-off: We skip project validation to avoid requiring cloudresourcemanager permissions.
// This means invalid project IDs won't be caught until the user tries to load instances,
// but it allows users without project listing permissions to still use the app.
func (a *App) loadInitialProject() tea.Cmd {
	return func() tea.Msg {
		// Use project ID directly without validation
		return InitialProjectLoadedMsg{
			Project: gcp.Project{
				ID:   a.initialProjectID,
				Name: a.initialProjectID,
			},
		}
	}
}

// sidebarActive returns true if sidebar should be shown
func (a *App) sidebarActive() bool {
	return a.selectedProject != nil && a.currentView != ViewProjects
}

// isViewMenuOpen checks if the current view has an action menu open.
func (a *App) isViewMenuOpen() bool {
	if model := a.getCurrentViewModel(); model != nil {
		if menuOpener, ok := model.(views.MenuOpener); ok {
			return menuOpener.IsMenuOpen()
		}
	}
	return false
}

// updateCurrentView sends a message to the current view and returns its command
func (a *App) updateCurrentView(msg tea.Msg) tea.Cmd {
	if model := a.getCurrentViewModel(); model != nil {
		return model.Update(msg)
	}
	return nil
}

// getCurrentViewModel returns the model for the currently active view.
func (a *App) getCurrentViewModel() views.View {
	switch a.currentView {
	case ViewProjects:
		return a.projectView
	case ViewInstances:
		return a.instancesView
	case ViewInstanceDetails:
		return a.instanceDetailsView
	case ViewMetadata:
		return a.metadataView
	case ViewProjectMetadata:
		return a.projectMetadataView
	case ViewDisks:
		return a.disksView
	case ViewDiskDetails:
		return a.diskDetailsView
	case ViewSnapshots:
		return a.snapshotsView
	case ViewSnapshotDetails:
		return a.snapshotDetailsView
	case ViewImages:
		return a.imagesView
	case ViewImageDetails:
		return a.imageDetailsView
	case ViewBuckets:
		return a.bucketsView
	case ViewObjects:
		return a.objectsView
	}
	return nil
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle command palette messages first (highest priority when active)
	if a.showCommandPalette {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			cmd := a.commandPalette.Update(msg)
			return a, cmd
		case commandpalette.CommandSelectedMsg:
			return a.handleCommandSelected(msg.Command)
		case commandpalette.CommandCancelMsg:
			a.showCommandPalette = false
			a.commandPalette.Reset()
			return a, nil
		}
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		// Handle mouse events
		return a, a.handleMouseEvent(msg)

	case tea.KeyMsg:
		// Handle back navigation first (before view-specific handlers)
		// But skip if a view has an action menu open - let the view handle Esc
		if key.Matches(msg, a.keys.Back) {
			// Check if any view has action menu open
			if a.isViewMenuOpen() {
				// Let the view handle Esc to close its menu
				return a, a.updateCurrentView(msg)
			}

			// If sidebar is focused and drilled down, go back in sidebar
			if a.focusedPanel == FocusSidebar && len(a.sidebar.GetPath()) > 0 {
				a.sidebar.Update(msg)
				return a, nil
			}

			// If view stack is not empty, pop and go back
			if len(a.viewStack) > 0 {
				leavingView := a.currentView

				// Pop the last view from the stack
				lastViewIndex := len(a.viewStack) - 1
				a.currentView = a.viewStack[lastViewIndex]
				a.viewStack = a.viewStack[:lastViewIndex]

				// Clean up the view model we are navigating away from
				switch leavingView {
				case ViewInstanceDetails:
					a.instanceDetailsView = nil
					a.selectedInstance = nil
				case ViewDiskDetails:
					a.diskDetailsView = nil
					a.selectedDisk = nil
				case ViewSnapshotDetails:
					a.snapshotDetailsView = nil
					a.selectedSnapshot = nil
				case ViewImageDetails:
					a.imageDetailsView = nil
					a.selectedImage = nil
				case ViewObjects:
					a.objectsView = nil
					a.selectedBucket = nil
				}

				a.updateSidebarActiveView()
				return a, nil
			}

			// If stack is empty, check for special internal back navigation (e.g. Objects view)
			// or quit from top-level views
			switch a.currentView {
			case ViewObjects:
				if a.objectsView != nil {
					// Handle internal back navigation (e.g., going up a folder)
					handled, cmd := a.objectsView.HandleBack()
					if handled {
						return a, cmd
					}
				}
				// If not handled, fall through to quit
				fallthrough

			case ViewInstances, ViewDisks, ViewSnapshots, ViewImages, ViewBuckets, ViewNetworks, ViewFirewall, ViewProjects:
				// Quit from top-level views or if stack is empty
				a.cleanup()
				return a, tea.Quit
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
		case key.Matches(msg, a.keys.SelectSidebar):
			// '[' - Focus sidebar (if visible)
			if a.sidebarActive() && a.focusedPanel != FocusSidebar {
				a.focusedPanel = FocusSidebar
				a.sidebar.SetFocused(true)
			}
			return a, nil
		case key.Matches(msg, a.keys.SelectContent):
			// ']' - Focus content
			if a.focusedPanel != FocusContent {
				a.focusedPanel = FocusContent
				a.sidebar.SetFocused(false)
			}
			return a, nil
		case key.Matches(msg, a.keys.ToggleSidebar):
			// '{' - Show/hide sidebar
			if a.sidebarActive() {
				a.sidebar.Toggle()
				a.updateViewSizes()
			}
			return a, nil
		case key.Matches(msg, a.keys.CommandPalette):
			// Open command palette, show ":" prefix only when triggered by colon key
			showPrefix := key.Matches(msg, key.NewBinding(key.WithKeys(":")))
			a.openCommandPalette(showPrefix)
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
		a.footer.SetWidth(msg.Width)
		a.updateViewSizes()
		a.syncContext()
		return a, nil

	case context.TaskClearMsg:
		// Remove completed task from tracking
		delete(a.ctx.Tasks, msg.TaskID)
		return a, nil

	case ErrorMsg:
		a.err = msg.Err
		return a, nil

	case views.ProjectSelectedMsg:
		return a, a.handleProjectSelected(msg)

	case views.InstanceSelectedMsg:
		return a, a.handleInstanceSelected(msg)

	case views.DiskSelectedMsg:
		return a, a.handleDiskSelected(msg)

	case views.InstanceDiskSelectedMsg:
		return a, a.handleInstanceDiskSelected(msg)

	case views.SnapshotSelectedMsg:
		return a, a.handleSnapshotSelected(msg)

	case views.SnapshotDiskSelectedMsg:
		return a, a.handleSnapshotDiskSelected(msg)

	case views.ImageSelectedMsg:
		return a, a.handleImageSelected(msg)

	case InitialProjectLoadedMsg:
		// Initial project loaded successfully, go directly to instances view
		a.loadingInitialProject = false
		a.selectedProject = &msg.Project
		a.currentView = ViewInstances
		a.instancesView = views.NewInstancesView(msg.Project.ID)
		a.focusedPanel = FocusContent
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a, a.instancesView.Init()

	case InitialProjectErrorMsg:
		// Failed to load initial project, fall back to selector with error displayed
		a.loadingInitialProject = false
		a.err = msg.Err
		a.initialProjectID = ""
		return a, a.projectView.Init()

	case sidebar.NavigateMsg:
		// Handle sidebar navigation
		return a, a.handleSidebarNavigation(msg)

	case views.BucketSelectedMsg:
		return a, a.handleBucketSelected(msg)
	}

	// Delegate to current view (only if content is focused)
	var cmd tea.Cmd
	if a.focusedPanel == FocusContent || !a.sidebarActive() {
		if model := a.getCurrentViewModel(); model != nil {
			cmd = model.Update(msg)
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

// startTask registers a new async task and returns a command to animate the spinner.
// Tasks are tracked in the context and displayed in the footer.
func (a *App) startTask(task context.Task) tea.Cmd {
	task.StartTime = time.Now()
	task.State = context.TaskRunning
	a.ctx.Tasks[task.ID] = task
	return nil // Could return spinner.Tick if we want animation
}

// GetContext returns the shared program context.
// Views can use this to access dimensions, styles, and task tracking.
func (a *App) GetContext() *context.ProgramContext {
	return a.ctx
}

// finishTask marks a task as complete and schedules its removal.
// Called when an async operation completes. Currently unused but will be
// integrated when views adopt the task system.
//
//nolint:unused
func (a *App) finishTask(taskID string, err error) tea.Cmd {
	if task, ok := a.ctx.Tasks[taskID]; ok {
		now := time.Now()
		task.FinishedTime = &now
		if err != nil {
			task.State = context.TaskError
			task.Error = err
		} else {
			task.State = context.TaskFinished
		}
		a.ctx.Tasks[taskID] = task

		// Schedule task removal after 2 seconds
		return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return context.TaskClearMsg{TaskID: taskID}
		})
	}
	return nil
}

// syncContext updates the shared context with current app state.
// Called after dimension changes or project selection.
func (a *App) syncContext() {
	contentWidth := a.layout.ContentWidth()
	contentHeight := a.layout.ContentHeight()

	a.ctx.SetDimensions(a.width, a.height, contentWidth, contentHeight)
	a.ctx.SidebarActive = a.sidebarActive()
	if a.sidebarActive() {
		a.ctx.SidebarWidth = a.sidebar.Width()
	}
	if a.selectedProject != nil {
		a.ctx.ProjectID = a.selectedProject.ID
	} else {
		a.ctx.ProjectID = ""
	}
	a.ctx.Error = a.err
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

// updateViewSizes recalculates sizes for all views using the layout manager.
// Uses SetContext to propagate shared context to all views.
func (a *App) updateViewSizes() {
	// Update layout with sidebar state
	if a.sidebarActive() {
		a.layout.SetSidebarWidth(a.sidebar.Width())
		a.layout.SetSidebarActive(true)
	} else {
		a.layout.SetSidebarActive(false)
	}

	// Sync context with current dimensions before propagating
	a.syncContext()

	// Sidebar uses content height directly
	if a.sidebarActive() {
		a.sidebar.SetSize(a.ctx.ContentHeight)
	}

	// Propagate context to all views - they read dimensions from ctx
	a.projectView.SetContext(a.ctx)

	if a.instancesView != nil {
		a.instancesView.SetContext(a.ctx)
	}
	if a.instanceDetailsView != nil {
		a.instanceDetailsView.SetContext(a.ctx)
	}
	if a.metadataView != nil {
		a.metadataView.SetContext(a.ctx)
	}
	if a.projectMetadataView != nil {
		a.projectMetadataView.SetContext(a.ctx)
	}
	if a.disksView != nil {
		a.disksView.SetContext(a.ctx)
	}
	if a.diskDetailsView != nil {
		a.diskDetailsView.SetContext(a.ctx)
	}
	if a.snapshotsView != nil {
		a.snapshotsView.SetContext(a.ctx)
	}
	if a.snapshotDetailsView != nil {
		a.snapshotDetailsView.SetContext(a.ctx)
	}
	if a.imagesView != nil {
		a.imagesView.SetContext(a.ctx)
	}
	if a.imageDetailsView != nil {
		a.imageDetailsView.SetContext(a.ctx)
	}
	if a.bucketsView != nil {
		a.bucketsView.SetContext(a.ctx)
	}
	if a.objectsView != nil {
		a.objectsView.SetContext(a.ctx)
	}
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

	// Calculate safe width for the component with the most emojis.
	// JoinVertical pads all components to match the widest one, so we must
	// use a single safe width based on the worst-case emoji count.
	// This ensures lines with emojis don't overflow after JoinVertical padding.
	allContent := header + "\n" + content + "\n" + footer
	safeWidth := SafeWidth(a.width, allContent)

	placedHeader := lipgloss.Place(safeWidth, headerHeight, lipgloss.Left, lipgloss.Top, header)
	placedContent := lipgloss.Place(safeWidth, contentHeight, lipgloss.Left, lipgloss.Top, content)
	placedFooter := lipgloss.Place(safeWidth, footerHeight, lipgloss.Left, lipgloss.Top, footer)

	debugLog("SafeWidth: %d (terminal=%d), maxEmojis=%d",
		safeWidth, a.width, maxLineEmojiCount(allContent))
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
		emojiCount := countWideEmojis(line)
		if tw > a.width {
			debugLog("! Line %d exceeds width: lipgloss=%d, terminal=%d > %d, emojis=%d", i, w, tw, a.width, emojiCount)
			// Show first 80 chars of raw line content for debugging
			preview := line
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			debugLog("  content: %q", preview)
		}
	}
	debugLog("")

	// Render command palette overlay if active
	if a.showCommandPalette {
		result = a.renderWithCommandPalette(result)
	}

	return result
}

// renderFooter syncs content to the footer component and renders it
func (a *App) renderFooter() string {
	if a.showHelp {
		return a.help.View(a.keys)
	}

	// Sync footer content based on current state
	a.syncFooter()
	return a.footer.View()
}
