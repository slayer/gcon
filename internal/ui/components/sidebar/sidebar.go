package sidebar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/mouse"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// Width constants for content area only. The actual rendered width
// is +1 for the right border (added by Width() method).
const (
	ExpandedWidth  = 26 // Content width for full sidebar
	CollapsedWidth = 4  // Content width for icon-only sidebar
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
	hoverIndex   int                  // Index of item being hovered (-1 if none)
	regionMgr    *mouse.RegionManager // Manages clickable regions for mouse events
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
		hoverIndex:   -1,
		regionMgr:    mouse.NewRegionManager(),
	}
}

// Init returns initial command (none for sidebar)
func (s *Sidebar) Init() tea.Cmd {
	return nil
}

// handleMouseEvent processes mouse interactions with the sidebar
// This is kept for backward compatibility and handles wheel scroll and hover.
// Click handling is now done via the Clickable interface (HandleRegionClick).
func (s *Sidebar) handleMouseEvent(msg tea.MouseMsg) tea.Cmd {
	switch msg.Action {
	case tea.MouseActionRelease:
		// Handle wheel scroll
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			s.moveUp()
		case tea.MouseButtonWheelDown:
			s.moveDown()
		}

	case tea.MouseActionMotion:
		// Track hover state
		// Calculate Y offset for header and divider
		yOffset := 2
		if len(s.path) > 0 {
			yOffset += 2 // Add "Back" link
		}

		if msg.Y >= yOffset {
			itemY := msg.Y - yOffset
			if itemY >= 0 && itemY < len(s.currentItems) {
				s.hoverIndex = itemY
			} else {
				s.hoverIndex = -1
			}
		}
	}

	return nil
}

// UpdateRegions recalculates clickable regions based on current state.
// Implements the Clickable interface.
func (s *Sidebar) UpdateRegions(offsetX, offsetY int) {
	s.regionMgr.Clear()

	// Calculate Y offset for header and divider
	// Header: 1 line
	// Divider: 1 line
	yOffset := offsetY + 2

	// Add "Back" link region if present (in submenu)
	if len(s.path) > 0 {
		s.regionMgr.Add(
			"back",
			mouse.Rect{
				X:      offsetX,
				Y:      yOffset,
				Width:  s.Width(),
				Height: 1,
			},
			nil,
		)
		// "< Back" line: 1 line
		// Empty separator: 1 line
		yOffset += 2
	}

	// Add region for each menu item
	for i := range s.currentItems {
		s.regionMgr.Add(
			fmt.Sprintf("item-%d", i),
			mouse.Rect{
				X:      offsetX,
				Y:      yOffset + i,
				Width:  s.Width(),
				Height: 1,
			},
			i, // Item index as metadata
		)
	}
}

// GetRegions returns current clickable regions.
// Implements the Clickable interface.
func (s *Sidebar) GetRegions() []mouse.Region {
	return s.regionMgr.GetRegions()
}

// HandleRegionClick processes a click on a specific region.
// Implements the Clickable interface.
func (s *Sidebar) HandleRegionClick(regionID string) tea.Cmd {
	// Handle back button
	if regionID == "back" {
		s.goBack()
		return nil
	}

	// Parse region ID to get item index
	var itemIdx int
	if _, err := fmt.Sscanf(regionID, "item-%d", &itemIdx); err != nil {
		return nil
	}

	if itemIdx < 0 || itemIdx >= len(s.currentItems) {
		return nil
	}

	// Update cursor and select item
	s.cursor = itemIdx
	return s.selectItem()
}

