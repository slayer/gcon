package commandpalette

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	maxVisibleItems = 10
	minWidth        = 40
)

// paletteKeyMap defines key bindings for the command palette
type paletteKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Cancel key.Binding
	CtrlN  key.Binding
	CtrlP  key.Binding
}

func defaultKeyMap() paletteKeyMap {
	return paletteKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
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

// CommandPalette is a fuzzy-searchable command selector
type CommandPalette struct {
	input           textinput.Model
	commands        []Command // All available commands
	filtered        []Command // Commands matching current query
	cursor          int       // Currently selected index
	width           int
	height          int
	styles          Styles
	keys            paletteKeyMap
	showPrefix      bool // Show ":" prefix (when opened with :)
	projectSelected bool // Whether a project is selected (enables nav commands)
}

// New creates a new command palette
func New() *CommandPalette {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40

	commands := DefaultCommands()

	return &CommandPalette{
		input:           ti,
		commands:        commands,
		filtered:        commands, // Initially show all
		cursor:          0,
		width:           60,
		height:          15,
		styles:          DefaultStyles(),
		keys:            defaultKeyMap(),
		showPrefix:      true,
		projectSelected: false,
	}
}

// SetSize updates the palette dimensions
func (p *CommandPalette) SetSize(width, height int) {
	p.width = width
	p.height = height
	// Adjust input width to fit container
	inputWidth := width - 6 // Account for borders and padding
	if inputWidth < minWidth {
		inputWidth = minWidth
	}
	p.input.Width = inputWidth
}

// SetCommands sets the available commands
func (p *CommandPalette) SetCommands(commands []Command) {
	p.commands = commands
	p.filtered = Filter(commands, p.input.Value())
}

// SetProjectSelected sets whether a project is selected (enables navigation commands)
func (p *CommandPalette) SetProjectSelected(selected bool) {
	p.projectSelected = selected
	p.updateEnabledState()
}

// SetShowPrefix sets whether to show ":" prefix
func (p *CommandPalette) SetShowPrefix(show bool) {
	p.showPrefix = show
}

// updateEnabledState updates enabled state of commands based on project selection
func (p *CommandPalette) updateEnabledState() {
	for i := range p.commands {
		if p.commands[i].Type == CommandTypeNavigation {
			p.commands[i].Enabled = p.projectSelected
		}
	}
	// Re-filter to update the filtered list
	p.filtered = Filter(p.commands, p.input.Value())
}

// Reset resets the palette to initial state
func (p *CommandPalette) Reset() {
	p.input.SetValue("")
	p.cursor = 0
	p.filtered = Filter(p.commands, "")
}

// Init initializes the command palette
func (p *CommandPalette) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (p *CommandPalette) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, p.keys.Cancel):
		return func() tea.Msg { return CommandCancelMsg{} }

	case key.Matches(keyMsg, p.keys.Select):
		if len(p.filtered) > 0 && p.cursor < len(p.filtered) {
			selected := p.filtered[p.cursor]
			if selected.Enabled {
				return func() tea.Msg { return CommandSelectedMsg{Command: selected} }
			}
		}
		return nil

	case key.Matches(keyMsg, p.keys.Up), key.Matches(keyMsg, p.keys.CtrlP):
		if p.cursor > 0 {
			p.cursor--
		}
		return nil

	case key.Matches(keyMsg, p.keys.Down), key.Matches(keyMsg, p.keys.CtrlN):
		if p.cursor < len(p.filtered)-1 {
			p.cursor++
		}
		return nil

	default:
		// Pass to text input for typing
		prevValue := p.input.Value()
		p.input, cmd = p.input.Update(keyMsg)

		// If input changed, re-filter
		if p.input.Value() != prevValue {
			p.filtered = Filter(p.commands, p.input.Value())
			p.cursor = 0 // Reset cursor on filter change
		}
		return cmd
	}
}

// View renders the command palette
func (p *CommandPalette) View() string {
	var b strings.Builder

	// Input line with optional prefix
	inputLine := p.renderInputLine()
	b.WriteString(inputLine)
	b.WriteString("\n")

	// Separator
	separator := p.styles.Separator.Render(strings.Repeat("─", p.width-4))
	b.WriteString(separator)
	b.WriteString("\n")

	// Command list
	if len(p.filtered) == 0 {
		b.WriteString(p.styles.NoResults.Render("No matching commands"))
	} else {
		b.WriteString(p.renderList())
	}

	// Wrap in container
	content := b.String()
	container := p.styles.Container.
		Width(p.width).
		Render(content)

	return container
}

// renderInputLine renders the input with optional prefix
func (p *CommandPalette) renderInputLine() string {
	var prefix string
	if p.showPrefix {
		prefix = p.styles.InputPrefix.Render(":")
	}
	return prefix + p.input.View()
}

// renderList renders the filtered command list
func (p *CommandPalette) renderList() string {
	var b strings.Builder

	// Determine visible range
	visibleCount := maxVisibleItems
	if len(p.filtered) < visibleCount {
		visibleCount = len(p.filtered)
	}

	// Calculate start index for scrolling
	start := 0
	if p.cursor >= visibleCount {
		start = p.cursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(p.filtered) {
		end = len(p.filtered)
		start = end - visibleCount
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		cmd := p.filtered[i]
		isSelected := i == p.cursor

		line := p.renderItem(cmd, isSelected)
		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderItem renders a single command item
func (p *CommandPalette) renderItem(cmd Command, selected bool) string { //nolint:gocritic // Command is small enough
	// Cursor indicator
	cursor := "  "
	if selected {
		cursor = p.styles.Cursor.Render("▸ ")
	}

	// Icon
	var icon string
	if cmd.Enabled {
		icon = p.styles.Icon.Render(cmd.Icon)
	} else {
		icon = p.styles.IconDisabled.Render(cmd.Icon)
	}

	// Label - use switch for clarity
	var label string
	switch {
	case selected && cmd.Enabled:
		label = p.styles.ItemSelected.Render(cmd.Label)
	case !cmd.Enabled:
		label = p.styles.ItemDisabled.Render(cmd.Label)
	default:
		label = p.styles.Item.Render(cmd.Label)
	}

	return cursor + icon + label
}

// CenterInScreen returns the styled palette centered on screen
func (p *CommandPalette) CenterInScreen(screenWidth, screenHeight int) string {
	// Calculate centered position
	paletteWidth := p.width + 2 // Account for border
	leftPad := (screenWidth - paletteWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	// Calculate vertical position (roughly 1/3 from top)
	topPad := screenHeight / 4
	if topPad < 2 {
		topPad = 2
	}

	view := p.View()

	// Create centered style
	centered := lipgloss.NewStyle().
		MarginLeft(leftPad).
		MarginTop(topPad)

	return centered.Render(view)
}
