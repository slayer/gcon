// Package links provides a component for navigable links within detail views.
// Links are rendered as a table-like list where users can navigate with j/k
// and press Enter to navigate to the linked resource.
package links

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// Link represents a navigable item within a detail view
type Link struct {
	ID    string // Unique identifier (e.g., disk name)
	Label string // Pre-formatted display text for the row
	Type  string // Link type for message routing (e.g., "disk", "image")
	Data  any    // Optional payload data for navigation
}

// LinkSelectedMsg is sent when user presses Enter on a focused link
type LinkSelectedMsg struct {
	Link Link
}

// Links manages a list of focusable links within content
type Links struct {
	items   []Link
	focused int
	width   int
	keys    keyMap
	styles  Styles
}

// keyMap defines link navigation key bindings
type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
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
			key.WithHelp("enter", "go to"),
		),
	}
}

// New creates a new links component
func New() *Links {
	return &Links{
		items:   []Link{},
		focused: 0,
		keys:    defaultKeyMap(),
		styles:  DefaultStyles(),
	}
}

// SetItems sets the list of navigable links
func (l *Links) SetItems(items []Link) {
	l.items = items
	// Reset focus if out of bounds
	if l.focused >= len(items) {
		l.focused = 0
	}
}

// SetWidth sets the available width for rendering
func (l *Links) SetWidth(width int) {
	l.width = width
}

// Update handles key events for link navigation
// Returns LinkSelectedMsg when Enter is pressed on a focused link
func (l *Links) Update(msg tea.Msg) tea.Cmd {
	if len(l.items) == 0 {
		return nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, l.keys.Up):
		if l.focused > 0 {
			l.focused--
		}
		return nil

	case key.Matches(keyMsg, l.keys.Down):
		if l.focused < len(l.items)-1 {
			l.focused++
		}
		return nil

	case key.Matches(keyMsg, l.keys.Select):
		if l.focused >= 0 && l.focused < len(l.items) {
			link := l.items[l.focused]
			return func() tea.Msg {
				return LinkSelectedMsg{Link: link}
			}
		}
		return nil
	}

	return nil
}

// RenderRow renders a single link row with optional focus highlighting
// The row parameter should be the pre-formatted content (e.g., table row)
// Returns the rendered row with cursor and highlighting if focused
func (l *Links) RenderRow(index int, row string) string {
	if index < 0 || index >= len(l.items) {
		return row
	}

	isFocused := index == l.focused

	// Add cursor indicator
	cursor := "  " // 2 spaces for alignment
	if isFocused {
		cursor = symbols.Cursor() + " "
	}

	// Apply styling
	if isFocused {
		// Full row highlighting with background
		return cursor + l.styles.Focused.Render(row)
	}

	return cursor + l.styles.Normal.Render(row)
}

// RenderHeader renders a table header row (no cursor, muted style)
func (l *Links) RenderHeader(header string) string {
	return "  " + l.styles.Header.Render(header)
}

// RenderDivider renders a divider line
func (l *Links) RenderDivider(width int) string {
	return "  " + l.styles.Divider.Render(strings.Repeat("─", width))
}

// FocusedIndex returns the currently focused link index
func (l *Links) FocusedIndex() int {
	return l.focused
}

// FocusedLink returns the currently focused link, or nil if no items
func (l *Links) FocusedLink() *Link {
	if l.focused >= 0 && l.focused < len(l.items) {
		return &l.items[l.focused]
	}
	return nil
}

// SetFocused sets the focused index
func (l *Links) SetFocused(index int) {
	if index >= 0 && index < len(l.items) {
		l.focused = index
	}
}

// Count returns the number of links
func (l *Links) Count() int {
	return len(l.items)
}

// HasItems returns true if there are navigable links
func (l *Links) HasItems() bool {
	return len(l.items) > 0
}

// HandleKey returns true if the key should be handled by the links component
// This helps parent components decide whether to route keys to links
func HandleKey(msg tea.KeyMsg) bool {
	keyStr := msg.String()

	// j/k and arrow keys for navigation
	if keyStr == "j" || keyStr == "k" || keyStr == "up" || keyStr == "down" {
		return true
	}

	// Enter for selection
	if keyStr == "enter" {
		return true
	}

	return false
}
