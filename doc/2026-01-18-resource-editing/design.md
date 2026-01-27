# Resource Editing & Creation Framework - Design Document

**Date:** 2026-01-18
**Status:** Design Phase
**Priority:** High

## Executive Summary

This design adds resource editing and creation capabilities to gcon, allowing users to:
- **Edit existing resources** (instance machine type, disk size, labels, etc.)
- **Create new resources** via three workflows: clone existing, use templates, start from scratch
- Focus on **Compute Instances** and **Disks/Snapshots** initially

### Key Design Decisions

1. **Hybrid UX Approach** - Form overlays for simple edits, wizards for complex creation, text editor for advanced users
2. **Clone-first workflow** - Make copying existing configurations easy to reduce errors
3. **Diff preview + confirmation** - Show before/after changes with cost implications
4. **Action menu + key bindings** - Consistent with existing navigation patterns
5. **Reusable component framework** - Share form UI logic across resource types

---

## Architecture Overview

### Three-Layer Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Layer 3: View Integration                │
│  (Instances, Disks, Snapshots views trigger editors)        │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│                  Layer 2: Resource Editors                  │
│  InstanceEditor | DiskEditor | SnapshotEditor               │
│  (Resource-specific schema, validation, API logic)          │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│                Layer 1: Shared Form Components              │
│  FormField | FormSection | FormView | DiffViewer            │
│  CreateWizard | ValidationDisplay                           │
└─────────────────────────────────────────────────────────────┘
```

### Message Flow Pattern

Consistent with existing Bubble Tea architecture:

```
┌──────────────┐
│  List View   │ User presses 'e' (edit) or 'c' (clone) or 'n' (new)
└──────┬───────┘
       │
       ↓ EditRequestMsg{resourceType, resourceID, mode}
┌──────────────┐
│  App.Update  │ Switches to editor overlay
└──────┬───────┘
       │
       ↓ Show editor with resource data
┌──────────────┐
│ Editor View  │ User edits fields, validation runs
└──────┬───────┘
       │
       ↓ Shows diff preview
┌──────────────┐
│ DiffViewer   │ User confirms or cancels
└──────┬───────┘
       │
       ↓ EditCompleteMsg{newConfig} or EditCancelledMsg
┌──────────────┐
│  App.Update  │ Calls GCP API, shows spinner, returns to list
└──────────────┘
```

---

## Layer 1: Shared Form Components

### 1. FormField Component

Generic input field supporting multiple types.

**File:** `internal/ui/components/forms/field.go`

**Field Types:**
- `Text` - single-line text input
- `Number` - numeric input with min/max validation
- `Dropdown` - single selection from options
- `MultiSelect` - multiple selections (for labels, tags)
- `Toggle` - boolean on/off
- `ReadOnly` - display-only field

**Features:**
- Focus management (highlights when active)
- Real-time validation with error display
- Help text below field
- Label with optional required indicator
- Tab/Shift+Tab navigation between fields
- Enter to activate dropdowns/toggles

**Data Structure:**
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

type FormField struct {
    ID          string
    Label       string
    Type        FieldType
    Value       interface{}
    Options     []string          // For dropdown/multiselect
    Required    bool
    Validator   func(interface{}) error
    HelpText    string
    Placeholder string

    // UI state
    focused     bool
    error       error
    width       int
}
```

**Key Methods:**
- `Init() tea.Cmd`
- `Update(tea.Msg) tea.Cmd`
- `View() string` - renders field based on type
- `SetFocus(bool)`
- `Validate() error`
- `GetValue() interface{}`
- `SetValue(interface{})`

### 2. FormSection Component

Groups related fields with a header.

**File:** `internal/ui/components/forms/section.go`

**Features:**
- Collapsible sections (optional)
- Section header with icon
- Manages focus between contained fields
- Can show/hide based on conditions

**Data Structure:**
```go
type FormSection struct {
    ID          string
    Title       string
    Icon        string
    Fields      []*FormField
    Collapsible bool
    Collapsed   bool

    // UI state
    focusedIdx  int
    width       int
    height      int
}
```

