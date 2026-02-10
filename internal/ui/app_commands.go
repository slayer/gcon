package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/ui/components/commandpalette"
	"github.com/slayer/gcon/internal/ui/components/projectselector"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/slayer/gcon/internal/ui/views"
)

// openCommandPalette shows the command palette and configures it based on current state
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
//
//nolint:gocritic // hugeParam: Command struct passed by value for clarity
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

// handleNavigationCommand navigates to the selected view from command palette
//
//nolint:gocritic // hugeParam: Command struct passed by value for clarity
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

// handleActionCommand executes the selected action from command palette
//
//nolint:gocritic // hugeParam: Command struct passed by value for clarity
func (a *App) handleActionCommand(cmd commandpalette.Command) (tea.Model, tea.Cmd) {
	switch cmd.ID {
	case "action:refresh":
		return a, func() tea.Msg { return RefreshMsg{} }
	case "action:toggle-sidebar":
		// Toggle sidebar mode (matches `{` key behavior)
		if a.sidebarActive() {
			if a.sidebar.Mode() == sidebar.SidebarModeAutoHide {
				a.sidebar.SetMode(sidebar.SidebarModeAlwaysOpen)
			} else {
				a.sidebar.SetMode(sidebar.SidebarModeAutoHide)
				if a.focusedPanel == FocusSidebar {
					a.focusedPanel = FocusContent
					a.sidebar.SetFocused(false)
				}
			}
			a.updateViewSizes()
		}
		return a, nil
	case "action:help":
		a.showHelp = !a.showHelp
		return a, nil
	case "action:quit":
		return a, tea.Quit
	case "action:switch-project":
		// Create new project selector with current project ID for highlighting
		currentProjectID := ""
		if a.selectedProject != nil {
			currentProjectID = a.selectedProject.ID
		}
		a.projectSelector = projectselector.New(a.gcpClient, currentProjectID)
		a.showProjectSelector = true
		return a, a.projectSelector.Init()
	case "action:form-demo":
		return a.openFormDemo()
	}
	return a, nil
}

// openFormDemo opens the form components demo view
func (a *App) openFormDemo() (tea.Model, tea.Cmd) {
	// Create the form demo view
	a.formDemoView = views.NewFormDemoView()
	a.formDemoView.SetContext(a.ctx)

	// Push current view to stack for back navigation
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewFormDemo

	return a, a.formDemoView.Init()
}

// handleRecentCommand navigates to a recently accessed resource from command palette
func (a *App) handleRecentCommand(cmd commandpalette.Command) (tea.Model, tea.Cmd) {
	// Parse the command ID: "recent:<type>:<id>"
	parts := strings.SplitN(cmd.ID, ":", 3)
	if len(parts) < 3 {
		return a, nil
	}

	recentType := parts[1]
	//nolint:gocritic // commentedOutCode: intentionally commented, available for future use
	// resourceID := parts[2] // Available if needed for direct navigation

	switch recentType {
	case "project":
		// Go back to project list - user can select from there
		a.currentView = ViewProjects
		if a.projectView == nil {
			a.projectView = views.NewProjectsView(a.gcpClient)
		}
		a.updateViewSizes()
		return a, a.projectView.Init()
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
