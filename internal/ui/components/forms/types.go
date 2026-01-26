// Package forms provides reusable form components for editing GCP resources.
// It includes FormField (individual inputs), FormSection (field groups),
// and FormView (complete forms with navigation and validation).
package forms

import tea "github.com/charmbracelet/bubbletea"

// FieldType defines the type of input for a form field
type FieldType int

const (
	// FieldText is a single-line text input
	FieldText FieldType = iota
	// FieldNumber is a numeric input with optional min/max validation
	FieldNumber
	// FieldDropdown is a single selection from options
	FieldDropdown
	// FieldMultiSelect allows multiple selections
	FieldMultiSelect
	// FieldToggle is a boolean on/off switch
	FieldToggle
	// FieldReadOnly displays a value without allowing edits
	FieldReadOnly
	// FieldTextArea is a multi-line text input
	FieldTextArea
)

// String returns a human-readable name for the field type
func (ft FieldType) String() string {
	switch ft {
	case FieldText:
		return "text"
	case FieldNumber:
		return "number"
	case FieldDropdown:
		return "dropdown"
	case FieldMultiSelect:
		return "multiselect"
	case FieldToggle:
		return "toggle"
	case FieldReadOnly:
		return "readonly"
	case FieldTextArea:
		return "textarea"
	default:
		return "unknown"
	}
}

// FormMode defines whether a form is for creating, editing, or cloning
type FormMode int

const (
	// FormModeCreate is for creating new resources
	FormModeCreate FormMode = iota
	// FormModeEdit is for editing existing resources
	FormModeEdit
	// FormModeClone is for cloning an existing resource
	FormModeClone
)

// String returns a human-readable name for the form mode
func (fm FormMode) String() string {
	switch fm {
	case FormModeCreate:
		return "create"
	case FormModeEdit:
		return "edit"
	case FormModeClone:
		return "clone"
	default:
		return "unknown"
	}
}

// Validator is a function that validates a field value.
// It returns nil if the value is valid, or an error describing the issue.
type Validator func(value any) error

// Option represents a selectable option in dropdown/multiselect fields
type Option struct {
	Value       string // Internal value used for data
	Label       string // Display label shown to user
	Description string // Optional description shown on hover/focus
}

// FormSubmitMsg is sent when the form is submitted via Ctrl+S
type FormSubmitMsg struct {
	Data map[string]any // All field values keyed by field ID
}

// FormCancelMsg is sent when the form is cancelled via Esc
type FormCancelMsg struct{}

// FieldChangedMsg is sent when a field value changes
type FieldChangedMsg struct {
	FieldID string
	Value   any
}

// ValidationErrorMsg is sent when validation fails
type ValidationErrorMsg struct {
	FieldID string
	Error   string
}

// Component defines the interface for form-related UI components
type Component interface {
	Update(msg tea.Msg) tea.Cmd
	View() string
	SetSize(width, height int)
}
