package forms

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Colors for form fields
var (
	fieldColorSecondary = lipgloss.Color("#34A853")
	fieldColorError     = lipgloss.Color("#EA4335")
	fieldColorMuted     = lipgloss.Color("#9AA0A6")
	fieldColorHint      = lipgloss.Color("#6B7280") // Subtle gray for help text
	fieldColorWhite     = lipgloss.Color("#FFFFFF")
	fieldColorBg        = lipgloss.Color("#1E3A5F") // Input background (dark blue)
	fieldColorBgFocused = lipgloss.Color("#2563EB") // Focused input background (bright blue)
	fieldColorBgMuted   = lipgloss.Color("#1A1A2E") // Disabled/readonly background (dark muted)
)

// fieldKeyMap defines key bindings specific to form fields
type fieldKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Select   key.Binding
	Toggle   key.Binding
	Clear    key.Binding
	NextChar key.Binding
	PrevChar key.Binding
}

func defaultFieldKeyMap() fieldKeyMap {
	return fieldKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "previous option"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "next option"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "select"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" ", "enter"),
			key.WithHelp("space/enter", "toggle"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "clear"),
		),
	}
}

// fieldStyles defines styles for form fields
type fieldStyles struct {
	Label           lipgloss.Style
	LabelRequired   lipgloss.Style
	InputBox        lipgloss.Style // Normal input background
	InputBoxFocused lipgloss.Style // Focused input background
	InputBoxMuted   lipgloss.Style // Disabled/readonly background
	Value           lipgloss.Style
	ValueFocused    lipgloss.Style
	ValueReadOnly   lipgloss.Style
	HelpText        lipgloss.Style
	Error           lipgloss.Style
	Option          lipgloss.Style
	OptionSelected  lipgloss.Style
	OptionHighlight lipgloss.Style
	Toggle          lipgloss.Style
	ToggleOn        lipgloss.Style
	ToggleOff       lipgloss.Style
	Container       lipgloss.Style
}

func defaultFieldStyles() fieldStyles {
	return fieldStyles{
		Label: lipgloss.NewStyle().
			Foreground(fieldColorWhite).
			Bold(true),
		LabelRequired: lipgloss.NewStyle().
			Foreground(fieldColorError),
		InputBox: lipgloss.NewStyle().
			Background(fieldColorBg).
			Padding(0, 1),
		InputBoxFocused: lipgloss.NewStyle().
			Background(fieldColorBgFocused).
			Padding(0, 1),
		InputBoxMuted: lipgloss.NewStyle().
			Background(fieldColorBgMuted).
			Padding(0, 1),
		Value: lipgloss.NewStyle().
			Foreground(fieldColorWhite),
		ValueFocused: lipgloss.NewStyle().
			Foreground(fieldColorWhite),
		ValueReadOnly: lipgloss.NewStyle().
			Foreground(fieldColorMuted),
		HelpText: lipgloss.NewStyle().
			Foreground(fieldColorHint).
			Faint(true),
		Error: lipgloss.NewStyle().
			Foreground(fieldColorError),
		Option: lipgloss.NewStyle().
			Foreground(fieldColorWhite),
		OptionSelected: lipgloss.NewStyle().
			Foreground(fieldColorSecondary).
			Bold(true),
		OptionHighlight: lipgloss.NewStyle().
			Foreground(fieldColorWhite).
			Background(fieldColorBgFocused),
		Toggle: lipgloss.NewStyle().
			Foreground(fieldColorMuted),
		ToggleOn: lipgloss.NewStyle().
			Foreground(fieldColorSecondary).
			Bold(true),
		ToggleOff: lipgloss.NewStyle().
			Foreground(fieldColorMuted),
		Container: lipgloss.NewStyle().
			MarginBottom(1),
	}
}

// Field represents a single form input field
type Field struct {
	// Configuration
	ID          string
	Label       string
	Type        FieldType
	Required    bool
	HelpText    string
	Placeholder string
	Validator   Validator

	// Value storage
	value         any
	originalValue any

	// Options for dropdown/multiselect
	Options       []Option
	selectedIndex int   // For dropdown: currently highlighted option
	selectedSet   []int // For multiselect: indices of selected options

	// UI state
	focused       bool
	dropdownOpen  bool
	validationErr string
	width         int
	height        int
	textInput     textinput.Model
	textArea      textarea.Model
	textAreaRows  int // Number of visible rows for textarea

	// Styling
	styles fieldStyles
	keys   fieldKeyMap
}

