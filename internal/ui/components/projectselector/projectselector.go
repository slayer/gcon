package projectselector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/symbols"
)

const (
	maxVisibleProjects = 10
	minWidth           = 60
	defaultWidth       = 80
)

// keyMap defines key bindings for the project selector
type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Cancel key.Binding
	CtrlN  key.Binding
	CtrlP  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		CtrlN: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("ctrl+n", "down"),
		),
		CtrlP: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "up"),
		),
	}
}

// Model is the project selector modal component
type Model struct {
	// Display
	projects         []gcp.Project
	filteredProjects []gcp.Project
	width            int
	height           int
	cursor           int

	// State
	loading        bool
	err            error
	textInput      textinput.Model
	spinner        spinner.Model
	currentProject string // ID of currently selected project (to highlight)

	// Dependencies
	client *gcp.Client

	// Styles
	keys   keyMap
	styles Styles
}

// New creates a new project selector
func New(client *gcp.Client, currentProjectID string) *Model {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = minWidth
	ti.Placeholder = "Filter projects..."

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &Model{
		client:         client,
		currentProject: currentProjectID,
		textInput:      ti,
		spinner:        sp,
		loading:        true,
		width:          defaultWidth,
		height:         20,
		keys:           defaultKeyMap(),
		styles:         DefaultStyles(),
	}
}

// SetSize updates dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	inputWidth := width - 8
	if inputWidth < minWidth {
		inputWidth = minWidth
	}
	m.textInput.Width = inputWidth
}

// Init loads projects asynchronously
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadProjects(),
	)
}

// loadProjects fetches projects from GCP
func (m *Model) loadProjects() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		projects, err := m.client.ListProjects(ctx)
		if err != nil {
			return projectsErrorMsg{err: err}
		}

		// Filter to only show ACTIVE projects
		activeProjects := make([]gcp.Project, 0, len(projects))
		for _, p := range projects {
			if p.State == "ACTIVE" {
				activeProjects = append(activeProjects, p)
			}
		}

		return projectsLoadedMsg{projects: activeProjects}
	}
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case projectsLoadedMsg:
		m.loading = false
		m.projects = msg.projects
		m.filteredProjects = msg.projects
		m.cursor = 0
		return nil

	case projectsErrorMsg:
		m.loading = false
		m.err = msg.err
		return nil

	case spinner.TickMsg:
		if m.loading {
			m.spinner, cmd = m.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		// If loading or error state, only allow cancel
		if m.loading || m.err != nil {
			if key.Matches(msg, m.keys.Cancel) {
				return func() tea.Msg { return ProjectSelectorCanceledMsg{} }
			}
			// On error, 'r' to retry
			if m.err != nil && msg.String() == "r" {
				m.err = nil
				m.loading = true
				return tea.Batch(
					m.spinner.Tick,
					m.loadProjects(),
				)
			}
			return nil
		}

		// Navigation
		switch {
		case key.Matches(msg, m.keys.Up):
			m.moveUp()
			return nil

		case key.Matches(msg, m.keys.Down):
			m.moveDown()
			return nil

		case key.Matches(msg, m.keys.CtrlP):
			m.moveUp()
			return nil

		case key.Matches(msg, m.keys.CtrlN):
			m.moveDown()
			return nil

		case key.Matches(msg, m.keys.Cancel):
			return func() tea.Msg { return ProjectSelectorCanceledMsg{} }

		case key.Matches(msg, m.keys.Select):
			// If there are no projects to select, ignore the selection
			if len(m.filteredProjects) == 0 {
				return nil
			}
			// If the cursor is out of bounds, reset it safely and ignore the selection
			if m.cursor < 0 || m.cursor >= len(m.filteredProjects) {
				m.cursor = 0
				return nil
			}
			selectedProject := m.filteredProjects[m.cursor]
			// If selecting the same project, just close modal
			if selectedProject.ID == m.currentProject {
				return func() tea.Msg { return ProjectSelectorCanceledMsg{} }
			}
			return func() tea.Msg {
				return ProjectSelectedMsg{Project: selectedProject}
			}

		default:
			// Update text input and filter
			m.textInput, cmd = m.textInput.Update(msg)
			m.filterProjects()
			return cmd
		}
	}

	return nil
}

// moveUp moves cursor up
func (m *Model) moveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// moveDown moves cursor down
func (m *Model) moveDown() {
	if m.cursor < len(m.filteredProjects)-1 {
		m.cursor++
	}
}

