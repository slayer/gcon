package forms

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewField(t *testing.T) {
	tests := []struct {
		name      string
		fieldType FieldType
	}{
		{name: "text field", fieldType: FieldText},
		{name: "number field", fieldType: FieldNumber},
		{name: "dropdown field", fieldType: FieldDropdown},
		{name: "multiselect field", fieldType: FieldMultiSelect},
		{name: "toggle field", fieldType: FieldToggle},
		{name: "readonly field", fieldType: FieldReadOnly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := NewField("test-id", "Test Label", tt.fieldType)
			assert.Equal(t, "test-id", field.ID)
			assert.Equal(t, "Test Label", field.Label)
			assert.Equal(t, tt.fieldType, field.Type)
		})
	}
}

func TestFieldBuilderMethods(t *testing.T) {
	field := NewTextField("name", "Name").
		SetRequired(true).
		SetHelpText("Enter your name").
		SetPlaceholder("John Doe").
		SetCharLimit(50)

	assert.True(t, field.Required)
	assert.Equal(t, "Enter your name", field.HelpText)
	assert.Equal(t, "John Doe", field.Placeholder)
}

func TestTextFieldValue(t *testing.T) {
	field := NewTextField("name", "Name")
	field.SetValue("Hello World")

	assert.Equal(t, "Hello World", field.GetValue())
	assert.Equal(t, "Hello World", field.GetStringValue())
}

func TestNumberFieldValue(t *testing.T) {
	field := NewNumberField("count", "Count")

	// Set int value
	field.SetValue(42)
	assert.Equal(t, int64(42), field.GetValue())
	assert.Equal(t, "42", field.GetStringValue())

	// Set int64 value
	field.SetValue(int64(100))
	assert.Equal(t, int64(100), field.GetValue())

	// Set float64 value (truncates)
	field.SetValue(float64(50.7))
	assert.Equal(t, int64(50), field.GetValue())
}

func TestDropdownFieldValue(t *testing.T) {
	field := NewDropdownField("zone", "Zone").
		SetOptionsFromStrings([]string{"us-central1-a", "us-central1-b", "us-east1-a"})

	// Set by value
	field.SetValue("us-central1-b")
	assert.Equal(t, "us-central1-b", field.GetValue())
	assert.Equal(t, 1, field.selectedIndex)
}

func TestDropdownFieldWithOptions(t *testing.T) {
	field := NewDropdownField("size", "Size").
		SetOptions([]Option{
			{Value: "small", Label: "Small (1 vCPU)", Description: "Good for dev"},
			{Value: "medium", Label: "Medium (2 vCPU)", Description: "Good for staging"},
			{Value: "large", Label: "Large (4 vCPU)", Description: "Good for production"},
		})

	field.SetValue("medium")
	assert.Equal(t, "medium", field.GetValue())
	assert.Equal(t, "Medium (2 vCPU)", field.Options[field.selectedIndex].Label)
}

func TestMultiSelectFieldValue(t *testing.T) {
	field := NewMultiSelectField("tags", "Tags").
		SetOptionsFromStrings([]string{"http-server", "https-server", "allow-ssh"})

	// Set multiple selections
	field.SetValue([]string{"http-server", "allow-ssh"})
	value := field.GetValue().([]string) //nolint:errcheck // Test knows type
	assert.Len(t, value, 2)
	assert.Contains(t, value, "http-server")
	assert.Contains(t, value, "allow-ssh")

	assert.Contains(t, field.GetStringValue(), "http-server")
	assert.Contains(t, field.GetStringValue(), "allow-ssh")
}

func TestToggleFieldValue(t *testing.T) {
	field := NewToggleField("enabled", "Enabled")

	// Default is false
	assert.Equal(t, false, field.GetValue())
	assert.Equal(t, "No", field.GetStringValue())

	// Set to true
	field.SetValue(true)
	assert.Equal(t, true, field.GetValue())
	assert.Equal(t, "Yes", field.GetStringValue())
}

func TestReadOnlyFieldValue(t *testing.T) {
	field := NewReadOnlyField("status", "Status", "Running")

	assert.Equal(t, "Running", field.GetValue())
	assert.Equal(t, "Running", field.GetStringValue())
	assert.False(t, field.IsEditable())
}

func TestFieldFocus(t *testing.T) {
	field := NewTextField("name", "Name")

	assert.False(t, field.IsFocused())

	field.Focus()
	assert.True(t, field.IsFocused())

	field.Blur()
	assert.False(t, field.IsFocused())
}

