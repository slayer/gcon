package taintedit

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

func TestNew_PopulatesFromInitial(t *testing.T) {
	initial := []gcp.NodeTaint{
		{Key: "dedicated", Value: "gpu", Effect: EffectNoSchedule},
		{Key: "critical", Value: "", Effect: EffectNoExecute},
	}
	e := New(initial)
	got := e.GetTaints()
	require.Len(t, got, 2)
	assert.Equal(t, "dedicated", got[0].Key)
	assert.Equal(t, "gpu", got[0].Value)
	assert.Equal(t, EffectNoSchedule, got[0].Effect)
	assert.Equal(t, "critical", got[1].Key)
	assert.Empty(t, got[1].Value)
	assert.Equal(t, EffectNoExecute, got[1].Effect)
	assert.False(t, e.IsDirty(), "fresh editor must not be dirty")
}

func TestEditor_GetTaintsReturnsCopy(t *testing.T) {
	initial := []gcp.NodeTaint{{Key: "k", Value: "v", Effect: EffectNoSchedule}}
	e := New(initial)
	out := e.GetTaints()
	out[0].Key = "mutated"
	second := e.GetTaints()
	assert.Equal(t, "k", second[0].Key, "internal slice must not share backing with caller")
}

func TestEditor_DeleteRow(t *testing.T) {
	initial := []gcp.NodeTaint{
		{Key: "a", Value: "1", Effect: EffectNoSchedule},
		{Key: "b", Value: "2", Effect: EffectNoExecute},
	}
	e := New(initial)
	// Cursor starts at 0. Pressing 'x' deletes row 0.
	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := e.GetTaints()
	require.Len(t, got, 1)
	assert.Equal(t, "b", got[0].Key)
	assert.True(t, e.IsDirty())
}

func TestEditor_AddTaintFlow(t *testing.T) {
	e := New(nil)
	// a → add mode → type key → Tab → type value → Tab → cycle effect → Enter → commit
	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.True(t, e.IsEditing())
	// Drive the key input by sending the rune messages.
	for _, r := range "dedicated" {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, r := range "gpu" {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	e.Update(tea.KeyMsg{Type: tea.KeyTab}) // focus → effect (default NO_SCHEDULE)
	e.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm

	got := e.GetTaints()
	require.Len(t, got, 1)
	assert.Equal(t, "dedicated", got[0].Key)
	assert.Equal(t, "gpu", got[0].Value)
	assert.Equal(t, EffectNoSchedule, got[0].Effect)
	assert.True(t, e.IsDirty())
}

func TestEditor_CtrlSEmitsSaveRequested(t *testing.T) {
	e := New(nil)
	cmd := e.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(SaveRequestedMsg)
	assert.True(t, ok)
}

func TestEditor_EscEmitsCancelRequested(t *testing.T) {
	e := New(nil)
	cmd := e.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	_, ok := cmd().(CancelRequestedMsg)
	assert.True(t, ok)
}

func TestEditor_IsDirty_AfterDelete(t *testing.T) {
	e := New([]gcp.NodeTaint{{Key: "k", Value: "v", Effect: EffectNoSchedule}})
	assert.False(t, e.IsDirty())
	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.True(t, e.IsDirty())
}

func TestEditor_CycleEffectViaSpace(t *testing.T) {
	e := New(nil)
	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	// type minimal valid key
	for _, r := range "x" {
		e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	e.Update(tea.KeyMsg{Type: tea.KeyTab}) // → value
	e.Update(tea.KeyMsg{Type: tea.KeyTab}) // → effect (default NO_SCHEDULE)
	e.Update(tea.KeyMsg{Type: tea.KeySpace}) // cycle to PREFER_NO_SCHEDULE
	e.Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit

	got := e.GetTaints()
	require.Len(t, got, 1)
	assert.Equal(t, EffectPreferNoSchedule, got[0].Effect)
}

// --- Additional tests beyond the spec ---

func TestNew_EmptyInitial(t *testing.T) {
	e := New(nil)
	assert.NotNil(t, e)
	assert.Empty(t, e.GetTaints())
	assert.False(t, e.IsDirty())
	assert.False(t, e.IsEditing())
}

func TestEditor_Navigation(t *testing.T) {
	initial := []gcp.NodeTaint{
		{Key: "a", Value: "1", Effect: EffectNoSchedule},
		{Key: "b", Value: "2", Effect: EffectNoSchedule},
		{Key: "c", Value: "3", Effect: EffectNoSchedule},
	}
	e := New(initial)
	assert.Equal(t, 0, e.cursor)

	e.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, e.cursor)

	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 2, e.cursor)

	// Can't go past end.
	e.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, e.cursor)

	e.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, e.cursor)

	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, e.cursor)

	// Can't go past start.
	e.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, e.cursor)
}

