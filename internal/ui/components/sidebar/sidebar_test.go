package sidebar

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	s := New()

	assert.NotNil(t, s)
	assert.Len(t, s.menu, 3, "should have 3 top-level categories")
	assert.Len(t, s.currentItems, 3, "currentItems should start with root menu")
	assert.Empty(t, s.path, "path should be empty initially")
	assert.Equal(t, 0, s.cursor, "cursor should start at 0")
	assert.True(t, s.collapsed, "should start collapsed in auto-hide mode")
	assert.False(t, s.focused, "should not be focused initially")
	assert.Equal(t, SidebarModeAutoHide, s.mode, "default mode should be auto-hide")
}

func TestMoveUpDown(t *testing.T) {
	s := New()
	s.SetFocused(true)

	// Initial position
	assert.Equal(t, 0, s.cursor)

	// Move down
	s.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, s.cursor)

	s.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, s.cursor)

	// Can't go past last item
	s.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, s.cursor)

	// Move up
	s.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, s.cursor)

	// Move to top
	s.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, s.cursor)

	// Can't go past first item
	s.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, s.cursor)
}

func TestDrillDown(t *testing.T) {
	s := New()
	s.SetFocused(true)

	// Select "Compute Engine" (category at index 0)
	cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should drill down, no navigation message for categories
	assert.Nil(t, cmd)
	assert.Equal(t, []string{"compute"}, s.path)
	assert.Len(t, s.currentItems, 5, "Compute Engine has 5 children")
	assert.Equal(t, 0, s.cursor, "cursor should reset to 0")
}

func TestGoBack(t *testing.T) {
	s := New()
	s.SetFocused(true)

	// Drill into Compute Engine
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Len(t, s.path, 1)

	// Go back
	s.Update(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Empty(t, s.path)
	assert.Len(t, s.currentItems, 3, "should be back at root menu")
}

func TestBackItemSelectableByArrows(t *testing.T) {
	s := New()
	s.SetFocused(true)

	// Drill into Compute Engine
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, []string{"compute"}, s.path)
	assert.Equal(t, 0, s.cursor)

	// Move up past first item to reach "Back"
	s.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, -1, s.cursor, "cursor should be on Back item")

	// Can't go above Back
	s.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, -1, s.cursor, "cursor should stay on Back item")

	// Press Enter on Back item to go back
	cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd, "going back should not emit navigation")
	assert.Empty(t, s.path, "should be back at root")
	assert.Equal(t, 0, s.cursor, "cursor should reset to 0 after going back")
}

func TestBackItemNotReachableAtRoot(t *testing.T) {
	s := New()
	s.SetFocused(true)

	// At root level, cursor should not go below 0
	s.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, s.cursor, "cursor should not go to -1 at root level")
}

func TestSelectLeafItem(t *testing.T) {
	s := New()
	s.SetFocused(true)

	// Drill into Compute Engine
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Select "VM instances" (leaf at index 0)
	cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should emit NavigateMsg
	assert.NotNil(t, cmd)
	msg := cmd()
	navMsg, ok := msg.(NavigateMsg)
	assert.True(t, ok, "should be NavigateMsg")
	assert.Equal(t, ViewInstances, navMsg.ViewType)
	assert.Equal(t, "vm-instances", navMsg.ItemID)
}

func TestToggleCollapsed(t *testing.T) {
	s := New()

	// Starts collapsed in auto-hide mode
	assert.True(t, s.IsCollapsed())
	assert.Equal(t, CollapsedWidth+1, s.Width()) // +1 for right border

	s.Toggle()
	assert.False(t, s.IsCollapsed())
	assert.Equal(t, ExpandedWidth+1, s.Width()) // +1 for right border

	s.Toggle()
	assert.True(t, s.IsCollapsed())
	assert.Equal(t, CollapsedWidth+1, s.Width()) // +1 for right border
}

func TestUnfocusedIgnoresKeys(t *testing.T) {
	s := New()
	// Not focused

	initialCursor := s.cursor
	s.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, initialCursor, s.cursor, "should not move when unfocused")
}

func TestSetActiveView(t *testing.T) {
	s := New()
	s.SetActiveView(ViewBuckets)

	assert.Equal(t, ViewBuckets, s.activeView)
}

func TestGetCurrentCategory(t *testing.T) {
	s := New()

	// At root
	assert.Empty(t, s.GetCurrentCategory())

	// Drill into Compute Engine
	s.SetFocused(true)
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, "Compute Engine", s.GetCurrentCategory())
}

func TestViewRendersWithoutPanic(t *testing.T) {
	s := New()
	s.SetSize(20)

	// Should not panic
	output := s.View()
	assert.NotEmpty(t, output)

	// Toggle collapsed and render again
	s.Toggle()
	output = s.View()
	assert.NotEmpty(t, output)
}

