# Patterns & Approaches - Resource Editing Framework

**Date:** 2026-01-18
**Purpose:** Reference guide for implementation patterns, best practices, and design decisions

---

## Table of Contents

1. [Form Component Patterns](#form-component-patterns)
2. [State Management Patterns](#state-management-patterns)
3. [Validation Patterns](#validation-patterns)
4. [API Integration Patterns](#api-integration-patterns)
5. [Error Handling Patterns](#error-handling-patterns)
6. [Testing Patterns](#testing-patterns)
7. [UI/UX Patterns](#uiux-patterns)
8. [Performance Patterns](#performance-patterns)

---

## Form Component Patterns

### Pattern 1: Composable Form Building

**Problem:** Need flexible way to build forms with varying complexity

**Solution:** Hierarchical composition with three levels

```go
// Level 1: Individual fields
nameField := forms.NewTextField("name", "Instance Name")
nameField.SetValidator(validators.ValidateGCPResourceName)
nameField.SetRequired(true)

// Level 2: Group fields into sections
basicSection := forms.NewFormSection("basic", "Basic Configuration")
basicSection.AddField(nameField)
basicSection.AddField(zoneField)

// Level 3: Combine sections into form
form := forms.NewFormView("Create Instance", forms.FormModeCreate)
form.AddSection(basicSection)
form.AddSection(advancedSection)
```

**Benefits:**
- Reusable components
- Easy to test in isolation
- Clear separation of concerns
- Can mix and match sections

**When to use:** Always, for any form construction

---

### Pattern 2: Field Type Strategy

**Problem:** Different input types need different rendering and behavior

**Solution:** Field type enum with polymorphic rendering

```go
type FieldType int

const (
    FieldText FieldType = iota
    FieldNumber
    FieldDropdown
    FieldMultiSelect
    FieldToggle
    FieldReadOnly
)

func (f *FormField) View() string {
    switch f.Type {
    case FieldText:
        return f.renderText()
    case FieldDropdown:
        return f.renderDropdown()
    // ... etc
    }
}
```

**Benefits:**
- Single field struct handles all types
- Easy to add new types
- Consistent interface

**When to use:** When building FormField component

---

### Pattern 3: Declarative Form Schema

**Problem:** Form construction code is verbose and repetitive

**Solution:** Define form structure declaratively

```go
type FieldSchema struct {
    ID          string
    Label       string
    Type        FieldType
    DefaultValue interface{}
    Options     []string
    Required    bool
    Validator   func(interface{}) error
    HelpText    string
}

func BuildFormFromSchema(schemas []FieldSchema) *FormView {
    form := forms.NewFormView(...)
    for _, schema := range schemas {
        field := forms.NewFormFieldFromSchema(schema)
        form.AddField(field)
    }
    return form
}
```

**Benefits:**
- DRY - schema can be reused
- Easy to generate from external config
- Clear structure

**When to use:** When forms have many similar fields (e.g., labels, metadata)

---

### Pattern 4: Form State Machine

**Problem:** Forms have multiple states (editing, validating, showing diff, saving)

**Solution:** Explicit state enum with transitions

```go
type editorState int

const (
    stateForm editorState = iota
    stateDiff
    stateSaving
    stateError
    stateDone
)

func (e *Editor) Update(msg tea.Msg) tea.Cmd {
    switch e.state {
    case stateForm:
        // Handle form editing
        if msg == SubmitMsg {
            if errs := e.form.Validate(); len(errs) == 0 {
                e.state = stateDiff
                e.diff = e.buildDiff()
            }
        }

    case stateDiff:
        // Handle diff confirmation
        if msg == ConfirmMsg {
            e.state = stateSaving
            return e.saveCmd()
        } else if msg == CancelMsg {
            e.state = stateForm
        }

    case stateSaving:
        // Handle async save result
        if msg == SaveSuccessMsg {
            e.state = stateDone
            return emitCompleteMsg
        } else if msg == SaveErrorMsg {
            e.state = stateError
        }
    }
}
```

**Benefits:**
- Clear state transitions
- Easy to reason about flow
- Prevents invalid states

**When to use:** For all editors with multi-step workflows

---

## State Management Patterns

### Pattern 5: Message-Driven Architecture

**Problem:** Need to coordinate between views, editors, and app

**Solution:** Use Bubble Tea's message passing pattern

```go
// Define message types
type EditRequestMsg struct {
    ResourceType string
    ResourceID   string
    Mode         EditorMode
}

type EditCompleteMsg struct {
    ResourceType string
    ResourceID   string
}

// Views emit messages
func (v *DisksView) Update(msg tea.Msg) tea.Cmd {
    if key.Matches(msg, keys.Edit) {
        return func() tea.Msg {
            return EditRequestMsg{
                ResourceType: "disk",
                ResourceID:   v.getSelectedDisk().ID,
                Mode:         EditorModeEdit,
            }
        }
    }
}

// App catches and routes messages
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case EditRequestMsg:
        editor := NewEditor(msg)
        a.currentView = ViewEditor
        return a, editor.Init()

    case EditCompleteMsg:
        a.showSuccess("Changes saved")
        return a, a.refreshView()
    }
}
```

**Benefits:**
- Loose coupling between components
- Easy to add new message types
- Testable (mock messages)

**When to use:** Always, for cross-component communication

---

### Pattern 6: View Stack Navigation

**Problem:** Need to track navigation history (back button)

**Solution:** Maintain view stack in app state

```go
type App struct {
    currentView View
    viewStack   []View  // History stack
}

func (a *App) pushView(view View) {
    a.viewStack = append(a.viewStack, a.currentView)
    a.currentView = view
}

func (a *App) popView() {
    if len(a.viewStack) == 0 {
        return
    }
    a.currentView = a.viewStack[len(a.viewStack)-1]
    a.viewStack = a.viewStack[:len(a.viewStack)-1]
}

// In Update
case key.Matches(msg, keys.Esc):
    a.popView()
    return a, nil
```

**Benefits:**
- Natural back navigation
- Preserves view state
- Easy breadcrumbs

**When to use:** Already implemented in gcon, extend for editors

---

### Pattern 7: Dirty State Tracking

**Problem:** Need to warn users about unsaved changes

**Solution:** Track original state and compare

```go
type Editor struct {
    originalData map[string]interface{}
    currentData  map[string]interface{}
}

func (e *Editor) IsDirty() bool {
    return !reflect.DeepEqual(e.originalData, e.currentData)
}

func (e *Editor) Update(msg tea.Msg) tea.Cmd {
    if key.Matches(msg, keys.Esc) {
        if e.IsDirty() {
            return showConfirmDialog("Discard unsaved changes?")
        }
        return emitCancelMsg
    }
}
```

**Benefits:**
- Prevents accidental data loss
- Clear user intent
- Better UX

**When to use:** All editors with user input

---

## Validation Patterns

### Pattern 8: Multi-Level Validation

**Problem:** Need validation at field, section, and form levels

**Solution:** Validate at each level, aggregate errors

```go
// Level 1: Field validation (on blur/change)
func (f *FormField) Validate() error {
    if f.Validator == nil {
        return nil
    }
    return f.Validator(f.Value)
}

// Level 2: Section validation (before proceeding to next section)
func (s *FormSection) Validate() []error {
    var errors []error
    for _, field := range s.Fields {
        if err := field.Validate(); err != nil {
            errors = append(errors, err)
        }
    }
    return errors
}

// Level 3: Form validation (before submit)
func (f *FormView) Validate() []string {
    var messages []string
    for _, section := range f.Sections {
        for _, err := range section.Validate() {
            messages = append(messages, err.Error())
        }
    }
    // Add cross-field validation
    if conflicts := f.checkConflicts(); len(conflicts) > 0 {
        messages = append(messages, conflicts...)
    }
    return messages
}
```

**Benefits:**
- Early feedback at field level
- Comprehensive check before submit
- Cross-field validation at form level

**When to use:** Always, for all forms

---

### Pattern 9: Validator Functions

**Problem:** Need reusable validation logic

**Solution:** Define validator function type, create library

```go
type Validator func(interface{}) error

// Common validators
func ValidateRequired(v interface{}) error {
    if v == nil || v == "" {
        return fmt.Errorf("required field")
    }
    return nil
}

func ValidateRange(min, max int) Validator {
    return func(v interface{}) error {
        val := v.(int)
        if val < min || val > max {
            return fmt.Errorf("must be between %d and %d", min, max)
        }
        return nil
    }
}

func ValidatePattern(pattern string) Validator {
    re := regexp.MustCompile(pattern)
    return func(v interface{}) error {
        if !re.MatchString(v.(string)) {
            return fmt.Errorf("invalid format")
        }
        return nil
    }
}

// Compose validators
func ComposeValidators(validators ...Validator) Validator {
    return func(v interface{}) error {
        for _, validator := range validators {
            if err := validator(v); err != nil {
                return err
            }
        }
        return nil
    }
}

// Usage
nameField.SetValidator(ComposeValidators(
    ValidateRequired,
    ValidatePattern(`^[a-z0-9-]+$`),
    ValidateRange(3, 63),
))
```

**Benefits:**
- Reusable across fields and forms
- Composable
- Easy to test
- Type-safe

**When to use:** For all validation logic

---

### Pattern 10: Debounced Validation

**Problem:** Real-time validation on every keystroke is expensive

**Solution:** Debounce validation after user stops typing

```go
type FormField struct {
    // ...
    validationDebounce time.Duration
    lastInput          time.Time
}

func (f *FormField) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Update value
        f.Value = msg.String()
        f.lastInput = time.Now()

        // Schedule validation after debounce period
        return tea.Tick(f.validationDebounce, func(t time.Time) tea.Msg {
            return validateMsg{fieldID: f.ID}
        })

    case validateMsg:
        // Only validate if no input since scheduled
        if time.Since(f.lastInput) >= f.validationDebounce {
            f.error = f.Validate()
        }
    }
}
```

**Benefits:**
- Reduces CPU usage
- Better UX (fewer flashing errors)
- Still feels real-time

**When to use:** For text fields with expensive validation (API calls, regex)

---

## API Integration Patterns

### Pattern 11: Async Command Pattern

**Problem:** GCP API calls block UI

**Solution:** Use tea.Cmd for non-blocking async calls

```go
func (e *Editor) saveChanges() tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        err := e.computeClient.SetMachineType(ctx, e.projectID, e.zone, e.instanceName, e.newMachineType)
        if err != nil {
            return SaveErrorMsg{err: err}
        }

        return SaveSuccessMsg{resourceID: e.instanceName}
    }
}

func (e *Editor) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case SaveSuccessMsg:
        e.state = stateDone
        return emitEditCompleteMsg

    case SaveErrorMsg:
        e.state = stateError
        e.errorMsg = msg.err.Error()
        return nil
    }
}
```

**Benefits:**
- Non-blocking UI
- Proper error handling
- Progress indication via state

**When to use:** Always, for all GCP API calls

---

### Pattern 12: Multi-Step Operations

**Problem:** Some changes require multiple API calls (stop, change, start)

**Solution:** Chain commands with progress tracking

```go
type operationStep int

const (
    stepStop operationStep = iota
    stepChange
    stepStart
    stepDone
)

func (e *Editor) executeMachineTypeChange() tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()

        // Step 1: Stop instance
        if e.instance.Status == "RUNNING" {
            if err := e.computeClient.StopInstance(ctx, ...); err != nil {
                return SaveErrorMsg{err: err}
            }
            // Wait for stop to complete
            if err := e.waitForOperation(ctx, ...); err != nil {
                return SaveErrorMsg{err: err}
            }
        }

        // Step 2: Change machine type
        if err := e.computeClient.SetMachineType(ctx, ...); err != nil {
            return SaveErrorMsg{err: err}
        }

        // Step 3: Restart instance
        if e.instance.Status == "RUNNING" {
            if err := e.computeClient.StartInstance(ctx, ...); err != nil {
                return SaveErrorMsg{err: err}
            }
        }

        return SaveSuccessMsg{}
    }
}

// Show progress
func (e *Editor) View() string {
    if e.state == stateSaving {
        switch e.currentStep {
        case stepStop:
            return e.renderProgress("Stopping instance...", 33)
        case stepChange:
            return e.renderProgress("Changing machine type...", 66)
        case stepStart:
            return e.renderProgress("Starting instance...", 100)
        }
    }
}
```

**Benefits:**
- User sees progress
- Can rollback on failure
- Clear status messages

**When to use:** For operations requiring multiple API calls

---

### Pattern 13: Optimistic Updates with Rollback

**Problem:** API calls are slow, want instant feedback

**Solution:** Update UI immediately, rollback on error

```go
func (v *DisksView) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case ResizeRequestMsg:
        // Optimistically update local state
        disk := v.getSelectedDisk()
        originalSize := disk.SizeGB
        disk.SizeGB = msg.newSize
        v.updateDisk(disk)

        // Call API
        return func() tea.Msg {
            err := v.computeClient.ResizeDisk(...)
            if err != nil {
                // Rollback on error
                disk.SizeGB = originalSize
                return ResizeErrorMsg{err: err, originalSize: originalSize}
            }
            return ResizeSuccessMsg{}
        }
    }
}
```

**Benefits:**
- Instant feedback
- Better perceived performance
- Handles errors gracefully

**When to use:** For operations that rarely fail (caution: can confuse users if fails often)

---

## Error Handling Patterns

### Pattern 14: Graceful Error Recovery

**Problem:** Errors shouldn't crash the app

**Solution:** Display errors with recovery options

```go
type ErrorDisplay struct {
    title   string
    message string
    err     error
    actions []ErrorAction
}

type ErrorAction struct {
    label   string
    handler func() tea.Cmd
}

func (e *Editor) handleSaveError(err error) tea.Cmd {
    e.errorDisplay = &ErrorDisplay{
        title:   "Failed to Save Changes",
        message: err.Error(),
        err:     err,
        actions: []ErrorAction{
            {
                label:   "Retry",
                handler: e.saveChanges,
            },
            {
                label:   "Edit Again",
                handler: func() tea.Cmd {
                    e.state = stateForm
                    return nil
                },
            },
            {
                label:   "Cancel",
                handler: func() tea.Cmd {
                    return emitCancelMsg
                },
            },
        },
    }
    e.state = stateError
    return nil
}
```

**Benefits:**
- User not stuck
- Clear error messages
- Multiple recovery paths

**When to use:** For all API errors and critical failures

---

### Pattern 15: Error Context Enrichment

**Problem:** Generic errors don't tell user what to do

**Solution:** Add context and suggestions

```go
func enrichError(err error, context string) error {
    if strings.Contains(err.Error(), "403") {
        return fmt.Errorf("%s: permission denied. Ensure you have 'compute.instances.setMachineType' permission", context)
    }
    if strings.Contains(err.Error(), "already exists") {
        return fmt.Errorf("%s: resource already exists. Try a different name", context)
    }
    if strings.Contains(err.Error(), "timeout") {
        return fmt.Errorf("%s: operation timed out. Check your internet connection and try again", context)
    }
    return fmt.Errorf("%s: %w", context, err)
}

// Usage
if err := e.computeClient.CreateInstance(...); err != nil {
    return SaveErrorMsg{err: enrichError(err, "Failed to create instance")}
}
```

**Benefits:**
- Actionable error messages
- Reduces support burden
- Better UX

**When to use:** Always, wrap all GCP API errors

---

### Pattern 16: Validation Error Display

**Problem:** Multiple validation errors hard to read

**Solution:** Group and format errors clearly

```go
func (f *FormView) renderValidationErrors() string {
    if len(f.errors) == 0 {
        return ""
    }

    var lines []string
    lines = append(lines, styles.ErrorHeader.Render("⚠ Please fix the following errors:"))

    // Group by section
    errorsBySection := make(map[string][]string)
    for _, err := range f.errors {
        section := f.getSectionForError(err)
        errorsBySection[section] = append(errorsBySection[section], err)
    }

    for section, errors := range errorsBySection {
        lines = append(lines, fmt.Sprintf("\n%s:", section))
        for _, err := range errors {
            lines = append(lines, fmt.Sprintf("  • %s", err))
        }
    }

    return styles.ErrorBox.Render(strings.Join(lines, "\n"))
}
```

**Benefits:**
- Easy to scan
- Shows which section has errors
- Clear action items

**When to use:** When displaying form-level validation errors

---

## Testing Patterns

### Pattern 17: Table-Driven Tests

**Problem:** Need to test many input/output combinations

**Solution:** Use Go's table-driven test pattern

```go
func TestFormFieldValidation(t *testing.T) {
    tests := []struct {
        name      string
        fieldType FieldType
        value     interface{}
        validator Validator
        wantError bool
    }{
        {
            name:      "required field with value",
            fieldType: FieldText,
            value:     "test",
            validator: ValidateRequired,
            wantError: false,
        },
        {
            name:      "required field empty",
            fieldType: FieldText,
            value:     "",
            validator: ValidateRequired,
            wantError: true,
        },
        {
            name:      "number in range",
            fieldType: FieldNumber,
            value:     50,
            validator: ValidateRange(0, 100),
            wantError: false,
        },
        {
            name:      "number out of range",
            fieldType: FieldNumber,
            value:     150,
            validator: ValidateRange(0, 100),
            wantError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            field := NewFormField("test", "Test", tt.fieldType)
            field.SetValue(tt.value)
            field.SetValidator(tt.validator)

            err := field.Validate()
            if tt.wantError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

**Benefits:**
- Comprehensive coverage
- Easy to add cases
- Clear documentation of behavior

**When to use:** For all validation, parsing, formatting logic

---

### Pattern 18: Mock GCP Client

**Problem:** Can't test without hitting real GCP API

**Solution:** Interface-based mocking

```go
// Define interface
type ComputeClientInterface interface {
    ResizeDisk(ctx context.Context, projectID, zone, diskName string, newSize int64) error
    CreateInstance(ctx context.Context, projectID, zone string, config *InstanceConfig) error
    // ... other methods
}

// Mock implementation
type MockComputeClient struct {
    ResizeDiskFunc       func(ctx context.Context, projectID, zone, diskName string, newSize int64) error
    CreateInstanceFunc   func(ctx context.Context, projectID, zone string, config *InstanceConfig) error
}

func (m *MockComputeClient) ResizeDisk(ctx context.Context, projectID, zone, diskName string, newSize int64) error {
    if m.ResizeDiskFunc != nil {
        return m.ResizeDiskFunc(ctx, projectID, zone, diskName, newSize)
    }
    return nil
}

// Test usage
func TestDiskEditorResize(t *testing.T) {
    mockClient := &MockComputeClient{
        ResizeDiskFunc: func(ctx context.Context, projectID, zone, diskName string, newSize int64) error {
            assert.Equal(t, "my-disk", diskName)
            assert.Equal(t, int64(100), newSize)
            return nil
        },
    }

    editor := NewDiskEditor(mockClient, testDisk, EditorModeEdit)
    // ... test editor
}
```

**Benefits:**
- Fast tests (no network)
- Control over responses
- Can test error cases

**When to use:** For all tests involving GCP API calls

---

### Pattern 19: Snapshot Testing for UI

**Problem:** Hard to test that UI renders correctly

**Solution:** Snapshot tests (golden files)

```go
func TestFormViewRender(t *testing.T) {
    form := buildTestForm()
    form.SetSize(80, 24)

    rendered := form.View()

    golden := filepath.Join("testdata", "form_view.golden")
    if *update {
        // Update golden file with -update flag
        os.WriteFile(golden, []byte(rendered), 0644)
    }

    expected, err := os.ReadFile(golden)
    require.NoError(t, err)

    assert.Equal(t, string(expected), rendered)
}
```

**Benefits:**
- Catches unintended UI changes
- Easy to review diffs
- Documents expected output

**When to use:** For complex UI components with stable output

---

## UI/UX Patterns

### Pattern 20: Progressive Disclosure

**Problem:** Forms with many fields are overwhelming

**Solution:** Show only essential fields, hide advanced behind toggle

```go
func (e *Editor) buildForm() *FormView {
    form := forms.NewFormView("Create Instance", forms.FormModeCreate)

    // Always visible
    basicSection := forms.NewFormSection("basic", "Basic Configuration")
    basicSection.AddField(nameField)
    basicSection.AddField(zoneField)
    form.AddSection(basicSection)

    // Collapsible advanced section
    advancedSection := forms.NewFormSection("advanced", "Advanced Options")
    advancedSection.SetCollapsible(true)
    advancedSection.SetCollapsed(true)  // Hidden by default
    advancedSection.AddField(deletionProtectionField)
    advancedSection.AddField(preemptibleField)
    form.AddSection(advancedSection)

    return form
}
```

**Benefits:**
- Simpler for beginners
- Power users can expand
- Less scrolling

**When to use:** For forms with 10+ fields or optional fields

---

### Pattern 21: Smart Defaults

**Problem:** Users don't know what values to choose

**Solution:** Set sensible defaults based on context

```go
func (e *InstanceEditor) buildCloneForm() *FormView {
    // Smart name generation
    defaultName := e.generateUniqueName(e.sourceInstance.Name)

    // Inherit source settings as defaults
    machineType := e.sourceInstance.MachineType
    zone := e.sourceInstance.Zone
    diskSize := e.sourceInstance.BootDisk.SizeGB

    // But suggest improvements
    if e.sourceInstance.IsOverutilized() {
        // Suggest larger machine type
        machineType = e.suggestLargerMachineType(machineType)
    }

    nameField.SetValue(defaultName)
    machineTypeField.SetValue(machineType)
    zoneField.SetValue(zone)
    diskSizeField.SetValue(diskSize)

    return form
}
```

**Benefits:**
- Faster workflow
- Fewer mistakes
- Guided experience

**When to use:** Always, for all forms

---

### Pattern 22: Inline Help

**Problem:** Users don't understand field purpose

**Solution:** Contextual help text below fields

```go
func (e *Editor) buildForm() *FormView {
    machineTypeField := forms.NewDropdownField("machine_type", "Machine Type")
    machineTypeField.SetHelpText(
        "e2: Cost-optimized | n2: Balanced | c3: Compute-optimized | m3: Memory-optimized",
    )

    diskTypeField := forms.NewDropdownField("disk_type", "Disk Type")
    diskTypeField.SetHelpText(
        "SSD: Fast, $0.17/GB/month | Standard: Slow, $0.04/GB/month",
    )

    return form
}
```

**Benefits:**
- Self-documenting
- No need to check docs
- Reduces errors

**When to use:** For all non-obvious fields

---

### Pattern 23: Confirmation with Preview

**Problem:** Users unsure of what will happen

**Solution:** Show exact changes before applying

```go
func (e *Editor) buildDiff() *DiffViewer {
    diff := forms.NewDiffViewer("Confirm Changes")

    // Add changed fields
    diff.AddField("Machine Type",
        e.originalInstance.MachineType,
        e.form.GetValue("machine_type").(string))

    diff.AddField("Disk Size",
        fmt.Sprintf("%d GB", e.originalInstance.DiskSize),
        fmt.Sprintf("%d GB", e.form.GetValue("disk_size").(int)))

    // Add warnings
    if e.requiresRestart() {
        diff.AddWarning("Instance will be restarted to apply changes")
        diff.AddWarning("Estimated downtime: 1-2 minutes")
    }

    // Add cost impact
    costDiff := e.calculateCostDiff()
    if costDiff > 0 {
        diff.SetCostImpact(fmt.Sprintf("+$%.2f/hour (+$%.0f/month)", costDiff, costDiff*730))
    } else if costDiff < 0 {
        diff.SetCostImpact(fmt.Sprintf("-$%.2f/hour (-$%.0f/month)", -costDiff, -costDiff*730))
    } else {
        diff.SetCostImpact("No cost change")
    }

    return diff
}
```

**Benefits:**
- User confidence
- Catch mistakes before commit
- No surprises

**When to use:** Always, before applying changes

---

## Performance Patterns

### Pattern 24: Lazy Loading

**Problem:** Loading all options upfront is slow

**Solution:** Load on-demand when field is activated

```go
type FormField struct {
    // ...
    optionsLoader func() ([]string, error)
    optionsLoaded bool
}

func (f *FormField) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.Type == tea.KeyEnter && f.Type == FieldDropdown {
            if !f.optionsLoaded {
                // Load options when user opens dropdown
                return func() tea.Msg {
                    options, err := f.optionsLoader()
                    if err != nil {
                        return optionsErrorMsg{err: err}
                    }
                    return optionsLoadedMsg{options: options}
                }
            }
        }

    case optionsLoadedMsg:
        f.Options = msg.options
        f.optionsLoaded = true
        f.showDropdown = true
    }
}
```

**Benefits:**
- Faster initial render
- Reduced API calls
- Better perceived performance

**When to use:** For dropdowns with 100+ options or options from API

---

### Pattern 25: Debounced Rendering

**Problem:** Every keystroke triggers expensive render

**Solution:** Batch renders with debouncing

```go
type FormView struct {
    // ...
    renderDebounce time.Duration
    lastUpdate     time.Time
    pendingUpdate  bool
}

func (f *FormView) Update(msg tea.Msg) tea.Cmd {
    // Update model
    f.handleMessage(msg)

    // Schedule render after debounce
    f.pendingUpdate = true
    f.lastUpdate = time.Now()

    return tea.Tick(f.renderDebounce, func(t time.Time) tea.Msg {
        return renderMsg{time: t}
    })
}

func (f *FormView) View() string {
    // Only render if debounce period passed
    if f.pendingUpdate && time.Since(f.lastUpdate) >= f.renderDebounce {
        f.cachedView = f.doRender()
        f.pendingUpdate = false
    }
    return f.cachedView
}
```

**Benefits:**
- Smoother typing experience
- Less CPU usage
- Still feels responsive

**When to use:** For forms with many fields or expensive rendering

---

### Pattern 26: Virtual Scrolling

**Problem:** Long lists (1000+ items) are slow to render

**Solution:** Only render visible items

```go
type Dropdown struct {
    options       []string
    viewport      viewport.Model
    itemHeight    int
    visibleCount  int
}

func (d *Dropdown) View() string {
    // Calculate which items are visible
    scrollOffset := d.viewport.YOffset / d.itemHeight
    visibleStart := scrollOffset
    visibleEnd := min(scrollOffset+d.visibleCount, len(d.options))

    // Render only visible items
    var lines []string
    for i := visibleStart; i < visibleEnd; i++ {
        lines = append(lines, d.renderOption(d.options[i]))
    }

    return d.viewport.View(strings.Join(lines, "\n"))
}
```

**Benefits:**
- Constant render time regardless of list size
- Smooth scrolling
- Lower memory usage

**When to use:** For lists with 1000+ items (e.g., image selector, zone picker)

---

## Anti-Patterns to Avoid

### ❌ Anti-Pattern 1: Blocking API Calls in Update

**Don't:**
```go
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
    case SubmitMsg:
        // WRONG: Blocks UI
        err := e.computeClient.CreateInstance(...)
        if err != nil {
            e.error = err
        }
}
```

**Do:**
```go
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
    case SubmitMsg:
        // RIGHT: Async command
        return func() tea.Msg {
            err := e.computeClient.CreateInstance(...)
            if err != nil {
                return SaveErrorMsg{err: err}
            }
            return SaveSuccessMsg{}
        }
}
```

---

### ❌ Anti-Pattern 2: Tight Coupling

**Don't:**
```go
// WRONG: Form directly calls GCP API
func (f *FormView) Submit() tea.Cmd {
    data := f.GetFormData()
    return func() tea.Msg {
        err := gcpClient.CreateInstance(...)
        // ...
    }
}
```

**Do:**
```go
// RIGHT: Form emits message, parent handles API
func (f *FormView) Submit() tea.Cmd {
    return func() tea.Msg {
        return FormSubmittedMsg{data: f.GetFormData()}
    }
}

// Editor handles API
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
    case FormSubmittedMsg:
        return e.createInstance(msg.data)
}
```

---

### ❌ Anti-Pattern 3: Silent Failures

**Don't:**
```go
// WRONG: Swallows error
err := e.computeClient.SetLabels(...)
if err != nil {
    log.Println("Error:", err)  // User doesn't see this!
}
```

**Do:**
```go
// RIGHT: Shows error to user
err := e.computeClient.SetLabels(...)
if err != nil {
    return SaveErrorMsg{err: enrichError(err, "Failed to set labels")}
}
```

---

### ❌ Anti-Pattern 4: Magic Values

**Don't:**
```go
// WRONG: Unclear constants
if diskSize > 65536 {
    return fmt.Errorf("disk too large")
}
```

**Do:**
```go
// RIGHT: Named constants with comments
const (
    // GCP limit for persistent disk size
    maxDiskSizeGB = 65536
)

if diskSize > maxDiskSizeGB {
    return fmt.Errorf("disk size exceeds maximum of %d GB", maxDiskSizeGB)
}
```

---

### ❌ Anti-Pattern 5: Testing Implementation Details

**Don't:**
```go
// WRONG: Tests internal state
func TestFormField(t *testing.T) {
    field := NewFormField(...)
    field.focused = true  // Testing internal implementation
    assert.True(t, field.focused)
}
```

**Do:**
```go
// RIGHT: Tests public behavior
func TestFormField(t *testing.T) {
    field := NewFormField(...)
    field.SetFocus(true)

    rendered := field.View()
    assert.Contains(t, rendered, "focused-style-marker")
}
```

---

## Decision Tree: When to Use Which Pattern

### Form Construction Decision
```
Need to build a form?
├─ Simple (1-3 fields)? → Use single FormSection
├─ Medium (4-10 fields)? → Use multiple FormSections
└─ Complex (10+ fields)? → Use FormView with collapsible sections or Wizard

Have many similar fields?
└─ Use declarative schema (Pattern 3)

Have optional/advanced fields?
└─ Use progressive disclosure (Pattern 20)
```

### Validation Decision
```
When to validate?
├─ On blur (user leaves field)? → Field-level validation
├─ On submit? → Form-level validation
└─ On every keystroke? → Use debounced validation (Pattern 10)

Complex validation rules?
├─ Reusable? → Create Validator function
└─ One-off? → Inline lambda
```

### State Management Decision
```
Need to pass data between components?
└─ Use message passing (Pattern 5)

Need navigation history?
└─ Use view stack (Pattern 6)

Need to track changes?
└─ Use dirty state tracking (Pattern 7)

Complex workflow (form → diff → save)?
└─ Use state machine (Pattern 4)
```

### API Integration Decision
```
Single API call?
└─ Use async command (Pattern 11)

Multiple sequential calls?
└─ Use multi-step operation (Pattern 12)

Can tolerate failure?
└─ Use optimistic update (Pattern 13)

Need progress indication?
└─ Track state and show spinner
```

### Error Handling Decision
```
Expected error (validation)?
└─ Show inline in form

Unexpected error (network)?
└─ Show error dialog with retry (Pattern 14)

API error?
└─ Enrich with context (Pattern 15)

Multiple validation errors?
└─ Group by section (Pattern 16)
```

---

## Quick Reference Checklist

### Before Implementing a New Form
- [ ] Identify required vs optional fields
- [ ] Define validation rules for each field
- [ ] Choose form mode (create, edit, clone)
- [ ] Plan sections and grouping
- [ ] Design diff preview
- [ ] Plan error recovery flows

### Before Implementing a New Editor
- [ ] Define message types (request, complete, error)
- [ ] Design state machine (form, diff, saving, done)
- [ ] Plan GCP API calls needed
- [ ] Consider multi-step operations
- [ ] Design progress indicators
- [ ] Plan rollback on failure

### Before Merging
- [ ] All tests pass
- [ ] Code coverage > 80%
- [ ] Linter passes
- [ ] Manual testing completed
- [ ] Documentation updated
- [ ] Error messages are helpful
- [ ] Keyboard navigation works
- [ ] Performance is acceptable (< 100ms render)

---

## Conclusion

These patterns form the foundation of the resource editing framework. Apply them consistently to ensure:
- **Maintainability** - clear patterns make code easy to understand
- **Testability** - interfaces and message passing enable testing
- **Performance** - async operations and lazy loading keep UI responsive
- **Usability** - validation, preview, and error handling guide users

Refer back to this document when implementing new editors or forms. Update it as new patterns emerge.

---

## Revision History

| Date       | Version | Changes                           |
|------------|---------|-----------------------------------|
| 2026-01-18 | 1.0     | Initial patterns documentation    |
