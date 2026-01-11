package views

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

// projectItem implements list.Item for the bubbles list component
type projectItem struct {
	project gcp.Project
}

func (i projectItem) Title() string { return i.project.Name }
func (i projectItem) Description() string {
	return fmt.Sprintf("ID: %s • State: %s", i.project.ID, i.project.State)
}
func (i projectItem) FilterValue() string { return i.project.Name + " " + i.project.ID }

// ProjectsView displays and manages the list of GCP projects
type ProjectsView struct {
	client   *gcp.Client
	list     list.Model
	spinner  spinner.Model
	loading  bool
	err      error
	width    int
	height   int
	projects []gcp.Project
}

// NewProjectsView creates a new projects view
func NewProjectsView(client *gcp.Client) *ProjectsView {
	// Configure the list delegate for custom styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#4285F4")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#CCCCCC")).
		Background(lipgloss.Color("#4285F4"))

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Select Project"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#4285F4")).
		Padding(0, 1)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &ProjectsView{
		client:  client,
		list:    l,
		spinner: s,
		loading: true,
	}
}

// Init starts loading projects
func (v *ProjectsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.loadProjects(),
	)
}

// loadProjects fetches projects from GCP
func (v *ProjectsView) loadProjects() tea.Cmd {
	return func() tea.Msg {
		projects, err := v.client.ListProjects(context.Background())
		if err != nil {
			return projectsErrorMsg{err: err}
		}
		return projectsLoadedMsg{projects: projects}
	}
}

// Message types for this view
type projectsLoadedMsg struct {
	projects []gcp.Project
}

type projectsErrorMsg struct {
	err error
}

// Update handles messages for the projects view
func (v *ProjectsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		v.loading = false
		v.projects = msg.projects

		// Convert to list items
		items := make([]list.Item, len(msg.projects))
		for i, p := range msg.projects {
			items[i] = projectItem{project: p}
		}
		v.list.SetItems(items)
		return nil

	case projectsErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		if msg.String() == "r" {
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadProjects())
		}
		if msg.String() == "enter" {
			if item, ok := v.list.SelectedItem().(projectItem); ok {
				return func() tea.Msg {
					return ProjectSelectedMsg{Project: item.project}
				}
			}
		}
	}

	// Delegate to list component
	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return cmd
}

// ProjectSelectedMsg is emitted when a project is selected
type ProjectSelectedMsg struct {
	Project gcp.Project
}

// View renders the projects view
func (v *ProjectsView) View() string {
	if v.loading {
		return fmt.Sprintf("\n  %s Loading projects...\n", v.spinner.View())
	}

	if v.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EA4335")).
			Render(fmt.Sprintf("\n  Error: %v\n\n  Press 'r' to retry", v.err))
	}

	if len(v.projects) == 0 {
		return "\n  No projects found.\n  Make sure you have access to GCP projects."
	}

	return v.list.View()
}

// SetSize updates the view dimensions
func (v *ProjectsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.list.SetSize(width, height)
}

// SelectedProject returns the currently selected project
func (v *ProjectsView) SelectedProject() *gcp.Project {
	if item, ok := v.list.SelectedItem().(projectItem); ok {
		return &item.project
	}
	return nil
}
