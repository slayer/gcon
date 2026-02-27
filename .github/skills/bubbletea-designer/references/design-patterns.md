# Bubble Tea Design Patterns (gcon-adapted)

Design patterns used in gcon, mapped from generic Bubble Tea patterns to project-specific abstractions.

> **Authoritative source**: `.claude/rules/adding-new-views.md` and `.claude/rules/component-patterns.md` are the canonical references for adding views. This document supplements with architectural context.

## Pattern 1: View Interface (gcon's tea.Model Alternative)

**When**: Every view in gcon
**Why**: Simplified interface — parent (App) controls model lifecycle, views just return commands

```go
// gcon uses a simplified View interface instead of full tea.Model
type View interface {
    Init() tea.Cmd
    Update(msg tea.Msg) tea.Cmd   // No (tea.Model, tea.Cmd) — parent handles model
    View() string
    SetContext(ctx *context.ProgramContext)
}
```

**Key difference from standard Bubble Tea**: Views don't return themselves from `Update()`. The App struct owns all view instances and handles navigation/switching. This enables centralized routing without views needing to know about each other.

## Pattern 2: App as Central Router

**When**: All cross-view communication
**Why**: Single place for navigation, message dispatch, and view lifecycle

```go
type App struct {
    currentView ViewType
    viewStack   []ViewType   // For back navigation

    // One field per view type
    instancesView       *views.InstancesView
    instanceDetailsView *views.InstanceDetailsView
    snapshotCreateView  *views.SnapshotCreateView
    // ...
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    // Cross-view messages handled at app level
    case views.InstanceSelectedMsg:
        return a, a.handleInstanceSelected(msg)
    case views.SnapshotCreateRequestMsg:
        return a, a.handleSnapshotCreateRequest(msg)
    case views.DiskActionResultMsg:
        return a, a.handleDiskActionResult(msg)
    }
    // Delegate to current view
    if view := a.getCurrentViewModel(); view != nil {
        return a, view.Update(msg)
    }
    return a, nil
}
```

## Pattern 3: List View with TableClickDelegate

**When**: Any view displaying a list of GCP resources
**Why**: Eliminates mouse click handling boilerplate across 15+ list views

```go
type InstancesView struct {
    TableClickDelegate          // Embed for mouse click delegation
    computeClient *gcp.ComputeClient
    projectID     string
    ctx           *context.ProgramContext
    table         table.Model   // gcon's custom table (not bubbles/table)
    spinner       spinner.Model
    loading       bool
    err           error
    instances     []gcp.Instance
    keys          instanceKeyMap
    actionMenu    *actionmenu.ActionMenu
}

func NewInstancesView(projectID string) *InstancesView {
    columns := []table.Column{/* ... */}
    t := table.New(columns, "Instances")
    v := &InstancesView{table: t, projectID: projectID}
    v.Table = &v.table  // Wire up the delegate
    return v
}
```

**Standard components in a list view**: table, spinner, actionMenu, confirm dialog, keys

## Pattern 4: Creation View with CreateViewBase

**When**: Any form that creates a GCP resource
**Why**: Handles 80% of creation view boilerplate (state machine, spinner, cancel-during-saving, error display, form sizing)

```go
type SnapshotCreateView struct {
    CreateViewBase               // Provides Init, View, SetError, SetContext, etc.
    computeClient *gcp.ComputeClient
    projectID     string
    diskName      string
}

func NewSnapshotCreateView(projectID, diskName string,
    client *gcp.ComputeClient) *SnapshotCreateView {
    v := &SnapshotCreateView{
        CreateViewBase: NewCreateViewBase("Creating snapshot..."),
        computeClient:  client,
        projectID:      projectID,
        diskName:       diskName,
    }
    v.buildForm()
    return v
}

func (v *SnapshotCreateView) Update(msg tea.Msg) tea.Cmd {
    // Base handles spinner ticks and cancel-during-saving
    if cmd, handled := v.HandleBaseUpdate(msg, SnapshotCreateCanceledMsg{}); handled {
        return cmd
    }
    switch msg := msg.(type) {
    case snapshotCreateSuccessMsg:
        return func() tea.Msg { return SnapshotCreateResultMsg{Success: true} }
    case snapshotCreateErrorMsg:
        v.SetError(msg.err)
        return nil
    case forms.FormSubmitMsg:
        return v.handleSubmit()
    case forms.FormCancelMsg:
        return func() tea.Msg { return SnapshotCreateCanceledMsg{} }
    }
    return v.UpdateForm(msg)
}
```

## Pattern 5: Three-Phase Message Flow

**When**: Any async GCP operation (create, delete, start/stop, etc.)
**Why**: Clean separation of user intent, execution, and result handling

```
Phase 1: Selection → Phase 2: Request → Phase 3: Result

View emits SelectionMsg     App handles, starts async     Result dispatched back
(user clicks item)     →    (GCP API call via tea.Cmd) →  (success or error)
```

Message naming convention:
- `XxxSelectedMsg` — user selected an item in a list
- `XxxRequestMsg` / `CreateXxxMsg` — request to perform an action
- `XxxActionResultMsg` / `XxxCreatedMsg` — async operation completed
- `XxxCanceledMsg` — user canceled a dialog/form