### 3. FormView Component

Container managing entire form with navigation, validation, and submission.

**File:** `internal/ui/components/forms/form.go`

**Features:**
- Manages multiple sections
- Tab/Shift+Tab navigation across all fields
- Global validation (runs all field validators)
- Submit/Cancel buttons
- Keyboard shortcuts display at bottom
- Responsive layout

**Data Structure:**
```go
type FormMode int

const (
    FormModeCreate FormMode = iota
    FormModeEdit
    FormModeClone
)

type FormView struct {
    Title       string
    Sections    []*FormSection
    Mode        FormMode

    // UI state
    focusedSectionIdx int
    focusedFieldIdx   int
    width             int
    height            int
    viewport          viewport.Model

    // Validation
    errors      []string
    showErrors  bool
}
```

**Key Methods:**
- `Init() tea.Cmd`
- `Update(tea.Msg) tea.Cmd`
- `View() string`
- `NextField()` - move focus to next field
- `PrevField()` - move focus to previous field
- `Validate() []string` - run all validators
- `GetFormData() map[string]interface{}` - collect all field values
- `SetFormData(map[string]interface{})` - populate form from data

**Key Bindings:**
- `Tab` / `Shift+Tab` - navigate fields
- `Enter` - activate dropdown/toggle, submit form (when focused on button)
- `Esc` - cancel
- `Ctrl+S` - submit/save
- `?` - show help

### 4. DiffViewer Component

Shows before/after comparison for confirmation.

**File:** `internal/ui/components/forms/diff.go`

**Features:**
- Side-by-side or unified diff view
- Highlights changed fields in color
- Shows cost implications (estimated if available)
- Confirms resource locks/constraints
- Yes/No confirmation at bottom

**Data Structure:**
```go
type DiffField struct {
    Label    string
    OldValue string
    NewValue string
    Changed  bool
    Warning  string  // Optional warning about the change
}

type DiffViewer struct {
    Title       string
    Fields      []DiffField
    CostImpact  string  // e.g., "+$0.05/hour", "No cost change"
    Warnings    []string

    // UI state
    width       int
    height      int
    viewport    viewport.Model
}
```

**Rendering Style:**
```
┌─ Confirm Changes ────────────────────────────────┐
│                                                  │
│  Machine Type                                    │
│    - e2-medium                                   │  (red)
│    + n2-standard-2                               │  (green)
│                                                  │
│  Boot Disk Size                                  │
│    - 10 GB                                       │
│    + 20 GB                                       │
│                                                  │
│  Labels                                          │
│    environment=dev (unchanged)                   │
│    + purpose=testing                             │  (new)
│                                                  │
│  ⚠ Estimated Cost Impact: +$0.08/hour            │
│  ⚠ Instance will restart to apply changes        │
│                                                  │
│  [Yes, apply changes]  [No, go back]             │
└──────────────────────────────────────────────────┘
```

### 5. CreateWizard Component

Multi-step flow controller for complex creation.

**File:** `internal/ui/components/forms/wizard.go`

**Features:**
- Step indicator at top (1/5, 2/5, etc.)
- Each step is a FormSection or custom view
- Next/Back navigation
- Can skip optional steps
- Review step shows all selections before confirm

**Data Structure:**
```go
type WizardStep struct {
    Title       string
    Content     tea.Model  // Could be FormSection or custom component
    Optional    bool
    Validator   func() error
}

type CreateWizard struct {
    Title       string
    Steps       []WizardStep
    CurrentStep int

    // Accumulated data across steps
    data        map[string]interface{}

    // UI state
    width       int
    height      int
}
```

**Key Methods:**
- `NextStep() error` - validate current, move to next
- `PrevStep()`
- `CanProceed() bool` - check if current step is valid
- `GetData() map[string]interface{}` - final accumulated data

---

## Layer 2: Resource Editors

