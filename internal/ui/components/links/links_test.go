package links

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	l := New()

	assert.NotNil(t, l)
	assert.Equal(t, 0, l.Count())
	assert.Equal(t, 0, l.FocusedIndex())
	assert.False(t, l.HasItems())
}

func TestSetItems(t *testing.T) {
	l := New()

	items := []Link{
		{ID: "disk1", Label: "boot-disk", Type: "disk"},
		{ID: "disk2", Label: "data-disk", Type: "disk"},
	}

	l.SetItems(items)

	assert.Equal(t, 2, l.Count())
	assert.True(t, l.HasItems())
	assert.Equal(t, 0, l.FocusedIndex())
}

func TestSetItemsResetsFocus(t *testing.T) {
	l := New()

	// Set initial items and move focus
	items := []Link{
		{ID: "disk1", Label: "disk1", Type: "disk"},
		{ID: "disk2", Label: "disk2", Type: "disk"},
		{ID: "disk3", Label: "disk3", Type: "disk"},
	}
	l.SetItems(items)
	l.SetFocused(2) // Focus last item

	// Set fewer items - focus should reset
	l.SetItems([]Link{
		{ID: "disk1", Label: "disk1", Type: "disk"},
	})

	assert.Equal(t, 0, l.FocusedIndex(), "Focus should reset when out of bounds")
}

func TestNavigation(t *testing.T) {
	l := New()
	l.SetItems([]Link{
		{ID: "disk1", Label: "disk1", Type: "disk"},
		{ID: "disk2", Label: "disk2", Type: "disk"},
		{ID: "disk3", Label: "disk3", Type: "disk"},
	})

	// Initial state
	assert.Equal(t, 0, l.FocusedIndex())

	// Move down with j
	l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, l.FocusedIndex())

	// Move down with down arrow
	l.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, l.FocusedIndex())

	// At bottom, can't go further
	l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 2, l.FocusedIndex())

	// Move up with k
	l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, l.FocusedIndex())

	// Move up with up arrow
	l.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, l.FocusedIndex())

	// At top, can't go further
	l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, l.FocusedIndex())
}

func TestSelectLink(t *testing.T) {
	l := New()
	l.SetItems([]Link{
		{ID: "disk1", Label: "boot-disk", Type: "disk", Data: "zone-a"},
		{ID: "disk2", Label: "data-disk", Type: "disk", Data: "zone-b"},
	})

	// Move to second item
	l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Press Enter
	cmd := l.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)

	msg := cmd()
	selected, ok := msg.(LinkSelectedMsg)
	require.True(t, ok)

	assert.Equal(t, "disk2", selected.Link.ID)
	assert.Equal(t, "data-disk", selected.Link.Label)
	assert.Equal(t, "disk", selected.Link.Type)
	assert.Equal(t, "zone-b", selected.Link.Data)
}

func TestSelectWithNoItems(t *testing.T) {
	l := New()

	// Press Enter with no items - should do nothing
	cmd := l.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd)
}

func TestFocusedLink(t *testing.T) {
	l := New()

	// No items - should return nil
	assert.Nil(t, l.FocusedLink())

	// Add items
	l.SetItems([]Link{
		{ID: "disk1", Label: "disk1", Type: "disk"},
		{ID: "disk2", Label: "disk2", Type: "disk"},
	})

	// First item focused
	link := l.FocusedLink()
	require.NotNil(t, link)
	assert.Equal(t, "disk1", link.ID)

	// Move to second
	l.SetFocused(1)
	link = l.FocusedLink()
	require.NotNil(t, link)
	assert.Equal(t, "disk2", link.ID)
}

func TestSetFocused(t *testing.T) {
	l := New()
	l.SetItems([]Link{
		{ID: "disk1", Label: "disk1", Type: "disk"},
		{ID: "disk2", Label: "disk2", Type: "disk"},
	})

	// Valid index
	l.SetFocused(1)
	assert.Equal(t, 1, l.FocusedIndex())

	// Out of bounds - no change
	l.SetFocused(10)
	assert.Equal(t, 1, l.FocusedIndex())

	l.SetFocused(-1)
	assert.Equal(t, 1, l.FocusedIndex())
}

func TestRenderRow(t *testing.T) {
	l := New()
	l.SetItems([]Link{
		{ID: "disk1", Label: "disk1", Type: "disk"},
		{ID: "disk2", Label: "disk2", Type: "disk"},
	})

	// Focused row (index 0) should have cursor
	row0 := l.RenderRow(0, "boot-disk  100GB  pd-ssd")
	assert.Contains(t, row0, "▶", "Focused row should have cursor")
	assert.Contains(t, row0, "boot-disk")

	// Non-focused row should not have cursor
	row1 := l.RenderRow(1, "data-disk  500GB  pd-standard")
	assert.NotContains(t, row1, "▶", "Non-focused row should not have cursor")
	assert.Contains(t, row1, "data-disk")
}

func TestRenderHeader(t *testing.T) {
	l := New()

	header := l.RenderHeader("Name          Size       Type")
	assert.Contains(t, header, "Name")
	assert.Contains(t, header, "Size")
}

func TestRenderDivider(t *testing.T) {
	l := New()

	divider := l.RenderDivider(40)
	assert.Contains(t, divider, "─")
}

func TestHandleKey(t *testing.T) {
	testCases := []struct {
		key      tea.KeyMsg
		expected bool
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, true},
		{tea.KeyMsg{Type: tea.KeyUp}, true},
		{tea.KeyMsg{Type: tea.KeyDown}, true},
		{tea.KeyMsg{Type: tea.KeyEnter}, true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}, false},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}, false},
		{tea.KeyMsg{Type: tea.KeyTab}, false},
		{tea.KeyMsg{Type: tea.KeyEscape}, false},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}, false},
	}

	for _, tc := range testCases {
		result := HandleKey(tc.key)
		assert.Equal(t, tc.expected, result, "HandleKey for %v", tc.key)
	}
}

func TestEmptyListNavigation(t *testing.T) {
	l := New()

	// Should not panic with empty list
	cmd := l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Nil(t, cmd)

	cmd = l.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Nil(t, cmd)
}

func TestRenderRowOutOfBounds(t *testing.T) {
	l := New()
	l.SetItems([]Link{
		{ID: "disk1", Label: "disk1", Type: "disk"},
	})

	// Out of bounds index should return row unchanged
	row := l.RenderRow(5, "some content")
	assert.Equal(t, "some content", row)

	row = l.RenderRow(-1, "some content")
	assert.Equal(t, "some content", row)
}
