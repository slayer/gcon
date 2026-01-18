# Resource Editing & Creation - Implementation Plan

**Date:** 2026-01-18
**Status:** Ready for Implementation
**Estimated Duration:** 9 weeks

---

## Overview

This plan breaks down the implementation of resource editing and creation features into 11 phases, each deliverable independently. Phases are ordered to deliver user value early while building foundational components.

---

## Phase 1: Core Form Framework
**Duration:** 1 week
**Branch:** `2026-01-18-form-framework`

### Objectives
- Build reusable form components that all resource editors will use
- Establish patterns for field types, validation, navigation
- Create testable, documented components

### Tasks

#### 1.1 Create Forms Package Structure
```bash
mkdir -p internal/ui/components/forms
```

**Files to create:**
- `internal/ui/components/forms/types.go` - shared types and constants
- `internal/ui/components/forms/field.go` - FormField component
- `internal/ui/components/forms/section.go` - FormSection component
- `internal/ui/components/forms/form.go` - FormView component
- `internal/ui/components/forms/diff.go` - DiffViewer component
- `internal/ui/components/forms/validators.go` - common validators

#### 1.2 Implement FormField Component
**File:** `internal/ui/components/forms/field.go`

Key features:
- Support 6 field types: Text, Number, Dropdown, MultiSelect, Toggle, ReadOnly
- Focus management with visual indicators
- Real-time validation with error display
- Keyboard navigation (Tab, Enter, Arrows)
- Help text rendering

**Key methods:**
```go
func NewFormField(id, label string, fieldType FieldType) *FormField
func (f *FormField) Init() tea.Cmd
func (f *FormField) Update(msg tea.Msg) tea.Cmd
func (f *FormField) View() string
func (f *FormField) SetFocus(focused bool)
func (f *FormField) Validate() error
func (f *FormField) GetValue() interface{}
func (f *FormField) SetValue(v interface{})
```

**Tests:** `field_test.go`
- Test each field type renders correctly
- Test validation triggers on value change
- Test focus behavior
- Test keyboard navigation

#### 1.3 Implement FormSection Component
**File:** `internal/ui/components/forms/section.go`

Key features:
- Group related fields
- Optional collapsible behavior
- Section header with icon
- Manage focus between fields

**Key methods:**
```go
func NewFormSection(id, title string) *FormSection
func (s *FormSection) AddField(field *FormField)
func (s *FormSection) Update(msg tea.Msg) tea.Cmd
func (s *FormSection) View() string
func (s *FormSection) NextField() bool
func (s *FormSection) PrevField() bool
func (s *FormSection) ToggleCollapse()
```

**Tests:** `section_test.go`
- Test field navigation within section
- Test collapse/expand behavior
- Test rendering with multiple fields

#### 1.4 Implement FormView Component
**File:** `internal/ui/components/forms/form.go`

Key features:
- Container for multiple sections
- Global navigation (Tab/Shift+Tab across sections)
- Form-level validation
- Submit/Cancel actions
- Keyboard shortcuts display

**Key methods:**
```go
func NewFormView(title string, mode FormMode) *FormView
func (f *FormView) AddSection(section *FormSection)
func (f *FormView) Init() tea.Cmd
func (f *FormView) Update(msg tea.Msg) tea.Cmd
func (f *FormView) View() string
func (f *FormView) Validate() []string
func (f *FormView) GetFormData() map[string]interface{}
func (f *FormView) SetFormData(data map[string]interface{})
func (f *FormView) SetSize(width, height int)
```

**Key bindings:**
- `Tab` / `Shift+Tab` - navigate fields
- `Enter` - activate field or submit
- `Esc` - cancel
- `Ctrl+S` - submit/save
- `?` - toggle help

**Tests:** `form_test.go`
- Test navigation across sections
- Test validation flow
- Test data collection
- Test keyboard shortcuts

#### 1.5 Implement DiffViewer Component
**File:** `internal/ui/components/forms/diff.go`

Key features:
- Show before/after comparison
- Highlight changed fields (red=removed, green=added)
- Display warnings (cost impact, restart required)
- Yes/No confirmation

**Key methods:**
```go
func NewDiffViewer(title string) *DiffViewer
func (d *DiffViewer) AddField(label, oldValue, newValue string)
func (d *DiffViewer) AddWarning(warning string)
func (d *DiffViewer) SetCostImpact(impact string)
func (d *DiffViewer) Update(msg tea.Msg) tea.Cmd
func (d *DiffViewer) View() string
```

**Tests:** `diff_test.go`
- Test rendering of unchanged vs changed fields
- Test warning display
- Test Yes/No selection

#### 1.6 Implement Common Validators
**File:** `internal/ui/components/forms/validators.go`

