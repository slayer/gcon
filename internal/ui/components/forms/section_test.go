package forms

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewSection(t *testing.T) {
	section := NewSection("basic", "Basic Configuration")

	assert.Equal(t, "basic", section.ID)
	assert.Equal(t, "Basic Configuration", section.Title)
	assert.Empty(t, section.fields)
	assert.False(t, section.Collapsible)
	assert.False(t, section.Collapsed)
}

func TestSectionBuilderMethods(t *testing.T) {
	section := NewSection("advanced", "Advanced").
		SetIcon("⚙").
		SetDescription("Advanced settings").
		SetCollapsible(true).
		SetCollapsed(true)

	assert.Equal(t, "⚙", section.Icon)
	assert.Equal(t, "Advanced settings", section.Description)
	assert.True(t, section.Collapsible)
	assert.True(t, section.Collapsed)
}

func TestSectionAddField(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name")).
		AddField(NewTextField("email", "Email"))

	assert.Equal(t, 2, section.FieldCount())
	assert.NotNil(t, section.GetField("name"))
	assert.NotNil(t, section.GetField("email"))
	assert.Nil(t, section.GetField("nonexistent"))
}

func TestSectionFields(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name")).
		AddField(NewReadOnlyField("id", "ID", "123"))

	fields := section.Fields()
	assert.Len(t, fields, 2)

	// Test editable field count
	assert.Equal(t, 1, section.EditableFieldCount())
}

func TestSectionFocus(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name")).
		AddField(NewTextField("email", "Email"))

	assert.False(t, section.IsFocused())

	section.Focus()
	assert.True(t, section.IsFocused())

	// Should focus first editable field
	focusedField := section.FocusedField()
	assert.NotNil(t, focusedField)
	assert.Equal(t, "name", focusedField.ID)
	assert.True(t, focusedField.IsFocused())

	section.Blur()
	assert.False(t, section.IsFocused())
}

func TestSectionFocusFirstEditable(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewReadOnlyField("id", "ID", "123")).
		AddField(NewTextField("name", "Name")).
		AddField(NewTextField("email", "Email"))

	// Focus should skip readonly and go to first editable
	found := section.FocusFirstEditable()
	assert.True(t, found)
	assert.Equal(t, "name", section.FocusedField().ID)
}

func TestSectionFocusLastEditable(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name")).
		AddField(NewTextField("email", "Email")).
		AddField(NewReadOnlyField("status", "Status", "OK"))

	// Focus should skip readonly and go to last editable
	found := section.FocusLastEditable()
	assert.True(t, found)
	assert.Equal(t, "email", section.FocusedField().ID)
}

func TestSectionNextField(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name")).
		AddField(NewReadOnlyField("id", "ID", "123")).
		AddField(NewTextField("email", "Email"))

	section.FocusFirstEditable()
	assert.Equal(t, "name", section.FocusedField().ID)

	// Next should skip readonly
	moved := section.NextField()
	assert.True(t, moved)
	assert.Equal(t, "email", section.FocusedField().ID)

	// No more fields
	moved = section.NextField()
	assert.False(t, moved)
}

func TestSectionPrevField(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name")).
		AddField(NewReadOnlyField("id", "ID", "123")).
		AddField(NewTextField("email", "Email"))

	section.FocusLastEditable()
	assert.Equal(t, "email", section.FocusedField().ID)

	// Prev should skip readonly
	moved := section.PrevField()
	assert.True(t, moved)
	assert.Equal(t, "name", section.FocusedField().ID)

	// No more fields
	moved = section.PrevField()
	assert.False(t, moved)
}

func TestSectionNoEditableFields(t *testing.T) {
	section := NewSection("info", "Info").
		AddField(NewReadOnlyField("id", "ID", "123")).
		AddField(NewReadOnlyField("status", "Status", "OK"))

	found := section.FocusFirstEditable()
	assert.False(t, found)
	assert.Nil(t, section.FocusedField())
}

func TestSectionSetSize(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name"))

	section.SetSize(80, 24)
	assert.Equal(t, 80, section.width)
	assert.Equal(t, 24, section.height)

	// Fields should have adjusted width
	assert.Equal(t, 76, section.fields[0].width) // 80 - 4 padding
}

func TestSectionValidation(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name").SetRequired(true)).
		AddField(NewTextField("email", "Email").SetRequired(true))

	// Empty fields should fail
	errors := section.Validate()
	assert.Len(t, errors, 2)
	assert.True(t, section.HasErrors())

	// Fill in values
	section.GetField("name").SetValue("John")
	section.GetField("email").SetValue("john@example.com")

	errors = section.Validate()
	assert.Empty(t, errors)
	assert.False(t, section.HasErrors())
}

func TestSectionIsDirty(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name")).
		AddField(NewTextField("email", "Email"))

	section.GetField("name").SetValue("original")
	section.GetField("email").SetValue("original@example.com")

	assert.False(t, section.IsDirty())

	// Modify a field
	section.GetField("name").textInput.SetValue("modified")
	assert.True(t, section.IsDirty())
}

