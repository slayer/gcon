package sidebar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Width constants
const (
	ExpandedWidth  = 26 // Full sidebar width with text
	CollapsedWidth = 4  // Icon-only width (hamburger + padding)
)

// Visual characters for better UI
const (
	IconHamburger = "☰" // Hamburger menu icon
	IconBack      = "◀" // Back arrow
	IconExpand    = "▸" // Expand/drill-down indicator
	IconCursor    = "▶" // Selection cursor
	IconActive    = "●" // Active item indicator
	DividerChar   = "─" // Horizontal divider
)

// KeyMap defines sidebar-specific key bindings
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Back    key.Binding
	Number1 key.Binding
	Number2 key.Binding
	Number3 key.Binding
	Number4 key.Binding
	Number5 key.Binding
	Number6 key.Binding
	Number7 key.Binding
	Number8 key.Binding
	Number9 key.Binding
}

// DefaultKeyMap returns the default sidebar key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Select:  key.NewBinding(key.WithKeys("enter", "l", "right"), key.WithHelp("enter/→", "select")),
		Back:    key.NewBinding(key.WithKeys("h", "left", "backspace"), key.WithHelp("←/h", "back")),
		Number1: key.NewBinding(key.WithKeys("1")),
		Number2: key.NewBinding(key.WithKeys("2")),
		Number3: key.NewBinding(key.WithKeys("3")),
		Number4: key.NewBinding(key.WithKeys("4")),
		Number5: key.NewBinding(key.WithKeys("5")),
		Number6: key.NewBinding(key.WithKeys("6")),
		Number7: key.NewBinding(key.WithKeys("7")),
		Number8: key.NewBinding(key.WithKeys("8")),
		Number9: key.NewBinding(key.WithKeys("9")),
	}
}

// NavigateMsg is sent when a leaf menu item is selected
type NavigateMsg struct {
	ViewType ViewType
	ItemID   string
}

// Sidebar is the navigation component
type Sidebar struct {
	menu         []MenuItem // Root menu items
	currentItems []MenuItem // Currently displayed items (changes during drill-down)
	path         []string   // Navigation path (stack of category IDs)
	cursor       int        // Selected item index
	collapsed    bool       // Icon-only mode
	focused      bool       // Has keyboard focus
	width        int        // Current width
	height       int        // Available height
	activeView   ViewType   // Currently active view (for highlighting)
	styles       Styles
	keys         KeyMap
}

// New creates a new sidebar with the default GCP menu
func New() *Sidebar {
	menu := DefaultMenu()
	return &Sidebar{
		menu:         menu,
		currentItems: menu,
		path:         []string{},
		cursor:       0,
		collapsed:    false,
		focused:      false,
		width:        ExpandedWidth,
		styles:       DefaultStyles(),
		keys:         DefaultKeyMap(),
		activeView:   ViewInstances,
	}
}

