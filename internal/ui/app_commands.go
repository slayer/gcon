package ui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/commandpalette"
	"github.com/slayer/gcon/internal/ui/components/sidebar"
	"github.com/slayer/gcon/internal/ui/views"
)

// openCommandPalette shows the command palette and configures it based on current state
func (a *App) openCommandPalette(showPrefix bool) {
	a.showCommandPalette = true
	a.showCommandPalette = true
	a.commandPalette.Reset()
	a.commandPalette.SetMode(commandpalette.ModeCommand)
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

// openProjectPalette shows the project selection palette
func (a *App) openProjectPalette() tea.Cmd {
	a.showCommandPalette = true
	a.commandPalette.Reset()
	a.commandPalette.SetMode(commandpalette.ModeProject)
	// Don't show prefix for project selection
	a.commandPalette.SetShowPrefix(false)

	// Set size
	paletteWidth := a.width * 6 / 10
	if paletteWidth < 50 {
		paletteWidth = 50
	}
	if paletteWidth > 80 {
		paletteWidth = 80
	}
	a.commandPalette.SetSize(paletteWidth, a.height)

	// Trigger project load
	return a.loadProjectsForPalette()
}

// loadProjectsForPalette fetches projects and updates the palette
func (a *App) loadProjectsForPalette() tea.Cmd {
	return func() tea.Msg {
		if a.gcpClient == nil {
			return nil
		}
		projects, err := a.gcpClient.ListProjects(context.Background())
		if err != nil {
			// For now just ignore error in palette, maybe show empty
			return nil
		}
		return projectsLoadedForPaletteMsg{projects: projects}
	}
}

// projectsLoadedForPaletteMsg carries loaded projects for the palette
type projectsLoadedForPaletteMsg struct {
	projects []gcp.Project
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
	case commandpalette.CommandTypeProject:
		return a.handleProjectCommand(cmd)
	}

	return a, nil
}

// handleNavigationCommand navigates to the selected view from command palette
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
func (a *App) handleActionCommand(cmd commandpalette.Command) (tea.Model, tea.Cmd) {
	switch cmd.ID {
	case "action:switch-project":
		return a, a.openProjectPalette()
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

// handleRecentCommand navigates to a recently accessed resource from command palette
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

// handleProjectCommand processes project selection from palette
func (a *App) handleProjectCommand(cmd commandpalette.Command) (tea.Model, tea.Cmd) {
	// Extract project ID from command ID "project:<id>"
	parts := strings.SplitN(cmd.ID, ":", 2)
	if len(parts) != 2 {
		return a, nil
	}
	projectID := parts[1]

	// Create a minimal project struct (we don't have full details here unless we passed them)
	// But ListProjects returns full details.
	// We can find the project in the palette's list if we want, but ID is enough for navigation.
	// Better to emit ProjectSelectedMsg which App handles exactly like from ProjectsView.

	project := gcp.Project{
		ID:   projectID,
		Name: projectID, // Fallback
	}

	return a, func() tea.Msg {
		return views.ProjectSelectedMsg{Project: project}
	}
}
