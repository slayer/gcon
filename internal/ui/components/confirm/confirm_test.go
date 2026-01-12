package confirm

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates dialog with provided values", func(t *testing.T) {
		dialog := New("Test Title", "Test message", []string{"detail1", "detail2"})

		assert.Equal(t, "Test Title", dialog.title)
		assert.Equal(t, "Test message", dialog.message)
		assert.Equal(t, []string{"detail1", "detail2"}, dialog.details)
		assert.False(t, dialog.focusedYes, "should default to No for safety")
	})

	t.Run("creates dialog with empty details", func(t *testing.T) {
		dialog := New("Title", "Message", nil)

		assert.Nil(t, dialog.details)
	})
}

func TestSetSize(t *testing.T) {
	dialog := New("Title", "Message", nil)
	dialog.SetSize(60, 20)

	assert.Equal(t, 60, dialog.width)
	assert.Equal(t, 20, dialog.height)
}

func TestUpdate(t *testing.T) {
	t.Run("left key focuses Yes", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.focusedYes = false

		dialog.Update(tea.KeyMsg{Type: tea.KeyLeft})

		assert.True(t, dialog.focusedYes)
	})

	t.Run("right key focuses No", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.focusedYes = true

		dialog.Update(tea.KeyMsg{Type: tea.KeyRight})

		assert.False(t, dialog.focusedYes)
	})

	t.Run("h key focuses Yes", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.focusedYes = false

		dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})

		assert.True(t, dialog.focusedYes)
	})

	t.Run("l key focuses No", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.focusedYes = true

		dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})

		assert.False(t, dialog.focusedYes)
	})

	t.Run("tab toggles focus", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.focusedYes = false

		dialog.Update(tea.KeyMsg{Type: tea.KeyTab})
		assert.True(t, dialog.focusedYes)

		dialog.Update(tea.KeyMsg{Type: tea.KeyTab})
		assert.False(t, dialog.focusedYes)
	})

	t.Run("enter confirms when Yes focused", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.focusedYes = true

		cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEnter})

		require.NotNil(t, cmd)
		msg := cmd()
		_, ok := msg.(ConfirmMsg)
		assert.True(t, ok, "expected ConfirmMsg")
	})

	t.Run("enter cancels when No focused", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.focusedYes = false

		cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEnter})

		require.NotNil(t, cmd)
		msg := cmd()
		_, ok := msg.(CancelMsg)
		assert.True(t, ok, "expected CancelMsg")
	})

	t.Run("y key confirms directly", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.focusedYes = false // Even when No is focused

		cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

		require.NotNil(t, cmd)
		msg := cmd()
		_, ok := msg.(ConfirmMsg)
		assert.True(t, ok, "expected ConfirmMsg")
	})

	t.Run("n key cancels directly", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.focusedYes = true // Even when Yes is focused

		cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

		require.NotNil(t, cmd)
		msg := cmd()
		_, ok := msg.(CancelMsg)
		assert.True(t, ok, "expected CancelMsg")
	})

	t.Run("esc key cancels", func(t *testing.T) {
		dialog := New("Title", "Message", nil)

		cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEsc})

		require.NotNil(t, cmd)
		msg := cmd()
		_, ok := msg.(CancelMsg)
		assert.True(t, ok, "expected CancelMsg")
	})

	t.Run("q key cancels", func(t *testing.T) {
		dialog := New("Title", "Message", nil)

		cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

		require.NotNil(t, cmd)
		msg := cmd()
		_, ok := msg.(CancelMsg)
		assert.True(t, ok, "expected CancelMsg")
	})
}

func TestView(t *testing.T) {
	t.Run("renders title", func(t *testing.T) {
		dialog := New("Delete File", "Are you sure?", nil)
		dialog.SetSize(50, 15)

		view := dialog.View()

		assert.Contains(t, view, "Delete File")
	})

	t.Run("renders message", func(t *testing.T) {
		dialog := New("Title", "This is a test message", nil)
		dialog.SetSize(50, 15)

		view := dialog.View()

		assert.Contains(t, view, "This is a test message")
	})

	t.Run("renders details", func(t *testing.T) {
		dialog := New("Title", "Message", []string{"file1.txt", "file2.txt"})
		dialog.SetSize(50, 15)

		view := dialog.View()

		assert.Contains(t, view, "file1.txt")
		assert.Contains(t, view, "file2.txt")
	})

	t.Run("renders buttons", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.SetSize(50, 15)

		view := dialog.View()

		assert.Contains(t, view, "Yes")
		assert.Contains(t, view, "No")
	})

	t.Run("renders help text", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.SetSize(50, 15)

		view := dialog.View()

		assert.Contains(t, view, "enter")
		assert.Contains(t, view, "esc")
	})

	t.Run("renders with border", func(t *testing.T) {
		dialog := New("Title", "Message", nil)
		dialog.SetSize(50, 15)

		view := dialog.View()

		// Check for rounded border characters
		assert.True(t, strings.Contains(view, "╭") || strings.Contains(view, "┌"),
			"expected border character in view")
	})
}

func TestIsFocusedYes(t *testing.T) {
	dialog := New("Title", "Message", nil)

	assert.False(t, dialog.IsFocusedYes())

	dialog.focusedYes = true
	assert.True(t, dialog.IsFocusedYes())
}