Common validation functions:
```go
func ValidateRequired(v interface{}) error
func ValidateGCPResourceName(name string) error
func ValidateNumber(min, max int) func(interface{}) error
func ValidateStringLength(min, max int) func(interface{}) error
func ValidatePattern(pattern string) func(interface{}) error
```

**Tests:** `validators_test.go`

#### 1.7 Create Form Demo View
**File:** `internal/ui/views/form_demo.go`

Purpose: Test forms in isolation without GCP integration

Features:
- Sample form with all field types
- Test validation
- Test diff viewer
- Accessible via command palette (":form-demo")

### Acceptance Criteria
- [ ] All form components implemented with full test coverage
- [ ] Forms are keyboard-accessible
- [ ] Validation works in real-time
- [ ] Demo view demonstrates all capabilities
- [ ] Code passes linter
- [ ] Documentation added to components

### Testing
```bash
make test
make lint
make run  # Test form-demo via command palette
```

---

## Phase 2: Disk Editing - Resize
**Duration:** 1 week
**Branch:** `2026-01-18-disk-resize`

### Objectives
- Implement simplest edit scenario
- Validate form framework with real GCP integration
- Deliver immediate user value

### Tasks

#### 2.1 Add GCP API Method
**File:** `internal/gcp/compute.go`

Add method:
```go
func (c *ComputeClient) ResizeDisk(ctx context.Context, projectID, zone, diskName string, newSizeGB int64) error
```

Implementation:
- Call `disks.resize` API endpoint
- Handle async operation (wait for completion)
- Return error with helpful message

**Tests:** `compute_test.go`
- Mock GCP API response
- Test error handling

#### 2.2 Implement DiskEditor - Resize Form
**File:** `internal/ui/views/disk_editor.go`

Create `DiskEditor` struct:
```go
type DiskEditor struct {
    computeClient *gcp.ComputeClient
    disk          *gcp.Disk
    mode          EditorMode
    form          *forms.FormView
    diff          *forms.DiffViewer
    state         editorState  // form, diff, saving, done

    width, height int
}
```

Implement resize form:
```go
func (e *DiskEditor) buildResizeForm() *forms.FormView {
    form := forms.NewFormView("Resize Disk", forms.FormModeEdit)

    // Warning section
    warningSection := forms.NewFormSection("warning", "⚠ Important")
    warningSection.AddField(forms.NewReadOnlyField("warn", "",
        "Disk size can only be increased. File system expansion may be needed."))
    form.AddSection(warningSection)

    // Size section
    sizeSection := forms.NewFormSection("size", "Size Configuration")
    sizeSection.AddField(forms.NewReadOnlyField("current", "Current Size",
        fmt.Sprintf("%d GB", e.disk.SizeGB)))

    newSizeField := forms.NewNumberField("new_size", "New Size (GB)")
    newSizeField.SetValue(e.disk.SizeGB)
    newSizeField.SetValidator(func(v interface{}) error {
        size := v.(int64)
        if size <= e.disk.SizeGB {
            return fmt.Errorf("must be greater than %d GB", e.disk.SizeGB)
        }
        if size > 65536 {
            return fmt.Errorf("maximum is 65536 GB")
        }
        return nil
    })
    newSizeField.SetHelpText(fmt.Sprintf("Min: %d GB, Max: 65536 GB", e.disk.SizeGB+1))
    sizeSection.AddField(newSizeField)

    form.AddSection(sizeSection)
    return form
}
```

Implement Update logic:
```go
func (e *DiskEditor) Update(msg tea.Msg) tea.Cmd {
    switch e.state {
    case stateForm:
        // Handle form updates
        // On Ctrl+S: validate, build diff, switch to stateDiff
    case stateDiff:
        // Handle diff viewer
        // On Yes: switch to stateSaving, call API
        // On No: return to stateForm
    case stateSaving:
        // Show spinner, wait for API response
        // On success: emit EditCompleteMsg
        // On error: show error, return to stateForm
    }
}
```

**Tests:** `disk_editor_test.go`
- Test form validation
- Test diff generation
- Test state transitions

#### 2.3 Integrate with Disks View
**File:** `internal/ui/views/disks.go`

Add action menu item:
```go
func (v *DisksView) buildActionMenu() []string {
    return []string{
        "View Details",
        "Resize Disk",  // NEW
        "Create Snapshot",
        "Delete",
    }
}
```

Handle menu selection:
```go
case "Resize Disk":
    return resizeRequestMsg{disk: v.getSelectedDisk()}
```

Add key binding (in Update):
```go
case "r":  // Resize
    if disk := v.getSelectedDisk(); disk != nil {
        return resizeRequestMsg{disk: disk}
    }
```

#### 2.4 Add Messages
**File:** `internal/ui/messages.go`

