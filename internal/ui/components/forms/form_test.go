package forms

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewForm(t *testing.T) {
	form := NewForm("Create Instance", FormModeCreate)

	assert.Equal(t, "Create Instance", form.Title)
	assert.Equal(t, FormModeCreate, form.Mode)
	assert.Empty(t, form.sections)
}

func TestFormModeString(t *testing.T) {
	tests := []struct {
		mode     FormMode
		expected string
	}{
		{FormModeCreate, "create"},
		{FormModeEdit, "edit"},
		{FormModeClone, "clone"},
		{FormMode(99), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.mode.String())
	}
}

func TestFormBuilderMethods(t *testing.T) {
	form := NewForm("Create Instance", FormModeCreate).
		SetSubtitle("Create a new VM instance").
		EnableViewport()

	assert.Equal(t, "Create a new VM instance", form.Subtitle)
	assert.True(t, form.useViewport)
}

func TestFormAddSection(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic")).
		AddSection(NewSection("advanced", "Advanced"))

	assert.Equal(t, 2, form.SectionCount())
	assert.NotNil(t, form.GetSection("basic"))
	assert.NotNil(t, form.GetSection("advanced"))
	assert.Nil(t, form.GetSection("nonexistent"))
}

func TestFormSections(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic")).
		AddSection(NewSection("advanced", "Advanced"))

	sections := form.Sections()
	assert.Len(t, sections, 2)
}

func TestFormGetField(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name"))).
		AddSection(NewSection("advanced", "Advanced").
			AddField(NewToggleField("debug", "Debug")))

	// Find field in first section
	name := form.GetField("name")
	assert.NotNil(t, name)
	assert.Equal(t, "Name", name.Label)

	// Find field in second section
	debug := form.GetField("debug")
	assert.NotNil(t, debug)
	assert.Equal(t, "Debug", debug.Label)

	// Non-existent field
	assert.Nil(t, form.GetField("nonexistent"))
}

func TestFormFocusedSection(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name"))).
		AddSection(NewSection("advanced", "Advanced").
			AddField(NewTextField("debug", "Debug")))

	form.Init()

	section := form.FocusedSection()
	assert.NotNil(t, section)
	assert.Equal(t, "basic", section.ID)
}

func TestFormInit(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("readonly", "Read Only").
			AddField(NewReadOnlyField("id", "ID", "123"))).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")))

	form.Init()

	// Should focus first section with editable fields
	assert.Equal(t, 1, form.focusSectionIdx)
	assert.Equal(t, "basic", form.FocusedSection().ID)
}

func TestFormNavigation(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")).
			AddField(NewTextField("email", "Email"))).
		AddSection(NewSection("advanced", "Advanced").
			AddField(NewTextField("debug", "Debug")))

	form.Init()

	// Start at first field of first section
	assert.Equal(t, "name", form.FocusedSection().FocusedField().ID)

	// Tab to next field in same section
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "email", form.FocusedSection().FocusedField().ID)

	// Tab to next section
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "advanced", form.FocusedSection().ID)
	assert.Equal(t, "debug", form.FocusedSection().FocusedField().ID)

	// Tab again should go to action bar
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.True(t, form.focusedOnActions)

	// Shift-Tab back to last field
	form.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	assert.False(t, form.focusedOnActions)
	assert.Equal(t, "debug", form.FocusedSection().FocusedField().ID)
}

func TestFormNavigationWithArrows(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")).
			AddField(NewTextField("email", "Email")))

	form.Init()

	// Down arrow
	form.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, "email", form.FocusedSection().FocusedField().ID)

	// Up arrow
	form.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "name", form.FocusedSection().FocusedField().ID)
}

func TestFormSetSize(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")))

	form.SetSize(80, 24)

	assert.Equal(t, 80, form.width)
	assert.Equal(t, 24, form.height)
	assert.True(t, form.contentReady)
}