func TestDefaultMenu(t *testing.T) {
	menu := DefaultMenu()

	assert.Len(t, menu, 3)

	// Check Compute Engine
	compute := menu[0]
	assert.Equal(t, "compute", compute.ID)
	assert.Equal(t, "Compute Engine", compute.Label)
	assert.Equal(t, MenuItemCategory, compute.Type)
	assert.Len(t, compute.Children, 5)

	// Check VM instances under Compute Engine
	vm := compute.Children[0]
	assert.Equal(t, "vm-instances", vm.ID)
	assert.Equal(t, MenuItemLeaf, vm.Type)
	assert.Equal(t, ViewInstances, vm.ViewType)
}

func TestNumberShortcuts(t *testing.T) {
	s := New()
	s.SetFocused(true)

	// Press "1" to select first item (Compute Engine) and drill down
	cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})

	// Should drill down into Compute Engine
	assert.Nil(t, cmd, "categories don't emit navigation")
	assert.Equal(t, []string{"compute"}, s.path)
	assert.Len(t, s.currentItems, 5)

	// Press "1" again to select VM instances
	cmd = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})

	// Should emit NavigateMsg
	assert.NotNil(t, cmd)
	msg := cmd()
	navMsg, ok := msg.(NavigateMsg)
	assert.True(t, ok)
	assert.Equal(t, ViewInstances, navMsg.ViewType)
}

func TestNumberShortcutOutOfRange(t *testing.T) {
	s := New()
	s.SetFocused(true)

	// Press "9" which is out of range (only 3 items)
	initialCursor := s.cursor
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}})

	// Should not change cursor or crash
	assert.Equal(t, initialCursor, s.cursor)
}

func TestViewWithFocusedAndUnfocused(t *testing.T) {
	s := New()
	s.SetSize(20)

	// Render unfocused
	s.SetFocused(false)
	outputUnfocused := s.View()
	assert.NotEmpty(t, outputUnfocused)

	// Render focused
	s.SetFocused(true)
	outputFocused := s.View()
	assert.NotEmpty(t, outputFocused)

	// They should be different (focused has brighter colors)
	// Can't easily test visual differences, but at least ensure no crash
}

func TestRenderHeader(t *testing.T) {
	s := New()
	s.Expand() // Expand to see full text
	s.SetSize(20)

	// At root, header should show "☰ Menu"
	output := s.View()
	assert.Contains(t, output, symbols.Hamburger())

	// Drill into Compute Engine
	s.SetFocused(true)
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Header should now show category name
	output = s.View()
	assert.Contains(t, output, "Compute Engine")
}

func TestSidebarMode_DefaultAutoHide(t *testing.T) {
	s := New()

	assert.Equal(t, SidebarModeAutoHide, s.Mode())
	assert.True(t, s.IsCollapsed(), "auto-hide mode starts collapsed")
}

func TestSetMode_AutoHide_Collapses(t *testing.T) {
	s := New()
	s.Expand() // Start expanded

	assert.False(t, s.IsCollapsed())

	s.SetMode(SidebarModeAutoHide)
	assert.True(t, s.IsCollapsed(), "switching to auto-hide should collapse")
	assert.Equal(t, SidebarModeAutoHide, s.Mode())
}

func TestSetMode_AlwaysOpen_Expands(t *testing.T) {
	s := New()

	assert.True(t, s.IsCollapsed())

	s.SetMode(SidebarModeAlwaysOpen)
	assert.False(t, s.IsCollapsed(), "switching to always-open should expand")
	assert.Equal(t, SidebarModeAlwaysOpen, s.Mode())
}

func TestCollapse_Idempotent(t *testing.T) {
	s := New()
	// Already collapsed from New()
	assert.True(t, s.IsCollapsed())

	s.Collapse()
	assert.True(t, s.IsCollapsed(), "collapsing already-collapsed sidebar should be no-op")
	assert.Equal(t, CollapsedWidth+1, s.Width())
}

func TestExpand_Idempotent(t *testing.T) {
	s := New()
	s.Expand()
	assert.False(t, s.IsCollapsed())

	s.Expand()
	assert.False(t, s.IsCollapsed(), "expanding already-expanded sidebar should be no-op")
	assert.Equal(t, ExpandedWidth+1, s.Width())
}

func TestSidebar_ViewHeightConsistency(t *testing.T) {
	// lipgloss Height(n) renders n lines which equals n-1 newlines
	tests := []struct {
		height           int
		expectedNewlines int
	}{
		{20, 19},
		{30, 29},
		{36, 35},
		{40, 39},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			s := New()
			s.SetSize(tt.height)

			view := s.View()
			newlines := strings.Count(view, "\n")

			t.Logf("Height set: %d, Actual newlines: %d", tt.height, newlines)

			assert.Equal(t, tt.expectedNewlines, newlines,
				"Sidebar with height %d should output %d newlines (height-1)", tt.height, tt.expectedNewlines)
		})
	}
}