```go
type resizeRequestMsg struct {
    disk *gcp.Disk
}
```

#### 2.5 Wire Up in App
**File:** `internal/ui/app.go`

Add to App struct:
```go
type App struct {
    // ... existing fields ...
    diskEditor *views.DiskEditor
}
```

Handle messages in Update:
```go
case resizeRequestMsg:
    editor := views.NewDiskEditor(a.computeClient, msg.disk, views.EditorModeEdit)
    a.diskEditor = editor
    a.currentView = ViewDiskEditor
    return a, editor.Init()

case EditCompleteMsg:
    a.footer.SetMessage("✓ Disk resized successfully")
    a.currentView = ViewDisks
    return a, a.disksView.Refresh()
```

#### 2.6 Update Documentation
**Files:**
- `CLAUDE.md` - add disk resize to implemented features
- `README.md` - document 'r' key binding in disks view
- `doc/2026-01-18-resource-editing/TODO.md` - track progress

### Acceptance Criteria
- [ ] User can press 'r' on disk to resize it
- [ ] Form validates size constraints
- [ ] Diff shows before/after size
- [ ] API call succeeds and disk is resized in GCP
- [ ] Success message shown, list refreshes
- [ ] Error handling works (network, permissions, validation)
- [ ] Tests pass
- [ ] Documentation updated

### Testing
1. Unit tests: `go test ./internal/ui/views/disk_editor_test.go`
2. Integration test: Run gcon, select disk, press 'r', resize, confirm
3. Verify in GCP console that disk was resized

---

## Phase 3: Snapshot Creation
**Duration:** 1 week
**Branch:** `2026-01-18-snapshot-create`

### Objectives
- Implement simple creation flow
- Enable users to create snapshots from disks
- Validate creation workflow

### Tasks

#### 3.1 Add GCP API Method
**File:** `internal/gcp/compute.go`

```go
func (c *ComputeClient) CreateSnapshot(ctx context.Context, projectID, zone, diskName string, snapshot *Snapshot) (*Snapshot, error)
```

Implementation:
- Call `disks.createSnapshot` API
- Wait for operation completion
- Return created snapshot details

**Tests:** `compute_test.go`

#### 3.2 Implement SnapshotEditor
**File:** `internal/ui/views/snapshot_editor.go`

Create form:
```go
func (e *SnapshotEditor) buildCreateForm() *forms.FormView {
    form := forms.NewFormView("Create Snapshot", forms.FormModeCreate)

    // Source section
    sourceSection := forms.NewFormSection("source", "Source")
    sourceSection.AddField(forms.NewReadOnlyField("disk", "Source Disk", e.sourceDisk.Name))
    sourceSection.AddField(forms.NewReadOnlyField("size", "Disk Size", fmt.Sprintf("%d GB", e.sourceDisk.SizeGB)))
    form.AddSection(sourceSection)

    // Config section
    configSection := forms.NewFormSection("config", "Snapshot Configuration")

    nameField := forms.NewTextField("name", "Snapshot Name")
    nameField.SetValidator(validators.ValidateGCPResourceName)
    nameField.SetRequired(true)
    nameField.SetHelpText("Lowercase letters, numbers, hyphens")
    configSection.AddField(nameField)

    locationField := forms.NewDropdownField("location", "Storage Location")
    locationField.SetOptions([]string{
        "us (multi-region)",
        "eu (multi-region)",
        "us-central1",
        "europe-west1",
    })
    locationField.SetValue("us (multi-region)")
    locationField.SetHelpText("Multi-regional is more durable")
    configSection.AddField(locationField)

    labelsField := forms.NewMultiSelectField("labels", "Labels")
    labelsField.SetHelpText("key=value pairs for organization")
    configSection.AddField(labelsField)

    form.AddSection(configSection)
    return form
}
```

**Tests:** `snapshot_editor_test.go`

#### 3.3 Integrate with Disks View
**File:** `internal/ui/views/disks.go`

Add action menu item:
```go
"Create Snapshot"  // NEW
```

Add key binding:
```go
case "s":  // Create snapshot
    return createSnapshotRequestMsg{disk: v.getSelectedDisk()}
```

#### 3.4 Integrate with Disk Details View
**File:** `internal/ui/views/disk_details.go`

Add button/action to create snapshot from details view.

#### 3.5 Wire Up in App
Handle `createSnapshotRequestMsg` in App.Update.

### Acceptance Criteria
- [ ] User can create snapshot from disks list or disk details
- [ ] Form validates snapshot name
- [ ] Snapshot created successfully in GCP
- [ ] New snapshot appears in snapshots list
- [ ] Tests pass
- [ ] Documentation updated

### Testing
1. Unit tests
2. Create snapshot via gcon, verify in GCP console
3. Test error cases (invalid name, permissions)

