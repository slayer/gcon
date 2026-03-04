# Async Operations Error Handling

This document describes the consistent pattern for handling errors in views that perform asynchronous GCP operations.

## Pattern Overview

Views with async operations (create, update, delete) must implement robust error handling to prevent getting stuck in loading/saving states.

## Preferred Pattern: CreateViewBase

For creation views (form → submit → async operation), embed `CreateViewBase` instead of
implementing the state machine manually. It provides state management, error handling,
spinner, cancel-during-saving, and form sizing out of the box.

```go
type XxxCreateView struct {
    CreateViewBase  // State, Err, Spinner, Form, etc.
    client *gcp.ComputeClient
}

func NewXxxCreateView(...) *XxxCreateView {
    v := &XxxCreateView{
        CreateViewBase: NewCreateViewBase("Creating xxx..."),
    }
    v.buildForm()
    return v
}

func (v *XxxCreateView) Update(msg tea.Msg) tea.Cmd {
    if cmd, handled := v.HandleBaseUpdate(msg, XxxCanceledMsg{}); handled {
        return cmd
    }
    switch msg := msg.(type) {
    case xxxSuccessMsg:
        return func() tea.Msg { return XxxResultMsg{Success: true} }
    case xxxErrorMsg:
        v.SetError(msg.err)
        return nil
    case forms.FormSubmitMsg:
        return v.handleSubmit()
    case forms.FormCancelMsg:
        return func() tea.Msg { return XxxCanceledMsg{} }
    }
    return v.UpdateForm(msg)
}
```

See: `snapshot_create.go`, `disk_create.go`, `image_create.go`

## Manual Pattern (For Complex Views)

For views with more complex lifecycles (e.g., `bucket_create.go` with diff preview,
`instance_editor.go` with label editing), implement the components manually:

### 1. State Machine

Define states for the operation lifecycle:

```go
type diskCreateState int

const (
    diskCreateStateForm    diskCreateState = iota
    diskCreateStateSaving
)

type DiskCreateView struct {
    state diskCreateState
    err   error
    // ... other fields
}
```

**Recommended states:**
- `stateForm` - User interaction with form
- `stateSaving` - Async operation in progress
- `stateError` (optional) - Dedicated error display state

### 2. SetError Method

Implement a public `SetError()` method for external error propagation:

```go
// SetError resets the view to form state and displays the error.
func (v *DiskCreateView) SetError(err error) {
    v.state = diskCreateStateForm
    v.err = err
}
```

**Purpose:** Allows the app-level handlers to propagate errors back to the view.

### 3. Internal Error Messages

Define internal message types for errors from async commands:

```go
type diskCreateErrorMsg struct{ err error }
```

Handle them in `Update()`:

```go
case diskCreateErrorMsg:
    v.state = diskCreateStateForm
    v.err = msg.err
    return nil
```

### 4. Error Display

Use `components.RenderInlineError()` for form/creation views (no retry hint),
or `components.RenderError()` for non-form views (includes retry hint).

```go
func (v *DiskCreateView) View() string {
    if v.state == diskCreateStateSaving {
        return renderSaving(v.spinner, "Creating disk...")
    }

    content := v.form.View()

    if v.err != nil {
        content += components.RenderInlineError(v.err)
    }

    return content
}
```

**Note:** `CreateViewBase.View()` handles this automatically if you embed it.

**Color standard:** Use `#EA4335` (Google red) for error text.

### 5. Result Messages

Define result messages for cross-view communication:

```go
type DiskActionResultMsg struct {
    Action  string  // "snapshot", "image", etc.
    Error   error
    // ... success fields
}
```

Emit after async operation completes:

```go
func (a *App) handleCreateSnapshotFromDisk(msg views.CreateSnapshotFromDiskMsg) tea.Cmd {
    return func() tea.Msg {
        err := a.computeClient.CreateSnapshot(ctx, msg.DiskID, msg.Name)
        if err != nil {
            return views.DiskActionResultMsg{
                Action: "snapshot",
                Error:  err,
            }
        }
        return views.DiskActionResultMsg{
            Action: "snapshot",
            // ... success data
        }
    }
}
```

### 6. App Handler Error Propagation

**Critical:** App handlers MUST call `SetError()` on views when operations fail:

```go
func (a *App) handleDiskActionResult(msg views.DiskActionResultMsg) tea.Cmd {
    if msg.Error != nil {
        a.err = msg.Error

        // Propagate error to the active creation view
        if msg.Action == "snapshot" && a.currentView == ViewSnapshotCreate && a.snapshotCreateView != nil {
            a.snapshotCreateView.SetError(msg.Error)
        }
        if msg.Action == "image" && a.currentView == ViewImageCreate && a.imageCreateView != nil {
            a.imageCreateView.SetError(msg.Error)
        }

        return nil
    }

    // Handle success...
}
```

**Without this:** Views get stuck in "saving" state with a spinner forever.

### 7. Cancel During Async Operations

Always allow users to cancel even during saving:

```go
func (v *DiskCreateView) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Allow cancel during saving
        if v.state == diskCreateStateSaving {
            if key.Matches(msg, v.keys.Cancel) {
                return func() tea.Msg { return ui.NavigateBackMsg{} }
            }
            return nil  // Block other keys
        }
        // ... handle other states
    }
}
```

## Message Flow

