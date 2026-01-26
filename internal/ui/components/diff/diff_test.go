package diff

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestField_IsChanged(t *testing.T) {
	tests := []struct {
		name     string
		field    Field
		expected bool
	}{
		{
			name:     "unchanged field",
			field:    Field{Label: "env", OldValue: "prod", NewValue: "prod"},
			expected: false,
		},
		{
			name:     "changed field",
			field:    Field{Label: "env", OldValue: "prod", NewValue: "staging"},
			expected: true,
		},
		{
			name:     "added field",
			field:    Field{Label: "team", OldValue: "", NewValue: "backend"},
			expected: true,
		},
		{
			name:     "removed field",
			field:    Field{Label: "owner", OldValue: "alice", NewValue: ""},
			expected: true,
		},
		{
			name:     "both empty",
			field:    Field{Label: "empty", OldValue: "", NewValue: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.field.IsChanged())
		})
	}
}

func TestViewer_New(t *testing.T) {
	fields := []Field{
		{Label: "env", OldValue: "prod", NewValue: "staging"},
		{Label: "team", OldValue: "", NewValue: "backend"},
	}

	viewer := New("Test Changes", fields)

	assert.NotNil(t, viewer)
	assert.Equal(t, "Test Changes", viewer.title)
	assert.Len(t, viewer.fields, 2)
	assert.True(t, viewer.focusedYes, "Yes button should be focused by default")
}

func TestViewer_HasChanges(t *testing.T) {
	tests := []struct {
		name     string
		fields   []Field
		expected bool
	}{
		{
			name: "with changes",
			fields: []Field{
				{Label: "env", OldValue: "prod", NewValue: "staging"},
			},
			expected: true,
		},
		{
			name: "no changes",
			fields: []Field{
				{Label: "env", OldValue: "prod", NewValue: "prod"},
			},
			expected: false,
		},
		{
			name:     "empty fields",
			fields:   []Field{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viewer := New("Test", tt.fields)
			assert.Equal(t, tt.expected, viewer.HasChanges())
		})
	}
}

func TestViewer_Update_Navigation(t *testing.T) {
	viewer := New("Test", []Field{})

	// Initially focused on Yes
	assert.True(t, viewer.IsFocusedYes())

	// Move right to No
	viewer.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.False(t, viewer.IsFocusedYes())

	// Move left to Yes
	viewer.Update(tea.KeyMsg{Type: tea.KeyLeft})
	assert.True(t, viewer.IsFocusedYes())

	// Tab to toggle
	viewer.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.False(t, viewer.IsFocusedYes())

	viewer.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.True(t, viewer.IsFocusedYes())
}

func TestViewer_Update_Confirm(t *testing.T) {
	viewer := New("Test", []Field{})

	// Confirm with Enter when Yes is focused
	cmd := viewer.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(ConfirmMsg)
	assert.True(t, ok, "Expected ConfirmMsg when Enter pressed with Yes focused")
}

func TestViewer_Update_Cancel(t *testing.T) {
	viewer := New("Test", []Field{})

	// Move to No button
	viewer.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Confirm with Enter when No is focused
	cmd := viewer.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(CancelMsg)
	assert.True(t, ok, "Expected CancelMsg when Enter pressed with No focused")
}

func TestViewer_Update_EscapeCancel(t *testing.T) {
	viewer := New("Test", []Field{})

	// Escape should always cancel
	cmd := viewer.Update(tea.KeyMsg{Type: tea.KeyEscape})
	assert.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(CancelMsg)
	assert.True(t, ok, "Expected CancelMsg on Escape")
}

func TestViewer_Update_QuickKeys(t *testing.T) {
	t.Run("y key confirms", func(t *testing.T) {
		viewer := New("Test", []Field{})
		cmd := viewer.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
		assert.NotNil(t, cmd)

		msg := cmd()
		_, ok := msg.(ConfirmMsg)
		assert.True(t, ok)
	})

	t.Run("n key cancels", func(t *testing.T) {
		viewer := New("Test", []Field{})
		cmd := viewer.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		assert.NotNil(t, cmd)

		msg := cmd()
		_, ok := msg.(CancelMsg)
		assert.True(t, ok)
	})
}

func TestViewer_View_RendersDiff(t *testing.T) {
	fields := []Field{
		{Label: "env", OldValue: "prod", NewValue: "staging"},
		{Label: "owner", OldValue: "alice", NewValue: "alice"}, // unchanged
	}

	viewer := New("Confirm Changes", fields)
	viewer.SetSize(60, 20)

	output := viewer.View()

	// Check title is rendered
	assert.Contains(t, output, "Confirm Changes")

	// Check changed field shows old and new values
	assert.Contains(t, output, "env")
	assert.Contains(t, output, "prod")
	assert.Contains(t, output, "staging")

	// Check buttons are rendered
	assert.Contains(t, output, "Yes")
	assert.Contains(t, output, "No")
}

func TestViewer_SetWarnings(t *testing.T) {
	viewer := New("Test", []Field{})
	viewer.SetWarnings([]string{"Warning 1", "Warning 2"})
	viewer.SetSize(60, 20)

	output := viewer.View()

	assert.Contains(t, output, "Warning 1")
	assert.Contains(t, output, "Warning 2")
}
