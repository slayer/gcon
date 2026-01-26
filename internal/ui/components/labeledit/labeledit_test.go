package labeledit

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNew_EmptyLabels(t *testing.T) {
	editor := New(nil)

	assert.NotNil(t, editor)
	assert.Empty(t, editor.entries)
	assert.False(t, editor.IsDirty())
}

func TestNew_WithLabels(t *testing.T) {
	labels := map[string]string{
		"env":   "prod",
		"team":  "backend",
		"owner": "alice",
	}

	editor := New(labels)

	assert.Len(t, editor.entries, 3)
	assert.False(t, editor.IsDirty())

	// Labels should be sorted by key
	assert.Equal(t, "env", editor.entries[0].key)
	assert.Equal(t, "owner", editor.entries[1].key)
	assert.Equal(t, "team", editor.entries[2].key)
}

func TestEditor_GetLabels(t *testing.T) {
	labels := map[string]string{
		"env":  "prod",
		"team": "backend",
	}

	editor := New(labels)
	result := editor.GetLabels()

	assert.Equal(t, labels, result)
}

func TestEditor_GetLabels_ExcludesDeleted(t *testing.T) {
	labels := map[string]string{
		"env":  "prod",
		"team": "backend",
	}

	editor := New(labels)
	editor.entries[0].markedForDelete = true

	result := editor.GetLabels()

	assert.Len(t, result, 1)
	assert.Equal(t, "backend", result["team"])
}

func TestEditor_IsDirty_NoChanges(t *testing.T) {
	labels := map[string]string{
		"env": "prod",
	}

	editor := New(labels)
	assert.False(t, editor.IsDirty())
}

func TestEditor_IsDirty_ModifiedValue(t *testing.T) {
	labels := map[string]string{
		"env": "prod",
	}

	editor := New(labels)
	editor.entries[0].value = "staging"

	assert.True(t, editor.IsDirty())
}

func TestEditor_IsDirty_DeletedLabel(t *testing.T) {
	labels := map[string]string{
		"env": "prod",
	}

	editor := New(labels)
	editor.entries[0].markedForDelete = true

	assert.True(t, editor.IsDirty())
}

func TestEditor_IsDirty_AddedLabel(t *testing.T) {
	editor := New(nil)
	editor.entries = append(editor.entries, labelEntry{
		key:   "env",
		value: "prod",
		isNew: true,
	})

	assert.True(t, editor.IsDirty())
}

func TestEditor_Navigation(t *testing.T) {
	labels := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}

	editor := New(labels)
	assert.Equal(t, 0, editor.cursor)

	// Move down
	editor.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, editor.cursor)

	editor.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, editor.cursor)

	// Can't go past end
	editor.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, editor.cursor)

	// Move up
	editor.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, editor.cursor)

	editor.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, editor.cursor)

	// Can't go past start
	editor.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, editor.cursor)
}

func TestEditor_NavigationWithKeys(t *testing.T) {
	labels := map[string]string{
		"a": "1",
		"b": "2",
	}

	editor := New(labels)

	// Move down with 'j'
	editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, editor.cursor)

	// Move up with 'k'
	editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, editor.cursor)
}

func TestEditor_StartAdding(t *testing.T) {
	editor := New(nil)

	// Press 'a' to add
	editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	assert.True(t, editor.adding)
	assert.True(t, editor.focusKey)
	assert.Empty(t, editor.keyInput.Value())
	assert.Empty(t, editor.valueInput.Value())
}

func TestEditor_AddLabel(t *testing.T) {
	editor := New(nil)

	// Start adding
	editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	// Type key
	for _, r := range "mykey" {
		editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter to move to value
	editor.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, editor.focusKey)

	// Type value
	for _, r := range "myvalue" {
		editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter to submit
	editor.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.False(t, editor.adding)
	assert.Len(t, editor.entries, 1)
	assert.Equal(t, "mykey", editor.entries[0].key)
	assert.Equal(t, "myvalue", editor.entries[0].value)
	assert.True(t, editor.entries[0].isNew)
}

func TestEditor_AddLabel_ValidateKey(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		expectErr bool
	}{
		{"valid key", "mykey", false},
		{"with numbers", "key123", false},
		{"with hyphens", "my-key", false},
		{"with underscores", "my_key", false},
		{"empty key", "", true},
		{"starts with number", "1key", true},
		{"uppercase", "MyKey", true},
		{"with spaces", "my key", true},
		{"special chars", "my@key", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			editor := New(nil)

			// Start adding
			editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

			// Set key directly for testing
			editor.keyInput.SetValue(tt.key)

			// Move to value
			editor.focusKey = false
			editor.valueInput.SetValue("testvalue")

			// Try to submit
			editor.submitEdit()

			if tt.expectErr {
				assert.NotEmpty(t, editor.err, "Expected error for key: %s", tt.key)
			} else {
				assert.Empty(t, editor.err, "Unexpected error for key: %s", tt.key)
			}
		})
	}
}