// filterProjects filters projects based on text input
func (m *Model) filterProjects() {
	query := strings.ToLower(strings.TrimSpace(m.textInput.Value()))
	if query == "" {
		m.filteredProjects = m.projects
		m.cursor = 0
		return
	}

	filtered := make([]gcp.Project, 0)
	for _, p := range m.projects {
		nameMatch := strings.Contains(strings.ToLower(p.Name), query)
		idMatch := strings.Contains(strings.ToLower(p.ID), query)
		if nameMatch || idMatch {
			filtered = append(filtered, p)
		}
	}
	m.filteredProjects = filtered

	// Cap cursor to last valid position instead of resetting to 0
	if m.cursor >= len(m.filteredProjects) {
		if len(m.filteredProjects) > 0 {
			m.cursor = len(m.filteredProjects) - 1
		} else {
			m.cursor = 0
		}
	}
}

// View renders the project selector
func (m *Model) View() string {
	var b strings.Builder

	// Title
	b.WriteString(m.styles.Title.Render("Select Project"))
	b.WriteString("\n\n")

	// Loading state
	if m.loading {
		b.WriteString(fmt.Sprintf("%s Loading projects...", m.spinner.View()))
		b.WriteString("\n\n")
		b.WriteString(m.styles.Help.Render("esc: cancel"))
		return m.styles.Container.Width(m.width).Render(b.String())
	}

	// Error state
	if m.err != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("Error: %s", m.err.Error())))
		b.WriteString("\n\n")
		b.WriteString(m.styles.Help.Render("r: retry  esc: cancel"))
		return m.styles.Container.Width(m.width).Render(b.String())
	}

	// Filter input
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	// Empty state
	if len(m.filteredProjects) == 0 {
		if len(m.projects) == 0 {
			b.WriteString(m.styles.EmptyState.Render("No projects found. Check permissions or create a project in GCP Console."))
		} else {
			b.WriteString(m.styles.EmptyState.Render(fmt.Sprintf("No projects match '%s'. Press backspace to clear.", m.textInput.Value())))
		}
		b.WriteString("\n\n")
		b.WriteString(m.styles.Help.Render("esc: cancel"))
		return m.styles.Container.Width(m.width).Render(b.String())
	}

	// Project list
	visibleStart := 0
	visibleEnd := len(m.filteredProjects)
	if len(m.filteredProjects) > maxVisibleProjects {
		// Calculate visible window around cursor
		visibleStart = m.cursor - maxVisibleProjects/2
		if visibleStart < 0 {
			visibleStart = 0
		}
		visibleEnd = visibleStart + maxVisibleProjects
		if visibleEnd > len(m.filteredProjects) {
			visibleEnd = len(m.filteredProjects)
			visibleStart = visibleEnd - maxVisibleProjects
			if visibleStart < 0 {
				visibleStart = 0
			}
		}
	}

	for i := visibleStart; i < visibleEnd; i++ {
		project := m.filteredProjects[i]
		line := m.renderProject(project, i == m.cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Show scroll indicator
	if len(m.filteredProjects) > maxVisibleProjects {
		scrollInfo := fmt.Sprintf("(%d/%d)", m.cursor+1, len(m.filteredProjects))
		b.WriteString("\n")
		b.WriteString(m.styles.Help.Render(scrollInfo))
	}

	// Divider
	b.WriteString("\n")
	dividerWidth := m.width - 4
	if dividerWidth < 1 {
		dividerWidth = 1
	}
	b.WriteString(m.styles.Divider.Render(strings.Repeat("─", dividerWidth)))
	b.WriteString("\n")

	// Help text (plain text, no lipgloss styling to avoid background)
	b.WriteString("j/k:nav  enter:select  esc:cancel")

	return m.styles.Container.Width(m.width).Render(b.String())
}

// renderProject renders a single project line
func (m *Model) renderProject(project gcp.Project, selected bool) string {
	cursor := "  "
	if selected {
		cursor = symbols.Cursor() + " "
	}

	// Show checkmark for currently selected project
	checkmark := " "
	if project.ID == m.currentProject {
		checkmark = "✓"
	}

	// Format: [cursor] [check] [name]  [id]
	var nameStyle, idStyle lipgloss.Style
	if selected {
		nameStyle = m.styles.ProjectNameSelected
		idStyle = m.styles.ProjectIDSelected
	} else {
		nameStyle = m.styles.ProjectName
		idStyle = m.styles.ProjectID
	}

	// Truncate name if too long
	name := project.Name
	if len(name) > 30 {
		name = name[:27] + "..."
	}

	line := cursor + checkmark + " " + nameStyle.Render(name) + "  " + idStyle.Render("("+project.ID+")")
	return line
}

// HasTextInputFocused returns true when the text input is focused.
// Used to prevent global keys (like 'q' for quit) from being triggered while typing.
func (m *Model) HasTextInputFocused() bool {
	// Text input is always focused when the selector is visible and not in loading/error state
	return !m.loading && m.err == nil
}
