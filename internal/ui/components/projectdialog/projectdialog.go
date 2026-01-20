// Package projectdialog provides a modal dialog for selecting GCP projects.
// It can be triggered from the command palette, footer click, or shown on startup
// when no default project is detected.
package projectdialog

import (
	gocontext "context"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/symbols"
)

const (
	maxVisibleItems = 12
	minWidth        = 50
	maxWidth        = 80
)

// Messages emitted by ProjectDialog

// ProjectDialogSelectedMsg is sent when a project is selected
type ProjectDialogSelectedMsg struct {
	Project gcp.Project
}

// ProjectDialogClosedMsg is sent when the dialog is closed without selection
type ProjectDialogClosedMsg struct{}

// dialogKeyMap defines key bindings for the project dialog
type dialogKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Cancel key.Binding
	Retry  key.Binding
	CtrlN  key.Binding
	CtrlP  key.Binding
	Filter key.Binding
}

func defaultKeyMap() dialogKeyMap {
	return dialogKeyMap{
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
		Retry: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "retry"),
		),
		CtrlN: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("ctrl+n", "down"),
		),
		CtrlP: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "up"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
	}
}

// Styles defines the visual styles for the dialog
type Styles struct {
	Container      lipgloss.Style
	Title          lipgloss.Style
	Input          lipgloss.Style
	InputFocused   lipgloss.Style
	Item           lipgloss.Style
	ItemSelected   lipgloss.Style
	ItemDisabled   lipgloss.Style
	CurrentProject lipgloss.Style
	Help           lipgloss.Style
	Error          lipgloss.Style
	Spinner        lipgloss.Style
}

// DefaultStyles returns the default dialog styles
func DefaultStyles() Styles {
	// GCP colors
	primary := lipgloss.Color("#4285F4")      // Google Blue
	bgColor := lipgloss.Color("#202124")      // Dark background
	borderColor := lipgloss.Color("#5F6368")  // Gray border
	textColor := lipgloss.Color("#E8EAED")    // Light text
	mutedColor := lipgloss.Color("#9AA0A6")   // Muted text
	successColor := lipgloss.Color("#34A853") // Green for current project

	return Styles{
		Container: lipgloss.NewStyle().
			Background(bgColor).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 2),
		Title: lipgloss.NewStyle().
			Foreground(primary).
			Bold(true).
			MarginBottom(1),
		Input: lipgloss.NewStyle().
			Foreground(textColor).
			Background(lipgloss.Color("#303134")).
			Padding(0, 1),
		InputFocused: lipgloss.NewStyle().
			Foreground(textColor).
			Background(lipgloss.Color("#3C4043")).
			BorderForeground(primary),
		Item: lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1),
		ItemSelected: lipgloss.NewStyle().
			Foreground(textColor).
			Background(lipgloss.Color("#3C4043")).
			Padding(0, 1),
		ItemDisabled: lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(0, 1),
		CurrentProject: lipgloss.NewStyle().
			Foreground(successColor),
		Help: lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EA4335")).
			MarginTop(1),
		Spinner: lipgloss.NewStyle().
			Foreground(primary),
	}
}

// ProjectDialog is a modal dialog for selecting GCP projects
type ProjectDialog struct {
	client           *gcp.Client
	input            textinput.Model
	projects         []gcp.Project
	filtered         []gcp.Project
	cursor           int
	width            int
	height           int
	styles           Styles
	keys             dialogKeyMap
	spinner          spinner.Model
	loading          bool
	err              error
	currentProjectID string // Currently selected project (to mark with checkmark)
	canCancel        bool   // Whether Esc can close the dialog (false on startup)
	filterMode       bool   // Whether we're in filter/search mode
}

