// Package tabs provides a reusable tab bar component for detail views.
// Tabs are navigated with Tab/Shift-Tab, h/l, or number keys 1-9.
package tabs

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Tab represents a single tab in the tab bar
type Tab struct {
	ID    string // Unique identifier for the tab
	Label string // Display text
}

// TabChangedMsg is sent when the active tab changes
type TabChangedMsg struct {
	TabID string
	Index int
}

// Tabs is the tab bar component
type Tabs struct {
	tabs   []Tab
	active int
	width  int
	keys   keyMap
	styles Styles
}

// keyMap defines tab navigation key bindings
type keyMap struct {
	Next  key.Binding
	Prev  key.Binding
	Left  key.Binding
	Right key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Next: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		Prev: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev tab"),
		),
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "prev tab"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l/→", "next tab"),
		),
	}
}

// New creates a new tab bar with the given tabs
func New(tabs []Tab) *Tabs {
	return &Tabs{
		tabs:   tabs,
		active: 0,
		keys:   defaultKeyMap(),
		styles: DefaultStyles(),
	}
}

// Init initializes the tabs component (no-op)
func (t *Tabs) Init() tea.Cmd {
	return nil
}

// Update handles input for tab navigation.
// Returns a TabChangedMsg command when the active tab changes.
func (t *Tabs) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	// Handle number keys 1-9 for direct tab selection
	keyStr := keyMsg.String()
	if len(keyStr) == 1 && keyStr[0] >= '1' && keyStr[0] <= '9' {
		idx := int(keyStr[0] - '1') // '1' -> 0, '2' -> 1, etc.
		if idx < len(t.tabs) && idx != t.active {
			t.active = idx
			return t.emitTabChanged()
		}
		return nil
	}

	switch {
	case key.Matches(keyMsg, t.keys.Next), key.Matches(keyMsg, t.keys.Right):
		if t.active < len(t.tabs)-1 {
			t.active++
			return t.emitTabChanged()
		}
		return nil

	case key.Matches(keyMsg, t.keys.Prev), key.Matches(keyMsg, t.keys.Left):
		if t.active > 0 {
			t.active--
			return t.emitTabChanged()
		}
		return nil
	}

	return nil
}

// emitTabChanged returns a command that sends TabChangedMsg
func (t *Tabs) emitTabChanged() tea.Cmd {
	tab := t.tabs[t.active]
	idx := t.active
	return func() tea.Msg {
		return TabChangedMsg{TabID: tab.ID, Index: idx}
	}
}

// View renders the tab bar
func (t *Tabs) View() string {
	var parts []string

	for i, tab := range t.tabs {
		var rendered string
		if i == t.active {
			// Active tab: [Label]
			rendered = t.styles.Active.Render("[" + tab.Label + "]")
		} else {
			// Inactive tab: Label
			rendered = t.styles.Inactive.Render(" " + tab.Label + " ")
		}
		parts = append(parts, rendered)
	}

	// Join tabs with double space separator
	bar := strings.Join(parts, " ")
	return t.styles.Bar.Render(bar)
}

// SetSize sets the available width for the tab bar
func (t *Tabs) SetSize(width int) {
	t.width = width
}

// ActiveTab returns the currently active tab
func (t *Tabs) ActiveTab() Tab {
	if t.active >= 0 && t.active < len(t.tabs) {
		return t.tabs[t.active]
	}
	return Tab{}
}

// ActiveIndex returns the index of the currently active tab
func (t *Tabs) ActiveIndex() int {
	return t.active
}

// SetActive sets the active tab by index
func (t *Tabs) SetActive(idx int) {
	if idx >= 0 && idx < len(t.tabs) {
		t.active = idx
	}
}

// SetActiveByID sets the active tab by its ID
func (t *Tabs) SetActiveByID(id string) {
	for i, tab := range t.tabs {
		if tab.ID == id {
			t.active = i
			return
		}
	}
}

// Count returns the number of tabs
func (t *Tabs) Count() int {
	return len(t.tabs)
}

// HandleKey returns true if the given key should be handled by tabs
// This is useful for parent components to check before routing keys
func HandleKey(msg tea.KeyMsg) bool {
	keyStr := msg.String()

	// Tab/Shift-Tab
	if keyStr == "tab" || keyStr == "shift+tab" {
		return true
	}

	// h/l arrow keys for tab navigation
	if keyStr == "h" || keyStr == "l" || keyStr == "left" || keyStr == "right" {
		return true
	}

	// Number keys 1-9
	if len(keyStr) == 1 && keyStr[0] >= '1' && keyStr[0] <= '9' {
		return true
	}

	return false
}
