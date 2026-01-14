package views

import (
	gocontext "context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	btable "github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// projectKeyMap defines project-specific key bindings
type projectKeyMap struct {
	Enter   key.Binding
	Refresh key.Binding
}

func defaultProjectKeyMap() projectKeyMap {
	return projectKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// Table column definitions for projects
func projectColumns() []btable.Column {
	return []btable.Column{
		{Title: "Name", Width: 35},
		{Title: "Project ID", Width: 35},
		{Title: "State", Width: 15},
	}
}

// ProjectsView displays and manages the list of GCP projects in a table format
type ProjectsView struct {
	client   *gcp.Client
	ctx      *context.ProgramContext // Shared context for dimensions and styles
	table    table.Model
	spinner  spinner.Model
	loading  bool
	err      error
	projects []gcp.Project
	keys     projectKeyMap
}

// NewProjectsView creates a new projects view with table display
func NewProjectsView(client *gcp.Client) *ProjectsView {
	t := table.New(projectColumns(), "Select Project")

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &ProjectsView{
		client:  client,
		table:   t,
		spinner: s,
		loading: true,
		keys:    defaultProjectKeyMap(),
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
		projects, err := v.client.ListProjects(gocontext.Background())
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

// stateIcon returns a symbol for project state
func stateIcon(state string) string {
	// Map project states to status symbols
	switch state {
	case "ACTIVE":
		return symbols.StatusRunning()
	case "DELETE_REQUESTED", "DELETE_IN_PROGRESS":
		return symbols.StatusStopped()
	default:
		return symbols.StatusUnknown()
	}
}

// projectToRow converts a GCP project to a table row
func projectToRow(p gcp.Project) table.Row {
	return table.Row{
		Data: []string{
			p.Name,
			p.ID,
			stateIcon(p.State) + " " + p.State,
		},
		FilterValue: p.Name + " " + p.ID + " " + p.State,
		ID:          p.ID,
	}
}

// Update handles messages for the projects view
func (v *ProjectsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		v.loading = false
		v.projects = msg.projects

		// Convert to table rows
		rows := make([]table.Row, len(msg.projects))
		for i, p := range msg.projects {
			rows[i] = projectToRow(p)
		}
		v.table.SetRows(rows)
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
		if v.loading {
			return nil
		}

		// Let table handle filtering mode
		if v.table.IsFiltering() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
		}

		switch {
		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadProjects())

		case key.Matches(msg, v.keys.Enter):
			if row := v.table.SelectedRow(); row != nil {
				proj := v.findProjectByID(row.ID)
				if proj != nil {
					return func() tea.Msg {
						return ProjectSelectedMsg{Project: *proj}
					}
				}
			}
		}
	}

	// Delegate to table component
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// findProjectByID looks up a project by ID
func (v *ProjectsView) findProjectByID(id string) *gcp.Project {
	for _, p := range v.projects {
		if p.ID == id {
			return &p
		}
	}
	return nil
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
		return "\n" + components.RenderError(v.err)
	}

	if len(v.projects) == 0 {
		return "\n  No projects found.\n  Make sure you have access to GCP projects."
	}

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: select • /: filter • r: refresh • q: quit")

	return v.table.View() + help
}

// SetSize updates the view dimensions (deprecated, use SetContext)
func (v *ProjectsView) SetSize(width, height int) {
	v.table.SetSize(width, height-4) // Reserve space for help
}

// SetContext updates the view with shared program context.
// Reads dimensions from the context for consistent sizing.
func (v *ProjectsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	// Projects view uses full screen width (no sidebar)
	v.table.SetSize(ctx.ScreenWidth, ctx.ContentHeight-4)
}

// SelectedProject returns the currently selected project
func (v *ProjectsView) SelectedProject() *gcp.Project {
	if row := v.table.SelectedRow(); row != nil {
		return v.findProjectByID(row.ID)
	}
	return nil
}