### 1. InstanceEditor

Handles instance creation and editing logic.

**File:** `internal/ui/views/instance_editor.go`

**Edit Capabilities:**
- Machine type (requires restart)
- Disk attachments (add/remove)
- Labels and tags
- Network settings (limited - mostly read-only after creation)
- Deletion protection flag
- Metadata (uses existing metadata editor)

**Create Modes:**

#### A. Clone Existing Instance
- Copy all settings from selected instance
- Auto-rename (append "-copy" or increment number)
- Optionally modify machine type, disks before creation
- Fast and safe

#### B. Create from Template
Predefined templates:
- **Web Server** - e2-medium, 20GB disk, HTTP/HTTPS firewall tags
- **Database** - n2-standard-4, 100GB SSD, no external IP
- **Dev Machine** - e2-small, 10GB disk, preemptible
- **Custom** - user-defined templates saved locally

#### C. Create from Scratch
Full wizard with steps:
1. **Basic Info** - name, zone
2. **Machine Type** - family, type, CPU/memory
3. **Boot Disk** - image selection, size, type
4. **Network** - VPC, subnet, external IP, firewall tags
5. **Review** - diff view of final config

**Form Schema Example (Machine Type Edit):**
```go
func (e *InstanceEditor) buildMachineTypeForm() *FormView {
    return &FormView{
        Title: "Edit Instance Machine Type",
        Mode:  FormModeEdit,
        Sections: []*FormSection{
            {
                Title: "Current Configuration",
                Fields: []*FormField{
                    {
                        ID:    "current_machine_type",
                        Label: "Current Machine Type",
                        Type:  FieldReadOnly,
                        Value: e.instance.MachineType,
                    },
                    {
                        ID:    "current_cpus",
                        Label: "Current vCPUs",
                        Type:  FieldReadOnly,
                        Value: e.instance.CPUs,
                    },
                },
            },
            {
                Title: "New Configuration",
                Fields: []*FormField{
                    {
                        ID:       "machine_family",
                        Label:    "Machine Family",
                        Type:     FieldDropdown,
                        Options:  []string{"e2", "n2", "n2d", "c3", "m3"},
                        Value:    "n2",
                        HelpText: "General-purpose: e2, n2. Compute-optimized: c3. Memory-optimized: m3",
                    },
                    {
                        ID:       "machine_type",
                        Label:    "Machine Type",
                        Type:     FieldDropdown,
                        Options:  e.getMachineTypesForFamily("n2"),
                        Value:    "n2-standard-2",
                        Required: true,
                    },
                },
            },
        },
    }
}
```

**GCP API Integration:**
```go
// Edit instance
func (e *InstanceEditor) applyChanges(ctx context.Context, changes map[string]interface{}) error {
    // Different changes require different API calls
    if machineType, ok := changes["machine_type"]; ok {
        // Stop instance if running
        if e.instance.Status == "RUNNING" {
            if err := e.computeClient.StopInstance(ctx, ...); err != nil {
                return err
            }
        }
        // Change machine type
        if err := e.computeClient.SetMachineType(ctx, ...); err != nil {
            return err
        }
        // Restart if it was running
    }

    if labels, ok := changes["labels"]; ok {
        if err := e.computeClient.SetLabels(ctx, ...); err != nil {
            return err
        }
    }

    return nil
}
```

### 2. DiskEditor

Handles disk editing and creation.

**File:** `internal/ui/views/disk_editor.go`

**Edit Capabilities:**
- Disk size (can only increase, never decrease)
- Labels
- Snapshot schedule
- Disk type (SSD ↔ Standard)

**Create Modes:**

#### A. Create from Snapshot
- Select source snapshot
- Choose size (≥ snapshot size)
- Select type (SSD/Standard)
- Name and zone

#### B. Create from Image
- Select source image (OS images or custom)
- Choose size (≥ image size)
- Similar to snapshot flow

#### C. Create Empty Disk
- Simple form: name, zone, size, type
- Optional labels

