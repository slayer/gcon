# Forms Framework Documentation

A comprehensive guide to using the forms framework in gcon for building GCP resource editing interfaces.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Field Types](#field-types)
- [Validation](#validation)
- [Sections](#sections)
- [Form Configuration](#form-configuration)
- [Keyboard Navigation](#keyboard-navigation)
- [Integration with Views](#integration-with-views)
- [Patterns](#patterns)
- [Anti-Patterns](#anti-patterns)
- [Edge Cases](#edge-cases)
- [Complete Examples](#complete-examples)
- [Reference](#reference)

---

## Overview

The forms framework provides a complete UI system for creating, editing, and validating data in gcon. Built on Bubble Tea, it offers:

- **7 field types** for different input scenarios
- **Comprehensive validation** with 15+ built-in validators
- **Keyboard-first navigation** with Tab, arrows, and shortcuts
- **Collapsible sections** for organizing complex forms
- **Viewport scrolling** for long forms
- **Type-safe value handling** with automatic conversions

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

Forms communicate with parent views through messages:

```
User Input → Form.Update() → FormSubmitMsg/FormCancelMsg → View.Update()
```

### Focus Hierarchy

```
Form → Section → Field
       ↑
       Tab/Arrow navigation
```

---

## Quick Start

```go
import "github.com/user/gcon/internal/ui/components/forms"

// 1. Create form
form := forms.NewForm("Create Instance", forms.FormModeCreate).
    SetSubtitle("Configure your VM instance").
    EnableViewport()

// 2. Create section with fields
basicSection := forms.NewSection("basic", "Basic Settings").
    AddField(forms.NewTextField("name", "Name").
        SetRequired(true).
        SetPlaceholder("my-instance").
        SetValidator(forms.ValidateGCPResourceName)).
    AddField(forms.NewDropdownField("zone", "Zone").
        SetRequired(true).
        SetOptionsFromStrings([]string{"us-central1-a", "us-east1-b"}))

form.AddSection(basicSection)

// 3. Initialize in your view
func (v *MyView) Init() tea.Cmd {
    return v.form.Init()  // Focus first field
}

// 4. Handle messages
func (v *MyView) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case forms.FormSubmitMsg:
        data := msg.Data  // map[string]any
        return v.saveData(data)
    case forms.FormCancelMsg:
        return v.navigateBack()
    }
    return v.form.Update(msg)
}

// 5. Implement TextInputFocusable (REQUIRED)
func (v *MyView) HasTextInputFocused() bool {
    return v.form.HasTextInputFocused()
}
```

---

## Field Types

### Text Field

Single-line text input for names, IDs, descriptions.

```go
forms.NewTextField("name", "Instance Name").
    SetRequired(true).
    SetPlaceholder("my-instance").
    SetHelpText("Must be lowercase with hyphens").
    SetCharLimit(63).
    SetValidator(forms.ValidateGCPResourceName)
```

**Returns:** `string`

### Number Field

Numeric input with automatic cleaning (only digits and leading minus).

```go
forms.NewNumberField("disk_size", "Disk Size (GB)").
    SetRequired(true).
    SetValidator(forms.ValidateNumber(10, 1000))
```

**Returns:** `int64`

**Note:** Float values passed to `SetValue()` are truncated to int64.

### Dropdown Field

Single selection from a list of options.

```go
forms.NewDropdownField("machine_type", "Machine Type").
    SetRequired(true).
    SetOptions([]forms.Option{
        {Value: "e2-micro", Label: "e2-micro (2 vCPU, 1 GB)"},
        {Value: "e2-small", Label: "e2-small (2 vCPU, 2 GB)"},
        {Value: "e2-medium", Label: "e2-medium (2 vCPU, 4 GB)"},
    }).
    SetHelpText("Select instance type")
```

**Returns:** `string` (the Value, not Label)

**Alternative:** Use `SetOptionsFromStrings()` when Label equals Value:
```go
SetOptionsFromStrings([]string{"us-central1-a", "us-east1-b"})
```

### MultiSelect Field

Multiple selections from a list.

```go
forms.NewMultiSelectField("network_tags", "Network Tags").
    SetOptionsFromStrings([]string{
        "http-server",
        "https-server",
        "allow-ssh",
    }).
    SetHelpText("Select firewall tags (multiple allowed)")
```

**Returns:** `[]string` (always a slice, empty if no selections)

### Toggle Field

Boolean on/off switch.

```go
forms.NewToggleField("deletion_protection", "Deletion Protection").
    SetHelpText("Prevent accidental deletion")
```

**Returns:** `bool`

### ReadOnly Field

Display-only value for context information.

```go
forms.NewReadOnlyField("created", "Created At", "2024-01-15T10:30:00Z")
```

**Note:** Not editable, updates are ignored, not included in dirty checking.

### TextArea Field

Multi-line text input for scripts, descriptions.

```go
forms.NewTextAreaField("startup_script", "Startup Script").
    SetRows(6).
    SetShowLineNumbers(true).
    SetPlaceholder("#!/bin/bash\necho 'Hello World'")
```

**Returns:** `string`

---

## Validation

### Built-in Validators

#### Basic Validators

| Validator | Description |
|-----------|-------------|
| `ValidateRequired` | Value must not be empty |
| `ValidateNotEmpty` | Alias for ValidateRequired |

#### GCP-Specific Validators

| Validator | Description |
|-----------|-------------|
| `ValidateGCPResourceName` | Lowercase, hyphens allowed, 1-63 chars, must start with letter |
| `ValidateGCPLabelKey` | Lowercase letters, numbers, hyphens, underscores |
| `ValidateGCPLabelValue` | Lowercase letters, numbers, hyphens, underscores |

#### Format Validators

| Validator | Description |
|-----------|-------------|
| `ValidateEmail()` | Email format validation |
| `ValidateURL()` | URL format validation |
| `ValidateIPAddress()` | IPv4/IPv6 address validation |
| `ValidateCIDR()` | CIDR notation validation (e.g., 10.0.0.0/8) |
| `ValidatePattern(pattern, errorMsg)` | Custom regex with custom error |

#### Numeric Validators

| Validator | Description |
|-----------|-------------|
| `ValidateNumber(min, max)` | Number within range (inclusive) |
| `ValidateDiskSize(minGB, maxGB)` | Disk size within range |

#### String Validators

| Validator | Description |
|-----------|-------------|
| `ValidateStringLength(min, max)` | String length within range |
| `ValidateOneOf(allowed)` | Value must be in allowed list |
| `ValidateNotOneOf(disallowed, errorMsg)` | Value must not be in list |

### Composing Validators

Combine multiple validators with short-circuit behavior:

```go
// Stops at first error
field.SetValidator(forms.ComposeValidators(
    forms.ValidateRequired,
    forms.ValidateStringLength(2, 63),
    forms.ValidateGCPResourceName,
))
```

Collect all errors:

```go
// Returns []error with all validation failures
validateAll := forms.ValidateAll(
    forms.ValidateRequired,
    forms.ValidateGCPResourceName,
)
errors := validateAll(value)
```

### Conditional Validation

```go
// Only validate if condition is met
field.SetValidator(forms.ConditionalValidator(
    func(value any) bool {
        return value != nil && value.(string) != ""
    },
    forms.ValidateEmail(),
))
```

### Custom Validators

```go
// Custom validator function
func ValidateProjectID(value any) error {
    s, ok := value.(string)
    if !ok || s == "" {
        return nil  // Let Required handle empty
    }
    if len(s) < 6 || len(s) > 30 {
        return fmt.Errorf("project ID must be 6-30 characters")
    }
    // Additional checks...
    return nil
}

field.SetValidator(ValidateProjectID)
```

### Validation Behavior

1. **Required check runs first** - Before custom validator
2. **Empty values pass most validators** - Let `ValidateRequired` handle empty state
3. **First error shown** - Only first validation error displayed per field
4. **Manual trigger** - Call `form.Validate()` to get all errors

---

## Sections

Sections group related fields and support collapsing.

### Creating Sections

```go
section := forms.NewSection("basic", "Basic Settings").
    SetIcon("📋").                    // Optional icon
    SetDescription("Core config").    // Optional description
    SetCollapsible(true).             // Allow collapsing
    SetCollapsed(false).              // Initial state
    AddField(field1).
    AddField(field2)
```

### Collapsible Sections

Use for advanced or optional settings:

```go
advancedSection := forms.NewSection("advanced", "Advanced Options").
    SetCollapsible(true).
    SetCollapsed(true).  // Start collapsed
    AddField(forms.NewTextAreaField("script", "Startup Script"))
```

**Behavior:**
- Collapsed sections show only title with expand indicator
- Press Enter/Space on collapsed section to expand
- Navigation skips over collapsed sections

### Section Methods

```go
section.Fields()              // Get all fields
section.GetField("name")      // Get field by ID
section.EditableFieldCount()  // Count of non-readonly fields
section.IsDirty()             // Any field changed?
section.Validate()            // Get []error for all fields
section.HasErrors()           // Any validation errors?
```

---

## Form Configuration

### Form Modes

```go
forms.FormModeCreate  // New resource
forms.FormModeEdit    // Editing existing
forms.FormModeClone   // Copying existing
```

Mode affects form title styling and can be used for conditional logic.

### Enabling Viewport

For long forms that need scrolling:

```go
form := forms.NewForm("title", mode).
    EnableViewport()
```

Viewport automatically scrolls to keep focused field visible.

### Setting Size

```go
// In your view's SetSize or SetContext method
form.SetSize(width, height)
```

### Getting/Setting Data

```go
// Get all field values
data := form.GetData()  // map[string]any

// Populate form from existing data
form.SetData(map[string]any{
    "name": "my-instance",
    "zone": "us-central1-a",
})
```

### Checking State

```go
form.IsDirty()      // Any field changed since SetValue?
form.HasErrors()    // Any validation errors?
form.Validate()     // Get []string of error messages
```

---

## Keyboard Navigation

### Global Form Keys

| Key | Action |
|-----|--------|
| `Tab` / `↓` | Next field |
| `Shift+Tab` / `↑` | Previous field |
| `Enter` / `Space` | Select/toggle (field-specific) |
| `Ctrl+S` | Submit form |
| `Esc` | Cancel form |
| `?` | Toggle help |

### Text Input Keys

| Key | Action |
|-----|--------|
| Characters | Input text |
| `Ctrl+U` | Clear field |
| `←` / `→` | Move cursor |

### Dropdown/MultiSelect Keys

| Key | Action (menu closed) | Action (menu open) |
|-----|---------------------|-------------------|
| `Enter` / `Space` | Open menu | Confirm selection |
| `↑` / `↓` | N/A | Navigate options |
| `Space` | N/A | Toggle option (multiselect) |
| `Esc` | N/A | Close without confirming |

### Toggle Keys

| Key | Action |
|-----|--------|
| `Space` / `Enter` | Toggle value |

---

## Integration with Views

### Required: TextInputFocusable Interface

**Critical:** Views with forms MUST implement this interface:

```go
type TextInputFocusable interface {
    HasTextInputFocused() bool
}

func (v *MyView) HasTextInputFocused() bool {
    if v.form != nil {
        return v.form.HasTextInputFocused()
    }
    return false
}
```

This prevents global keys like 'q' (quit) from being triggered when typing in text fields.

### Complete View Integration

```go
type ResourceEditView struct {
    form   *forms.Form
    client *gcp.Client
    // ...
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
        // Validate and save
        if errors := v.form.Validate(); len(errors) > 0 {
            return nil  // Form handles error display
        }
        return v.saveResource(msg.Data)

    case forms.FormCancelMsg:
        return func() tea.Msg {
            return ui.NavigateBackMsg{}
        }
    }

    return v.form.Update(msg)
}

func (v *ResourceEditView) HasTextInputFocused() bool {
    return v.form.HasTextInputFocused()
}

func (v *ResourceEditView) SetSize(width, height int) {
    v.form.SetSize(width-4, height-8)  // Account for padding
}

func (v *ResourceEditView) View() string {
    return v.form.View()
}
```

### App-Level Integration

In `app.go`, check for text input focus before processing global keys:

```go
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Let view handle keys when text input is focused
        if a.hasTextInputFocused() {
            return a, a.updateCurrentView(msg)
        }

        // Now safe to process global keys
        switch {
        case key.Matches(msg, a.keys.Quit):
            return a, tea.Quit()
        }
    }
    // ...
}
```

---

## Patterns

### 1. Builder Pattern for Configuration

Chain configuration methods for readable form definitions:

```go
forms.NewTextField("name", "Name").
    SetRequired(true).
    SetPlaceholder("my-resource").
    SetHelpText("Lowercase letters and hyphens only").
    SetValidator(forms.ComposeValidators(
        forms.ValidateRequired,
        forms.ValidateGCPResourceName,
    ))
```

### 2. Section Organization

Group related fields logically:

```go
// Basic settings - always visible
basicSection := forms.NewSection("basic", "Basic Settings")

// Advanced settings - collapsed by default
advancedSection := forms.NewSection("advanced", "Advanced").
    SetCollapsible(true).
    SetCollapsed(true)

// Metadata section
metadataSection := forms.NewSection("metadata", "Labels & Tags")
```

### 3. Conditional Field Visibility

Use multiple forms or sections for different modes:

```go
func (v *View) buildForm(mode forms.FormMode) *forms.Form {
    form := forms.NewForm("Resource", mode)

    if mode == forms.FormModeCreate {
        // Name is editable only on create
        form.AddSection(forms.NewSection("basic", "Basic").
            AddField(forms.NewTextField("name", "Name").SetRequired(true)))
    } else {
        // Show name as readonly on edit
        form.AddSection(forms.NewSection("basic", "Basic").
            AddField(forms.NewReadOnlyField("name", "Name", v.resource.Name)))
    }

    return form
}
```

### 4. Pre-populating Edit Forms

```go
func (v *View) populateForm(resource *Resource) {
    v.form.SetData(map[string]any{
        "name":        resource.Name,
        "zone":        resource.Zone,
        "machine_type": resource.MachineType,
        "preemptible": resource.Preemptible,
    })
}
```

### 5. Custom Validation with Context

```go
func (v *View) validateUniqueInProject(value any) error {
    name := value.(string)
    // Check against existing resources
    for _, existing := range v.existingResources {
        if existing.Name == name {
            return fmt.Errorf("resource '%s' already exists", name)
        }
    }
    return nil
}

field.SetValidator(v.validateUniqueInProject)
```

### 6. Dynamic Options Loading

```go
func (v *View) loadZones() tea.Cmd {
    return func() tea.Msg {
        zones, err := v.client.ListZones(ctx)
        if err != nil {
            return errorMsg{err}
        }
        return zonesLoadedMsg{zones}
    }
}

// In Update:
case zonesLoadedMsg:
    field := v.form.GetField("zone")
    field.SetOptionsFromStrings(msg.zones)
```

---

## Anti-Patterns

### 1. Don't Modify textInput Directly

```go
// WRONG - breaks dirty tracking
field.textInput.SetValue("new value")

// CORRECT - keeps originalValue in sync
field.SetValue("new value")
```

### 2. Don't Check Dirty Before SetValue

```go
// WRONG - always false before user interaction
if form.IsDirty() {
    // Never executes
}

// CORRECT - check after user has had chance to edit
func (v *View) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case forms.FormCancelMsg:
        if v.form.IsDirty() {
            // Ask for confirmation
        }
    }
}
```

### 3. Don't Reuse Stale Form Instances

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

### 4. Don't Forget TextInputFocusable

```go
// WRONG - 'q' key will quit when typing
type MyView struct {
    form *forms.Form
}

// CORRECT - implement the interface
func (v *MyView) HasTextInputFocused() bool {
    return v.form != nil && v.form.HasTextInputFocused()
}
```

### 5. Don't Ignore Form Messages

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

### 6. Don't Use Vague Validation Errors

```go
// WRONG - unhelpful error
return fmt.Errorf("invalid value")

// CORRECT - contextual error
return fmt.Errorf("project ID must be 6-30 lowercase characters")
```

---

## Edge Cases

### Collapsed Sections

- Focused collapsed section shows no field focus
- Enter/Space expands and focuses first editable field
- Navigation skips collapsed sections with no editable fields

### Number Field Type Handling

- `SetValue()` accepts: int, int64, float64, string
- Float values truncated (not rounded)
- Returns `int64`

### MultiSelect Empty State

- Always returns `[]string`, never nil
- Empty slice when no selections

### Dropdown Open State

- Up/Down keys captured by dropdown when open
- Form navigation paused while dropdown is open
- Esc closes dropdown without changing value

### ReadOnly Fields

- Not editable, updates ignored
- Not included in dirty checking
- Skipped during Tab navigation

### Viewport Scrolling

- Auto-scrolls to focused field
- Estimates ~4 lines per field
- Action bar scrolled into view when focused

### Character Limits

- Text: 256 chars default
- Number: Platform default
- TextArea: No limit (0)

### Value Change Detection

- `SetValue()` sets both `value` and `originalValue`
- `IsDirty()` compares current to original via string representation
- User edits don't update `originalValue`

---

## Complete Examples

### Create Instance Form

```go
func buildCreateInstanceForm() *forms.Form {
    form := forms.NewForm("Create Instance", forms.FormModeCreate).
        SetSubtitle("Configure a new VM instance").
        EnableViewport()

    // Basic settings
    basicSection := forms.NewSection("basic", "Basic Settings").
        AddField(forms.NewTextField("name", "Instance Name").
            SetRequired(true).
            SetPlaceholder("my-instance").
            SetValidator(forms.ComposeValidators(
                forms.ValidateRequired,
                forms.ValidateGCPResourceName,
            ))).
        AddField(forms.NewDropdownField("zone", "Zone").
            SetRequired(true).
            SetOptionsFromStrings([]string{
                "us-central1-a",
                "us-central1-b",
                "us-east1-a",
                "us-east1-b",
            })).
        AddField(forms.NewDropdownField("machine_type", "Machine Type").
            SetRequired(true).
            SetOptions([]forms.Option{
                {Value: "e2-micro", Label: "e2-micro (2 vCPU, 1 GB)"},
                {Value: "e2-small", Label: "e2-small (2 vCPU, 2 GB)"},
                {Value: "e2-medium", Label: "e2-medium (2 vCPU, 4 GB)"},
            }))

    form.AddSection(basicSection)

    // Boot disk
    diskSection := forms.NewSection("disk", "Boot Disk").
        AddField(forms.NewDropdownField("boot_image", "Image").
            SetRequired(true).
            SetOptionsFromStrings([]string{
                "debian-cloud/debian-11",
                "ubuntu-os-cloud/ubuntu-2204-lts",
                "centos-cloud/centos-stream-9",
            })).
        AddField(forms.NewNumberField("disk_size", "Size (GB)").
            SetRequired(true).
            SetValidator(forms.ValidateNumber(10, 2048)))

    form.AddSection(diskSection)

    // Network
    networkSection := forms.NewSection("network", "Networking").
        AddField(forms.NewMultiSelectField("network_tags", "Network Tags").
            SetOptionsFromStrings([]string{
                "http-server",
                "https-server",
                "allow-ssh",
                "allow-internal",
            })).
        AddField(forms.NewToggleField("external_ip", "External IP").
            SetHelpText("Assign ephemeral external IP"))

    form.AddSection(networkSection)

    // Scheduling
    schedulingSection := forms.NewSection("scheduling", "Scheduling").
        SetCollapsible(true).
        SetCollapsed(true).
        AddField(forms.NewToggleField("preemptible", "Preemptible").
            SetHelpText("Lower cost, may be terminated")).
        AddField(forms.NewToggleField("deletion_protection", "Deletion Protection"))

    form.AddSection(schedulingSection)

    // Automation
    automationSection := forms.NewSection("automation", "Automation").
        SetCollapsible(true).
        SetCollapsed(true).
        AddField(forms.NewTextAreaField("startup_script", "Startup Script").
            SetRows(8).
            SetShowLineNumbers(true).
            SetPlaceholder("#!/bin/bash\n# Your startup commands here"))

    form.AddSection(automationSection)

    return form
}
```

### Edit Labels Form

```go
func buildLabelsForm(labels map[string]string) *forms.Form {
    form := forms.NewForm("Edit Labels", forms.FormModeEdit).
        SetSubtitle("Modify resource labels")

    section := forms.NewSection("labels", "Labels")

    // Add existing labels as fields
    keys := make([]string, 0, len(labels))
    for k := range labels {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    for _, key := range keys {
        section.AddField(forms.NewTextField(key, key).
            SetValue(labels[key]).
            SetValidator(forms.ValidateGCPLabelValue))
    }

    form.AddSection(section)
    return form
}
```

---

## Reference

### Field Methods

| Method | Description |
|--------|-------------|
| `SetRequired(bool)` | Mark field as required |
| `SetHelpText(string)` | Add help text below field |
| `SetPlaceholder(string)` | Set placeholder text |
| `SetValidator(Validator)` | Set validation function |
| `SetOptions([]Option)` | Set dropdown/multiselect options |
| `SetOptionsFromStrings([]string)` | Set options from string slice |
| `SetCharLimit(int)` | Set max character limit |
| `SetRows(int)` | Set textarea rows |
| `SetShowLineNumbers(bool)` | Show line numbers in textarea |
| `SetValue(any)` | Set field value |
| `GetValue() any` | Get typed value |
| `GetStringValue() string` | Get string representation |
| `IsDirty() bool` | Check if value changed |
| `Validate() error` | Run validation |
| `HasError() bool` | Check for validation error |
| `IsTextInput() bool` | Check if captures char keys |

### Section Methods

| Method | Description |
|--------|-------------|
| `SetIcon(string)` | Set section icon |
| `SetDescription(string)` | Set section description |
| `SetCollapsible(bool)` | Enable collapsing |
| `SetCollapsed(bool)` | Set initial collapse state |
| `AddField(*Field)` | Add field to section |
| `Fields() []*Field` | Get all fields |
| `GetField(string) *Field` | Get field by ID |
| `GetData() map[string]any` | Get all field values |
| `IsDirty() bool` | Any field changed? |
| `Validate() []error` | Get all validation errors |
| `ToggleCollapse()` | Toggle collapsed state |

### Form Methods

| Method | Description |
|--------|-------------|
| `SetSubtitle(string)` | Set form subtitle |
| `AddSection(*Section)` | Add section |
| `EnableViewport()` | Enable scrolling |
| `GetSection(string)` | Get section by ID |
| `GetField(string)` | Get field by ID |
| `GetData() map[string]any` | Get all values |
| `SetData(map[string]any)` | Populate all fields |
| `IsDirty() bool` | Any field changed? |
| `Validate() []string` | Get error messages |
| `HasErrors() bool` | Any validation errors? |
| `HasTextInputFocused() bool` | Text input has focus? |
| `SetSize(w, h int)` | Set form dimensions |
| `Init() tea.Cmd` | Initialize and focus |
| `Update(tea.Msg) tea.Cmd` | Handle messages |
| `View() string` | Render form |

### Message Types

| Message | Description |
|---------|-------------|
| `FormSubmitMsg{Data}` | Form submitted with all values |
| `FormCancelMsg{}` | Form cancelled |
| `FieldChangedMsg{FieldID, Value}` | Field value changed |
| `ValidationErrorMsg{FieldID, Error}` | Validation failed |

### Validators

| Validator | Description |
|-----------|-------------|
| `ValidateRequired` | Not empty |
| `ValidateGCPResourceName` | Valid GCP resource name |
| `ValidateGCPLabelKey` | Valid GCP label key |
| `ValidateGCPLabelValue` | Valid GCP label value |
| `ValidateEmail()` | Email format |
| `ValidateURL()` | URL format |
| `ValidateIPAddress()` | IP address |
| `ValidateCIDR()` | CIDR notation |
| `ValidatePattern(pattern, msg)` | Regex with custom error |
| `ValidateNumber(min, max)` | Numeric range |
| `ValidateDiskSize(min, max)` | Disk size range |
| `ValidateStringLength(min, max)` | String length range |
| `ValidateOneOf(allowed)` | Value in list |
| `ValidateNotOneOf(disallowed, msg)` | Value not in list |
| `ComposeValidators(...)` | Combine validators |
| `ValidateAll(...)` | Collect all errors |
| `ConditionalValidator(cond, v)` | Conditional validation |