---

## Phase 4: Disk Creation
**Duration:** 1 week
**Branch:** `2026-01-18-disk-create`

### Objectives
- Allow creating disks from snapshots, images, or blank
- Complete disk lifecycle (create, resize, snapshot, delete)

### Tasks

#### 4.1 Add GCP API Methods
**File:** `internal/gcp/compute.go`

```go
func (c *ComputeClient) CreateDisk(ctx context.Context, projectID, zone string, disk *DiskConfig) (*Disk, error)
func (c *ComputeClient) ListOSImages(ctx context.Context) ([]*Image, error)
```

#### 4.2 Implement Disk Creation Forms
**File:** `internal/ui/views/disk_editor.go`

Three creation modes:
1. **From Snapshot:** disk name, zone, size (≥ snapshot size), type
2. **From Image:** image selector, disk name, zone, size (≥ image size), type
3. **Empty Disk:** disk name, zone, size, type

#### 4.3 Add Image Selector Component
**File:** `internal/ui/components/image_selector.go`

Searchable list of OS images (Debian, Ubuntu, CentOS, Windows, etc.)

#### 4.4 Integrate with Views
Add "Create Disk" action to:
- Disks list view (key 'n')
- Snapshots list (create disk from this snapshot)
- Images list (create disk from this image)

#### 4.5 Wire Up in App

### Acceptance Criteria
- [ ] User can create disk from snapshot
- [ ] User can create disk from OS image
- [ ] User can create blank disk
- [ ] All three paths work end-to-end
- [ ] Tests pass
- [ ] Documentation updated

---

## Phase 5: Instance Labels Editing
**Duration:** 1 week
**Branch:** `2026-01-18-instance-labels`