func TestSectionGetData(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name")).
		AddField(NewNumberField("age", "Age")).
		AddField(NewToggleField("active", "Active"))

	section.GetField("name").SetValue("John")
	section.GetField("age").SetValue(30)
	section.GetField("active").SetValue(true)

	data := section.GetData()
	assert.Equal(t, "John", data["name"])
	assert.Equal(t, int64(30), data["age"])
	assert.Equal(t, true, data["active"])
}

func TestSectionToggleCollapse(t *testing.T) {
	section := NewSection("basic", "Basic").
		SetCollapsible(true)

	assert.False(t, section.Collapsed)

	section.ToggleCollapse()
	assert.True(t, section.Collapsed)

	section.ToggleCollapse()
	assert.False(t, section.Collapsed)
}

func TestSectionCollapseNotAllowed(t *testing.T) {
	section := NewSection("basic", "Basic")
	// Not marked as collapsible

	section.ToggleCollapse()
	assert.False(t, section.Collapsed) // Should remain uncollapsed
}

func TestSectionView(t *testing.T) {
	t.Run("basic section", func(t *testing.T) {
		section := NewSection("basic", "Basic Configuration").
			AddField(NewTextField("name", "Name"))

		view := section.View()
		assert.Contains(t, view, "Basic Configuration")
		assert.Contains(t, view, "Name")
	})

	t.Run("section with icon", func(t *testing.T) {
		section := NewSection("settings", "Settings").
			SetIcon("⚙")

		view := section.View()
		assert.Contains(t, view, "⚙")
		assert.Contains(t, view, "Settings")
	})

	t.Run("collapsible section expanded", func(t *testing.T) {
		section := NewSection("advanced", "Advanced").
			SetCollapsible(true).
			AddField(NewTextField("debug", "Debug"))

		view := section.View()
		assert.Contains(t, view, "▾") // Expanded indicator
		assert.Contains(t, view, "Debug")
	})

	t.Run("collapsible section collapsed", func(t *testing.T) {
		section := NewSection("advanced", "Advanced").
			SetCollapsible(true).
			SetCollapsed(true).
			AddField(NewTextField("debug", "Debug"))

		view := section.View()
		assert.Contains(t, view, "▸")        // Collapsed indicator
		assert.NotContains(t, view, "Debug") // Fields should be hidden
	})

	t.Run("section with description", func(t *testing.T) {
		section := NewSection("basic", "Basic").
			SetDescription("Enter basic information")

		view := section.View()
		assert.Contains(t, view, "Enter basic information")
	})
}

func TestSectionUpdate(t *testing.T) {
	t.Run("navigation with tab", func(t *testing.T) {
		section := NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")).
			AddField(NewTextField("email", "Email"))

		section.Focus()
		assert.Equal(t, "name", section.FocusedField().ID)

		// Tab to next field
		section.Update(tea.KeyMsg{Type: tea.KeyTab})
		assert.Equal(t, "email", section.FocusedField().ID)
	})

	t.Run("navigation with arrows", func(t *testing.T) {
		section := NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")).
			AddField(NewTextField("email", "Email"))

		section.Focus()
		section.FocusLastEditable()

		// Up arrow to previous field
		section.Update(tea.KeyMsg{Type: tea.KeyUp})
		assert.Equal(t, "name", section.FocusedField().ID)
	})

	t.Run("expand collapsed section", func(t *testing.T) {
		section := NewSection("advanced", "Advanced").
			SetCollapsible(true).
			SetCollapsed(true)

		section.Focus()
		assert.True(t, section.Collapsed)

		// Enter to expand
		section.Update(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, section.Collapsed)
	})

	t.Run("collapse expanded section with minus key", func(t *testing.T) {
		section := NewSection("advanced", "Advanced").
			SetCollapsible(true).
			SetCollapsed(false).
			AddField(NewTextField("name", "Name"))

		section.Focus()
		assert.False(t, section.Collapsed)

		// Minus key to collapse
		section.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
		assert.True(t, section.Collapsed)
	})

	t.Run("delegate to focused field", func(t *testing.T) {
		section := NewSection("basic", "Basic").
			AddField(NewToggleField("enabled", "Enabled"))

		section.Focus()
		toggle := section.GetField("enabled")

		assert.Equal(t, false, toggle.GetValue())

		// Space should toggle the field
		section.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
		assert.Equal(t, true, toggle.GetValue())
	})
}

func TestSectionFocusedFieldIndex(t *testing.T) {
	section := NewSection("basic", "Basic").
		AddField(NewTextField("name", "Name")).
		AddField(NewTextField("email", "Email"))

	assert.Equal(t, -1, section.FocusedFieldIndex())

	section.FocusFirstEditable()
	assert.Equal(t, 0, section.FocusedFieldIndex())

	section.NextField()
	assert.Equal(t, 1, section.FocusedFieldIndex())
}