func TestEditor_IsEditing_States(t *testing.T) {
	e := New(nil)
	assert.False(t, e.IsEditing())

	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.True(t, e.IsEditing())

	e.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, e.IsEditing())
}

func TestEditor_HasTextInputFocused(t *testing.T) {
	e := New(nil)
	assert.False(t, e.HasTextInputFocused())

	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	// starts at key focus
	assert.True(t, e.HasTextInputFocused())

	// move to value
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.True(t, e.HasTextInputFocused())

	// move to effect picker
	e.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.False(t, e.HasTextInputFocused(), "effect picker is not a text input")
}

func TestEditor_ValidationRejectsInvalidKey(t *testing.T) {
	e := New(nil)
	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	// Set an invalid key directly and move to effect then press Enter.
	e.keyInput.SetValue("invalid key with spaces")
	e.editFocus = focusEffect
	e.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.NotEmpty(t, e.err)
	assert.Empty(t, e.GetTaints(), "invalid key must not be committed")
}

func TestEditor_ValidationRejectsInvalidValue(t *testing.T) {
	e := New(nil)
	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	e.keyInput.SetValue("valid-key")
	e.valueInput.SetValue("bad value!")
	e.editFocus = focusEffect
	e.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.NotEmpty(t, e.err)
	assert.Empty(t, e.GetTaints())
}

func TestEditor_CancelEditInAddMode(t *testing.T) {
	e := New(nil)
	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.True(t, e.IsEditing())

	// Esc in edit mode cancels (does NOT emit CancelRequestedMsg).
	cmd := e.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, cmd, "cancel-edit must not emit a tea.Cmd")
	assert.False(t, e.IsEditing())
	assert.Empty(t, e.GetTaints())
}

func TestEditor_EditExistingTaint(t *testing.T) {
	initial := []gcp.NodeTaint{{Key: "oldkey", Value: "oldval", Effect: EffectNoSchedule}}
	e := New(initial)

	// Enter edit mode on the single row.
	e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.True(t, e.IsEditing())
	assert.Equal(t, "oldkey", e.keyInput.Value())
	assert.Equal(t, "oldval", e.valueInput.Value())
	assert.Equal(t, EffectNoSchedule, e.editEffect)

	// Clear key and type a new one.
	e.keyInput.SetValue("newkey")
	e.editFocus = focusEffect
	e.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm

	got := e.GetTaints()
	require.Len(t, got, 1)
	assert.Equal(t, "newkey", got[0].Key)
	assert.True(t, e.IsDirty())
}

func TestEditor_AllEffectsCycle(t *testing.T) {
	e := New(nil)
	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	e.keyInput.SetValue("k")
	e.editFocus = focusEffect

	assert.Equal(t, EffectNoSchedule, e.editEffect)

	e.Update(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, EffectPreferNoSchedule, e.editEffect)

	e.Update(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, EffectNoExecute, e.editEffect)

	e.Update(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, EffectNoSchedule, e.editEffect, "cycle must wrap around")
}

func TestEditor_DeleteRowCursorClamp(t *testing.T) {
	initial := []gcp.NodeTaint{
		{Key: "a", Value: "", Effect: EffectNoSchedule},
		{Key: "b", Value: "", Effect: EffectNoSchedule},
	}
	e := New(initial)
	// Move cursor to last item.
	e.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, e.cursor)

	// Delete it — cursor should stay in bounds.
	e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	require.Len(t, e.GetTaints(), 1)
	assert.Equal(t, 0, e.cursor)
}

func TestEditor_View_ContainsExpectedText(t *testing.T) {
	initial := []gcp.NodeTaint{
		{Key: "dedicated", Value: "gpu", Effect: EffectNoSchedule},
	}
	e := New(initial)
	e.SetSize(100, 30)
	out := e.View()

	assert.Contains(t, out, "Edit Taints")
	assert.Contains(t, out, "dedicated")
	assert.Contains(t, out, "gpu")
	assert.Contains(t, out, EffectNoSchedule)
}

func TestEditor_View_EmptyState(t *testing.T) {
	e := New(nil)
	e.SetSize(100, 30)
	out := e.View()

	assert.Contains(t, out, "No taints")
}