// Init returns initial command (none for sidebar)
func (s *Sidebar) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input when sidebar is focused
func (s *Sidebar) Update(msg tea.Msg) tea.Cmd {
	if !s.focused {
		return nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	// Handle number shortcuts (1-9)
	if num := s.getNumberKey(keyMsg); num > 0 && num <= len(s.currentItems) {
		s.cursor = num - 1
		return s.selectItem()
	}

	switch {
	case key.Matches(keyMsg, s.keys.Up):
		s.moveUp()
	case key.Matches(keyMsg, s.keys.Down):
		s.moveDown()
	case key.Matches(keyMsg, s.keys.Select):
		return s.selectItem()
	case key.Matches(keyMsg, s.keys.Back):
		s.goBack()
	}
	return nil
}

// getNumberKey returns the number (1-9) if a number key was pressed, 0 otherwise
func (s *Sidebar) getNumberKey(msg tea.KeyMsg) int {
	switch {
	case key.Matches(msg, s.keys.Number1):
		return 1
	case key.Matches(msg, s.keys.Number2):
		return 2
	case key.Matches(msg, s.keys.Number3):
		return 3
	case key.Matches(msg, s.keys.Number4):
		return 4
	case key.Matches(msg, s.keys.Number5):
		return 5
	case key.Matches(msg, s.keys.Number6):
		return 6
	case key.Matches(msg, s.keys.Number7):
		return 7
	case key.Matches(msg, s.keys.Number8):
		return 8
	case key.Matches(msg, s.keys.Number9):
		return 9
	}
	return 0
}

// View renders the sidebar
func (s *Sidebar) View() string {
	styles := s.styles
	if s.focused {
		styles = styles.WithFocus()
	} else {
		styles = styles.Dimmed()
	}

	var lines []string

	// Header with hamburger icon and breadcrumb
	lines = append(lines, s.renderHeader(styles))
	lines = append(lines, s.renderDivider(styles))

	// Show "< Back" when drilled down
	if len(s.path) > 0 {
		backLabel := fmt.Sprintf(" %s Back", IconBack)
		if s.collapsed {
			backLabel = IconBack
		}
		lines = append(lines, styles.BackItem.Render(backLabel))
		lines = append(lines, "") // Empty line separator
	}

	// Render menu items with number shortcuts
	for i, item := range s.currentItems {
		line := s.renderItem(item, i, styles)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	// Apply container styling with fixed width
	width := s.width
	if s.collapsed {
		width = CollapsedWidth
	}

	container := styles.Container.
		Width(width).
		Height(s.height)

	return container.Render(content)
}

// renderHeader renders the sidebar header with hamburger and breadcrumb
func (s *Sidebar) renderHeader(styles Styles) string {
	if s.collapsed {
		return styles.Header.Render(IconHamburger)
	}

	// Build breadcrumb: ☰ Menu > Category
	header := IconHamburger + " Menu"
	if len(s.path) > 0 {
		// Find category label
		for _, item := range s.menu {
			if item.ID == s.path[0] {
				header = IconHamburger + " " + item.Label
				break
			}
		}
	}

	return styles.Header.Render(header)
}

// renderDivider renders a horizontal divider line
func (s *Sidebar) renderDivider(styles Styles) string {
	width := s.width - 2 // Account for padding
	if s.collapsed {
		width = CollapsedWidth - 2
	}
	if width < 1 {
		width = 1
	}
	return styles.Divider.Render(strings.Repeat(DividerChar, width))
}

// renderItem renders a single menu item
func (s *Sidebar) renderItem(item MenuItem, index int, styles Styles) string {
	isSelected := index == s.cursor && s.focused
	isActive := item.Type == MenuItemLeaf && item.ViewType == s.activeView

	var label string
	shortcut := ""
	if index < 9 {
		shortcut = fmt.Sprintf("[%d]", index+1)
	}

	if s.collapsed {
		// In collapsed mode, show icon or active indicator
		if isActive {
			label = IconActive
		} else {
			label = item.Icon
		}
	} else {
		// Build full label with consistent spacing:
		// [cursor 2ch] [active 2ch] [icon 2ch] [label] [shortcut]
		cursor := "  " // 2 chars: no cursor
		if isSelected {
			cursor = IconCursor + " " // 2 chars: cursor + space
		}

		active := "  " // 2 chars: no active indicator
		if isActive {
			active = IconActive + " " // 2 chars: dot + space
		}

		icon := item.Icon + " " // icon + space

		if item.Type == MenuItemCategory {
			// Category: cursor + icon + label + expand arrow (no active indicator)
			label = fmt.Sprintf("%s%s%s %s", cursor, icon, item.Label, IconExpand)
		} else {
			// Leaf: cursor + active + icon + label + shortcut
			label = fmt.Sprintf("%s%s%s%s", cursor, active, icon, item.Label)
			if shortcut != "" && s.focused {
				label += " " + shortcut
			}
		}
	}

	// Apply appropriate style
	var style lipgloss.Style
	switch {
	case isSelected:
		style = styles.ItemSelected
	case isActive:
		style = styles.ItemActive
	case item.Type == MenuItemCategory:
		style = styles.Category
	default:
		style = styles.Item
	}

	return style.Render(label)
}

// moveUp moves cursor up
func (s *Sidebar) moveUp() {
	if s.cursor > 0 {
		s.cursor--
	}
}

// moveDown moves cursor down
func (s *Sidebar) moveDown() {
	if s.cursor < len(s.currentItems)-1 {
		s.cursor++
	}
}

// selectItem handles selection of current item
func (s *Sidebar) selectItem() tea.Cmd {
	if s.cursor >= len(s.currentItems) {
		return nil
	}

	item := s.currentItems[s.cursor]

	if item.Type == MenuItemCategory {
		// Drill down into category
		s.path = append(s.path, item.ID)
		s.currentItems = item.Children
		s.cursor = 0
		return nil
	}

	// Leaf item - emit navigation message
	return func() tea.Msg {
		return NavigateMsg{
			ViewType: item.ViewType,
			ItemID:   item.ID,
		}
	}
}

// goBack navigates up one level
func (s *Sidebar) goBack() {
	if len(s.path) == 0 {
		return
	}

	// Pop the last path element
	s.path = s.path[:len(s.path)-1]

	// Rebuild currentItems from path
	s.currentItems = s.menu
	for _, id := range s.path {
		for _, item := range s.currentItems {
			if item.ID == id {
				s.currentItems = item.Children
				break
			}
		}
	}
	s.cursor = 0
}

// SetSize updates the available height
func (s *Sidebar) SetSize(height int) {
	s.height = height
}

// SetFocused sets the focus state
func (s *Sidebar) SetFocused(focused bool) {
	s.focused = focused
}

// IsFocused returns whether the sidebar has focus
func (s *Sidebar) IsFocused() bool {
	return s.focused
}

// Toggle switches between expanded and collapsed mode
func (s *Sidebar) Toggle() {
	s.collapsed = !s.collapsed
	if s.collapsed {
		s.width = CollapsedWidth
	} else {
		s.width = ExpandedWidth
	}
}

// IsCollapsed returns whether sidebar is in collapsed mode
func (s *Sidebar) IsCollapsed() bool {
	return s.collapsed
}

// Width returns the current sidebar width
func (s *Sidebar) Width() int {
	if s.collapsed {
		return CollapsedWidth
	}
	return ExpandedWidth
}

// SetActiveView sets the currently active view for highlighting
func (s *Sidebar) SetActiveView(vt ViewType) {
	s.activeView = vt
}

// GetPath returns the current navigation path
func (s *Sidebar) GetPath() []string {
	return s.path
}

// GetCurrentCategory returns the current category label (for breadcrumb)
func (s *Sidebar) GetCurrentCategory() string {
	if len(s.path) == 0 {
		return ""
	}

	// Find the category by ID
	for _, item := range s.menu {
		if item.ID == s.path[0] {
			return item.Label
		}
	}
	return ""
}
