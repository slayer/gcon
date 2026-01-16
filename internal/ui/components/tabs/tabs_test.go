package tabs

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tabs := []Tab{
		{ID: "details", Label: "Details"},
		{ID: "observability", Label: "Observability"},
	}

	component := New(tabs)

	assert.Equal(t, 2, component.Count())
	assert.Equal(t, 0, component.ActiveIndex())
	assert.Equal(t, "details", component.ActiveTab().ID)
}

func TestTabNavigation(t *testing.T) {
	tabs := []Tab{
		{ID: "tab1", Label: "Tab 1"},
		{ID: "tab2", Label: "Tab 2"},
		{ID: "tab3", Label: "Tab 3"},
	}

	component := New(tabs)

	// Initial state
	assert.Equal(t, 0, component.ActiveIndex())

	// Tab key moves forward
	cmd := component.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.NotNil(t, cmd)
	msg := cmd()
	changed, ok := msg.(TabChangedMsg)
	require.True(t, ok)
	assert.Equal(t, "tab2", changed.TabID)
	assert.Equal(t, 1, changed.Index)
	assert.Equal(t, 1, component.ActiveIndex())

	// Tab again
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.NotNil(t, cmd)
	msg = cmd()
	changed, ok = msg.(TabChangedMsg)
	require.True(t, ok)
	assert.Equal(t, "tab3", changed.TabID)
	assert.Equal(t, 2, component.ActiveIndex())

	// Tab at end does nothing
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Nil(t, cmd)
	assert.Equal(t, 2, component.ActiveIndex())

	// Shift+Tab moves backward
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	require.NotNil(t, cmd)
	msg = cmd()
	changed, ok = msg.(TabChangedMsg)
	require.True(t, ok)
	assert.Equal(t, "tab2", changed.TabID)
	assert.Equal(t, 1, component.ActiveIndex())
}

func TestArrowKeyNavigation(t *testing.T) {
	tabs := []Tab{
		{ID: "tab1", Label: "Tab 1"},
		{ID: "tab2", Label: "Tab 2"},
	}

	component := New(tabs)

	// 'l' moves right
	cmd := component.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	require.NotNil(t, cmd)
	assert.Equal(t, 1, component.ActiveIndex())

	// 'h' moves left
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	require.NotNil(t, cmd)
	assert.Equal(t, 0, component.ActiveIndex())

	// 'h' at start does nothing
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	assert.Nil(t, cmd)
	assert.Equal(t, 0, component.ActiveIndex())

	// Right arrow
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyRight})
	require.NotNil(t, cmd)
	assert.Equal(t, 1, component.ActiveIndex())

	// Left arrow
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyLeft})
	require.NotNil(t, cmd)
	assert.Equal(t, 0, component.ActiveIndex())
}

func TestNumberKeyNavigation(t *testing.T) {
	tabs := []Tab{
		{ID: "tab1", Label: "Tab 1"},
		{ID: "tab2", Label: "Tab 2"},
		{ID: "tab3", Label: "Tab 3"},
	}

	component := New(tabs)

	// Press '2' to go to second tab
	cmd := component.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	require.NotNil(t, cmd)
	msg := cmd()
	changed, ok := msg.(TabChangedMsg)
	require.True(t, ok)
	assert.Equal(t, "tab2", changed.TabID)
	assert.Equal(t, 1, changed.Index)

	// Press '3' to go to third tab
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	require.NotNil(t, cmd)
	msg = cmd()
	changed, ok = msg.(TabChangedMsg)
	require.True(t, ok)
	assert.Equal(t, "tab3", changed.TabID)
	assert.Equal(t, 2, component.ActiveIndex())

	// Press '1' to go back to first tab
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	require.NotNil(t, cmd)
	assert.Equal(t, 0, component.ActiveIndex())

	// Press same number (already on tab 1) does nothing
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	assert.Nil(t, cmd)

	// Press '9' (out of bounds) does nothing
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}})
	assert.Nil(t, cmd)
	assert.Equal(t, 0, component.ActiveIndex())
}

func TestView(t *testing.T) {
	tabs := []Tab{
		{ID: "details", Label: "Details"},
		{ID: "observability", Label: "Observability"},
	}

	component := New(tabs)

	// First tab active
	view := component.View()
	assert.Contains(t, view, "[Details]", "Active tab should have brackets")
	assert.Contains(t, view, "Observability", "Inactive tab should be present")
	// Active tab should not have plain "Details" without brackets
	// (checking that brackets are actually there)
	assert.True(t, strings.Contains(view, "[Details]"))

	// Switch to second tab
	component.SetActive(1)
	view = component.View()
	assert.Contains(t, view, "Details", "First tab now inactive")
	assert.Contains(t, view, "[Observability]", "Second tab now active with brackets")
}

func TestSetActive(t *testing.T) {
	tabs := []Tab{
		{ID: "tab1", Label: "Tab 1"},
		{ID: "tab2", Label: "Tab 2"},
	}

	component := New(tabs)

	// Valid index
	component.SetActive(1)
	assert.Equal(t, 1, component.ActiveIndex())

	// Out of bounds - no change
	component.SetActive(5)
	assert.Equal(t, 1, component.ActiveIndex())

	component.SetActive(-1)
	assert.Equal(t, 1, component.ActiveIndex())
}

func TestSetActiveByID(t *testing.T) {
	tabs := []Tab{
		{ID: "details", Label: "Details"},
		{ID: "observability", Label: "Observability"},
	}

	component := New(tabs)

	// Set by valid ID
	component.SetActiveByID("observability")
	assert.Equal(t, 1, component.ActiveIndex())
	assert.Equal(t, "observability", component.ActiveTab().ID)

	// Set back
	component.SetActiveByID("details")
	assert.Equal(t, 0, component.ActiveIndex())

	// Invalid ID - no change
	component.SetActiveByID("nonexistent")
	assert.Equal(t, 0, component.ActiveIndex())
}

func TestHandleKey(t *testing.T) {
	testCases := []struct {
		key      tea.KeyMsg
		expected bool
	}{
		{tea.KeyMsg{Type: tea.KeyTab}, true},
		{tea.KeyMsg{Type: tea.KeyShiftTab}, true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}, true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}, true},
		{tea.KeyMsg{Type: tea.KeyLeft}, true},
		{tea.KeyMsg{Type: tea.KeyRight}, true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}, true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}}, true},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, false},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, false},
		{tea.KeyMsg{Type: tea.KeyEnter}, false},
		{tea.KeyMsg{Type: tea.KeyEscape}, false},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}}, false},
	}

	for _, tc := range testCases {
		result := HandleKey(tc.key)
		assert.Equal(t, tc.expected, result, "HandleKey for %v", tc.key)
	}
}

func TestEmptyTabs(t *testing.T) {
	component := New([]Tab{})

	assert.Equal(t, 0, component.Count())
	assert.Equal(t, 0, component.ActiveIndex())
	assert.Equal(t, Tab{}, component.ActiveTab())

	// Should not panic
	view := component.View()
	assert.NotNil(t, view)
}

func TestSingleTab(t *testing.T) {
	tabs := []Tab{
		{ID: "only", Label: "Only Tab"},
	}

	component := New(tabs)

	assert.Equal(t, 1, component.Count())
	assert.Equal(t, 0, component.ActiveIndex())

	// Tab forward does nothing (already at end)
	cmd := component.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Nil(t, cmd)
	assert.Equal(t, 0, component.ActiveIndex())

	// Tab backward does nothing (already at start)
	cmd = component.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	assert.Nil(t, cmd)
	assert.Equal(t, 0, component.ActiveIndex())
}