// NewField creates a new form field with the given configuration
func NewField(id, label string, fieldType FieldType) *Field {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 40
	// Configure textinput styles - no backgrounds to prevent bleeding
	ti.PromptStyle = lipgloss.NewStyle().Foreground(fieldColorMuted)
	ti.TextStyle = lipgloss.NewStyle().Foreground(fieldColorWhite)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(fieldColorMuted)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(fieldColorWhite)
	ti.Cursor.TextStyle = lipgloss.NewStyle().Foreground(fieldColorWhite)

	// Initialize textarea - focused uses blue background, unfocused renders custom
	ta := textarea.New()
	ta.Placeholder = "Enter text..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // No limit by default
	ta.SetWidth(40)
	ta.SetHeight(4)               // Default 4 rows
	ta.Prompt = ""                // Remove the prompt character
	ta.EndOfBufferCharacter = ' ' // Use space instead of ~ for empty lines
	// Focused style with blue background
	ta.FocusedStyle.Base = lipgloss.NewStyle().Background(fieldColorBgFocused)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(fieldColorBgFocused)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().
		Foreground(fieldColorMuted).Background(fieldColorBgFocused)
	ta.FocusedStyle.Text = lipgloss.NewStyle().
		Foreground(fieldColorWhite).Background(fieldColorBgFocused)
	ta.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Background(fieldColorBgFocused)
	ta.FocusedStyle.LineNumber = lipgloss.NewStyle().Background(fieldColorBgFocused)
	ta.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Background(fieldColorBgFocused)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Background(fieldColorBgFocused)
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(fieldColorWhite)
	// Blurred style (not used - we render custom box when unfocused)
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(fieldColorMuted)
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(fieldColorWhite)
	ta.BlurredStyle.CursorLineNumber = lipgloss.NewStyle()
	ta.BlurredStyle.LineNumber = lipgloss.NewStyle()
	ta.BlurredStyle.EndOfBuffer = lipgloss.NewStyle()
	ta.BlurredStyle.Prompt = lipgloss.NewStyle()

	return &Field{
		ID:           id,
		Label:        label,
		Type:         fieldType,
		styles:       defaultFieldStyles(),
		keys:         defaultFieldKeyMap(),
		textInput:    ti,
		textArea:     ta,
		textAreaRows: 4, // Default height
		selectedSet:  []int{},
	}
}

// NewTextField creates a text input field
func NewTextField(id, label string) *Field {
	return NewField(id, label, FieldText)
}

// NewNumberField creates a numeric input field
func NewNumberField(id, label string) *Field {
	f := NewField(id, label, FieldNumber)
	f.value = int64(0)
	return f
}

// NewDropdownField creates a dropdown selection field
func NewDropdownField(id, label string) *Field {
	return NewField(id, label, FieldDropdown)
}

// NewMultiSelectField creates a multi-selection field
func NewMultiSelectField(id, label string) *Field {
	return NewField(id, label, FieldMultiSelect)
}

// NewToggleField creates a boolean toggle field
func NewToggleField(id, label string) *Field {
	f := NewField(id, label, FieldToggle)
	f.value = false
	return f
}

// NewReadOnlyField creates a read-only display field
func NewReadOnlyField(id, label string, value string) *Field {
	f := NewField(id, label, FieldReadOnly)
	f.value = value
	return f
}

// NewTextAreaField creates a multi-line text input field
func NewTextAreaField(id, label string) *Field {
	return NewField(id, label, FieldTextArea)
}

// SetRows sets the number of visible rows for textarea fields
func (f *Field) SetRows(rows int) *Field {
	if rows < 2 {
		rows = 2
	}
	f.textAreaRows = rows
	f.textArea.SetHeight(rows)
	return f
}

// SetShowLineNumbers enables or disables line numbers for textarea
func (f *Field) SetShowLineNumbers(show bool) *Field {
	f.textArea.ShowLineNumbers = show
	return f
}

// Builder methods for configuration

// SetRequired marks the field as required
func (f *Field) SetRequired(required bool) *Field {
	f.Required = required
	return f
}

// SetHelpText sets the help text shown below the field
func (f *Field) SetHelpText(text string) *Field {
	f.HelpText = text
	return f
}