### Objectives
- Enable safe instance editing (labels don't require restart)
- Introduce InstanceEditor component

### Tasks

#### 5.1 Add GCP API Method
**File:** `internal/gcp/compute.go`

```go
func (c *ComputeClient) SetInstanceLabels(ctx context.Context, projectID, zone, instanceName string, labels map[string]string, fingerprint string) error
```

#### 5.2 Implement InstanceEditor - Labels
**File:** `internal/ui/views/instance_editor.go`

Create `InstanceEditor` struct (similar to DiskEditor).

Implement labels form with MultiSelect field:
```go
func (e *InstanceEditor) buildLabelsForm() *forms.FormView {
    form := forms.NewFormView("Edit Instance Labels", forms.FormModeEdit)

    labelsSection := forms.NewFormSection("labels", "Labels")

    labelsField := forms.NewMultiSelectField("labels", "Labels")
    labelsField.SetValue(e.instance.Labels)  // map[string]string
    labelsField.SetHelpText("key=value pairs. Press '+' to add, 'x' to remove")
    labelsSection.AddField(labelsField)

    form.AddSection(labelsSection)
    return form
}
```

#### 5.3 Integrate with Instance Details View
**File:** `internal/ui/views/instance_details.go`

Add action to tabs or action menu: "Edit Labels"

#### 5.4 Wire Up in App

### Acceptance Criteria
- [ ] User can edit instance labels
- [ ] Diff shows added/removed labels
- [ ] Labels updated without instance restart
- [ ] Tests pass
- [ ] Documentation updated

---

## Phase 6: Instance Machine Type Editing
**Duration:** 1.5 weeks
**Branch:** `2026-01-18-instance-machine-type`

### Objectives
- Enable machine type changes (requires restart)
- Handle multi-step operations (stop, change, restart)
- Show progress during long operations

### Tasks

#### 6.1 Add GCP API Method
**File:** `internal/gcp/compute.go`

```go
func (c *ComputeClient) SetMachineType(ctx context.Context, projectID, zone, instanceName, machineType string) error
func (c *ComputeClient) ListMachineTypes(ctx context.Context, zone string) ([]*MachineType, error)
```

#### 6.2 Implement Machine Type Form
**File:** `internal/ui/views/instance_editor.go`

Two-level selection:
1. Machine family (e2, n2, n2d, c3, m3)
2. Machine type within family (standard-2, standard-4, etc.)

Show vCPU/memory specs for selected type.

#### 6.3 Implement Multi-Step Operation
Handle workflow:
1. User confirms change in diff viewer
2. Stop instance (if running) - show spinner "Stopping instance..."
3. Wait for stop completion
4. Change machine type - show spinner "Changing machine type..."
5. Restart instance (if was running) - show spinner "Starting instance..."
6. Show success

#### 6.4 Add Restart Warning
In diff viewer, show:
```
⚠ Instance must be stopped to change machine type
⚠ Estimated downtime: 1-2 minutes
```

#### 6.5 Integrate with Instance Views
Add "Change Machine Type" to action menu.

### Acceptance Criteria
- [ ] User can change instance machine type
- [ ] Clear warnings shown about restart
- [ ] Progress shown during multi-step operation
- [ ] Instance successfully restarted after change
- [ ] Tests pass (including error mid-operation)
- [ ] Documentation updated

---

## Phase 7: CreateWizard Component
**Duration:** 1 week
**Branch:** `2026-01-18-wizard-component`

### Objectives
- Build wizard framework for multi-step flows
- Prepare for instance creation wizard

### Tasks

#### 7.1 Implement CreateWizard
**File:** `internal/ui/components/forms/wizard.go`

Features:
- Step indicator (1/5, 2/5, etc.)
- Next/Back navigation
- Per-step validation
- Review step at end
- Data accumulation across steps

Key methods:
```go
func NewCreateWizard(title string) *CreateWizard
func (w *CreateWizard) AddStep(step WizardStep)
func (w *CreateWizard) NextStep() error
func (w *CreateWizard) PrevStep()
func (w *CreateWizard) GetData() map[string]interface{}
func (w *CreateWizard) Update(msg tea.Msg) tea.Cmd
func (w *CreateWizard) View() string
```

#### 7.2 Create Tests
**File:** `internal/ui/components/forms/wizard_test.go`

Test navigation, validation, data collection.

#### 7.3 Create Wizard Demo
Add wizard demo to form demo view.

### Acceptance Criteria
- [ ] Wizard navigates between steps
- [ ] Validation prevents proceeding with invalid data
- [ ] Data accumulated correctly
- [ ] Review step shows all data
- [ ] Tests pass
- [ ] Documentation added

---

## Phase 8: Instance Creation - Clone
**Duration:** 1 week
**Branch:** `2026-01-18-instance-clone`

### Objectives
- Easiest instance creation path
- Copy config from existing instance
- Deliver high-value user feature

### Tasks

#### 8.1 Add GCP API Method
**File:** `internal/gcp/compute.go`

```go
func (c *ComputeClient) CreateInstance(ctx context.Context, projectID, zone string, instance *InstanceConfig) (*Instance, error)
```

#### 8.2 Implement Clone Form
**File:** `internal/ui/views/instance_editor.go`

```go
func (e *InstanceEditor) buildCloneForm() *forms.FormView {
    form := forms.NewFormView("Clone Instance", forms.FormModeClone)

    // Source section (read-only)
    sourceSection := forms.NewFormSection("source", "Source Instance")
    sourceSection.AddField(forms.NewReadOnlyField("source", "Cloning from", e.sourceInstance.Name))
    sourceSection.AddField(forms.NewReadOnlyField("type", "Machine Type", e.sourceInstance.MachineType))
    form.AddSection(sourceSection)

    // New instance section
    newSection := forms.NewFormSection("new", "New Instance")

    nameField := forms.NewTextField("name", "Instance Name")
    nameField.SetValue(e.generateCloneName())  // e.g., "my-instance-copy"
    nameField.SetValidator(validators.ValidateGCPResourceName)
    nameField.SetRequired(true)
    newSection.AddField(nameField)

    zoneField := forms.NewDropdownField("zone", "Zone")
    zoneField.SetOptions(e.getAvailableZones())
    zoneField.SetValue(e.sourceInstance.Zone)
    newSection.AddField(zoneField)

    // Optional modifications section
    modSection := forms.NewFormSection("mods", "Optional Modifications")
    modSection.SetCollapsible(true)

    machineTypeField := forms.NewDropdownField("machine_type", "Machine Type")
    machineTypeField.SetOptions(e.getMachineTypes())
    machineTypeField.SetValue(e.sourceInstance.MachineType)
    machineTypeField.SetHelpText("Leave unchanged to clone exact machine type")
    modSection.AddField(machineTypeField)

    form.AddSection(newSection)
    form.AddSection(modSection)
    return form
}
```

#### 8.3 Implement Clone Name Generation
```go
func (e *InstanceEditor) generateCloneName() string {
    base := e.sourceInstance.Name
    // Check if "base-copy" exists, if so try "base-copy-2", etc.
    suffix := 1
    name := base + "-copy"
    for e.instanceExists(name) {
        suffix++
        name = fmt.Sprintf("%s-copy-%d", base, suffix)
    }
    return name
}
```

#### 8.4 Integrate with Instances View
Add "Clone Instance" action (key 'c').

#### 8.5 Build Instance Config from Clone
Copy all settings:
- Machine type
- Boot disk (create from same image)
- Additional disks (optionally clone)
- Network settings
- Metadata
- Labels
- Service account

### Acceptance Criteria
- [ ] User can clone instance with 'c' key
- [ ] Auto-generated name is unique
- [ ] User can modify machine type before cloning
- [ ] Cloned instance created successfully
- [ ] All settings copied correctly
- [ ] Tests pass
- [ ] Documentation updated

---

## Phase 9: Instance Creation - Templates
**Duration:** 1 week
**Branch:** `2026-01-18-instance-templates`

### Objectives
- Provide predefined instance configurations
- Speed up common use cases

### Tasks

#### 9.1 Define Template Structure
**File:** `internal/templates/instances.go`

```go
type InstanceTemplate struct {
    Name        string
    Description string
    MachineType string
    DiskSizeGB  int64
    DiskType    string
    Image       string
    NetworkTags []string
    Labels      map[string]string
    Metadata    map[string]string
}

var PredefinedTemplates = []InstanceTemplate{
    {
        Name:        "Web Server",
        Description: "Nginx/Apache web server - e2-medium, 20GB SSD",
        MachineType: "e2-medium",
        DiskSizeGB:  20,
        DiskType:    "pd-ssd",
        Image:       "debian-11",
        NetworkTags: []string{"http-server", "https-server"},
        Labels: map[string]string{
            "purpose": "webserver",
        },
    },
    {
        Name:        "Database",
        Description: "MySQL/Postgres server - n2-standard-4, 100GB SSD, no external IP",
        MachineType: "n2-standard-4",
        DiskSizeGB:  100,
        DiskType:    "pd-ssd",
        Image:       "debian-11",
        NetworkTags: []string{},
        Labels: map[string]string{
            "purpose": "database",
        },
        Metadata: map[string]string{
            "no-external-ip": "true",
        },
    },
    {
        Name:        "Dev Machine",
        Description: "Development environment - e2-small, 10GB, preemptible",
        MachineType: "e2-small",
        DiskSizeGB:  10,
        DiskType:    "pd-standard",
        Image:       "debian-11",
        NetworkTags: []string{},
        Labels: map[string]string{
            "purpose": "development",
            "preemptible": "true",
        },
    },
}
```

#### 9.2 Implement Template Selector
**File:** `internal/ui/views/instance_editor.go`

Create template selection screen:
- List templates with descriptions
- Select with Enter
- Show template details on focus

After selection, show form to customize:
- Instance name (required)
- Zone (required)
- Optionally modify any template field

#### 9.3 Integrate with Instances View
Add "Create from Template" option when user presses 'n'.

Show choice modal:
```
Create Instance:
  1. Clone existing instance
  2. From template
  3. From scratch
  [Esc] Cancel
```

#### 9.4 Add Template Tests
**File:** `internal/templates/instances_test.go`

Test template validity (all required fields present).

### Acceptance Criteria
- [ ] User can create instance from predefined templates
- [ ] 3+ useful templates available
- [ ] Templates can be customized before creation
- [ ] Tests pass
- [ ] Documentation updated

---

## Phase 10: Instance Creation - Full Wizard
**Duration:** 2 weeks
**Branch:** `2026-01-18-instance-wizard`

### Objectives
- Complete control for advanced users
- Full instance creation from scratch
- Showcase wizard component

### Tasks

#### 10.1 Implement 5-Step Wizard

**Step 1: Basic Info**
- Instance name (required, validated)
- Zone selection (dropdown)
- Description (optional)

**Step 2: Machine Type**
- Machine family selection
- Machine type within family
- Show vCPU/memory specs
- Show pricing estimate if available

**Step 3: Boot Disk**
- Image selector (OS images + custom images)
- Search/filter images
- Disk size (≥ image size)
- Disk type (SSD vs Standard)

**Step 4: Network**
- VPC selection (default or custom)
- Subnet selection
- External IP (ephemeral, static, none)
- Firewall tags (multiselect)

**Step 5: Review**
- Show all selected config
- Estimated monthly cost
- Confirm and create

#### 10.2 Implement Image Browser
**File:** `internal/ui/components/image_browser.go`

Features:
- List images grouped by OS family (Debian, Ubuntu, CentOS, etc.)
- Search by name
- Show image details (size, family, creation date)
- Select with Enter

#### 10.3 Wire Up Wizard
In `instance_editor.go`, implement wizard flow:

```go
func (e *InstanceEditor) buildCreateWizard() *forms.CreateWizard {
    wizard := forms.NewCreateWizard("Create Instance")

    wizard.AddStep(forms.WizardStep{
        Title:   "Basic Information",
        Content: e.buildBasicInfoForm(),
    })

    wizard.AddStep(forms.WizardStep{
        Title:   "Machine Type",
        Content: e.buildMachineTypeForm(),
    })

    wizard.AddStep(forms.WizardStep{
        Title:   "Boot Disk",
        Content: e.buildBootDiskForm(),
    })

    wizard.AddStep(forms.WizardStep{
        Title:   "Network",
        Content: e.buildNetworkForm(),
    })

    wizard.AddStep(forms.WizardStep{
        Title:   "Review",
        Content: e.buildReviewForm(),
    })

    return wizard
}
```

#### 10.4 Integrate with Instances View
When user presses 'n', show modal:
- Clone existing
- From template
- **From scratch (new)**

### Acceptance Criteria
- [ ] User can create instance from scratch
- [ ] All 5 steps work correctly
- [ ] Navigation (Next/Back) works
- [ ] Validation prevents proceeding with bad data
- [ ] Review step shows complete config
- [ ] Instance created successfully
- [ ] Tests pass
- [ ] Documentation updated

---

## Phase 11: Polish & Advanced Features
**Duration:** 1 week
**Branch:** `2026-01-18-editing-polish`

### Objectives
- Production-ready editing experience
- Nice-to-have enhancements
- Comprehensive documentation

### Tasks

#### 11.1 Cost Estimation
Add cost estimates to diff viewer:
- Query GCP pricing API (or use hardcoded estimates)
- Show hourly and monthly cost impact
- Display in diff: "Estimated cost: +$0.08/hr (+$58/month)"

#### 11.2 Template Saving
Allow saving custom templates locally:
- Save template to `~/.gcon/templates.json`
- Load user templates at startup
- Add "Save as Template" option after configuring instance

#### 11.3 Form Field Search
For long dropdown lists (100+ options):
- Add search/filter capability
- Type to filter options
- Highlight matching text

#### 11.4 Keyboard Shortcuts Help
Add help overlay (press '?'):
- Show all form shortcuts
- Context-sensitive help

#### 11.5 Auto-Save
Implement form state persistence:
- Save form state to temp file periodically
- Recover on crash/restart
- Prompt user: "Recover unsaved changes?"

#### 11.6 Performance Optimization
- Lazy-load dropdown options
- Debounce validation (300ms after typing stops)
- Optimize re-renders (only changed components)

#### 11.7 Comprehensive Documentation
Update all docs:
- `CLAUDE.md` - add editing features to implemented list
- `README.md` - document all key bindings and features
- Create `doc/2026-01-18-resource-editing/user-guide.md`
- Add screenshots/GIFs to documentation

### Acceptance Criteria
- [ ] Cost estimates shown where applicable
- [ ] User templates can be saved and loaded
- [ ] Long dropdowns are searchable
- [ ] Help overlay is comprehensive
- [ ] Auto-save prevents data loss
- [ ] Forms feel snappy (< 100ms render)
- [ ] Documentation is complete
- [ ] All tests pass
- [ ] Code is linted and formatted

---

## Testing Strategy

### Unit Tests
- Test all form components in isolation
- Test validation logic
- Test state management
- Test data serialization/parsing
- Target: 80%+ code coverage

### Integration Tests
- Test complete workflows (resize disk, create instance, etc.)
- Test error handling (network failures, permission errors)
- Test edge cases (max values, special characters, etc.)

### Manual Testing Checklist
For each feature:
- [ ] Happy path works end-to-end
- [ ] Validation catches bad inputs
- [ ] Cancel/Esc works at each step
- [ ] Error messages are helpful
- [ ] UI is responsive (keyboard navigation works)
- [ ] Changes persist in GCP
- [ ] List views refresh correctly

### Performance Testing
- Test form rendering with large datasets (1000+ dropdown options)
- Test wizard with many steps (10+)
- Test on slow connections (simulate with network throttling)

---

## Rollout Plan

### Week 1-2: Foundation
- Phase 1: Form framework
- Phase 2: Disk resize
- **Milestone:** Users can resize disks

### Week 3-4: Basic Creation
- Phase 3: Snapshot creation
- Phase 4: Disk creation
- **Milestone:** Users can create disks and snapshots

### Week 5-6: Instance Editing
- Phase 5: Instance labels
- Phase 6: Instance machine type
- **Milestone:** Users can edit instance properties

### Week 7-8: Instance Creation
- Phase 7: Wizard component
- Phase 8: Clone instances
- Phase 9: Instance templates
- **Milestone:** Users can clone and create from templates

### Week 9: Advanced & Polish
- Phase 10: Full instance creation wizard
- Phase 11: Polish and documentation
- **Milestone:** Full editing capabilities released

---

## Risk Mitigation

### Technical Risks

**Risk:** Form framework too complex, slows development
- **Mitigation:** Start with minimal components, add features incrementally
- **Fallback:** Simplify to text-based editors if forms prove too complex

**Risk:** GCP API rate limits hit during testing
- **Mitigation:** Implement caching, use test project with low limits
- **Fallback:** Add configurable delays between API calls

**Risk:** Bubble Tea state management becomes unwieldy
- **Mitigation:** Keep editor state isolated, use clear message types
- **Fallback:** Refactor to simpler state machine if needed

### UX Risks

**Risk:** Forms feel clunky compared to web UI
- **Mitigation:** Focus on keyboard shortcuts, minimize typing
- **Fallback:** Add text-based alternative (YAML/JSON edit)

**Risk:** Users make costly mistakes (create expensive instances)
- **Mitigation:** Show cost estimates, require confirmation for high-cost ops
- **Fallback:** Add "dry-run" mode

### Project Risks

**Risk:** Scope creep, timeline extends beyond 9 weeks
- **Mitigation:** Prioritize ruthlessly, ship MVPs early
- **Fallback:** Cut Phase 10 (full wizard) and Phase 11 (polish) if needed

**Risk:** User adoption is low
- **Mitigation:** Announce features, create tutorial videos
- **Fallback:** Gather feedback, iterate on UX

---

## Success Metrics

### Quantitative
- **Feature adoption:** 50%+ of gcon users try editing features in first month
- **Error rate:** < 5% of operations fail due to bugs
- **Performance:** Forms render in < 100ms
- **Test coverage:** > 80% for form components

### Qualitative
- **User feedback:** Positive reviews on GitHub
- **Support burden:** < 10 bug reports per 100 operations
- **Developer experience:** New resource editor can be added in < 1 day

---

## Post-Implementation

### Monitoring
- Track feature usage via telemetry (opt-in)
- Monitor error rates in logs
- Gather user feedback via GitHub issues

### Iteration
- Address bugs within 1 week
- Add requested features in follow-up releases
- Improve documentation based on support questions

### Future Work
See design.md "Future Enhancements" section for ideas:
- Bulk operations
- Template marketplace
- Form plugins
- Undo/redo
- Additional resource types (Cloud SQL, GKE, Cloud Run)

---

## Resources

### Documentation
- GCP Compute API: https://cloud.google.com/compute/docs/reference/rest/v1
- Bubble Tea Guide: https://github.com/charmbracelet/bubbletea/tree/master/tutorials
- Lip Gloss Styling: https://github.com/charmbracelet/lipgloss

### Examples
- Existing metadata editor: `internal/ui/components/metadata_editor.go`
- Form inspiration: gh-dash, lazygit, k9s

### Tools
- GCP Console: Validate changes made via gcon
- gcloud CLI: Test API calls independently
- Postman: Test GCP API endpoints directly

---

## Appendix

### Key Bindings Summary

| Key | Context | Action |
|-----|---------|--------|
| `e` | Resource list | Edit selected resource |
| `c` | Instance list | Clone selected instance |
| `n` | Resource list | Create new resource |
| `m` | Any list | Open action menu (includes edit/create) |
| `r` | Disk list | Resize selected disk |
| `s` | Disk list | Create snapshot from disk |
| `Tab` / `Shift+Tab` | Form | Navigate between fields |
| `Enter` | Form field | Activate dropdown/toggle |
| `Ctrl+S` | Form | Submit/save |
| `Esc` | Form | Cancel |
| `?` | Form | Show help |
| `↑/↓` | Dropdown | Navigate options |
| `Space` | Toggle | Toggle on/off |

### Message Flow Diagram

```
┌─────────────┐
│  List View  │
└──────┬──────┘
       │ User presses 'e', 'c', 'n', or selects from menu
       │
       ↓
┌──────────────────┐
│ EditRequestMsg   │ {resourceType, resourceID, mode, initialData}
└──────┬───────────┘
       │
       ↓
┌──────────────┐
│ App.Update   │ Creates editor, switches view
└──────┬───────┘
       │
       ↓
┌──────────────┐
│ Editor View  │ User edits fields
└──────┬───────┘
       │ User presses Ctrl+S (save)
       │
       ↓
┌──────────────┐
│ Form.Validate│ Check all fields
└──────┬───────┘
       │ Valid
       │
       ↓
┌──────────────┐
│ DiffViewer   │ Show before/after
└──────┬───────┘
       │ User confirms (Yes)
       │
       ↓
┌──────────────┐
│ Editor.save  │ Call GCP API
└──────┬───────┘
       │ Success
       │
       ↓
┌──────────────────┐
│ EditCompleteMsg  │ {resourceType, resourceID, action}
└──────┬───────────┘
       │
       ↓
┌──────────────┐
│ App.Update   │ Show success, refresh list, return to list view
└──────┬───────┘
       │
       ↓
┌──────────────┐
│  List View   │ Shows updated resource
└──────────────┘
```

---

## Revision History

| Date       | Version | Changes                           |
|------------|---------|-----------------------------------|
| 2026-01-18 | 1.0     | Initial implementation plan       |