```go
// Phase 1: View emits request
case forms.FormSubmitMsg:
    cmd := v.BeginSaving()
    return tea.Batch(cmd, func() tea.Msg {
        return CreateSnapshotFromDiskMsg{Name: name, DiskID: diskID}
    })

// Phase 2: App handles request, starts async
case views.CreateSnapshotFromDiskMsg:
    return func() tea.Msg {
        err := a.computeClient.CreateSnapshot(ctx, msg.DiskID, msg.Name)
        return views.DiskActionResultMsg{Action: "snapshot", Error: err}
    }

// Phase 3: App dispatches result
case views.DiskActionResultMsg:
    if msg.Error != nil {
        // Critical: propagate error back to view
        if a.snapshotCreateView != nil {
            a.snapshotCreateView.SetError(msg.Error)
        }
        return nil
    }
    // Navigate back on success
```

## Pattern 6: Master-Detail with Tabs

**When**: Resource details views (instance details, network details, SQL details)
**Why**: Show overview + related data in organized tabs

```go
type InstanceDetailsView struct {
    instance  *gcp.InstanceDetails
    tabs      *tabs.Tabs           // gcon's custom tab component
    viewport  *viewport.Model      // Scrollable content per tab
    links     *links.Links         // Clickable navigation links
    spinner   spinner.Model
    loading   bool
    err       error
    // Tab-specific data
    observability *ObservabilityData
}
```

Tab navigation uses `h/l` or number keys (`1/2/3`), with `Tab` switching focus between tabs and content.

## Pattern 7: Form Flow (gcon's Forms Framework)

**When**: Multi-field data entry with validation
**Why**: gcon has a full forms framework with sections, field types, and validators

```go
func (v *BucketCreateView) buildForm() {
    v.form = forms.NewForm("Create Bucket", forms.FormModeCreate).
        EnableViewport()

    basicSection := forms.NewSection("basic", "Basic Settings").
        AddField(forms.NewTextField("name", "Bucket Name").
            SetRequired(true).
            SetValidator(forms.ValidateGCPResourceName)).
        AddField(forms.NewDropdownField("location_type", "Location Type").
            SetOptions([]forms.Option{
                {Value: "region", Label: "Region"},
                {Value: "dual-region", Label: "Dual-region"},
                {Value: "multi-region", Label: "Multi-region"},
            }))

    advancedSection := forms.NewSection("advanced", "Advanced Options").
        SetCollapsible(true).
        SetCollapsed(true).
        AddField(forms.NewToggleField("versioning", "Object Versioning"))

    v.form.AddSection(basicSection).AddSection(advancedSection)
}
```

Field types: `FieldText`, `FieldNumber`, `FieldDropdown`, `FieldMultiSelect`, `FieldToggle`, `FieldReadOnly`, `FieldTextArea`

## Layout Patterns

### App Layout (Sidebar + Content)

```go
// Sidebar + content joined horizontally, matching newline counts
lipgloss.JoinHorizontal(lipgloss.Top,
    sidebar.View(),    // Fixed width (26 or collapsed to 4)
    content.View(),    // Fills remaining width
)
```

### Vertical Stack (Header + Content + Footer)

```go
lipgloss.JoinVertical(lipgloss.Left,
    header.View(),     // Breadcrumbs + project info
    content,           // Current view
    footer.View(),     // Status bar + key hints
)
```

### Detail View with Tabs

```go
lipgloss.JoinVertical(lipgloss.Left,
    tabs.View(),       // Tab bar
    viewport.View(),   // Tab content (scrollable)
)
```

## Navigation Pattern

gcon uses a view stack for back navigation:

```go
// Push current view, switch to new one
func (a *App) handleInstanceSelected(msg views.InstanceSelectedMsg) tea.Cmd {
    a.viewStack = append(a.viewStack, a.currentView)
    a.currentView = ViewInstanceDetails
    a.instanceDetailsView = views.NewInstanceDetailsView(msg.Instance)
    a.updateViewSizes()
    return a.instanceDetailsView.Init()
}

// Pop on Esc
func (a *App) navigateBack() {
    if len(a.viewStack) > 0 {
        a.currentView = a.viewStack[len(a.viewStack)-1]
        a.viewStack = a.viewStack[:len(a.viewStack)-1]
    }
}
```

## Error Handling Pattern

Two error display modes in gcon:

```go
// For form/creation views — no retry hint
content += components.RenderInlineError(v.err)

// For list/detail views — includes retry hint
content = components.RenderError(v.err)
```

Error propagation from async ops requires `SetError()` on the active view:

```go
if msg.Error != nil {
    a.err = msg.Error
    if a.snapshotCreateView != nil {
        a.snapshotCreateView.SetError(msg.Error)  // Critical!
    }
}
```

## TextInputFocusable Pattern

**Critical for any view with text input** — prevents `q` from quitting while typing:

```go
type TextInputFocusable interface {
    HasTextInputFocused() bool
}

// Every view with forms, dialogs, or search fields must implement this
func (v *InstancesView) HasTextInputFocused() bool {
    if v.showDeleteConfirm && v.deleteConfirm != nil {
        return true
    }
    return false
}
```
