// Package layout provides tile-based layout management using teatile.
// It ensures consistent dimension handling between sidebar and content,
// preventing rendering glitches when content overflows.
package layout

import (
	"github.com/bevicted/teatile"
	"github.com/charmbracelet/lipgloss"
)

const (
	// HeaderHeight is the fixed height for the header tile
	HeaderHeight = 3
	// FooterHeight is the fixed height for the footer tile
	FooterHeight = 1
)

// Layout manages the tile hierarchy for the application
type Layout struct {
	root    *teatile.Tile
	header  *teatile.Tile
	content *teatile.Tile
	sidebar *teatile.Tile
	main    *teatile.Tile
	footer  *teatile.Tile

	// Sidebar state
	sidebarWidth  int
	sidebarActive bool
}

// New creates a new tile-based layout manager
func New() *Layout {
	l := &Layout{
		root:         teatile.New(),
		sidebarWidth: 0,
	}

	// Create vertical structure: header -> content -> footer
	l.header = l.root.NewSubtile().WithSize(0, HeaderHeight)
	l.content = l.root.NewSubtile() // Flexible height
	l.footer = l.root.NewSubtile().WithSize(0, FooterHeight)
	teatile.JoinVertical(l.header, l.content, l.footer)

	// Create horizontal structure inside content: sidebar -> main
	l.sidebar = l.content.NewSubtile().WithSize(0, 0) // Width set dynamically
	l.main = l.content.NewSubtile()                   // Flexible width
	teatile.JoinHorizontal(l.sidebar, l.main)

	return l
}

// SetSize updates the root tile dimensions and recalculates the layout
func (l *Layout) SetSize(width, height int) {
	l.root.WithSize(width, height)
	l.updateSidebarWidth()
	l.root.Recalculate()
}

// SetSidebarActive controls whether the sidebar is visible
func (l *Layout) SetSidebarActive(active bool) {
	l.sidebarActive = active
	l.updateSidebarWidth()
	l.root.Recalculate()
}

// SetSidebarWidth sets the sidebar width (use 0 to hide)
func (l *Layout) SetSidebarWidth(width int) {
	l.sidebarWidth = width
	l.updateSidebarWidth()
	l.root.Recalculate()
}

// updateSidebarWidth applies the current sidebar width to the tile
func (l *Layout) updateSidebarWidth() {
	if l.sidebarActive && l.sidebarWidth > 0 {
		l.sidebar.WithSize(l.sidebarWidth, 0)
	} else {
		l.sidebar.WithSize(0, 0)
	}
}

// HeaderSize returns the header tile dimensions
func (l *Layout) HeaderSize() (width, height int) {
	return l.header.GetSize()
}

// ContentSize returns the content tile dimensions
func (l *Layout) ContentSize() (width, height int) {
	return l.content.GetSize()
}

// SidebarSize returns the sidebar tile dimensions
func (l *Layout) SidebarSize() (width, height int) {
	return l.sidebar.GetSize()
}

// MainSize returns the main content tile dimensions
func (l *Layout) MainSize() (width, height int) {
	return l.main.GetSize()
}

// FooterSize returns the footer tile dimensions
func (l *Layout) FooterSize() (width, height int) {
	return l.footer.GetSize()
}

// ContentWidth returns the width available for main content
// When sidebar is inactive, returns full content width.
// When sidebar is active, returns content width minus sidebar width.
func (l *Layout) ContentWidth() int {
	cW, _ := l.content.GetSize()
	if !l.sidebarActive || l.sidebarWidth == 0 {
		return cW
	}
	return cW - l.sidebarWidth
}

// ContentHeight returns the height available for content (excluding header/footer)
func (l *Layout) ContentHeight() int {
	_, cH := l.content.GetSize()
	return cH
}

// IsSidebarActive returns whether the sidebar is currently visible
func (l *Layout) IsSidebarActive() bool {
	return l.sidebarActive && l.sidebarWidth > 0
}

// HeaderStyle returns a lipgloss style with header dimensions applied
func (l *Layout) HeaderStyle(style lipgloss.Style) lipgloss.Style { //nolint:gocritic // Style param is acceptable
	return teatile.SetStyleSize(style, l.header)
}

// ContentStyle returns a lipgloss style with content dimensions applied
func (l *Layout) ContentStyle(style lipgloss.Style) lipgloss.Style { //nolint:gocritic // Style param is acceptable
	return teatile.SetStyleSize(style, l.content)
}

// SidebarStyle returns a lipgloss style with sidebar dimensions applied
func (l *Layout) SidebarStyle(style lipgloss.Style) lipgloss.Style { //nolint:gocritic // Style param is acceptable
	return teatile.SetStyleSize(style, l.sidebar)
}

// MainStyle returns a lipgloss style with main content dimensions applied
func (l *Layout) MainStyle(style lipgloss.Style) lipgloss.Style {
	return teatile.SetStyleSize(style, l.main)
}

// FooterStyle returns a lipgloss style with footer dimensions applied
func (l *Layout) FooterStyle(style lipgloss.Style) lipgloss.Style {
	return teatile.SetStyleSize(style, l.footer)
}