func TestFormValidation(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name").SetRequired(true)).
			AddField(NewTextField("email", "Email").SetRequired(true)))

	// Empty fields should fail
	errors := form.Validate()
	assert.Len(t, errors, 2)
	assert.True(t, form.HasErrors())
	assert.True(t, form.showErrors)

	// Fill in values
	form.GetField("name").SetValue("John")
	form.GetField("email").SetValue("john@example.com")

	errors = form.Validate()
	assert.Empty(t, errors)
	assert.False(t, form.HasErrors())
}

func TestFormIsDirty(t *testing.T) {
	form := NewForm("Test", FormModeEdit).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")))

	form.GetField("name").SetValue("original")

	assert.False(t, form.IsDirty())

	// Simulate modification
	form.GetField("name").textInput.SetValue("modified")
	assert.True(t, form.IsDirty())
}

func TestFormGetData(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")).
			AddField(NewNumberField("count", "Count"))).
		AddSection(NewSection("options", "Options").
			AddField(NewToggleField("enabled", "Enabled")))

	form.GetField("name").SetValue("Test")
	form.GetField("count").SetValue(42)
	form.GetField("enabled").SetValue(true)

	data := form.GetData()
	assert.Equal(t, "Test", data["name"])
	assert.Equal(t, int64(42), data["count"])
	assert.Equal(t, true, data["enabled"])
}

func TestFormSetData(t *testing.T) {
	form := NewForm("Test", FormModeEdit).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")).
			AddField(NewNumberField("count", "Count")))

	form.SetData(map[string]any{
		"name":  "John",
		"count": 100,
	})

	assert.Equal(t, "John", form.GetField("name").GetValue())
	assert.Equal(t, int64(100), form.GetField("count").GetValue())
}

func TestFormSubmit(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")))

	form.Init()
	form.GetField("name").SetValue("Test")

	// Ctrl+S to submit
	cmd := form.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)

	msg := cmd()
	submitMsg, ok := msg.(FormSubmitMsg)
	assert.True(t, ok)
	assert.Equal(t, "Test", submitMsg.Data["name"])
}

func TestFormSubmitWithValidationErrors(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name").SetRequired(true)))

	form.Init()
	// Don't set value - leave empty

	// Ctrl+S should not submit when validation fails
	cmd := form.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	assert.Nil(t, cmd)
	assert.True(t, form.showErrors)
}

func TestFormCancel(t *testing.T) {
	form := NewForm("Test", FormModeCreate)
	form.Init()

	// Esc to cancel
	cmd := form.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(FormCancelMsg)
	assert.True(t, ok)
}

func TestFormHelp(t *testing.T) {
	form := NewForm("Test", FormModeCreate)
	form.Init()

	assert.False(t, form.showHelp)

	// Press ? to toggle help
	form.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	assert.True(t, form.showHelp)

	// Press ? again to hide
	form.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	assert.False(t, form.showHelp)
}

func TestFormView(t *testing.T) {
	t.Run("basic form", func(t *testing.T) {
		form := NewForm("Create Instance", FormModeCreate).
			SetSubtitle("Create a new VM").
			AddSection(NewSection("basic", "Basic").
				AddField(NewTextField("name", "Name")))

		view := form.View()
		assert.Contains(t, view, "Create Instance")
		assert.Contains(t, view, "Create a new VM")
		assert.Contains(t, view, "Basic")
		assert.Contains(t, view, "Name")
		assert.Contains(t, view, "[Ctrl+S]")
		assert.Contains(t, view, "[Esc]")
	})

	t.Run("form with validation errors", func(t *testing.T) {
		form := NewForm("Test", FormModeCreate).
			AddSection(NewSection("basic", "Basic").
				AddField(NewTextField("name", "Name").SetRequired(true)))

		form.Validate() // Trigger errors

		view := form.View()
		assert.Contains(t, view, "Please fix")
	})

	t.Run("form with help", func(t *testing.T) {
		form := NewForm("Test", FormModeCreate)
		form.showHelp = true

		view := form.View()
		assert.Contains(t, view, "Keyboard Shortcuts")
		assert.Contains(t, view, "Tab")
		assert.Contains(t, view, "Shift+Tab")
	})

	t.Run("edit mode action bar", func(t *testing.T) {
		form := NewForm("Edit Instance", FormModeEdit)
		view := form.View()
		assert.Contains(t, view, "Save Changes")
	})

	t.Run("clone mode action bar", func(t *testing.T) {
		form := NewForm("Clone Instance", FormModeClone)
		view := form.View()
		assert.Contains(t, view, "Clone")
	})
}

