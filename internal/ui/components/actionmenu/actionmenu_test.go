package actionmenu

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: true},
		{Key: 'x', Label: "Stop", Enabled: true},
	}

	menu := New("Test Menu", actions)

	assert.Equal(t, "Test Menu", menu.title)
	assert.Equal(t, 2, len(menu.actions))
	assert.Equal(t, 0, menu.cursor)
}

func TestNavigation(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: true},
		{Key: 'x', Label: "Stop", Enabled: true},
		{Key: 'r', Label: "Refresh", Enabled: true},
	}

	menu := New("Test", actions)

	// Move down
	menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, menu.cursor)

	menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 2, menu.cursor)

	// At bottom, can't go further
	menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 2, menu.cursor)

	// Move up
	menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, menu.cursor)

	// At top after moving up twice
	menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, menu.cursor)

	menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, menu.cursor)
}

func TestNavigationSkipsDisabled(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: true},
		{Key: 'x', Label: "Stop", Enabled: false}, // disabled
		{Key: 'r', Label: "Refresh", Enabled: true},
	}

	menu := New("Test", actions)

	// Move down should skip disabled item
	menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 2, menu.cursor, "Should skip disabled item at index 1")

	// Move up should skip disabled item
	menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, menu.cursor, "Should skip disabled item going up")
}

func TestDirectHotkeySelection(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: true},
		{Key: 'x', Label: "Stop", Enabled: true},
		{Key: 'R', Label: "Reset", Enabled: true, Dangerous: true},
	}

	menu := New("Test", actions)

	// Press 's' directly
	cmd := menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	require.NotNil(t, cmd)

	msg := cmd()
	selected, ok := msg.(ActionSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 's', selected.Key)

	// Press 'R' (uppercase)
	cmd = menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	require.NotNil(t, cmd)

	msg = cmd()
	selected, ok = msg.(ActionSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 'R', selected.Key)
}

func TestDisabledHotkeyIgnored(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: false}, // disabled
		{Key: 'x', Label: "Stop", Enabled: true},
	}

	menu := New("Test", actions)

	// Press 's' for disabled action - should be ignored
	cmd := menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.Nil(t, cmd, "Disabled action hotkey should be ignored")
}

func TestEnterSelection(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: true},
		{Key: 'x', Label: "Stop", Enabled: true},
	}

	menu := New("Test", actions)
	menu.SetCursor(1)

	cmd := menu.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)

	msg := cmd()
	selected, ok := msg.(ActionSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, 'x', selected.Key)
}

func TestEnterOnDisabledDoesNothing(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: false},
	}

	menu := New("Test", actions)

	cmd := menu.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd, "Enter on disabled action should do nothing")
}

func TestCloseKeys(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: true},
	}

	menu := New("Test", actions)

	// Test Escape
	cmd := menu.Update(tea.KeyMsg{Type: tea.KeyEscape})
	require.NotNil(t, cmd)
	_, ok := cmd().(ActionMenuClosedMsg)
	assert.True(t, ok, "Escape should close menu")

	// Test 'q'
	cmd = menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	require.NotNil(t, cmd)
	_, ok = cmd().(ActionMenuClosedMsg)
	assert.True(t, ok, "'q' should close menu")

	// Test '.'
	cmd = menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	require.NotNil(t, cmd)
	_, ok = cmd().(ActionMenuClosedMsg)
	assert.True(t, ok, "'.' should close menu (toggle)")
}

func TestView(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: true},
		{Key: 'x', Label: "Stop", Enabled: false},
		{Key: 'R', Label: "Reset", Enabled: true, Dangerous: true},
	}

	menu := New("Instance Actions", actions)
	view := menu.View()

	// Check title is present
	assert.Contains(t, view, "Instance Actions")

	// Check actions are rendered
	assert.Contains(t, view, "Start")
	assert.Contains(t, view, "Stop")
	assert.Contains(t, view, "Reset")

	// Check help text
	assert.Contains(t, view, "j/k:nav")
	assert.Contains(t, view, "enter:sel")
	assert.Contains(t, view, "esc:close")
}

func TestViewCursorIndicator(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: true},
		{Key: 'x', Label: "Stop", Enabled: true},
	}

	menu := New("Test", actions)

	// Cursor on first item
	view := menu.View()
	lines := strings.Split(view, "\n")

	// Find line with "Start" - should have cursor indicator
	var startLine string
	for _, line := range lines {
		if strings.Contains(line, "Start") {
			startLine = line
			break
		}
	}
	assert.Contains(t, startLine, "▶", "First item should have cursor")

	// Move cursor to second item
	menu.SetCursor(1)
	view = menu.View()
	lines = strings.Split(view, "\n")

	var stopLine string
	for _, line := range lines {
		if strings.Contains(line, "Stop") {
			stopLine = line
			break
		}
	}
	assert.Contains(t, stopLine, "▶", "Second item should have cursor after move")
}

func TestWidthCalculation(t *testing.T) {
	// Short title, short labels
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: true},
	}
	menu := New("A", actions)
	assert.True(t, menu.width > 0)

	// Long title should affect width
	menuLong := New("Very Long Title For Action Menu", actions)
	assert.True(t, menuLong.width > menu.width, "Longer title should increase width")

	// Long labels should also affect width
	actionsLong := []Action{
		{Key: 's', Label: "Start the virtual machine instance now", Enabled: true},
	}
	menuLongLabel := New("A", actionsLong)
	assert.True(t, menuLongLabel.width > menu.width, "Longer labels should increase width")
}

func TestGettersAndSetters(t *testing.T) {
	actions := []Action{
		{Key: 's', Label: "Start", Enabled: true},
		{Key: 'x', Label: "Stop", Enabled: true},
	}

	menu := New("Test", actions)

	assert.Equal(t, 0, menu.GetCursor())

	menu.SetCursor(1)
	assert.Equal(t, 1, menu.GetCursor())

	// Out of bounds should not change
	menu.SetCursor(10)
	assert.Equal(t, 1, menu.GetCursor())

	menu.SetCursor(-1)
	assert.Equal(t, 1, menu.GetCursor())

	retrieved := menu.GetActions()
	assert.Equal(t, 2, len(retrieved))
}