**Form Schema Example (Resize Disk):**
```go
func (e *DiskEditor) buildResizeForm() *FormView {
    return &FormView{
        Title: "Resize Persistent Disk",
        Mode:  FormModeEdit,
        Sections: []*FormSection{
            {
                Title: "⚠ Disk Resize Warning",
                Fields: []*FormField{
                    {
                        ID:    "warning",
                        Label: "",
                        Type:  FieldReadOnly,
                        Value: "Disk size can only be increased, never decreased. " +
                               "File system may need manual expansion after resize.",
                    },
                },
            },
            {
                Title: "Size Configuration",
                Fields: []*FormField{
                    {
                        ID:    "current_size",
                        Label: "Current Size",
                        Type:  FieldReadOnly,
                        Value: fmt.Sprintf("%d GB", e.disk.SizeGB),
                    },
                    {
                        ID:       "new_size",
                        Label:    "New Size (GB)",
                        Type:     FieldNumber,
                        Value:    e.disk.SizeGB,
                        Required: true,
                        Validator: func(v interface{}) error {
                            newSize := v.(int)
                            if newSize <= e.disk.SizeGB {
                                return fmt.Errorf("new size must be greater than %d GB", e.disk.SizeGB)
                            }
                            if newSize > 65536 {
                                return fmt.Errorf("maximum disk size is 65536 GB")
                            }
                            return nil
                        },
                        HelpText: fmt.Sprintf("Minimum: %d GB, Maximum: 65536 GB", e.disk.SizeGB+1),
                    },
                },
            },
        },
    }
}
```

### 3. SnapshotEditor

Handles snapshot creation (snapshots are mostly immutable after creation).

**File:** `internal/ui/views/snapshot_editor.go`

**Create Flow:**
- Select source disk
- Name the snapshot
- Choose storage location (regional/multi-regional)
- Optional labels
- Confirm creation

**Simple form (not a wizard):**
```go
func (e *SnapshotEditor) buildCreateForm(sourceDisk *gcp.Disk) *FormView {
    return &FormView{
        Title: "Create Snapshot",
        Mode:  FormModeCreate,
        Sections: []*FormSection{
            {
                Title: "Source",
                Fields: []*FormField{
                    {
                        ID:    "source_disk",
                        Label: "Source Disk",
                        Type:  FieldReadOnly,
                        Value: sourceDisk.Name,
                    },
                },
            },
            {
                Title: "Snapshot Configuration",
                Fields: []*FormField{
                    {
                        ID:        "name",
                        Label:     "Snapshot Name",
                        Type:      FieldText,
                        Required:  true,
                        Validator: validateGCPResourceName,
                        HelpText:  "Lowercase letters, numbers, hyphens",
                    },
                    {
                        ID:       "location",
                        Label:    "Storage Location",
                        Type:     FieldDropdown,
                        Options:  []string{"us (multi-region)", "eu (multi-region)", "us-central1", "europe-west1"},
                        Value:    "us (multi-region)",
                        HelpText: "Multi-regional is more durable but costs slightly more",
                    },
                    {
                        ID:       "labels",
                        Label:    "Labels",
                        Type:     FieldMultiSelect,
                        HelpText: "key=value pairs for organization",
                    },
                },
            },
        },
    }
}
```

---

## Layer 3: View Integration

### Triggering Editors from Views

Existing views (instances.go, disks.go, snapshots.go) will add edit/create actions.

**1. Action Menu Integration**

Add to context menu when user presses `m`:

```go
// In instances.go
func (v *InstancesView) buildActionMenu() []string {
    selected := v.getSelectedInstance()

    actions := []string{
        "View Details",
        "Edit Configuration",      // NEW
        "Clone Instance",           // NEW
    }

    if selected.Status == "RUNNING" {
        actions = append(actions, "Stop")
    } else {
        actions = append(actions, "Start")
    }

    actions = append(actions, "Reset", "Delete")
    return actions
}
```