func TestFormShortHelp(t *testing.T) {
	form := NewForm("Test", FormModeCreate)
	help := form.ShortHelp()

	assert.Contains(t, help, "tab")
	assert.Contains(t, help, "ctrl+s")
	assert.Contains(t, help, "esc")
}

func TestFormWithDropdownOpen(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewDropdownField("zone", "Zone").
				SetOptionsFromStrings([]string{"us-central1-a", "us-central1-b", "us-east1-a"})))

	form.Init()

	// Open dropdown
	form.Update(tea.KeyMsg{Type: tea.KeyEnter})

	dropdown := form.GetField("zone")
	assert.True(t, dropdown.dropdownOpen)

	// Down arrow should navigate within dropdown, not form
	form.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, dropdown.selectedIndex) // Moved in dropdown

	// Form should still be on same section
	assert.Equal(t, "basic", form.FocusedSection().ID)
}

func TestFormCollapsedSectionNavigation(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name"))).
		AddSection(NewSection("collapsed", "Collapsed").
			SetCollapsible(true).
			SetCollapsed(true).
			AddField(NewTextField("hidden", "Hidden"))).
		AddSection(NewSection("advanced", "Advanced").
			AddField(NewTextField("debug", "Debug")))

	form.Init()
	assert.Equal(t, "name", form.FocusedSection().FocusedField().ID)

	// Tab should focus collapsed section (so user can expand it)
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "collapsed", form.FocusedSection().ID)
	assert.True(t, form.FocusedSection().Collapsed)
	assert.Nil(t, form.FocusedSection().FocusedField()) // No field focused when collapsed

	// Press Enter to expand
	form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, form.FocusedSection().Collapsed)
	assert.Equal(t, "hidden", form.FocusedSection().FocusedField().ID)

	// Tab to next section
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "advanced", form.FocusedSection().ID)
	assert.Equal(t, "debug", form.FocusedSection().FocusedField().ID)
}

func TestFormFocus(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")))

	// Focus without Init
	form.Focus()

	section := form.FocusedSection()
	require.NotNil(t, section)
	assert.True(t, section.IsFocused())
}

func TestFormActionBarNavigation(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")))

	form.Init()

	// Navigate to action bar
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.True(t, form.focusedOnActions)
	assert.Equal(t, 0, form.actionIndex) // Submit is focused by default

	// Navigate to cancel button using Tab
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 1, form.actionIndex) // Cancel is now focused

	// Navigate back to submit using Shift+Tab
	form.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	assert.Equal(t, 0, form.actionIndex) // Submit is focused again

	// Navigate to cancel using Right arrow
	form.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 1, form.actionIndex) // Cancel is focused

	// Navigate back using Left arrow
	form.Update(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 0, form.actionIndex) // Submit is focused
}

func TestFormCancelButtonActivation(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")))

	form.Init()

	// Navigate to action bar
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.True(t, form.focusedOnActions)

	// Navigate to cancel button
	form.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 1, form.actionIndex)

	// Press Enter to activate cancel
	cmd := form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(FormCancelMsg)
	assert.True(t, ok, "Expected FormCancelMsg when pressing Enter on cancel button")
}

func TestFormSubmitButtonActivation(t *testing.T) {
	form := NewForm("Test", FormModeCreate).
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("name", "Name")))

	form.Init()
	form.GetField("name").SetValue("Test")

	// Navigate to action bar
	form.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.True(t, form.focusedOnActions)
	assert.Equal(t, 0, form.actionIndex) // Submit is focused by default

	// Press Enter to activate submit
	cmd := form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)

	msg := cmd()
	submitMsg, ok := msg.(FormSubmitMsg)
	assert.True(t, ok, "Expected FormSubmitMsg when pressing Enter on submit button")
	assert.Equal(t, "Test", submitMsg.Data["name"])
}

