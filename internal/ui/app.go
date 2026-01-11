package ui

import (
	"context"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/views"
)

// ViewType represents different screens in the application
type ViewType int

const (
	ViewProjects ViewType = iota
	ViewInstances
	ViewBuckets
	ViewLogs
)

// App is the main application model
type App struct {
	gcpClient *gcp.Client
	styles    Styles
	keys      KeyMap
	help      help.Model
	width     int
	height    int

	// Current view state
	currentView   ViewType
	projectView   *views.ProjectsView
	instancesView *views.InstancesView

	// Selected context
	selectedProject *gcp.Project

	// UI state
	showHelp bool
	err      error

	// Initial project from config/flag (skip project selector if set)
	initialProjectID string
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
		currentView:      ViewProjects,
		projectView:      views.NewProjectsView(client),
		initialProjectID: opts.InitialProjectID,
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

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle back navigation first (before view-specific handlers)
		if key.Matches(msg, a.keys.Back) {
			if a.currentView != ViewProjects {
				a.currentView = ViewProjects
				a.instancesView = nil
				return a, nil
			}
		}

		// Global key handlers
		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.Help):
			a.showHelp = !a.showHelp
			return a, nil
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.Width = msg.Width
		a.projectView.SetSize(msg.Width, msg.Height-4)
		if a.instancesView != nil {
			a.instancesView.SetSize(msg.Width, msg.Height-4)
		}
		return a, nil

	case ErrorMsg:
		a.err = msg.Err
		return a, nil

	case views.ProjectSelectedMsg:
		project := msg.Project
		a.selectedProject = &project
		// Navigate to instances view
		a.currentView = ViewInstances
		a.instancesView = views.NewInstancesView(project.ID)
		a.instancesView.SetSize(a.width, a.height-4)
		return a, a.instancesView.Init()

	case InitialProjectLoadedMsg:
		// Initial project loaded successfully, go directly to instances view
		a.selectedProject = &msg.Project
		a.currentView = ViewInstances
		a.instancesView = views.NewInstancesView(msg.Project.ID)
		a.instancesView.SetSize(a.width, a.height-4)
		return a, a.instancesView.Init()

	case InitialProjectErrorMsg:
		// Failed to load initial project, fall back to selector with error displayed
		a.err = msg.Err
		a.initialProjectID = ""
		return a, a.projectView.Init()
	}

	// Delegate to current view
	var cmd tea.Cmd
	switch a.currentView {
	case ViewProjects:
		cmd = a.projectView.Update(msg)
	case ViewInstances:
		if a.instancesView != nil {
			cmd = a.instancesView.Update(msg)
		}
	}

	return a, cmd
}

// View implements tea.Model
func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	// Header with breadcrumb navigation
	header := a.styles.Title.Render("☁ gcon")
	if a.selectedProject != nil {
		header += a.styles.Muted.Render(" • " + a.selectedProject.ID)
		if a.currentView == ViewInstances {
			header += a.styles.Muted.Render(" • Compute Engine")
		}
	}

	// Main content based on current view
	var content string
	switch a.currentView {
	case ViewProjects:
		content = a.projectView.View()
	case ViewInstances:
		if a.instancesView != nil {
			content = a.instancesView.View()
		}
	default:
		content = "View not implemented"
	}

	// Error display
	if a.err != nil {
		content += "\n" + a.styles.Error.Render("Error: "+a.err.Error())
	}

	// Help footer
	var footer string
	if a.showHelp {
		footer = a.help.View(a.keys)
	} else {
		helpText := "? help • q quit"
		if a.currentView != ViewProjects {
			helpText = "esc back • " + helpText
		}
		footer = a.styles.Help.Render(helpText)
	}

	// Compose final layout
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		footer,
	)
}