At the bottom of resource list views, add:
```
Press 'n' to create new | 'e' to edit | 'c' to clone
```

**2. Key Bindings**

Add to `internal/ui/keys.go`:
```go
type KeyMap struct {
    // ... existing keys ...

    Edit   key.Binding  // 'e' - edit selected resource
    Clone  key.Binding  // 'c' - clone selected resource
    Create key.Binding  // 'n' - create new resource
}

var Keys = KeyMap{
    // ... existing bindings ...

    Edit: key.NewBinding(
        key.WithKeys("e"),
        key.WithHelp("e", "edit"),
    ),
    Clone: key.NewBinding(
        key.WithKeys("c"),
        key.WithHelp("c", "clone"),
    ),
    Create: key.NewBinding(
        key.WithKeys("n"),
        key.WithHelp("n", "create new"),
    ),
}
```

**3. Message Types**

Add to `internal/ui/messages.go`:
```go
// EditRequestMsg requests opening an editor
type EditRequestMsg struct {
    ResourceType string        // "instance", "disk", "snapshot"
    ResourceID   string        // Full resource ID/name
    Mode         EditorMode    // Edit, Clone, or Create
    InitialData  interface{}   // Resource object or template
}

// EditCompleteMsg indicates successful edit/creation
type EditCompleteMsg struct {
    ResourceType string
    ResourceID   string
    Action       string  // "edited", "created", "cloned"
}

// EditCancelledMsg indicates user cancelled
type EditCancelledMsg struct{}

// EditorMode determines editor behavior
type EditorMode int

const (
    EditorModeEdit EditorMode = iota
    EditorModeClone
    EditorModeCreate
    EditorModeCreateFromTemplate
)
```

**4. App.Update Handling**

Add to `internal/ui/app.go`:
```go
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case EditRequestMsg:
        // Create appropriate editor based on resource type and mode
        switch msg.ResourceType {
        case "instance":
            editor := NewInstanceEditor(a.computeClient, msg.ResourceID, msg.Mode, msg.InitialData)
            a.currentView = ViewInstanceEditor
            a.instanceEditor = editor
            return a, editor.Init()

        case "disk":
            editor := NewDiskEditor(a.computeClient, msg.ResourceID, msg.Mode, msg.InitialData)
            a.currentView = ViewDiskEditor
            a.diskEditor = editor
            return a, editor.Init()
        }

    case EditCompleteMsg:
        // Show success message, refresh view, return to previous
        a.statusBar.SetMessage(fmt.Sprintf("✓ %s %s successfully", msg.ResourceType, msg.Action))

        // Refresh the list view that triggered the edit
        switch msg.ResourceType {
        case "instance":
            a.currentView = ViewInstances
            return a, a.instancesView.Refresh()
        case "disk":
            a.currentView = ViewDisks
            return a, a.disksView.Refresh()
        }

    case EditCancelledMsg:
        // Return to previous view without changes
        a.popView()
        return a, nil
    }

    // ... rest of update logic
}
```

---

## Implementation Phases

### Phase 1: Core Form Framework (Week 1)
**Goal:** Build reusable form components

- [ ] Create `internal/ui/components/forms/` package
- [ ] Implement `FormField` component with all field types
- [ ] Implement `FormSection` component
- [ ] Implement `FormView` component with navigation
- [ ] Implement `DiffViewer` component
- [ ] Add comprehensive tests for form components
- [ ] Create demo/example view to test forms in isolation

**Deliverable:** Working form framework that can be tested standalone

### Phase 2: Disk Editing (Week 2)
**Goal:** Simplest edit scenario - resize disk

- [ ] Create GCP API methods: `ResizeDisk()` in `internal/gcp/compute.go`
- [ ] Implement `DiskEditor` with resize form
- [ ] Add "Resize" action to disks list view (action menu + 'r' key)
- [ ] Wire up messages in App.Update
- [ ] Add diff viewer for resize confirmation
- [ ] Test resize workflow end-to-end
- [ ] Update documentation with resize feature

