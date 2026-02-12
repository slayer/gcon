package sortmenu

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testColumns() []SortColumn {
	return []SortColumn{
		{Label: "Name", ColIndex: 0},
		{Label: "Zone", ColIndex: 1},
		{Label: "Size", ColIndex: 2},
	}
}

func TestNew(t *testing.T) {
	m := New(testColumns(), -1, true)
	assert.Equal(t, 3, len(m.columns))
	assert.Equal(t, 0, m.cursor)
	assert.Equal(t, -1, m.activeColIndex)
}

func TestNew_CursorOnActiveColumn(t *testing.T) {
	m := New(testColumns(), 2, true) // Size is active
	assert.Equal(t, 2, m.cursor)     // Cursor should be on Size (index 2)
}

func TestNavigation(t *testing.T) {
	m := New(testColumns(), -1, true)

	// Move down
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, m.cursor)

	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, m.cursor)

	// Can't go past last
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, m.cursor)

	// Move up
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, m.cursor)

	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, m.cursor)

	// Can't go above first
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, m.cursor)
}

func TestHotkeySelection(t *testing.T) {
	m := New(testColumns(), -1, true)

	// Press "2" to select Zone (index 1)
	cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	require.NotNil(t, cmd)

	msg := cmd()
	sortMsg, ok := msg.(SortSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, sortMsg.ColIndex)
	assert.True(t, sortMsg.Ascending) // New column defaults ascending
}

func TestEnterSelection(t *testing.T) {
	m := New(testColumns(), -1, true)
	m.cursor = 1 // Zone

	cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)

	msg := cmd()
	sortMsg, ok := msg.(SortSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, sortMsg.ColIndex)
	assert.True(t, sortMsg.Ascending)
}

func TestDirectionToggle(t *testing.T) {
	// Zone is already sorted ascending
	m := New(testColumns(), 1, true)

	// Select Zone again -> should toggle to descending
	cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	require.NotNil(t, cmd)

	msg := cmd()
	sortMsg, ok := msg.(SortSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, sortMsg.ColIndex)
	assert.False(t, sortMsg.Ascending) // Toggled to descending

	// Now test: Zone sorted descending -> toggle back to ascending
	m2 := New(testColumns(), 1, false)
	cmd = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	msg = cmd()
	sortMsg, ok = msg.(SortSelectedMsg)
	require.True(t, ok)
	assert.True(t, sortMsg.Ascending)
}

func TestNewColumnDefaultsAscending(t *testing.T) {
	// Zone is sorted descending; selecting Name should default to ascending
	m := New(testColumns(), 1, false)

	cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}) // Name
	require.NotNil(t, cmd)

	msg := cmd()
	sortMsg, ok := msg.(SortSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 0, sortMsg.ColIndex)
	assert.True(t, sortMsg.Ascending)
}

func TestClose(t *testing.T) {
	m := New(testColumns(), -1, true)

	cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(SortMenuClosedMsg)
	assert.True(t, ok)
}

func TestCloseWithQ(t *testing.T) {
	m := New(testColumns(), -1, true)

	cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(SortMenuClosedMsg)
	assert.True(t, ok)
}

func TestView_NotEmpty(t *testing.T) {
	m := New(testColumns(), -1, true)
	view := m.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Sort by Column")
	assert.Contains(t, view, "Name")
	assert.Contains(t, view, "Zone")
	assert.Contains(t, view, "Size")
}

func TestView_ShowsSortIndicator(t *testing.T) {
	m := New(testColumns(), 1, true) // Zone sorted ascending
	view := m.View()
	assert.Contains(t, view, "▲")
}

func TestView_ShowsDescendingIndicator(t *testing.T) {
	m := New(testColumns(), 1, false) // Zone sorted descending
	view := m.View()
	assert.Contains(t, view, "▼")
}

func TestHotkey(t *testing.T) {
	assert.Equal(t, "1", hotkey(0))
	assert.Equal(t, "9", hotkey(8))
	assert.Equal(t, "a", hotkey(9))
	assert.Equal(t, "b", hotkey(10))
}

func TestHotkeyToIndex(t *testing.T) {
	assert.Equal(t, 0, hotkeyToIndex('1'))
	assert.Equal(t, 8, hotkeyToIndex('9'))
	assert.Equal(t, 9, hotkeyToIndex('a'))
	assert.Equal(t, -1, hotkeyToIndex('0'))
	assert.Equal(t, -1, hotkeyToIndex('A'))
}

func TestInvalidHotkeyIgnored(t *testing.T) {
	m := New(testColumns(), -1, true)

	// Press "5" which is beyond 3 columns -> should return nil
	cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	assert.Nil(t, cmd)
}

func TestNavKeysNotConsumedAsHotkeys(t *testing.T) {
	// With 10+ columns, 'j' would map to index 9 and 'k' to index 10.
	// Navigation keys must take priority over hotkey matching.
	manyColumns := make([]SortColumn, 12)
	for i := range manyColumns {
		manyColumns[i] = SortColumn{Label: fmt.Sprintf("Col%d", i), ColIndex: i}
	}
	m := New(manyColumns, -1, true)

	// 'j' should navigate down, not select column at index 9
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, m.cursor, "j should navigate down, not select hotkey")

	// 'k' should navigate up, not select column at index 10
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, m.cursor, "k should navigate up, not select hotkey")
}