// SetPlaceholder sets the placeholder text for text/number/textarea fields
func (f *Field) SetPlaceholder(placeholder string) *Field {
	f.Placeholder = placeholder
	f.textInput.Placeholder = placeholder
	f.textArea.Placeholder = placeholder
	return f
}

// SetValidator sets the validation function for the field
func (f *Field) SetValidator(validator Validator) *Field {
	f.Validator = validator
	return f
}

// SetOptions sets the available options for dropdown/multiselect fields
func (f *Field) SetOptions(options []Option) *Field {
	f.Options = options
	return f
}

// SetOptionsFromStrings creates options from simple string values
func (f *Field) SetOptionsFromStrings(values []string) *Field {
	options := make([]Option, len(values))
	for i, v := range values {
		options[i] = Option{Value: v, Label: v}
	}
	f.Options = options
	return f
}

// SetCharLimit sets the character limit for text input
func (f *Field) SetCharLimit(limit int) *Field {
	f.textInput.CharLimit = limit
	return f
}

// Value accessors

// SetValue sets the field value
func (f *Field) SetValue(value any) *Field {
	f.value = value
	f.originalValue = value

	switch f.Type {
	case FieldText:
		if s, ok := value.(string); ok {
			f.textInput.SetValue(s)
		}
	case FieldTextArea:
		if s, ok := value.(string); ok {
			f.textArea.SetValue(s)
		}
	case FieldNumber:
		switch v := value.(type) {
		case int:
			f.value = int64(v)
			f.textInput.SetValue(strconv.FormatInt(int64(v), 10))
		case int64:
			f.value = v
			f.textInput.SetValue(strconv.FormatInt(v, 10))
		case float64:
			f.value = int64(v)
			f.textInput.SetValue(strconv.FormatInt(int64(v), 10))
		}
	case FieldDropdown:
		if s, ok := value.(string); ok {
			for i, opt := range f.Options {
				if opt.Value == s {
					f.selectedIndex = i
					break
				}
			}
		}
	case FieldMultiSelect:
		if selected, ok := value.([]string); ok {
			f.selectedSet = []int{}
			for _, sel := range selected {
				for i, opt := range f.Options {
					if opt.Value == sel {
						f.selectedSet = append(f.selectedSet, i)
						break
					}
				}
			}
		}
	case FieldToggle:
		if b, ok := value.(bool); ok {
			f.value = b
		}
	}

	return f
}

// GetValue returns the current field value
func (f *Field) GetValue() any {
	switch f.Type {
	case FieldText:
		return f.textInput.Value()
	case FieldTextArea:
		return f.textArea.Value()
	case FieldNumber:
		if val, err := strconv.ParseInt(f.textInput.Value(), 10, 64); err == nil {
			return val
		}
		return f.value
	case FieldDropdown:
		if f.selectedIndex >= 0 && f.selectedIndex < len(f.Options) {
			return f.Options[f.selectedIndex].Value
		}
		return ""
	case FieldMultiSelect:
		selected := make([]string, 0, len(f.selectedSet))
		for _, idx := range f.selectedSet {
			if idx >= 0 && idx < len(f.Options) {
				selected = append(selected, f.Options[idx].Value)
			}
		}
		return selected
	case FieldToggle:
		return f.value
	case FieldReadOnly:
		return f.value
	}
	return f.value
}