**Deliverable:** Users can resize disks from disks list view

### Phase 3: Snapshot Creation (Week 2-3)
**Goal:** Simple creation flow

- [ ] Create GCP API methods: `CreateSnapshot()` in `internal/gcp/compute.go`
- [ ] Implement `SnapshotEditor` with create form
- [ ] Add "Create Snapshot" action to disk details view
- [ ] Add "Create Snapshot" action to disks list view (action menu + 'n' key)
- [ ] Wire up creation flow
- [ ] Test snapshot creation end-to-end
- [ ] Update documentation

**Deliverable:** Users can create snapshots from disk views

### Phase 4: Disk Creation (Week 3)
**Goal:** Create disks from snapshots/images

- [ ] Create GCP API methods: `CreateDisk()`, `ListImages()` in `internal/gcp/compute.go`
- [ ] Implement disk creation form (from snapshot, from image, empty)
- [ ] Add "Create Disk" to snapshots view (create disk from this snapshot)
- [ ] Add "Create Disk" to images view (create disk from this image)
- [ ] Add "Create New Disk" to disks list view
- [ ] Test all disk creation paths
- [ ] Update documentation

**Deliverable:** Users can create disks via multiple workflows

### Phase 5: Instance Editing - Labels (Week 4)
**Goal:** Safe, non-disruptive edit

- [ ] Create GCP API methods: `SetInstanceLabels()` in `internal/gcp/compute.go`
- [ ] Implement instance labels editor (multi-select field)
- [ ] Add "Edit Labels" action to instance details view
- [ ] Test labels editing
- [ ] Update documentation

**Deliverable:** Users can edit instance labels

### Phase 6: Instance Editing - Machine Type (Week 4-5)
**Goal:** More complex edit requiring restart

- [ ] Create GCP API methods: `SetMachineType()` in `internal/gcp/compute.go`
- [ ] Implement machine type form with family/type selection
- [ ] Add restart warning to diff viewer
- [ ] Add "Change Machine Type" to action menu
- [ ] Handle instance stop/change/start workflow
- [ ] Show progress during multi-step operation
- [ ] Test machine type changes
- [ ] Update documentation

**Deliverable:** Users can change instance machine types

### Phase 7: CreateWizard Component (Week 5)
**Goal:** Multi-step flow framework

- [ ] Implement `CreateWizard` component
- [ ] Add step navigation (Next/Back)
- [ ] Add progress indicator
- [ ] Add review step template
- [ ] Test wizard navigation

**Deliverable:** Working wizard framework

### Phase 8: Instance Creation - Clone (Week 6)
**Goal:** Easiest creation path

- [ ] Create GCP API method: `CloneInstance()` or build config from existing
- [ ] Implement clone workflow in `InstanceEditor`
- [ ] Add "Clone Instance" action to instances list
- [ ] Auto-generate unique name (append -copy or -2, etc.)
- [ ] Show diff of cloned config before creation
- [ ] Test cloning instances
- [ ] Update documentation

**Deliverable:** Users can clone existing instances

### Phase 9: Instance Creation - Templates (Week 6-7)
**Goal:** Predefined configurations

- [ ] Define template data structure
- [ ] Create 3-4 predefined templates (web, db, dev)
- [ ] Implement template selection UI
- [ ] Allow template customization before creation
- [ ] Add "Create from Template" to instances view
- [ ] Test template-based creation
- [ ] Update documentation

**Deliverable:** Users can create instances from templates

### Phase 10: Instance Creation - Full Wizard (Week 7-8)
**Goal:** Complete control for advanced users

- [ ] Implement 5-step instance creation wizard:
  1. Basic Info (name, zone)
  2. Machine Type (family, type)
  3. Boot Disk (image selection, size, type)
  4. Network (VPC, subnet, firewall tags)
  5. Review (full config diff)
- [ ] Add image browser/selector component
- [ ] Add network configuration form
- [ ] Wire up full creation flow
- [ ] Test comprehensive instance creation
- [ ] Update documentation