// Update handles keyboard and mouse input when sidebar is focused
func (s *Sidebar) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return s.handleMouseEvent(msg)

	case tea.KeyMsg:
		if !s.focused {
			return nil
		}

		// Handle number shortcuts (1-9)
		if num := s.getNumberKey(msg); num > 0 && num <= len(s.currentItems) {
			s.cursor = num - 1
			return s.selectItem()
		}

		// Handle letter hotkeys (case-sensitive)
		if idx := s.findItemByHotkey(msg.String()); idx >= 0 {
			s.cursor = idx
			return s.selectItem()
		}

		switch {
		case key.Matches(msg, s.keys.Up):
			s.moveUp()
		case key.Matches(msg, s.keys.Down):
			s.moveDown()
		case key.Matches(msg, s.keys.Select):
			return s.selectItem()
		case key.Matches(msg, s.keys.Back):
			s.goBack()
		}
	}
	return nil
}

// findItemByHotkey returns the index of the item matching the hotkey, or -1
func (s *Sidebar) findItemByHotkey(keyStr string) int {
	if len(keyStr) != 1 {
		return -1
	}
	pressedKey := rune(keyStr[0])
	for i, item := range s.currentItems {
		if item.Hotkey == pressedKey {
			return i
		}
	}
	return -1
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
		backLabel := fmt.Sprintf(" %s Back", symbols.Back())
		if s.collapsed {
			backLabel = symbols.Back()
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

	// Height pads short content, MaxHeight truncates tall content
	container := styles.Container.
		Width(width).
		Height(s.height).
		MaxHeight(s.height)

	return container.Render(content)
}

// renderHeader renders the sidebar header with hamburger and breadcrumb
func (s *Sidebar) renderHeader(styles Styles) string {
	if s.collapsed {
		return styles.Header.Render(symbols.Hamburger())
	}

	// Build breadcrumb: ☰ Menu > Category
	header := symbols.Hamburger() + " Menu"
	if len(s.path) > 0 {
		// Find category label
		for _, item := range s.menu {
			if item.ID == s.path[0] {
				header = symbols.Hamburger() + " " + item.Label
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
	return styles.Divider.Render(strings.Repeat(symbols.Divider(), width))
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
			label = symbols.Active()
		} else {
			label = item.Icon
		}
	} else {
		// Build full label with consistent spacing:
		// [cursor 2ch] [active 2ch] [icon 2ch] [label] [shortcut]
		cursor := "  " // 2 chars: no cursor
		if isSelected {
			cursor = symbols.Cursor() + " " // 2 chars: cursor + space
		}

		active := "  " // 2 chars: no active indicator
		if isActive {
			active = symbols.Active() + " " // 2 chars: dot + space
		}

		icon := item.Icon + " " // icon + space

		// Highlight hotkey in label (only when focused and not selected)
		displayLabel := s.highlightHotkey(item.Label, item.Hotkey, styles, isSelected)

		if item.Type == MenuItemCategory {
			// Category: cursor + icon + label + expand arrow (no active indicator)
			label = fmt.Sprintf("%s%s%s %s", cursor, icon, displayLabel, symbols.Expand())
		} else {
			// Leaf: cursor + active + icon + label + shortcut
			label = fmt.Sprintf("%s%s%s%s", cursor, active, icon, displayLabel)
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

// highlightHotkey returns the label with the hotkey character highlighted
func (s *Sidebar) highlightHotkey(label string, hotkey rune, styles Styles, isSelected bool) string {
	if hotkey == 0 || !s.focused || isSelected {
		// No hotkey, unfocused, or selected (bg color makes it hard to see)
		return label
	}

	// Find the hotkey character in the label (case-sensitive match)
	hotkeyStr := string(hotkey)
	idx := strings.Index(label, hotkeyStr)
	if idx == -1 {
		// Hotkey char not found in label, return as-is
		return label
	}

	// Split and highlight
	before := label[:idx]
	highlighted := styles.Hotkey.Render(hotkeyStr)
	after := label[idx+len(hotkeyStr):]

	return before + highlighted + after
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

// Width returns the current sidebar width including the border
func (s *Sidebar) Width() int {
	// Add 1 for the right border that the container style adds
	if s.collapsed {
		return CollapsedWidth + 1
	}
	return ExpandedWidth + 1
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
