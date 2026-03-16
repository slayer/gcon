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

### Complete View Integration (Creation Views)

For creation views, embed `CreateViewBase` which handles Init, View, SetError, SetContext,
HasTextInputFocused, spinner, and form sizing automatically:

```go
type ResourceCreateView struct {
    CreateViewBase  // Embeds form lifecycle
    client *gcp.Client
}

func NewResourceCreateView(client *gcp.Client) *ResourceCreateView {
    v := &ResourceCreateView{
        CreateViewBase: NewCreateViewBase("Creating resource..."),
        client:         client,
    }
    v.buildForm()
    return v
}

func (v *ResourceCreateView) buildForm() {
    v.Form = forms.NewForm("Create Resource", forms.FormModeCreate).
        EnableViewport()
    // Add sections and fields...
}

func (v *ResourceCreateView) Update(msg tea.Msg) tea.Cmd {
    // Base handles spinner ticks and cancel-during-saving
    if cmd, handled := v.HandleBaseUpdate(msg, ResourceCanceledMsg{}); handled {
        return cmd
    }
    switch msg := msg.(type) {
    case forms.FormSubmitMsg:
        return v.handleSubmit()
    case forms.FormCancelMsg:
        return func() tea.Msg { return ResourceCanceledMsg{} }
    }
    return v.UpdateForm(msg)
}

func (v *ResourceCreateView) handleSubmit() tea.Cmd {
    if errors := v.Form.Validate(); len(errors) > 0 {
        return nil
    }
    data := v.Form.GetData()
    cmd := v.BeginSaving()
    return tea.Batch(cmd, func() tea.Msg {
        return CreateResourceMsg{Name: data["name"].(string)}
    })
}
```

See: `snapshot_create.go`, `disk_create.go`, `image_create.go`

### Complete View Integration (Editor Views)

For more complex views (diff preview, multiple states), implement manually:

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
    v.form.SetSize(width-formWidthPadding, height-formHeightPadding)
}
```

See: `bucket_create.go`, `instance_editor.go`

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

### Don't Reverse-Map Display Strings to API Values

When populating a form from existing data for editing, never reverse-map human-readable display labels back to API enum values. Display strings are fragile — if they change, the reverse mapping silently fails.

```go
// WRONG - fragile reverse mapping from display strings
switch details.VPCEgress {  // "All traffic" (human-readable)
case "All traffic":
    data["vpc_egress"] = "ALL_TRAFFIC"
case "Private ranges only":
    data["vpc_egress"] = "PRIVATE_RANGES_ONLY"
default:
    data["vpc_egress"] = ""  // Silent data loss if label changes
}

// CORRECT - store raw API values alongside display strings
type ServiceDetails struct {
    VPCEgress    string  // "All traffic" (for display)
    VPCEgressRaw string  // "ALL_TRAFFIC" (for form round-tripping)
}
// Then use the raw value directly:
data["vpc_egress"] = details.VPCEgressRaw
```

**Rule**: Details/model structs should carry raw API enum values (with `Raw` suffix) when those values need to survive a form edit round-trip.

### Don't Assume Dropdown SetValue Matches

`Field.SetValue()` on a dropdown silently falls back to showing option[0] when the value doesn't match any option. This is dangerous when populating edit forms from API data where the actual value (e.g., a custom machine type like `n2-custom-8-20480`) might not be in the curated option list.

```go
// WRONG - if details.MachineType isn't in options, shows first option silently
field.SetValue(details.MachineType)

// CORRECT - check for match first, fall back to alternative field
found := false
for _, opt := range field.Options {
    if opt.Value == details.MachineType {
        found = true
        break
    }
}
if found {
    field.SetValue(details.MachineType)
} else if customField := f.GetField("custom_machine_type"); customField != nil {
    customField.SetValue(details.MachineType)
}
```

**Rule**: When populating dropdowns from API data in edit forms, always verify the value exists in options before calling `SetValue`. Provide a fallback (text field override, read-only display) for values outside the curated list.

### Don't SetData on Dropdowns Without Options (Testing Gotcha)

`form.SetData()` on a dropdown field silently does nothing when the value doesn't match any option. In tests, dropdowns start with no options (async-loaded), so `SetData({"machine_type": "e2-medium"})` appears to succeed but the field stays empty. Form validation then fails on the "required" check, causing `handleSubmit()` to return nil — a confusing test failure.

```go
// WRONG — SetData silently fails because dropdown has no options
v := NewInstanceCreateView("project-id", nil)
v.Form.SetData(map[string]any{
    "machine_type": "e2-medium",  // silently ignored
})
cmd := v.handleSubmit()  // returns nil — validation fails, not obvious why