```
View submits form
    ↓
View enters stateSaving
    ↓
View emits CreateXxxMsg
    ↓
App handler starts async operation
    ↓
GCP API call completes
    ↓
Returns XxxActionResultMsg
    ↓
App calls handleXxxActionResult()
    ↓
If error: calls view.SetError(err)
    ↓
View resets to stateForm and displays error
```

## Advanced Pattern: Dedicated Error State

For operations that may benefit from retry/recovery UI:

```go
const (
    stateForm
    stateSaving
    stateError  // Dedicated error state
)

func (v *View) renderError() string {
    var b strings.Builder
    errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
    helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

    b.WriteString("\n")
    b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", v.err)))
    b.WriteString("\n\n")
    b.WriteString(helpStyle.Render("  r: retry  esc: go back"))
    b.WriteString("\n")

    return b.String()
}
```

Handle retry:

```go
case tea.KeyMsg:
    if v.state == stateError {
        if key.Matches(msg, v.keys.Retry) {
            v.state = stateForm
            v.err = nil
            return nil
        }
    }
```

## Checklist for New Async Views

### If using `CreateViewBase` (preferred for simple form → submit → async):

- [ ] Embed `CreateViewBase` and call `NewCreateViewBase("Creating xxx...")`
- [ ] Implement `buildForm()` with view-specific fields
- [ ] Implement `Update()` — call `HandleBaseUpdate()` first, handle view-specific messages
- [ ] Implement `handleSubmit()` — validate, extract data, call `BeginSaving()`
- [ ] Define internal error message type (e.g., `xxxErrorMsg`)
- [ ] Define result message type (e.g., `XxxActionResultMsg`)
- [ ] Create app handler for result messages
- [ ] **Call `SetError()` in app handler when operation fails**
- [ ] Register footer task with `registerRunningTask()` before starting async operation
- [ ] Clear footer task with `finishTask()` in result handler (both success and error)

### If implementing manually:

- [ ] Define state machine with at least `stateForm` and `stateSaving`
- [ ] Add `err error` field to view struct
- [ ] Implement `SetError(err error)` method
- [ ] Define internal error message type (e.g., `xxxErrorMsg`)
- [ ] Handle error messages in `Update()`
- [ ] Display errors with `components.RenderInlineError(v.err)`
- [ ] Use `renderSaving(v.spinner, "...")` for saving state
- [ ] Define result message type (e.g., `XxxActionResultMsg`)
- [ ] Create app handler for result messages
- [ ] **Call `SetError()` in app handler when operation fails**
- [ ] Allow cancel during `stateSaving`
- [ ] Clear error when retrying or navigating away
- [ ] Register footer task with `registerRunningTask()` before starting async operation
- [ ] Clear footer task with `finishTask()` in result handler (both success and error)

## Examples

**CreateViewBase pattern:** `snapshot_create.go`, `disk_create.go`, `image_create.go`

**Advanced pattern with error state:** `bucket_create.go`

**Complex editor pattern:** `instance_editor.go` (labels editor)

## Common Mistakes

### ❌ Forgetting SetError in App Handler

```go
func (a *App) handleDiskActionResult(msg views.DiskActionResultMsg) tea.Cmd {
    if msg.Error != nil {
        a.err = msg.Error
        return nil  // View stays in stateSaving forever!
    }
}
```

### ✅ Correct

```go
func (a *App) handleDiskActionResult(msg views.DiskActionResultMsg) tea.Cmd {
    if msg.Error != nil {
        a.err = msg.Error
        if a.currentView == ViewSnapshotCreate && a.snapshotCreateView != nil {
            a.snapshotCreateView.SetError(msg.Error)
        }
        return nil
    }
}
```

### ❌ Blocking Cancel During Saving

```go
if v.state == diskCreateStateSaving {
    return nil  // User can't press ESC!
}
```

### ✅ Correct

```go
if v.state == diskCreateStateSaving {
    if key.Matches(msg, v.keys.Cancel) {
        return func() tea.Msg { return ui.NavigateBackMsg{} }
    }
    return nil  // Block other keys
}
```

### ❌ Not Displaying Errors

```go
func (v *DiskCreateView) View() string {
    return v.form.View()  // Error never shown!
}
```

### ✅ Correct (or just embed CreateViewBase which handles this)

```go
func (v *DiskCreateView) View() string {
    content := v.form.View()

    if v.err != nil {
        content += components.RenderInlineError(v.err)
    }

    return content
}
```

### ❌ SetError Only Visible in Empty/Loading State

When `SetError()` stores the error but `View()` only renders it when data is nil (e.g., loading failed), errors from async actions on already-loaded data (like traffic updates, deletes) become invisible:

```go
func (v *DetailsView) View() string {
    if v.details == nil {
        if v.err != nil {
            return components.RenderError(v.err)  // Only shown when no data
        }
        return renderLoading(...)
    }
    return v.renderDetails()  // Error from traffic update never shown!
}
```

### ✅ Correct — Always Render Errors Regardless of Data State

```go
func (v *DetailsView) View() string {
    if v.details == nil {
        if v.err != nil {
            return components.RenderError(v.err)
        }
        return renderLoading(...)
    }
    content := v.renderDetails()

    // Action errors (traffic update, delete) shown even when data is loaded
    if v.actionErr != nil {
        content += components.RenderInlineError(v.actionErr)
    }

    return content
}
```

**Tip:** Use separate error fields (`detailsErr` for load failures, `actionErr` for action failures) to distinguish recoverable action errors from fatal load errors.
