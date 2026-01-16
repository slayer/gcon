// Package actionmenu provides a popup action menu component for views.
// Triggered by '.' key, it shows available actions with hotkeys for quick selection.
package actionmenu

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// Action represents a single menu action
type Action struct {
	Key         rune   // Hotkey (e.g., 's' for Start)
	Label       string // Display text (e.g., "Start")
	Description string // Optional description
	Enabled     bool   // Grayed out if false
	Dangerous   bool   // Red styling if true (for destructive actions)
}

// ActionSelectedMsg is sent when an action is selected
type ActionSelectedMsg struct {
	Key rune
}

// ActionMenuClosedMsg is sent when the menu is closed without selection
type ActionMenuClosedMsg struct{}

// ActionMenu is the popup menu component
type ActionMenu struct {
	title   string
	actions []Action
	cursor  int
	width   int
	keys    keyMap
	styles  Styles
}

// keyMap defines action menu key bindings
type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Close  key.Binding
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
		Close: key.NewBinding(
			key.WithKeys("esc", "q", "."),
			key.WithHelp("esc", "close"),
		),
	}
}

// New creates a new action menu
func New(title string, actions []Action) *ActionMenu {
	// Calculate width based on longest action label
	maxWidth := len(title) + 6
	for _, a := range actions {
		// Format: "  x  Label  " = 5 + len(label)
		w := 5 + len(a.Label)
		if w > maxWidth {
			maxWidth = w
		}
	}
	// Add padding for border and extra breathing room
	maxWidth += 10

	// Find first enabled action for cursor
	firstEnabled := 0
	for i, action := range actions {
		if action.Enabled {
			firstEnabled = i
			break
		}
	}

	return &ActionMenu{
		title:   title,
		actions: actions,
		cursor:  firstEnabled,
		width:   maxWidth,
		keys:    defaultKeyMap(),
		styles:  DefaultStyles(),
	}
}

// Init initializes the menu (no-op)
func (m *ActionMenu) Init() tea.Cmd {
	return nil
}

// Update handles input for the action menu
func (m *ActionMenu) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	// Direct hotkey selection (single character)
	keyStr := keyMsg.String()
	if len(keyStr) == 1 {
		pressedKey := rune(keyStr[0])
		for _, action := range m.actions {
			if action.Enabled && action.Key == pressedKey {
				return m.selectAction(action.Key)
			}
		}
	}

	switch {
	case key.Matches(keyMsg, m.keys.Up):
		m.moveUp()
		return nil

	case key.Matches(keyMsg, m.keys.Down):
		m.moveDown()
		return nil

	case key.Matches(keyMsg, m.keys.Select):
		if m.cursor < len(m.actions) && m.actions[m.cursor].Enabled {
			return m.selectAction(m.actions[m.cursor].Key)
		}
		return nil

	case key.Matches(keyMsg, m.keys.Close):
		return func() tea.Msg { return ActionMenuClosedMsg{} }
	}

	return nil
}

// selectAction returns a command that emits ActionSelectedMsg
func (m *ActionMenu) selectAction(key rune) tea.Cmd {
	return func() tea.Msg {
		return ActionSelectedMsg{Key: key}
	}
}

// moveUp moves cursor up, skipping disabled items
func (m *ActionMenu) moveUp() {
	for i := m.cursor - 1; i >= 0; i-- {
		if m.actions[i].Enabled {
			m.cursor = i
			return
		}
	}
}

// moveDown moves cursor down, skipping disabled items
func (m *ActionMenu) moveDown() {
	for i := m.cursor + 1; i < len(m.actions); i++ {
		if m.actions[i].Enabled {
			m.cursor = i
			return
		}
	}
}

// View renders the action menu
func (m *ActionMenu) View() string {
	var b strings.Builder

	// Title
	b.WriteString(m.styles.Title.Render(m.title))
	b.WriteString("\n")

	// Actions
	for i, action := range m.actions {
		line := m.renderAction(action, i == m.cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Divider
	dividerWidth := m.width - 4 // Account for border padding
	if dividerWidth < 1 {
		dividerWidth = 1
	}
	b.WriteString(m.styles.Divider.Render(strings.Repeat("─", dividerWidth)))
	b.WriteString("\n")

	// Help
	b.WriteString(m.styles.Help.Render("j/k:nav  enter:sel  esc:close"))

	// Container
	return m.styles.Container.Width(m.width).Render(b.String())
}

// renderAction renders a single action line
func (m *ActionMenu) renderAction(action Action, selected bool) string {
	// Format: [cursor] [key] [label]
	cursor := "  "
	if selected && action.Enabled {
		cursor = symbols.Cursor() + " "
	}

	keyStr := string(action.Key)

	var line string
	if action.Enabled {
		// Determine style based on dangerous flag
		var keyStyle, labelStyle lipgloss.Style
		if action.Dangerous {
			keyStyle = m.styles.KeyDangerous
			labelStyle = m.styles.LabelDangerous
		} else {
			keyStyle = m.styles.Key
			labelStyle = m.styles.Label
		}

		if selected {
			keyStyle = m.styles.KeySelected
			labelStyle = m.styles.LabelSelected
		}

		line = cursor + keyStyle.Render(keyStr) + "  " + labelStyle.Render(action.Label)
	} else {
		// Disabled action
		line = cursor + m.styles.KeyDisabled.Render(keyStr) + "  " + m.styles.LabelDisabled.Render(action.Label)
	}

	return line
}

// SetCursor sets the cursor position (for testing)
func (m *ActionMenu) SetCursor(pos int) {
	if pos >= 0 && pos < len(m.actions) {
		m.cursor = pos
	}
}

// GetCursor returns the current cursor position
func (m *ActionMenu) GetCursor() int {
	return m.cursor
}

// GetActions returns the actions list
func (m *ActionMenu) GetActions() []Action {
	return m.actions
}