// New creates a new project dialog
func New(client *gcp.Client) *ProjectDialog {
	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.CharLimit = 50
	ti.Width = 40

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	return &ProjectDialog{
		client:     client,
		input:      ti,
		projects:   nil,
		filtered:   nil,
		cursor:     0,
		width:      60,
		height:     20,
		styles:     DefaultStyles(),
		keys:       defaultKeyMap(),
		spinner:    s,
		loading:    true,
		err:        nil,
		canCancel:  true,
		filterMode: false,
	}
}

// SetSize sets the dialog dimensions
func (d *ProjectDialog) SetSize(width, height int) {
	d.width = width
	d.height = height
	// Adjust input width to fit container
	inputWidth := width - 8 // Account for borders and padding
	if inputWidth < minWidth-8 {
		inputWidth = minWidth - 8
	}
	d.input.Width = inputWidth
}

// SetCurrentProject sets the currently selected project (to mark with checkmark)
func (d *ProjectDialog) SetCurrentProject(projectID string) {
	d.currentProjectID = projectID
}

// SetCanCancel sets whether the dialog can be cancelled with Esc
func (d *ProjectDialog) SetCanCancel(canCancel bool) {
	d.canCancel = canCancel
}

// Init starts loading projects
func (d *ProjectDialog) Init() tea.Cmd {
	d.loading = true
	d.err = nil
	return tea.Batch(
		d.spinner.Tick,
		d.loadProjects(),
	)
}

// loadProjects fetches projects from GCP
func (d *ProjectDialog) loadProjects() tea.Cmd {
	return func() tea.Msg {
		projects, err := d.client.ListProjects(gocontext.Background())
		if err != nil {
			return projectsErrorMsg{err: err}
		}
		return projectsLoadedMsg{projects: projects}
	}
}

// Internal messages
type projectsLoadedMsg struct {
	projects []gcp.Project
}

type projectsErrorMsg struct {
	err error
}

// Update handles messages
func (d *ProjectDialog) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		d.loading = false
		d.projects = msg.projects
		d.filterProjects()
		return nil

	case projectsErrorMsg:
		d.loading = false
		d.err = msg.err
		return nil

	case spinner.TickMsg:
		if d.loading {
			var cmd tea.Cmd
			d.spinner, cmd = d.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		// Handle error state - only allow retry or cancel
		if d.err != nil {
			switch {
			case key.Matches(msg, d.keys.Retry):
				d.loading = true
				d.err = nil
				return tea.Batch(d.spinner.Tick, d.loadProjects())
			case key.Matches(msg, d.keys.Cancel):
				if d.canCancel {
					return func() tea.Msg { return ProjectDialogClosedMsg{} }
				}
			}
			return nil
		}

		// Handle loading state - only allow cancel
		if d.loading {
			if key.Matches(msg, d.keys.Cancel) && d.canCancel {
				return func() tea.Msg { return ProjectDialogClosedMsg{} }
			}
			return nil
		}

		// Handle filter mode
		if d.filterMode {
			switch {
			case key.Matches(msg, d.keys.Cancel):
				// Exit filter mode
				d.filterMode = false
				d.input.Blur()
				return nil
			case key.Matches(msg, d.keys.Select):
				// Exit filter mode and select if we have results
				d.filterMode = false
				d.input.Blur()
				if len(d.filtered) > 0 && d.cursor < len(d.filtered) {
					return d.selectProject(d.filtered[d.cursor])
				}
				return nil
			case key.Matches(msg, d.keys.Up), key.Matches(msg, d.keys.CtrlP):
				d.moveCursor(-1)
				return nil
			case key.Matches(msg, d.keys.Down), key.Matches(msg, d.keys.CtrlN):
				d.moveCursor(1)
				return nil
			default:
				// Update text input
				var cmd tea.Cmd
				d.input, cmd = d.input.Update(msg)
				d.filterProjects()
				return cmd
			}
		}

		// Normal mode (not filtering)
		switch {
		case key.Matches(msg, d.keys.Cancel):
			if d.canCancel {
				return func() tea.Msg { return ProjectDialogClosedMsg{} }
			}
			return nil
		case key.Matches(msg, d.keys.Filter):
			// Enter filter mode
			d.filterMode = true
			d.input.Focus()
			return textinput.Blink
		case key.Matches(msg, d.keys.Up), key.Matches(msg, d.keys.CtrlP):
			d.moveCursor(-1)
			return nil
		case key.Matches(msg, d.keys.Down), key.Matches(msg, d.keys.CtrlN):
			d.moveCursor(1)
			return nil
		case key.Matches(msg, d.keys.Select):
			if len(d.filtered) > 0 && d.cursor < len(d.filtered) {
				return d.selectProject(d.filtered[d.cursor])
			}
			return nil
		}

		// Handle typing to start filter mode
		if len(msg.String()) == 1 && msg.String() != " " {
			d.filterMode = true
			d.input.Focus()
			d.input.SetValue(msg.String())
			d.filterProjects()
			return textinput.Blink
		}
	}

	return nil
}