func TestEditor_AddLabel_ValidateValue(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		expectErr bool
	}{
		{"valid value", "myvalue", false},
		{"empty value", "", false}, // Empty values are allowed
		{"with numbers", "value123", false},
		{"with hyphens", "my-value", false},
		{"with underscores", "my_value", false},
		{"uppercase", "MyValue", true},
		{"with spaces", "my value", true},
		{"special chars", "my@value", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			editor := New(nil)

			// Start adding
			editor.adding = true
			editor.keyInput.SetValue("validkey")
			editor.valueInput.SetValue(tt.value)
			editor.focusKey = false

			// Try to submit
			editor.submitEdit()

			if tt.expectErr {
				assert.NotEmpty(t, editor.err, "Expected error for value: %s", tt.value)
			} else {
				assert.Empty(t, editor.err, "Unexpected error for value: %s", tt.value)
			}
		})
	}
}

func TestEditor_DeleteLabel_New(t *testing.T) {
	editor := New(nil)

	// Add a new label
	editor.entries = append(editor.entries, labelEntry{
		key:   "test",
		value: "value",
		isNew: true,
	})

	// Delete it (should remove immediately since it's new)
	editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	assert.Empty(t, editor.entries)
}

func TestEditor_DeleteLabel_Existing(t *testing.T) {
	labels := map[string]string{
		"env": "prod",
	}

	editor := New(labels)

	// Delete (should mark for deletion)
	editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	assert.True(t, editor.entries[0].markedForDelete)

	// Toggle back
	editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	assert.False(t, editor.entries[0].markedForDelete)
}

func TestEditor_CancelEdit(t *testing.T) {
	editor := New(nil)

	// Start adding
	editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.True(t, editor.adding)

	// Cancel
	editor.Update(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, editor.adding)
	assert.Empty(t, editor.entries)
}

func TestEditor_SaveRequested(t *testing.T) {
	editor := New(nil)

	// Press Ctrl+S
	cmd := editor.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	assert.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(SaveRequestedMsg)
	assert.True(t, ok)
}

func TestEditor_DuplicateKeyPrevention(t *testing.T) {
	labels := map[string]string{
		"env": "prod",
	}

	editor := New(labels)

	// Try to add duplicate key
	editor.adding = true
	editor.keyInput.SetValue("env")
	editor.valueInput.SetValue("staging")
	editor.focusKey = false

	editor.submitEdit()

	assert.Equal(t, "Key already exists", editor.err)
	assert.Len(t, editor.entries, 1) // Original still there
}

func TestEditor_View_RendersList(t *testing.T) {
	labels := map[string]string{
		"env":  "prod",
		"team": "backend",
	}

	editor := New(labels)
	editor.SetSize(80, 20)

	output := editor.View()

	assert.Contains(t, output, "Labels")
	assert.Contains(t, output, "env")
	assert.Contains(t, output, "prod")
	assert.Contains(t, output, "team")
	assert.Contains(t, output, "backend")
}

func TestEditor_View_EmptyState(t *testing.T) {
	editor := New(nil)
	editor.SetSize(80, 20)

	output := editor.View()

	assert.Contains(t, output, "No labels")
	assert.Contains(t, output, "Press 'a' to add")
}

func TestEditor_IsEditing(t *testing.T) {
	editor := New(nil)

	assert.False(t, editor.IsEditing())

	editor.adding = true
	assert.True(t, editor.IsEditing())

	editor.adding = false
	editor.editing = true
	assert.True(t, editor.IsEditing())
}

func TestEditor_TabSwitchesFocus(t *testing.T) {
	editor := New(nil)

	// Start adding
	editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.True(t, editor.focusKey)

	// Tab to value
	editor.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.False(t, editor.focusKey)

	// Tab back to key
	editor.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.True(t, editor.focusKey)
}