func TestFieldValidation(t *testing.T) {
	t.Run("required validation", func(t *testing.T) {
		field := NewTextField("name", "Name").SetRequired(true)

		// Empty value should fail
		err := field.Validate()
		assert.Error(t, err)
		assert.True(t, field.HasError())

		// Non-empty value should pass
		field.SetValue("John")
		err = field.Validate()
		assert.NoError(t, err)
		assert.False(t, field.HasError())
	})

	t.Run("custom validator", func(t *testing.T) {
		field := NewTextField("name", "Name").
			SetValidator(ValidateGCPResourceName)

		// Invalid name
		field.SetValue("Invalid_Name")
		err := field.Validate()
		assert.Error(t, err)

		// Valid name
		field.SetValue("valid-name")
		err = field.Validate()
		assert.NoError(t, err)
	})

	t.Run("composed validators", func(t *testing.T) {
		field := NewNumberField("size", "Size").
			SetValidator(ValidateNumber(1, 100))

		field.SetValue(50)
		assert.NoError(t, field.Validate())

		field.SetValue(150)
		assert.Error(t, field.Validate())
	})
}

func TestFieldIsDirty(t *testing.T) {
	field := NewTextField("name", "Name")
	field.SetValue("original")

	// After SetValue, originalValue is set, so not dirty
	assert.False(t, field.IsDirty())

	// Simulate user edit by directly modifying textInput
	field.textInput.SetValue("modified")
	assert.True(t, field.IsDirty())
}

func TestFieldSetSize(t *testing.T) {
	field := NewTextField("name", "Name")
	field.SetSize(80, 24)

	assert.Equal(t, 80, field.width)
	assert.Equal(t, 24, field.height)
}

func TestFieldView(t *testing.T) {
	t.Run("text field view", func(t *testing.T) {
		field := NewTextField("name", "Name").
			SetRequired(true).
			SetHelpText("Enter your name")

		view := field.View()
		assert.Contains(t, view, "Name")
		assert.Contains(t, view, "*") // Required indicator
		assert.Contains(t, view, "Enter your name")
	})

	t.Run("toggle field view", func(t *testing.T) {
		field := NewToggleField("enabled", "Enabled")
		view := field.View()
		assert.Contains(t, view, "Enabled")
		assert.Contains(t, view, "OFF")

		field.SetValue(true)
		view = field.View()
		assert.Contains(t, view, "ON")
	})

	t.Run("dropdown field view", func(t *testing.T) {
		field := NewDropdownField("zone", "Zone").
			SetOptionsFromStrings([]string{"us-central1-a", "us-central1-b"})
		field.SetValue("us-central1-a")

		view := field.View()
		assert.Contains(t, view, "Zone")
		assert.Contains(t, view, "us-central1-a")
		assert.Contains(t, view, "▼") // Dropdown indicator
	})

	t.Run("validation error view", func(t *testing.T) {
		field := NewTextField("name", "Name").SetRequired(true)
		_ = field.Validate() //nolint:errcheck // Intentional: trigger validation state

		view := field.View()
		assert.Contains(t, view, "⚠")
		assert.Contains(t, view, "required")
	})
}

func TestToggleFieldUpdate(t *testing.T) {
	field := NewToggleField("enabled", "Enabled")
	field.Focus()

	// Initial value is false
	assert.Equal(t, false, field.GetValue())

	// Press space to toggle
	cmd := field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	require.NotNil(t, cmd)

	// Value should be toggled
	assert.Equal(t, true, field.GetValue())

	// Check that FieldChangedMsg is returned
	msg := cmd()
	changedMsg, ok := msg.(FieldChangedMsg)
	assert.True(t, ok)
	assert.Equal(t, "enabled", changedMsg.FieldID)
	assert.Equal(t, true, changedMsg.Value)
}

func TestDropdownFieldNavigation(t *testing.T) {
	field := NewDropdownField("zone", "Zone").
		SetOptionsFromStrings([]string{"us-central1-a", "us-central1-b", "us-east1-a"})
	field.Focus()
	field.SetValue("us-central1-a")

	// Open dropdown
	field.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, field.dropdownOpen)

	// Navigate down
	field.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, field.selectedIndex)

	// Navigate down again
	field.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, field.selectedIndex)

	// Navigate up
	field.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, field.selectedIndex)

	// Select with enter
	cmd := field.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, field.dropdownOpen)
	assert.Equal(t, "us-central1-b", field.GetValue())
	require.NotNil(t, cmd)
}