// moveCursor moves the selection cursor
func (d *ProjectDialog) moveCursor(delta int) {
	d.cursor += delta
	if d.cursor < 0 {
		d.cursor = 0
	}
	if d.cursor >= len(d.filtered) {
		d.cursor = len(d.filtered) - 1
	}
	if d.cursor < 0 {
		d.cursor = 0
	}
}

// filterProjects filters the project list based on input
func (d *ProjectDialog) filterProjects() {
	query := strings.ToLower(d.input.Value())
	if query == "" {
		d.filtered = d.projects
	} else {
		d.filtered = nil
		for _, p := range d.projects {
			if strings.Contains(strings.ToLower(p.Name), query) ||
				strings.Contains(strings.ToLower(p.ID), query) {
				d.filtered = append(d.filtered, p)
			}
		}
	}
	// Reset cursor if out of bounds
	if d.cursor >= len(d.filtered) {
		d.cursor = len(d.filtered) - 1
	}
	if d.cursor < 0 {
		d.cursor = 0
	}
}

// selectProject emits a selection message
func (d *ProjectDialog) selectProject(project gcp.Project) tea.Cmd {
	return func() tea.Msg {
		return ProjectDialogSelectedMsg{Project: project}
	}
}

// View renders the dialog
func (d *ProjectDialog) View() string {
	var b strings.Builder

	// Title
	title := "Select Project"
	if !d.canCancel {
		title = "Select Project (required)"
	}
	b.WriteString(d.styles.Title.Render(title))
	b.WriteString("\n")

	// Loading state
	if d.loading {
		b.WriteString("\n")
		b.WriteString(d.styles.Spinner.Render(d.spinner.View()))
		b.WriteString(" Loading projects...")
		b.WriteString("\n")
		if d.canCancel {
			b.WriteString(d.styles.Help.Render("\nesc: cancel"))
		}
		return d.styles.Container.Width(d.dialogWidth()).Render(b.String())
	}

	// Error state
	if d.err != nil {
		b.WriteString("\n")
		b.WriteString(d.styles.Error.Render("Error: " + d.err.Error()))
		b.WriteString("\n\n")
		help := "r: retry"
		if d.canCancel {
			help += " • esc: cancel"
		}
		b.WriteString(d.styles.Help.Render(help))
		return d.styles.Container.Width(d.dialogWidth()).Render(b.String())
	}

	// Empty state
	if len(d.projects) == 0 {
		b.WriteString("\n")
		b.WriteString(d.styles.Error.Render("No projects found."))
		b.WriteString("\n")
		b.WriteString(d.styles.Help.Render("Make sure you have access to GCP projects."))
		b.WriteString("\n")
		help := "r: refresh"
		if d.canCancel {
			help += " • esc: cancel"
		}
		b.WriteString(d.styles.Help.Render(help))
		return d.styles.Container.Width(d.dialogWidth()).Render(b.String())
	}

	// Search input (only show in filter mode or if there's a query)
	if d.filterMode || d.input.Value() != "" {
		b.WriteString(d.input.View())
		b.WriteString("\n\n")
	}

	// Project list
	visibleStart := 0
	visibleEnd := len(d.filtered)
	if visibleEnd > maxVisibleItems {
		// Scroll to keep cursor visible
		if d.cursor >= maxVisibleItems {
			visibleStart = d.cursor - maxVisibleItems + 1
		}
		visibleEnd = visibleStart + maxVisibleItems
		if visibleEnd > len(d.filtered) {
			visibleEnd = len(d.filtered)
			visibleStart = visibleEnd - maxVisibleItems
		}
	}

	for i := visibleStart; i < visibleEnd; i++ {
		p := d.filtered[i]
		isSelected := i == d.cursor
		isCurrent := p.ID == d.currentProjectID

		// Build the line
		var line strings.Builder

		// Cursor indicator
		if isSelected {
			line.WriteString("▸ ")
		} else {
			line.WriteString("  ")
		}

		// Current project checkmark
		if isCurrent {
			line.WriteString(d.styles.CurrentProject.Render("✓ "))
		} else {
			line.WriteString("  ")
		}

		// Project name and ID
		line.WriteString(p.Name)
		if p.Name != p.ID {
			line.WriteString(" ")
			line.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Render("(" + p.ID + ")"))
		}

		// State indicator
		line.WriteString(" ")
		line.WriteString(stateIcon(p.State))

		// Apply styling
		var style lipgloss.Style
		if isSelected {
			style = d.styles.ItemSelected
		} else {
			style = d.styles.Item
		}
		b.WriteString(style.Render(line.String()))
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(d.filtered) > maxVisibleItems {
		scrollInfo := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Render(
			strings.Repeat(" ", 4) + "... and more (scroll with j/k)")
		b.WriteString(scrollInfo)
		b.WriteString("\n")
	}

	// No results from filter
	if len(d.filtered) == 0 && len(d.projects) > 0 {
		b.WriteString(d.styles.Help.Render("  No projects match your search."))
		b.WriteString("\n")
	}

	// Help text
	var help string
	if d.filterMode {
		help = "enter: select • ↑/↓: navigate • esc: clear filter"
	} else {
		help = "enter: select • /: filter • ↑/↓: navigate"
		if d.canCancel {
			help += " • esc: cancel"
		}
	}
	b.WriteString(d.styles.Help.Render(help))

	return d.styles.Container.Width(d.dialogWidth()).Render(b.String())
}

