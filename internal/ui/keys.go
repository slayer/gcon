package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all application key bindings
type KeyMap struct {
	Up              key.Binding
	Down            key.Binding
	Left            key.Binding
	Right           key.Binding
	Select          key.Binding
	Back            key.Binding
	Quit            key.Binding
	Help            key.Binding
	Refresh         key.Binding
	Search          key.Binding
	Tab             key.Binding
	ShiftTab        key.Binding
	SelectSidebar   key.Binding // '[' - focus sidebar
	SelectContent   key.Binding // ']' - focus content
	ToggleSidebar   key.Binding // '{' - show/hide sidebar
	ActionMenu      key.Binding
	CommandPalette  key.Binding
	CancelUsageScan key.Binding // 'ctrl+x' - cancel active bucket usage scan
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next region"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("S-tab", "prev region"),
		),
		SelectSidebar: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "sidebar"),
		),
		SelectContent: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "content"),
		),
		ToggleSidebar: key.NewBinding(
			key.WithKeys("{"),
			key.WithHelp("{", "pin sidebar"),
		),
		ActionMenu: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "actions"),
		),
		CommandPalette: key.NewBinding(
			key.WithKeys(":", "ctrl+k"),
			key.WithHelp(":/^k", "command palette"),
		),
		CancelUsageScan: key.NewBinding(
			key.WithKeys("ctrl+x"),
			key.WithHelp("ctrl+x", "cancel all scans"),
		),
	}
}

// ShortHelp returns key bindings for the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Back, k.CommandPalette, k.Help}
}

// FullHelp returns key bindings for the expanded help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Select, k.Back, k.Refresh},
		{k.Search, k.Tab, k.ShiftTab},
		{k.SelectSidebar, k.SelectContent, k.ToggleSidebar, k.CommandPalette, k.Help, k.Quit},
	}
}