func TestDropdownEmptyOptionsShowsPlaceholder(t *testing.T) {
	field := NewDropdownField("machine_type", "Machine Type").
		SetRequired(true)
	field.Focus()

	// Open dropdown with no options — should open but show placeholder
	field.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, field.dropdownOpen, "dropdown should open even with no options")

	view := field.View()
	assert.Contains(t, view, "no options available", "open empty dropdown should show placeholder")

	// Close with Esc
	field.Update(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, field.dropdownOpen)

	// Closed state shows "(none)"
	view = field.View()
	assert.Contains(t, view, "(none)")
}

func TestDropdownRenderShowsOptionsWhenOpen(t *testing.T) {
	field := NewDropdownField("zone", "Zone").
		SetOptionsFromStrings([]string{"us-central1-a", "us-central1-b"})
	field.Focus()
	field.SetValue("us-central1-a")

	// Open dropdown
	field.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, field.dropdownOpen)

	view := field.View()
	assert.Contains(t, view, "us-central1-a")
	assert.Contains(t, view, "us-central1-b")
}

func TestDropdownEstimatedHeight(t *testing.T) {
	field := NewDropdownField("zone", "Zone").
		SetOptionsFromStrings([]string{"a", "b", "c"})

	// Closed dropdown without help text: label + input + margin = 3
	assert.Equal(t, 3, field.EstimatedHeight())

	// Open dropdown: 3 base + (3-1) options = 5
	field.Focus()
	field.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, 5, field.EstimatedHeight())

	// With help text: adds 1 line
	field2 := NewDropdownField("region", "Region").
		SetOptionsFromStrings([]string{"a", "b"}).
		SetHelpText("Select a region")
	assert.Equal(t, 4, field2.EstimatedHeight()) // 3 base + 1 help
}

func TestDropdownScrollableWindow(t *testing.T) {
	// Create dropdown with more options than dropdownMaxVisible
	opts := make([]string, 20)
	for i := range opts {
		opts[i] = fmt.Sprintf("option-%02d", i)
	}
	field := NewDropdownField("big", "Big List").
		SetOptionsFromStrings(opts)
	field.Focus()
	field.SetValue("option-00")

	// Open dropdown
	field.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, field.dropdownOpen)

	view := field.View()
	// Should show first 10 options
	assert.Contains(t, view, "option-00")
	assert.Contains(t, view, "option-09")
	// Should NOT show option-10 (beyond visible window)
	assert.NotContains(t, view, "option-10")
	// Should show "more" indicator at bottom
	assert.Contains(t, view, "↓ 10 more")
	// Should NOT show "more" indicator at top (at start)
	assert.NotContains(t, view, "↑")

	// Navigate down past visible window
	for range 10 {
		field.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	assert.Equal(t, 10, field.selectedIndex)

	view = field.View()
	// Now option-10 should be visible, option-00 may not
	assert.Contains(t, view, "option-10")
	// Should show top scroll indicator
	assert.Contains(t, view, "↑")
}

func TestDropdownSetOptionsShrinkClampsState(t *testing.T) {
	// Regression: replacing a long options list with a shorter one while
	// selectedIndex pointed past the new list caused an index-out-of-range panic.
	opts := make([]string, 50)
	for i := range opts {
		opts[i] = fmt.Sprintf("machine-type-%02d", i)
	}
	field := NewDropdownField("mt", "Machine Type").
		SetOptionsFromStrings(opts)
	field.Focus()

	// Navigate to index 30 (well past any short replacement list)
	field.selectedIndex = 30
	field.dropdownScrollOffset = 20

	// Replace with a short list — must not panic
	field.SetOptionsFromStrings([]string{"e2-micro", "e2-small", "e2-medium"})

	assert.Equal(t, 0, field.selectedIndex, "selectedIndex must be clamped to valid range")
	assert.Equal(t, 0, field.dropdownScrollOffset, "scroll offset must be reset")
	assert.False(t, field.dropdownOpen, "dropdown should close on option replacement to empty-ish list")

	// Rendering must not panic
	view := field.View()
	assert.Contains(t, view, "e2-micro")
}

func TestDropdownSetOptionsEmptyCloseDropdown(t *testing.T) {
	field := NewDropdownField("mt", "Machine Type").
		SetOptionsFromStrings([]string{"a", "b", "c"})
	field.Focus()
	field.selectedIndex = 2
	field.dropdownOpen = true

	// Replace with empty list
	field.SetOptions(nil)

	assert.Equal(t, 0, field.selectedIndex)
	assert.False(t, field.dropdownOpen, "dropdown must close when options become empty")

	// Rendering must not panic
	view := field.View()
	assert.NotEmpty(t, view)
}

