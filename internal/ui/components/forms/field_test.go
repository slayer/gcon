package forms

import (
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
	value := field.GetValue().([]string)
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
		_ = field.Validate() // Trigger error

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
	value := field.GetValue().([]string)
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
