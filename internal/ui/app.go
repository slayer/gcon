package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/commandpalette"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/slayer/gcon/internal/ui/context"
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
	ViewDiskDetails
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
	projectView         *views.ProjectsView
	instancesView       *views.InstancesView
	instanceDetailsView *views.InstanceDetailsView
	disksView           *views.DisksView
	diskDetailsView     *views.DiskDetailsView
	bucketsView         *views.BucketsView
	objectsView         *views.ObjectsView

	// Selected context
	selectedProject  *gcp.Project
	selectedInstance *gcp.Instance
	selectedDisk     *gcp.Disk
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
}

// AppOptions configures the application
type AppOptions struct {
	// InitialProjectID skips project selector and goes directly to this project
	InitialProjectID string
}

// NewApp creates a new application instance
func NewApp(client *gcp.Client, opts AppOptions) *App {
	ctx := context.New()

	a := &App{
		gcpClient:        client,
		ctx:              ctx,
		styles:           DefaultStyles(),
		keys:             DefaultKeyMap(),
		help:             help.New(),
		layout:           layout.New(),
		currentView:      ViewProjects,
		projectView:      views.NewProjectsView(client),
		initialProjectID: opts.InitialProjectID,
		sidebar:          sidebar.New(),
		focusedPanel:     FocusContent,
		commandPalette:   commandpalette.New(),
		recentTracker:    commandpalette.NewRecentTracker(),
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
			case ViewDiskDetails:
				// Go back to disks list
				a.currentView = ViewDisks
				a.diskDetailsView = nil
				a.selectedDisk = nil
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
				a.disksView = nil
				a.diskDetailsView = nil
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
		project := msg.Project
		a.selectedProject = &project
		// Track recent project access
		a.recentTracker.Track(commandpalette.RecentTypeProject, project.ID, project.Name)
		// Navigate to instances view with sidebar
		a.currentView = ViewInstances
		a.instancesView = views.NewInstancesView(project.ID)
		a.focusedPanel = FocusContent
		a.updateSidebarActiveView()
		a.updateViewSizes()
		a.syncContext()
		return a, a.instancesView.Init()

	case views.InstanceSelectedMsg:
		// Navigate to instance details view
		inst := msg.Instance
		a.selectedInstance = &inst
		// Track recent instance access
		a.recentTracker.Track(commandpalette.RecentTypeInstance, inst.Name, inst.Name)
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

	case views.DiskSelectedMsg:
		// Navigate to disk details view
		disk := msg.Disk
		a.selectedDisk = &disk
		// Track recent disk access
		a.recentTracker.Track(commandpalette.RecentTypeDisk, disk.Name, disk.Name)
		a.currentView = ViewDiskDetails
		// Pass compute client from disks view to avoid re-initialization
		a.diskDetailsView = views.NewDiskDetailsView(
			a.selectedProject.ID,
			disk.Zone,
			disk.Name,
			a.disksView.GetComputeClient(),
		)
		a.updateSidebarActiveView()
		a.updateViewSizes()
		return a, a.diskDetailsView.Init()

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
		// Track recent bucket access
		a.recentTracker.Track(commandpalette.RecentTypeBucket, bucket.Name, bucket.Name)
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
		case ViewDisks:
			if a.disksView != nil {
				cmd = a.disksView.Update(msg)
			}
		case ViewDiskDetails:
			if a.diskDetailsView != nil {
				cmd = a.diskDetailsView.Update(msg)
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

// openCommandPalette shows the command palette
func (a *App) openCommandPalette(showPrefix bool) {
	a.showCommandPalette = true
	a.commandPalette.Reset()
	a.commandPalette.SetShowPrefix(showPrefix)
	a.commandPalette.SetProjectSelected(a.selectedProject != nil)

	// Build command list with recent items at the top
	commands := a.recentTracker.Commands()
	commands = append(commands, commandpalette.DefaultCommands()...)
	a.commandPalette.SetCommands(commands)

	// Set size for centered display
	paletteWidth := a.width * 6 / 10 // 60% of screen width
	if paletteWidth < 50 {
		paletteWidth = 50
	}
	if paletteWidth > 80 {
		paletteWidth = 80
	}
	a.commandPalette.SetSize(paletteWidth, a.height)
}

// handleCommandSelected processes a selected command from the palette
func (a *App) handleCommandSelected(cmd commandpalette.Command) (tea.Model, tea.Cmd) {
	a.showCommandPalette = false
	a.commandPalette.Reset()

	switch cmd.Type {
	case commandpalette.CommandTypeNavigation:
		return a.handleNavigationCommand(cmd)
	case commandpalette.CommandTypeAction:
		return a.handleActionCommand(cmd)
	case commandpalette.CommandTypeRecent:
		return a.handleRecentCommand(cmd)
	}

	return a, nil
}

// handleNavigationCommand navigates to the selected view
func (a *App) handleNavigationCommand(cmd commandpalette.Command) (tea.Model, tea.Cmd) {
	// Navigation commands require a project to be selected
	if a.selectedProject == nil {
		return a, nil
	}

	// Navigate to the view (reuse sidebar navigation logic via NavigateMsg)
	return a, func() tea.Msg {
		return sidebar.NavigateMsg{ViewType: sidebar.ViewType(cmd.ViewType)}
	}
}

// handleActionCommand executes the selected action
func (a *App) handleActionCommand(cmd commandpalette.Command) (tea.Model, tea.Cmd) {
	switch cmd.ID {
	case "action:refresh":
		return a, func() tea.Msg { return RefreshMsg{} }
	case "action:toggle-sidebar":
		if a.sidebarActive() {
			a.sidebar.Toggle()
			a.updateViewSizes()
		}
		return a, nil
	case "action:help":
		a.showHelp = !a.showHelp
		return a, nil
	case "action:quit":
		return a, tea.Quit
	}
	return a, nil
}

// handleRecentCommand navigates to a recently accessed resource
func (a *App) handleRecentCommand(cmd commandpalette.Command) (tea.Model, tea.Cmd) {
	// Parse the command ID: "recent:<type>:<id>"
	parts := strings.SplitN(cmd.ID, ":", 3)
	if len(parts) < 3 {
		return a, nil
	}

	recentType := parts[1]
	// resourceID := parts[2] // Available if needed for direct navigation

	switch recentType {
	case "project":
		// Go back to project list - user can select from there
		a.currentView = ViewProjects
		return a, nil
	case "bucket":
		// Navigate to buckets view if we have a project
		if a.selectedProject != nil {
			return a, func() tea.Msg {
				return sidebar.NavigateMsg{ViewType: sidebar.ViewBuckets}
			}
		}
	case "instance":
		// Navigate to instances view if we have a project
		if a.selectedProject != nil {
			return a, func() tea.Msg {
				return sidebar.NavigateMsg{ViewType: sidebar.ViewInstances}
			}
		}
	case "disk":
		// Navigate to disks view if we have a project
		if a.selectedProject != nil {
			return a, func() tea.Msg {
				return sidebar.NavigateMsg{ViewType: sidebar.ViewDisks}
			}
		}
	}

	return a, nil
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
	if a.disksView != nil {
		a.disksView.SetSize(contentWidth, contentHeight)
	}
	if a.diskDetailsView != nil {
		a.diskDetailsView.SetSize(contentWidth, contentHeight)
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
	case ViewDisks, ViewDiskDetails:
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
		if a.currentView != ViewDisks {
			a.currentView = ViewDisks
			if a.disksView == nil {
				a.disksView = views.NewDisksView(a.selectedProject.ID)
				a.updateViewSizes()
				cmd = a.disksView.Init()
			}
		}
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

		if a.currentView == ViewDiskDetails && a.selectedDisk != nil {
			header += a.styles.Muted.Render(" • " + a.selectedDisk.Name)
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

	// Calculate safe width accounting for emojis in BOTH sidebar and content.
	// JoinHorizontal combines lines side-by-side, so emojis from both views
	// end up on the same terminal line. We need to account for the maximum
	// combined emoji count on any single line.
	sidebarMaxEmojis := maxLineEmojiCount(sidebarView)
	contentMaxEmojis := maxLineEmojiCount(contentView)
	totalMaxEmojis := sidebarMaxEmojis + contentMaxEmojis

	// Debug: show emoji count per sidebar line
	sidebarLines := strings.Split(sidebarView, "\n")
	for i, line := range sidebarLines {
		ec := countWideEmojis(line)
		if ec > 0 {
			debugLog("  sidebar line %d: %d emojis, width=%d", i, ec, lipgloss.Width(line))
		}
	}

	mainWidth := a.layout.ContentWidth() - totalMaxEmojis
	if mainWidth < 10 {
		mainWidth = 10
	}
	// Use MaxWidth to constrain content that may be wider than available space.
	// Width() sets minimum width, MaxWidth() sets maximum and truncates/wraps.
	mainStyle := lipgloss.NewStyle().Width(mainWidth).MaxWidth(mainWidth)
	styledContent := mainStyle.Render(contentView)

	debugLog("renderWithSidebar: sidebar=%d, contentWidth=%d, mainWidth=%d, emojis=%d+%d",
		lipgloss.Width(sidebarView), a.layout.ContentWidth(), mainWidth,
		sidebarMaxEmojis, contentMaxEmojis)

	// Join horizontally - parent View() will enforce overall height
	result := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarView,
		styledContent,
	)

	// Debug: show width breakdown after join
	resultLines := strings.Split(result, "\n")
	expectedWidth := a.layout.ContentWidth()
	for i, line := range resultLines[:min(10, len(resultLines))] {
		lw := lipgloss.Width(line)
		tw := TerminalWidth(line)
		// Only log lines where width differs from expected layout width
		if lw != expectedWidth || tw > a.width {
			debugLog("  result line %d: lipgloss=%d, terminal=%d, emojis=%d", i, lw, tw, countWideEmojis(line))
		}
	}
	debugLog("renderWithSidebar: result maxLineWidth=%d", MaxLineWidth(result))
	return result
}

// renderWithCommandPalette overlays the command palette on top of the content
func (a *App) renderWithCommandPalette(background string) string {
	// Get the command palette view
	paletteView := a.commandPalette.View()

	// Split background into lines
	bgLines := strings.Split(background, "\n")

	// Split palette into lines
	paletteLines := strings.Split(paletteView, "\n")

	// Find max palette width for consistent positioning
	maxPaletteWidth := 0
	for _, line := range paletteLines {
		w := lipgloss.Width(line)
		if w > maxPaletteWidth {
			maxPaletteWidth = w
		}
	}

	// Calculate position (centered horizontally, 1/4 down vertically)
	leftPad := (a.width - maxPaletteWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	topPad := a.height / 4
	if topPad < 2 {
		topPad = 2
	}

	// Overlay the palette on the background, preserving content on both sides
	result := make([]string, len(bgLines))
	copy(result, bgLines)

	// Right side always starts at the same position for alignment
	rightStart := leftPad + maxPaletteWidth

	for i, paletteLine := range paletteLines {
		bgIndex := topPad + i
		if bgIndex < len(result) {
			bgLine := result[bgIndex]
			bgWidth := lipgloss.Width(bgLine)

			// Build the overlayed line:
			// 1. Left part of background (truncated to leftPad width)
			// 2. Palette line (padded to maxPaletteWidth)
			// 3. Right part of background (from rightStart onwards)
			var newLine strings.Builder

			// Left part: truncate background to leftPad characters
			if leftPad > 0 {
				leftPart := truncateRight(bgLine, leftPad)
				newLine.WriteString(leftPart)
				// Pad if background was shorter than leftPad
				leftWidth := lipgloss.Width(leftPart)
				if leftWidth < leftPad {
					newLine.WriteString(strings.Repeat(" ", leftPad-leftWidth))
				}
			}

			// Middle: the palette line, padded to consistent width
			newLine.WriteString(paletteLine)
			paletteLineWidth := lipgloss.Width(paletteLine)
			if paletteLineWidth < maxPaletteWidth {
				newLine.WriteString(strings.Repeat(" ", maxPaletteWidth-paletteLineWidth))
			}

			// Right part: skip first rightStart characters of background
			if rightStart < bgWidth {
				rightPart := truncateLeft(bgLine, rightStart)
				newLine.WriteString(rightPart)
			}

			result[bgIndex] = newLine.String()
		}
	}

	return strings.Join(result, "\n")
}

// truncateRight keeps the first n visible columns of an ANSI string.
// Uses runewidth to correctly handle wide characters (e.g., CJK, some symbols).
func truncateRight(s string, n int) string {
	if n <= 0 {
		return ""
	}

	var result strings.Builder
	var visibleWidth int
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}
		if inEscape {
			result.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}

		rw := runewidth.RuneWidth(r)
		if visibleWidth+rw > n {
			break
		}
		result.WriteRune(r)
		visibleWidth += rw
	}

	return result.String()
}

// truncateLeft removes the first n visible columns from an ANSI string.
// Uses runewidth to correctly handle wide characters.
func truncateLeft(s string, n int) string {
	width := lipgloss.Width(s)
	if n >= width {
		return ""
	}

	var result strings.Builder
	var visibleWidth int
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			if visibleWidth >= n {
				result.WriteRune(r)
			}
			continue
		}
		if inEscape {
			if visibleWidth >= n {
				result.WriteRune(r)
			}
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}

		rw := runewidth.RuneWidth(r)
		if visibleWidth >= n {
			result.WriteRune(r)
		}
		visibleWidth += rw
	}

	return result.String()
}

// renderCurrentView renders the content area based on current view
func (a *App) renderCurrentView() string {
	// Show loading message while fetching initial project from config
	if a.loadingInitialProject {
		return "Loading project " + a.initialProjectID + "..."
	}

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
		if a.disksView != nil {
			return a.disksView.View()
		}
	case ViewDiskDetails:
		if a.diskDetailsView != nil {
			return a.diskDetailsView.View()
		}
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