func TestMultiSelectFieldNavigation(t *testing.T) {
	field := NewMultiSelectField("tags", "Tags").
		SetOptionsFromStrings([]string{"http-server", "https-server", "allow-ssh"})
	field.Focus()

	// Open dropdown
	field.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, field.dropdownOpen)

	// Select first option with space
	field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	// Navigate down
	field.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Select second option
	field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	// Check selections
	value := field.GetValue().([]string) //nolint:errcheck // Test knows type
	assert.Len(t, value, 2)
	assert.Contains(t, value, "http-server")
	assert.Contains(t, value, "https-server")

	// Close with enter
	field.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, field.dropdownOpen)
}

func TestReadOnlyFieldUpdate(t *testing.T) {
	field := NewReadOnlyField("status", "Status", "Running")
	field.Focus()

	// Updates should be ignored
	cmd := field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Nil(t, cmd)
	assert.Equal(t, "Running", field.GetValue())
}

func TestFieldTypeString(t *testing.T) {
	tests := []struct {
		fieldType FieldType
		expected  string
	}{
		{FieldText, "text"},
		{FieldNumber, "number"},
		{FieldDropdown, "dropdown"},
		{FieldMultiSelect, "multiselect"},
		{FieldToggle, "toggle"},
		{FieldReadOnly, "readonly"},
		{FieldTextArea, "textarea"},
		{FieldType(99), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.fieldType.String())
	}
}

func TestTextAreaFieldValue(t *testing.T) {
	field := NewTextAreaField("script", "Script")

	// Set multi-line value
	script := "#!/bin/bash\necho 'Hello'\necho 'World'"
	field.SetValue(script)
	assert.Equal(t, script, field.GetValue())

	// Test SetRows
	field.SetRows(6)
	assert.Equal(t, 6, field.textAreaRows)

	// Test SetShowLineNumbers
	field.SetShowLineNumbers(true)
	assert.True(t, field.textArea.ShowLineNumbers)
}

func TestTextAreaFieldFocus(t *testing.T) {
	field := NewTextAreaField("script", "Script")

	assert.False(t, field.IsFocused())

	field.Focus()
	assert.True(t, field.IsFocused())

	field.Blur()
	assert.False(t, field.IsFocused())
}

func TestTextAreaFieldView(t *testing.T) {
	field := NewTextAreaField("script", "Startup Script").
		SetPlaceholder("#!/bin/bash").
		SetHelpText("Enter your startup script")

	view := field.View()
	assert.Contains(t, view, "Startup Script")
	assert.Contains(t, view, "Enter your startup script")
}

func TestHiddenField(t *testing.T) {
	t.Run("hidden field is not editable", func(t *testing.T) {
		field := NewTextField("ip", "IP Address").SetHidden(true)
		assert.False(t, field.IsEditable())
	})

	t.Run("hidden field renders empty", func(t *testing.T) {
		field := NewTextField("ip", "IP Address").
			SetHidden(true).
			SetValue("10.0.0.1")
		assert.Equal(t, "", field.View())
	})

	t.Run("hidden field skips validation", func(t *testing.T) {
		field := NewTextField("ip", "IP Address").
			SetRequired(true).
			SetHidden(true)
		// Required + hidden + empty value → should still pass
		err := field.Validate()
		assert.NoError(t, err)
		assert.False(t, field.HasError())
	})

	t.Run("hidden field has zero height", func(t *testing.T) {
		field := NewTextField("ip", "IP Address").SetHidden(true)
		assert.Equal(t, 0, field.EstimatedHeight())
	})

	t.Run("SetHidden toggles visibility", func(t *testing.T) {
		field := NewTextField("ip", "IP Address")
		assert.False(t, field.Hidden)
		assert.True(t, field.IsEditable())

		field.SetHidden(true)
		assert.True(t, field.Hidden)
		assert.False(t, field.IsEditable())

		field.SetHidden(false)
		assert.False(t, field.Hidden)
		assert.True(t, field.IsEditable())
	})
}

func TestFieldIsTextInput(t *testing.T) {
	// Text fields that accept character input
	assert.True(t, NewTextField("name", "Name").IsTextInput())
	assert.True(t, NewNumberField("count", "Count").IsTextInput())
	assert.True(t, NewTextAreaField("script", "Script").IsTextInput())

	// Fields that don't accept free text input
	assert.False(t, NewDropdownField("zone", "Zone").IsTextInput())
	assert.False(t, NewMultiSelectField("tags", "Tags").IsTextInput())
	assert.False(t, NewToggleField("enabled", "Enabled").IsTextInput())
	assert.False(t, NewReadOnlyField("status", "Status", "Running").IsTextInput())
}
