package commandpalette

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	p := New()

	assert.NotNil(t, p)
	assert.Greater(t, len(p.commands), 0, "should have default commands")
	assert.Equal(t, len(p.commands), len(p.filtered), "filtered should match commands initially")
	assert.Equal(t, 0, p.cursor)
}

func TestSetSize(t *testing.T) {
	p := New()
	p.SetSize(80, 24)

	assert.Equal(t, 80, p.width)
	assert.Equal(t, 24, p.height)
}

func TestSetProjectSelected(t *testing.T) {
	p := New()

	// Initially not selected
	p.SetProjectSelected(false)
	for _, cmd := range p.commands {
		if cmd.Type == CommandTypeNavigation {
			assert.False(t, cmd.Enabled, "navigation commands should be disabled")
		}
	}

	// After project selection
	p.SetProjectSelected(true)
	for _, cmd := range p.commands {
		if cmd.Type == CommandTypeNavigation {
			assert.True(t, cmd.Enabled, "navigation commands should be enabled")
		}
	}
}

func TestReset(t *testing.T) {
	p := New()

	// Modify state
	p.cursor = 3
	p.input.SetValue("test")

	// Reset
	p.Reset()

	assert.Equal(t, 0, p.cursor)
	assert.Equal(t, "", p.input.Value())
}

func TestUpdateNavigation(t *testing.T) {
	p := New()
	p.SetSize(80, 24)

	t.Run("down key moves cursor", func(t *testing.T) {
		p.Reset()
		p.cursor = 0

		p.Update(tea.KeyMsg{Type: tea.KeyDown})
		assert.Equal(t, 1, p.cursor)
	})

	t.Run("up key moves cursor", func(t *testing.T) {
		p.Reset()
		p.cursor = 2

		p.Update(tea.KeyMsg{Type: tea.KeyUp})
		assert.Equal(t, 1, p.cursor)
	})

	t.Run("cursor stops at top", func(t *testing.T) {
		p.Reset()
		p.cursor = 0

		p.Update(tea.KeyMsg{Type: tea.KeyUp})
		assert.Equal(t, 0, p.cursor)
	})

	t.Run("cursor stops at bottom", func(t *testing.T) {
		p.Reset()
		p.cursor = len(p.filtered) - 1
		lastPos := p.cursor

		p.Update(tea.KeyMsg{Type: tea.KeyDown})
		assert.Equal(t, lastPos, p.cursor)
	})

	t.Run("ctrl+n moves down", func(t *testing.T) {
		p.Reset()
		p.cursor = 0

		p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: true})
		// ctrl+n is sent as Runes with Alt modifier in some terminals
		// Let's test with explicit key matching
		msg := tea.KeyMsg{Type: tea.KeyCtrlN}
		if key.Matches(msg, p.keys.CtrlN) {
			p.Update(msg)
			assert.Equal(t, 1, p.cursor)
		}
	})
}

func TestUpdateCancel(t *testing.T) {
	p := New()

	cmd := p.Update(tea.KeyMsg{Type: tea.KeyEscape})

	// Execute the command and check the result
	msg := cmd()
	_, ok := msg.(CommandCancelMsg)
	assert.True(t, ok, "should emit CommandCancelMsg")
}

func TestUpdateSelect(t *testing.T) {
	p := New()
	p.SetProjectSelected(true) // Enable navigation commands

	cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Execute the command and check the result
	msg := cmd()
	selectedMsg, ok := msg.(CommandSelectedMsg)
	assert.True(t, ok, "should emit CommandSelectedMsg")
	assert.NotEmpty(t, selectedMsg.Command.ID)
}

func TestUpdateSelectDisabled(t *testing.T) {
	p := New()
	p.SetProjectSelected(false) // Disable navigation commands

	// Move to a navigation command
	p.cursor = 0
	for i, cmd := range p.filtered {
		if cmd.Type == CommandTypeNavigation {
			p.cursor = i
			break
		}
	}

	cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should return nil for disabled command
	assert.Nil(t, cmd, "should not emit message for disabled command")
}

func TestUpdateFiltering(t *testing.T) {
	p := New()

	// Type to filter
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v', 'm'}})

	// Check that filtering occurred
	assert.Contains(t, p.input.Value(), "vm")
	// Cursor should reset on filter
	assert.Equal(t, 0, p.cursor)

	// Should have filtered results
	for _, cmd := range p.filtered {
		assert.Contains(t, strings.ToLower(cmd.Label), "vm",
			"filtered commands should match query")
	}
}

func TestView(t *testing.T) {
	p := New()
	p.SetSize(60, 20)
	p.SetShowPrefix(true)

	view := p.View()

	// Check that view contains expected elements
	assert.Contains(t, view, ":", "should show prefix")
	assert.Contains(t, view, "─", "should show separator")
	assert.Contains(t, view, "▸", "should show cursor")
}

func TestViewNoResults(t *testing.T) {
	p := New()
	p.SetSize(60, 20)

	// Filter to no results
	p.input.SetValue("xyznonexistent")
	p.filtered = Filter(p.commands, "xyznonexistent")

	view := p.View()

	assert.Contains(t, view, "No matching commands")
}

func TestCenterInScreen(t *testing.T) {
	p := New()
	p.SetSize(50, 15)

	centered := p.CenterInScreen(100, 40)

	// Should have some left margin for centering
	lines := strings.Split(centered, "\n")
	assert.Greater(t, len(lines), 0)

	// First non-empty line should have leading spaces
	for _, line := range lines {
		if line != "" {
			assert.True(t, strings.HasPrefix(line, " "),
				"should have left margin for centering")
			break
		}
	}
}
