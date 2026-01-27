---
description: Forms framework usage for GCP resource editing
globs:
  - "internal/ui/components/forms/**/*.go"
  - "internal/ui/views/*edit*.go"
  - "internal/ui/views/*form*.go"
  - "internal/ui/views/*create*.go"
---

# Forms Framework

The `internal/ui/components/forms/` package provides reusable form components for editing GCP resources.

## Architecture

```
Form (container)
├── Section 1 (collapsible group)
│   ├── Field 1 (text input)
│   ├── Field 2 (dropdown)
│   └── Field 3 (toggle)
├── Section 2
│   └── ...
└── Action Bar (Submit/Cancel)
```

### Message Flow

```
User Input → Form.Update() → FormSubmitMsg/FormCancelMsg → View.Update()
```

## Field Types

| Type | Constructor | Returns |
|------|-------------|---------|
| `FieldText` | `NewTextField(id, label)` | `string` |
| `FieldNumber` | `NewNumberField(id, label)` | `int64` |
| `FieldDropdown` | `NewDropdownField(id, label)` | `string` (Value, not Label) |
| `FieldMultiSelect` | `NewMultiSelectField(id, label)` | `[]string` (always slice, never nil) |
| `FieldToggle` | `NewToggleField(id, label)` | `bool` |
| `FieldReadOnly` | `NewReadOnlyField(id, label, value)` | N/A (display-only) |
| `FieldTextArea` | `NewTextAreaField(id, label)` | `string` |

### Field Configuration

```go
// Text field with all options
forms.NewTextField("name", "Instance Name").
    SetRequired(true).
    SetPlaceholder("my-instance").
    SetHelpText("Must be lowercase with hyphens").
    SetCharLimit(63).
    SetValidator(forms.ValidateGCPResourceName)

// Dropdown with labels different from values
forms.NewDropdownField("machine_type", "Machine Type").
    SetOptions([]forms.Option{
        {Value: "e2-micro", Label: "e2-micro (2 vCPU, 1 GB)"},
        {Value: "e2-small", Label: "e2-small (2 vCPU, 2 GB)"},
    })

// TextArea with line numbers
forms.NewTextAreaField("startup_script", "Startup Script").
    SetRows(6).
    SetShowLineNumbers(true).
    SetPlaceholder("#!/bin/bash\n# Your commands here")
```

## Validators

### Built-in Validators

| Validator | Description |
|-----------|-------------|
| `ValidateRequired` | Value must not be empty |
| `ValidateGCPResourceName` | Lowercase, hyphens, 1-63 chars, starts with letter |
| `ValidateGCPLabelKey` | Lowercase letters, numbers, hyphens, underscores |
| `ValidateGCPLabelValue` | Lowercase letters, numbers, hyphens, underscores |
| `ValidateNumber(min, max)` | Numeric range (inclusive) |
| `ValidateStringLength(min, max)` | String length range |
| `ValidateEmail()` | Email format |
| `ValidateURL()` | URL format |
| `ValidateIPAddress()` | IPv4/IPv6 address |
| `ValidateCIDR()` | CIDR notation (e.g., 10.0.0.0/8) |
| `ValidatePattern(pattern, errorMsg)` | Custom regex |
| `ValidateOneOf(allowed)` | Value must be in list |

### Composing Validators

```go
// Stops at first error (short-circuit)
field.SetValidator(forms.ComposeValidators(
    forms.ValidateRequired,
    forms.ValidateStringLength(2, 63),
    forms.ValidateGCPResourceName,
))

// Collect all errors
validateAll := forms.ValidateAll(
    forms.ValidateRequired,
    forms.ValidateGCPResourceName,
)
errors := validateAll(value)

// Conditional validation
field.SetValidator(forms.ConditionalValidator(
    func(value any) bool { return value != nil && value.(string) != "" },
    forms.ValidateEmail(),
))
```

## Sections

```go
// Basic section
section := forms.NewSection("basic", "Basic Settings").
    AddField(field1).
    AddField(field2)

// Collapsible section (for advanced options)
advancedSection := forms.NewSection("advanced", "Advanced Options").
    SetCollapsible(true).
    SetCollapsed(true).  // Start collapsed
    AddField(forms.NewTextAreaField("script", "Startup Script"))
```

**Collapsible behavior:**
- Collapsed sections show only title with expand indicator
- Press Enter/Space on collapsed section to expand
- Navigation skips over collapsed sections

## Form Integration

### Required: TextInputFocusable Interface

**Critical:** Views with forms MUST implement this interface:

```go
func (v *MyView) HasTextInputFocused() bool {
    if v.form != nil {
        return v.form.HasTextInputFocused()
    }
    return false
}
```

This prevents global keys like 'q' (quit) from being triggered when typing.

### Complete View Integration

```go
type ResourceEditView struct {
    form   *forms.Form
    client *gcp.Client
}

func (v *ResourceEditView) Init() tea.Cmd {
    v.form = forms.NewForm("Edit Resource", forms.FormModeEdit).
        EnableViewport()
    // Add sections and fields...
    return v.form.Init()
}

func (v *ResourceEditView) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case forms.FormSubmitMsg:
        if errors := v.form.Validate(); len(errors) > 0 {
            return nil  // Form handles error display
        }
        return v.saveResource(msg.Data)
    case forms.FormCancelMsg:
        return func() tea.Msg { return ui.NavigateBackMsg{} }
    }
    return v.form.Update(msg)
}

func (v *ResourceEditView) HasTextInputFocused() bool {
    return v.form.HasTextInputFocused()
}

func (v *ResourceEditView) SetSize(width, height int) {
    v.form.SetSize(width-4, height-8)  // Account for padding
}
```

## Form Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Tab` / `↓` | Next field |
| `Shift+Tab` / `↑` | Previous field |
| `Enter` / `Space` | Select/toggle option |
| `Ctrl+S` | Submit form |
| `Esc` | Cancel |
| `?` | Toggle help |

## Anti-Patterns

### Don't Modify textInput Directly

```go
// WRONG - breaks dirty tracking
field.textInput.SetValue("new value")

// CORRECT - keeps originalValue in sync
field.SetValue("new value")
```

### Don't Reuse Stale Form Instances

```go
// WRONG - form may have stale state
if a.editForm == nil {
    a.editForm = buildForm()
}
a.showEditForm = true

// CORRECT - always create fresh instance
a.editForm = buildForm()
a.showEditForm = true
return a, a.editForm.Init()
```

### Don't Forget TextInputFocusable

```go
// WRONG - 'q' key will quit when typing
type MyView struct { form *forms.Form }

// CORRECT - implement the interface
func (v *MyView) HasTextInputFocused() bool {
    return v.form != nil && v.form.HasTextInputFocused()
}
```

### Don't Ignore Form Messages

```go
// WRONG - form buttons won't work
func (v *View) Update(msg tea.Msg) tea.Cmd {
    return v.form.Update(msg)  // Missing message handling!
}

// CORRECT - handle submit and cancel
func (v *View) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case forms.FormSubmitMsg:
        return v.handleSubmit(msg.Data)
    case forms.FormCancelMsg:
        return v.handleCancel()
    }
    return v.form.Update(msg)
}
```

## Edge Cases

- **Number field**: `SetValue()` accepts int, int64, float64, string. Float values truncated.
- **MultiSelect**: Always returns `[]string`, never nil. Empty slice when no selections.
- **Dropdown open**: Up/Down keys captured by dropdown when open. Form navigation paused.
- **ReadOnly fields**: Not editable, not included in dirty checking, skipped during Tab navigation.
- **Character limits**: Text 256 chars default, TextArea no limit (0).

## Getting/Setting Data

```go
// Get all field values
data := form.GetData()  // map[string]any

// Populate form from existing data
form.SetData(map[string]any{
    "name":         "my-instance",
    "zone":         "us-central1-a",
    "machine_type": "e2-micro",
})

// Check state
form.IsDirty()      // Any field changed since SetValue?
form.HasErrors()    // Any validation errors?
form.Validate()     // Get []string of error messages
```

## Demo View

Access the form demo via command palette with "Form Demo (dev)" to see all field types and validation in action.

## Key Binding Conflicts with Text Input

When adding keyboard shortcuts to form sections, single-character keys conflict with text input.

**Problem**: Character shortcuts (like `-` for collapse) are consumed before reaching text input fields, preventing users from typing those characters (e.g., "my-bucket-name").

**Solution**: Always check `HasTextInputFocused()` before handling character keys in section/form Update():

```go
case key.Matches(keyMsg, s.keys.Collapse):
    // Skip collapse action if user is typing in a text field
    if s.Collapsible && !s.HasTextInputFocused() {
        s.Collapsed = true
        return nil
    }
    // Fall through to let the focused field handle the '-' key
```

**Rules for form shortcuts**:
1. Prefer non-character keys (Ctrl+X, function keys) when possible
2. If character key required, gate it with `!HasTextInputFocused()`
3. Update tests - shortcuts should NOT work when text input is focused