// CORRECT — populate options first, then set data
v := NewInstanceCreateView("project-id", nil)
if f := v.Form.GetField("machine_type"); f != nil {
    f.SetOptions([]forms.Option{{Value: "e2-medium", Label: "e2-medium"}})
}
v.Form.SetData(map[string]any{
    "machine_type": "e2-medium",  // now matches an option, works
})
```

**Rule**: In tests, always populate dropdown options before calling `SetData` or `SetValue` with values that need to match. This mirrors runtime behavior where options are loaded asynchronously before user interaction.

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

### Don't Replace Dropdown Options Without Considering Cursor State

`SetOptions()`/`SetOptionsFromStrings()` now clamp `selectedIndex` and `dropdownScrollOffset` automatically — but callers should be aware that replacing options resets the selection to index 0 when the old index exceeds the new list length. The dropdown also closes on empty replacement to prevent rendering stale state.

If you need to preserve the user's selection across an option list update (e.g., zone cache refresh), re-apply `SetValue()` after `SetOptions()`:

```go
// After refreshing options, restore previous selection if still valid
prevValue := field.GetValue()
field.SetOptions(newOptions)  // clamps index, may reset to 0
if prevValue != nil {
    field.SetValue(prevValue)  // re-selects if value still exists in new list
}
```

### Don't Open Dropdowns Without Handling Empty Options

When `dropdownOpen = true` and `len(Options) == 0`, the render loop produces an empty string — the field value vanishes. Always render a fallback message. The dropdown uses `field.Placeholder` for this: if set, it shows the placeholder; otherwise "(no options available)".

```go
// In renderDropdown() — empty options case
if len(f.Options) == 0 {
    msg := "(no options available)"
    if f.Placeholder != "" {
        msg = f.Placeholder
    }
    return f.styles.HelpText.Inline(true).Render(msg)
}
```

### Don't Assume Default Dropdown Selection Triggers FieldChangedMsg

Dropdowns default to `selectedIndex = 0` on creation, but `FieldChangedMsg` is only emitted when the user explicitly selects a value. If downstream logic depends on the selected value (e.g., loading machine types for a zone), the initial default is silently ignored.

```go
// WRONG — machine types never load for the default zone
func (v *CreateView) Init() tea.Cmd {
    return v.CreateViewBase.Init()
}

// CORRECT — read default value and trigger dependent loading
func (v *CreateView) Init() tea.Cmd {
    cmds := []tea.Cmd{v.CreateViewBase.Init()}
    if field := v.Form.GetField("zone"); field != nil {
        if zone, ok := field.GetValue().(string); ok && zone != "" {
            cmds = append(cmds, v.onZoneChanged(zone))
        }
    }
    return tea.Batch(cmds...)
}
```

### Use Placeholder for Dropdown Loading States

When a dropdown's options are loaded asynchronously, set `field.SetPlaceholder("Loading...")` before the fetch and clear it when options arrive. The closed dropdown shows the placeholder instead of "(none)" when `len(Options) == 0`.

```go
// Before async fetch
field.SetPlaceholder("Loading...")

// After options loaded
field.SetPlaceholder("")
field.SetOptions(options)
```

## Dropdown Scrolling

Large dropdowns (100+ options like machine types) render a scrollable window of `dropdownMaxVisible = 10` items with `↑ N more` / `↓ N more` scroll indicators. The `dropdownScrollOffset` field tracks the first visible option.

- `ensureDropdownScrollVisible()` adjusts offset after each navigation
- `EstimatedHeight()` returns the capped visible height (not total options) for `scrollToFocused()` calculations
- `scrollToFocused()` uses `field.EstimatedHeight()` instead of hardcoded 4 lines per field

## Scroll Position Estimation

`scrollToFocused()` and `estimateSectionLines()` estimate field positions by summing section headers and `EstimatedHeight()` per field. These estimates **must account for extra lines that `section.View()` adds** beyond individual field heights:

- **Expanded section**: +2 extra lines (Container `MarginBottom(1)` wrapping fields + trailing `"\n"` between sections)
- **Collapsed section**: +1 extra line (trailing `"\n"`, no Container since content is empty)

Without these, the cumulative drift is ~2 lines per preceding section. By a 4th section, the estimated position is 6+ lines off, causing the viewport to scroll to the wrong place.

**Debugging tip**: When scroll estimates seem wrong, write a test that renders actual section content and compares newline counts rather than reasoning about lipgloss margin interactions:

```go
var content strings.Builder
for _, sec := range form.Sections() {
    content.WriteString(sec.View())
}
actualNewlines := strings.Count(content.String(), "\n")
estimated := form.estimateSectionLines()
assert.InDelta(t, actualNewlines, estimated, 1)
```

## Scroll After Every Section Delegation in Form.Update()

**Critical**: Every code path in `Form.Update()` that calls `section.Update(msg)` **must** call `f.scrollToFocused()` afterward. Dropdown open/close changes the field's `EstimatedHeight()`, and the viewport needs to adjust.

The generic delegation at the bottom of `Update()` already does this, but early-return paths (Enter key, Down/Up when dropdown is open) bypass it. If you add a new key handler that delegates to a section, always pair it:

```go
// WRONG — early return skips scroll update
if field.dropdownOpen {
    return section.Update(msg)
}

// CORRECT — scroll after delegation
if field.dropdownOpen {
    cmd := section.Update(msg)
    f.scrollToFocused()
    return cmd
}
```

**Symptom**: Dropdown options render outside the visible viewport area when navigating with arrow keys.

## Edge Cases

- **Number field**: `SetValue()` accepts int, int64, float64, string. Float values truncated.
- **MultiSelect**: Always returns `[]string`, never nil. Empty slice when no selections.
- **Dropdown open**: Up/Down keys captured by dropdown when open. Form navigation paused.
- **Dropdown scrolling**: Large lists show max 10 items with scroll indicators. Offset resets on open.
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