// GetStringValue returns the field value as a string
func (f *Field) GetStringValue() string {
	switch v := f.GetValue().(type) {
	case string:
		return v
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	case bool:
		if v {
			return "Yes"
		}
		return "No"
	case []string:
		return strings.Join(v, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Focus management

// Focus sets focus on this field
func (f *Field) Focus() {
	f.focused = true
	switch f.Type {
	case FieldText, FieldNumber:
		f.textInput.Focus()
		// Set focused background on textinput styles
		f.textInput.TextStyle = lipgloss.NewStyle().
			Foreground(fieldColorWhite).
			Background(fieldColorBgFocused)
		f.textInput.PlaceholderStyle = lipgloss.NewStyle().
			Foreground(fieldColorMuted).
			Background(fieldColorBgFocused)
		f.textInput.PromptStyle = lipgloss.NewStyle().
			Foreground(fieldColorMuted).
			Background(fieldColorBgFocused)
		f.textInput.Cursor.TextStyle = lipgloss.NewStyle().
			Foreground(fieldColorWhite).
			Background(fieldColorBgFocused)
	case FieldTextArea:
		f.textArea.Focus()
	}
}

// Blur removes focus from this field
func (f *Field) Blur() {
	f.focused = false
	f.dropdownOpen = false
	f.textInput.Blur()
	f.textArea.Blur()
	// Reset textinput styles (no background when unfocused)
	f.textInput.TextStyle = lipgloss.NewStyle().Foreground(fieldColorWhite)
	f.textInput.PlaceholderStyle = lipgloss.NewStyle().Foreground(fieldColorMuted)
	f.textInput.PromptStyle = lipgloss.NewStyle().Foreground(fieldColorMuted)
	f.textInput.Cursor.TextStyle = lipgloss.NewStyle().Foreground(fieldColorWhite)
}

// IsFocused returns true if the field is focused
func (f *Field) IsFocused() bool {
	return f.focused
}

// IsEditable returns true if the field can be edited
func (f *Field) IsEditable() bool {
	return f.Type != FieldReadOnly
}

// IsTextInput returns true if this field accepts free text input (and thus
// should capture character keys instead of letting them be handled globally)
func (f *Field) IsTextInput() bool {
	return f.Type == FieldText || f.Type == FieldNumber || f.Type == FieldTextArea
}

// SetSize sets the field width and height
func (f *Field) SetSize(width, height int) {
	f.width = width
	f.height = height
	// Set textinput width - cap at reasonable max for readability
	inputWidth := width - 6 // Leave room for prompt and padding
	if inputWidth < 20 {
		inputWidth = 20
	}
	if inputWidth > 50 {
		inputWidth = 50 // Cap max width for better appearance
	}
	f.textInput.Width = inputWidth
	// Textarea uses more width but still capped
	textareaWidth := width - 4
	if textareaWidth < 30 {
		textareaWidth = 30
	}
	if textareaWidth > 80 {
		textareaWidth = 80
	}
	f.textArea.SetWidth(textareaWidth)
}

// Validation

// Validate runs the field's validator and returns any error
func (f *Field) Validate() error {
	f.validationErr = ""

	// Check required first
	if f.Required {
		value := f.GetValue()
		isEmpty := false
		switch v := value.(type) {
		case string:
			isEmpty = strings.TrimSpace(v) == ""
		case []string:
			isEmpty = len(v) == 0
		case nil:
			isEmpty = true
		}
		if isEmpty {
			//nolint:err113 // Field validation errors need field-specific messages
			err := fmt.Errorf("%s is required", f.Label)
			f.validationErr = err.Error()
			return err
		}
	}

	// Run custom validator
	if f.Validator != nil {
		if err := f.Validator(f.GetValue()); err != nil {
			f.validationErr = err.Error()
			return err
		}
	}

	return nil
}

// HasError returns true if the field has a validation error
func (f *Field) HasError() bool {
	return f.validationErr != ""
}

// IsDirty returns true if the value has changed from the original
func (f *Field) IsDirty() bool {
	return fmt.Sprintf("%v", f.GetValue()) != fmt.Sprintf("%v", f.originalValue)
}

// Update handles input messages
func (f *Field) Update(msg tea.Msg) tea.Cmd {
	if !f.focused || f.Type == FieldReadOnly {
		return nil
	}

	switch f.Type {
	case FieldText, FieldNumber:
		return f.updateTextInput(msg)
	case FieldTextArea:
		return f.updateTextArea(msg)
	case FieldDropdown:
		return f.updateDropdown(msg)
	case FieldMultiSelect:
		return f.updateMultiSelect(msg)
	case FieldToggle:
		return f.updateToggle(msg)
	}

	return nil
}

// updateTextInput handles input for text/number fields
func (f *Field) updateTextInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	f.textInput, cmd = f.textInput.Update(msg)

	// For number fields, validate as numeric
	if f.Type == FieldNumber {
		val := f.textInput.Value()
		if val != "" && val != "-" {
			if _, err := strconv.ParseInt(val, 10, 64); err != nil {
				// Keep only numeric characters
				cleaned := ""
				for _, r := range val {
					if r >= '0' && r <= '9' || (r == '-' && len(cleaned) == 0) {
						cleaned += string(r)
					}
				}
				f.textInput.SetValue(cleaned)
			}
		}
	}

	return cmd
}

// updateTextArea handles input for textarea fields
func (f *Field) updateTextArea(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	f.textArea, cmd = f.textArea.Update(msg)
	return cmd
}

// updateDropdown handles input for dropdown fields
func (f *Field) updateDropdown(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	if !f.dropdownOpen {
		// Open dropdown on Enter or Space
		switch keyMsg.String() {
		case "enter", " ":
			f.dropdownOpen = true
			return nil
		}
		return nil
	}

	// Dropdown is open - handle navigation
	switch {
	case key.Matches(keyMsg, f.keys.Up):
		if f.selectedIndex > 0 {
			f.selectedIndex--
		}
		return nil

	case key.Matches(keyMsg, f.keys.Down):
		if f.selectedIndex < len(f.Options)-1 {
			f.selectedIndex++
		}
		return nil

	case key.Matches(keyMsg, f.keys.Select):
		f.dropdownOpen = false
		return func() tea.Msg {
			return FieldChangedMsg{FieldID: f.ID, Value: f.GetValue()}
		}

	case keyMsg.String() == "esc":
		f.dropdownOpen = false
		return nil
	}

	return nil
}

// updateMultiSelect handles input for multi-select fields
func (f *Field) updateMultiSelect(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	if !f.dropdownOpen {
		// Open on Enter or Space
		switch keyMsg.String() {
		case "enter", " ":
			f.dropdownOpen = true
			return nil
		}
		return nil
	}

	switch {
	case key.Matches(keyMsg, f.keys.Up):
		if f.selectedIndex > 0 {
			f.selectedIndex--
		}
		return nil

	case key.Matches(keyMsg, f.keys.Down):
		if f.selectedIndex < len(f.Options)-1 {
			f.selectedIndex++
		}
		return nil

	case keyMsg.String() == " ":
		// Toggle selection of current option
		f.toggleMultiSelectOption(f.selectedIndex)
		return nil

	case keyMsg.String() == "enter":
		f.dropdownOpen = false
		return func() tea.Msg {
			return FieldChangedMsg{FieldID: f.ID, Value: f.GetValue()}
		}

	case keyMsg.String() == "esc":
		f.dropdownOpen = false
		return nil
	}

	return nil
}

// toggleMultiSelectOption toggles an option in multi-select
func (f *Field) toggleMultiSelectOption(index int) {
	// Check if already selected
	for i, idx := range f.selectedSet {
		if idx == index {
			// Remove from selection
			f.selectedSet = append(f.selectedSet[:i], f.selectedSet[i+1:]...)
			return
		}
	}
	// Add to selection
	f.selectedSet = append(f.selectedSet, index)
}

// updateToggle handles input for toggle fields
func (f *Field) updateToggle(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch keyMsg.String() {
	case " ", "enter":
		if b, ok := f.value.(bool); ok {
			f.value = !b
		} else {
			f.value = true
		}
		return func() tea.Msg {
			return FieldChangedMsg{FieldID: f.ID, Value: f.GetValue()}
		}
	}

	return nil
}

// View renders the field
func (f *Field) View() string {
	var b strings.Builder

	// Label with required indicator
	label := f.styles.Label.Render(f.Label)
	if f.Required {
		label += f.styles.LabelRequired.Render(" *")
	}
	b.WriteString(label)
	b.WriteString("\n")

	// Field value based on type
	switch f.Type {
	case FieldText, FieldNumber:
		b.WriteString(f.renderTextInput())
	case FieldTextArea:
		b.WriteString(f.renderTextArea())
	case FieldDropdown:
		b.WriteString(f.renderDropdown())
	case FieldMultiSelect:
		b.WriteString(f.renderMultiSelect())
	case FieldToggle:
		b.WriteString(f.renderToggle())
	case FieldReadOnly:
		b.WriteString(f.renderReadOnly())
	}
	// Help text - use Inline(true) to prevent width-based background fill
	if f.HelpText != "" && f.validationErr == "" {
		b.WriteString("\n")
		b.WriteString(f.styles.HelpText.Inline(true).Render(f.HelpText))
	}

	// Validation error
	if f.validationErr != "" {
		b.WriteString("\n")
		b.WriteString(f.styles.Error.Render("⚠ " + f.validationErr))
	}

	return f.styles.Container.Render(b.String())
}

// renderTextInput renders text/number input
func (f *Field) renderTextInput() string {
	// Focused: show textinput directly (it has its own styling with background)
	if f.focused {
		return f.textInput.View()
	}

	// Unfocused: show value or placeholder with background box matching textinput width
	boxStyle := f.styles.InputBox.Width(f.textInput.Width + 2) // +2 for padding
	value := f.textInput.Value()
	if value == "" {
		return boxStyle.Render(f.styles.ValueReadOnly.Render(f.Placeholder))
	}
	return boxStyle.Render(f.styles.Value.Render(value))
}

// renderTextArea renders multi-line textarea input
func (f *Field) renderTextArea() string {
	// When focused, use the textarea component for editing
	if f.focused {
		return f.textArea.View()
	}

	// When not focused, render a simple styled box with the value or placeholder
	boxStyle := f.styles.InputBox
	value := f.textArea.Value()

	var content string
	if value == "" {
		content = f.styles.ValueReadOnly.Render(f.Placeholder)
	} else {
		content = f.styles.Value.Render(value)
	}

	// Pad to match textarea height
	lines := strings.Split(content, "\n")
	for len(lines) < f.textAreaRows {
		lines = append(lines, "")
	}
	content = strings.Join(lines, "\n")

	return boxStyle.Render(content)
}

// renderDropdown renders dropdown field
func (f *Field) renderDropdown() string {
	var b strings.Builder

	// Current selected value
	selectedLabel := "(none)"
	if f.selectedIndex >= 0 && f.selectedIndex < len(f.Options) {
		selectedLabel = f.Options[f.selectedIndex].Label
	}

	boxStyle := f.styles.InputBox
	if f.focused {
		boxStyle = f.styles.InputBoxFocused
	}

	// Closed dropdown: show selected value
	if !f.dropdownOpen {
		style := f.styles.Value
		if f.focused {
			style = f.styles.ValueFocused
		}
		return boxStyle.Render(style.Render(selectedLabel + " ▼"))
	}

	// Open dropdown: show all options
	for i, opt := range f.Options {
		prefix := "  "
		style := f.styles.Option

		if i == f.selectedIndex {
			prefix = "▶ "
			style = f.styles.OptionHighlight
		}

		b.WriteString(style.Render(prefix + opt.Label))
		if i < len(f.Options)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderMultiSelect renders multi-select field
func (f *Field) renderMultiSelect() string {
	var b strings.Builder

	// Check if an index is selected
	isSelected := func(idx int) bool {
		for _, i := range f.selectedSet {
			if i == idx {
				return true
			}
		}
		return false
	}

	boxStyle := f.styles.InputBox
	if f.focused {
		boxStyle = f.styles.InputBoxFocused
	}

	// Closed: show selected values
	if !f.dropdownOpen {
		var selected []string
		for _, idx := range f.selectedSet {
			if idx >= 0 && idx < len(f.Options) {
				selected = append(selected, f.Options[idx].Label)
			}
		}

		value := "(none)"
		if len(selected) > 0 {
			value = strings.Join(selected, ", ")
		}

		style := f.styles.Value
		if f.focused {
			style = f.styles.ValueFocused
		}
		return boxStyle.Render(style.Render(value + " ▼"))
	}

	// Open: show all options with checkboxes
	for i, opt := range f.Options {
		checkbox := "[ ]"
		if isSelected(i) {
			checkbox = "[✓]"
		}

		prefix := "  "
		style := f.styles.Option

		if i == f.selectedIndex {
			prefix = "▶ "
			style = f.styles.OptionHighlight
		}

		if isSelected(i) {
			style = f.styles.OptionSelected
		}

		b.WriteString(style.Render(prefix + checkbox + " " + opt.Label))
		if i < len(f.Options)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderToggle renders toggle field
func (f *Field) renderToggle() string {
	isOn, _ := f.value.(bool)

	boxStyle := f.styles.InputBox
	if f.focused {
		boxStyle = f.styles.InputBoxFocused
	}

	var toggle string
	if isOn {
		toggle = f.styles.ToggleOn.Render("● ON")
	} else {
		toggle = f.styles.ToggleOff.Render("○ OFF")
	}

	return boxStyle.Render(toggle)
}

// renderReadOnly renders read-only field
func (f *Field) renderReadOnly() string {
	value := ""
	if f.value != nil {
		value = fmt.Sprintf("%v", f.value)
	}
	return f.styles.InputBoxMuted.Render(f.styles.ValueReadOnly.Render(value))
}