// dialogWidth calculates the dialog width
func (d *ProjectDialog) dialogWidth() int {
	// Find the widest project name
	maxProjectWidth := 0
	for _, p := range d.projects {
		w := len(p.Name)
		if p.Name != p.ID {
			w += len(p.ID) + 3 // " (id)"
		}
		w += 8 // cursor, checkmark, state icon, padding
		if w > maxProjectWidth {
			maxProjectWidth = w
		}
	}

	// Use the widest of: min width, max project width, or container width
	width := maxProjectWidth + 6 // Add padding
	if width < minWidth {
		width = minWidth
	}
	if width > maxWidth {
		width = maxWidth
	}
	if width > d.width-4 && d.width > 0 {
		width = d.width - 4
	}
	return width
}

// stateIcon returns a symbol for project state
func stateIcon(state string) string {
	switch state {
	case "ACTIVE":
		return symbols.StatusRunning()
	case "DELETE_REQUESTED", "DELETE_IN_PROGRESS":
		return symbols.StatusStopped()
	default:
		return symbols.StatusUnknown()
	}
}

// Reset resets the dialog state (for reuse)
func (d *ProjectDialog) Reset() {
	d.input.SetValue("")
	d.cursor = 0
	d.filterMode = false
	d.input.Blur()
	d.filterProjects()
}