func TestScrollToFocusedWithOpenDropdown(t *testing.T) {
	// Create a form with enough fields that the dropdown near the bottom
	// extends below the viewport when opened
	form := NewForm("Test", FormModeCreate).
		EnableViewport().
		AddSection(NewSection("basic", "Basic").
			AddField(NewTextField("f1", "Field 1")).
			AddField(NewTextField("f2", "Field 2")).
			AddField(NewTextField("f3", "Field 3")).
			AddField(NewTextField("f4", "Field 4")).
			AddField(NewTextField("f5", "Field 5")).
			AddField(NewDropdownField("dropdown", "Dropdown").
				SetOptionsFromStrings([]string{
					"opt1", "opt2", "opt3", "opt4", "opt5",
					"opt6", "opt7", "opt8", "opt9", "opt10",
				})))

	// Small viewport so the dropdown overflows
	form.SetSize(80, 28) // contentHeight = 28-8 = 20
	form.Init()

	// Navigate to the dropdown field (5 Tabs to skip 5 text fields)
	for range 5 {
		form.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	require.Equal(t, "dropdown", form.FocusedSection().FocusedField().ID)

	// Render once to populate viewport content (SetYOffset clamps based on content)
	form.View()

	// Record offset before opening dropdown
	offsetBefore := form.viewport.YOffset

	// Open the dropdown — EstimatedHeight jumps from 4 to ~13
	form.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Render again so viewport content reflects expanded dropdown
	form.View()
	dropdown := form.GetField("dropdown")
	require.True(t, dropdown.dropdownOpen, "dropdown should be open")

	fieldHeight := dropdown.EstimatedHeight()
	assert.Greater(t, fieldHeight, 4, "open dropdown should be taller than default")

	viewportHeight := form.viewport.Height
	currentTop := form.viewport.YOffset

	// The viewport should have scrolled down when the dropdown opened
	assert.Greater(t, currentTop, offsetBefore,
		"viewport should scroll down when dropdown opens near bottom")

	// The dropdown's rendered content should be within the visible viewport.
	// We verify this by checking the viewport output contains dropdown options.
	viewOutput := form.View()
	assert.Contains(t, viewOutput, "opt1", "first dropdown option should be visible")
	assert.Contains(t, viewOutput, "opt5", "middle dropdown option should be visible")

	// Sanity: viewport height hasn't changed
	assert.Equal(t, viewportHeight, form.viewport.Height)
}

func TestFormHasTextInputFocused(t *testing.T) {
	t.Run("text field focused", func(t *testing.T) {
		form := NewForm("Test", FormModeCreate).
			AddSection(NewSection("basic", "Basic").
				AddField(NewTextField("name", "Name")))

		form.Init()
		// After Init, the text field should be focused
		assert.True(t, form.HasTextInputFocused())
	})

	t.Run("toggle field focused", func(t *testing.T) {
		form := NewForm("Test", FormModeCreate).
			AddSection(NewSection("basic", "Basic").
				AddField(NewToggleField("enabled", "Enabled")))

		form.Init()
		// Toggle field is not a text input
		assert.False(t, form.HasTextInputFocused())
	})

	t.Run("action bar focused", func(t *testing.T) {
		form := NewForm("Test", FormModeCreate).
			AddSection(NewSection("basic", "Basic").
				AddField(NewTextField("name", "Name")))

		form.Init()
		// Navigate to action bar
		form.Update(tea.KeyMsg{Type: tea.KeyTab})
		assert.True(t, form.focusedOnActions)
		// When on action bar, text input is not focused
		assert.False(t, form.HasTextInputFocused())
	})

	t.Run("textarea field focused", func(t *testing.T) {
		form := NewForm("Test", FormModeCreate).
			AddSection(NewSection("basic", "Basic").
				AddField(NewTextAreaField("script", "Script")))

		form.Init()
		// TextArea is a text input
		assert.True(t, form.HasTextInputFocused())
	})
}