**Deliverable:** Users can create instances from scratch with full control

### Phase 11: Polish & Advanced Features (Week 9)
**Goal:** Nice-to-have enhancements

- [ ] Add cost estimation in diff viewer (query GCP pricing API)
- [ ] Add template saving (save custom templates locally)
- [ ] Add form field search/filter (for long dropdown lists)
- [ ] Add keyboard shortcuts help overlay
- [ ] Add form auto-save/recovery on crash
- [ ] Performance optimization for large forms
- [ ] Update comprehensive documentation

**Deliverable:** Production-ready editing framework

---

## Technical Considerations

### 1. GCP API Rate Limits
- Cache machine type lists, image lists (rarely change)
- Implement exponential backoff on API errors
- Show loading states during API calls

### 2. Error Handling
- Network errors: show retry option
- Permission errors: show helpful message with required roles
- Validation errors: inline field-level errors
- API errors: display GCP error message verbatim

### 3. State Management
- Editors should be stateful (don't lose edits on window resize)
- Implement dirty state tracking (warn on cancel if modified)
- Store form state in App struct to survive view switches

### 4. Testing Strategy
- Unit tests for all form components
- Unit tests for parsers/validators
- Integration tests for editor workflows
- Manual testing in real GCP project

### 5. Accessibility
- Clear focus indicators
- Keyboard navigation for all actions
- Help text for complex fields
- Error messages that explain how to fix

### 6. Performance
- Lazy-load dropdown options (don't fetch all images upfront)
- Debounce validation on text input
- Virtual scrolling for long forms
- Efficient re-renders (only update changed sections)

---

## UX Patterns & Conventions

### Visual Hierarchy
```
┌─ Edit Instance: my-instance ────────────────┐
│                                              │
│  ▸ Basic Configuration                      │  ← Section (collapsible)
│    Name: my-instance                         │  ← Read-only field (grayed)
│    Zone: us-central1-a                       │
│                                              │
│  ▾ Machine Configuration                     │  ← Expanded section
│    Machine Family: [n2          ▼]          │  ← Dropdown (focused - blue border)
│    Select machine family                     │  ← Help text (gray, small)
│                                              │
│    Machine Type: [n2-standard-2 ▼]          │
│    2 vCPUs, 8 GB memory                      │
│                                              │
│  ▾ Labels                                    │
│    [+] Add label                             │  ← Action button
│    environment = production   [×]            │  ← Key-value with remove
│    team = backend            [×]            │
│                                              │
├──────────────────────────────────────────────┤
│  [Esc] Cancel   [Ctrl+S] Save changes       │  ← Action bar
└──────────────────────────────────────────────┘
```

### Color Scheme (GCP Colors)
- **Primary:** `#4285F4` (Google Blue) - focused fields, primary buttons
- **Success:** `#34A853` (Green) - added items in diff, success messages
- **Warning:** `#FBBC04` (Yellow) - warnings, optional fields
- **Danger:** `#EA4335` (Red) - removed items in diff, errors, destructive actions
- **Gray:** `#5F6368` - read-only fields, help text, borders

### Confirmation Pattern
Always show diff before applying changes:
1. User fills form
2. Presses Ctrl+S or clicks Save
3. System validates (show errors if any)
4. System generates diff
5. Shows DiffViewer with Yes/No
6. On Yes: apply changes, show spinner, show success
7. On No: return to form with edits preserved

### Error Display Pattern
- **Field-level errors:** Red border + error text below field (inline)
- **Form-level errors:** Error panel at top of form (global issues)
- **API errors:** Modal overlay with error message + retry/cancel options

---

## Files to Create

### New Directories
- `internal/ui/components/forms/`
- `internal/ui/editors/` (alternative to putting editors in views/)

### New Files

**Form Components:**
- `internal/ui/components/forms/field.go`
- `internal/ui/components/forms/field_test.go`
- `internal/ui/components/forms/section.go`
- `internal/ui/components/forms/section_test.go`
- `internal/ui/components/forms/form.go`
- `internal/ui/components/forms/form_test.go`
- `internal/ui/components/forms/diff.go`
- `internal/ui/components/forms/diff_test.go`
- `internal/ui/components/forms/wizard.go`
- `internal/ui/components/forms/wizard_test.go`
- `internal/ui/components/forms/validators.go` (common validation functions)
- `internal/ui/components/forms/validators_test.go`

**Resource Editors:**
- `internal/ui/views/instance_editor.go`
- `internal/ui/views/instance_editor_test.go`
- `internal/ui/views/disk_editor.go`
- `internal/ui/views/disk_editor_test.go`
- `internal/ui/views/snapshot_editor.go`
- `internal/ui/views/snapshot_editor_test.go`

**GCP API Extensions:**
- Extend `internal/gcp/compute.go` with new methods:
  - `SetMachineType()`
  - `ResizeDisk()`
  - `CreateDisk()`
  - `CreateSnapshot()`
  - `SetInstanceLabels()`
  - `ListMachineTypes()`
  - `ListOSImages()`

**Templates:**
- `internal/templates/instances.go` (instance templates data)
- `internal/templates/instances_test.go`

**Message Types:**
- Extend `internal/ui/messages.go` with editor messages

---

## Success Metrics

### User Experience
- **Time to resize disk:** < 30 seconds from disk list to confirmation
- **Time to clone instance:** < 60 seconds from instance list to creation
- **Error rate:** < 5% of edit operations fail due to validation issues
- **User satisfaction:** Positive feedback on editing UX

### Code Quality
- **Test coverage:** > 80% for form components
- **Performance:** Form renders in < 100ms
- **Maintainability:** New resource editor can be added in < 1 day

### Adoption
- **Usage:** 50%+ of gcon users try editing features within first month
- **Support:** < 10 bug reports per 100 edit operations
- **Documentation:** Comprehensive guide for all editing features

---

## Future Enhancements (Post-MVP)

### Advanced Editing
- **Bulk operations** - edit multiple resources at once (e.g., add label to 10 instances)
- **Templates marketplace** - share templates with community
- **Form plugins** - allow custom field types
- **Undo/redo** - revert recent changes

### Other Resource Types
- **Cloud SQL instances** - create, edit configurations
- **Cloud Storage buckets** - edit lifecycle policies, IAM
- **GKE clusters** - edit node pools, autoscaling
- **Cloud Run services** - edit revisions, traffic splits

### Advanced UX
- **Form themes** - let users customize colors
- **Voice control** - experimental voice commands for forms
- **AI suggestions** - recommend machine types based on usage patterns
- **Diff export** - save diff as text file for documentation

---

## Open Questions

1. **Template storage:** Should templates be stored locally (~/.gcon/templates.json) or support syncing across machines?
   - **Recommendation:** Local first, add sync later as enhancement

2. **Undo mechanism:** Should we track edit history and allow undo for recent changes?
   - **Recommendation:** Yes, but as Phase 11 feature (not MVP)

3. **Form auto-complete:** Should machine type field auto-complete as user types?
   - **Recommendation:** Yes for dropdowns with >20 options

4. **Multi-project editing:** Should we allow creating resources in different projects from current?
   - **Recommendation:** No for MVP, add project selector in future

5. **Offline mode:** Should forms work offline and queue changes?
   - **Recommendation:** No, require internet connection for GCP API calls

---

## References

- GCP Compute API Docs: https://cloud.google.com/compute/docs/reference/rest/v1
- Bubble Tea Examples: https://github.com/charmbracelet/bubbletea/tree/master/examples
- GCP Resource Limits: https://cloud.google.com/compute/quotas
- Existing metadata editor: `internal/ui/components/metadata_editor.go`

---

## Revision History

| Date       | Version | Changes                           |
|------------|---------|-----------------------------------|
| 2026-01-18 | 1.0     | Initial design document created   |
