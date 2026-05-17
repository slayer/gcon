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

### ❌ Global Loading Flag for Parameterized Fetches

When async loading is parameterized (by zone, by region, by ID), a global boolean flag blocks concurrent fetches. Rapid parameter changes cause only the first fetch to proceed; subsequent changes are silently dropped.

```go
// WRONG — global flag blocks fetch for new zone while old zone is in flight
loadingMachineTypes bool

func (v *View) onZoneChanged(zone string) tea.Cmd {
    if !v.loadingMachineTypes {  // zone B fetch blocked while zone A in flight
        v.loadingMachineTypes = true
        return v.fetchMachineTypes(zone)
    }
    return nil  // zone B never fetched
}
```

### ✅ Correct — Track the Parameter Being Loaded

```go
// Track which zone is loading, not just whether something is loading
loadingMachineZone string

func (v *View) onZoneChanged(zone string) tea.Cmd {
    v.loadingMachineZone = zone
    return v.fetchMachineTypes(zone)  // always starts fetch for new zone
}
```

### ❌ Fetch Error Silently Clears Pre-Populated Dropdown

When a dropdown has a default value (e.g., "default" network) and the async fetch fails, returning nil data and unconditionally updating options wipes the default.

```go
// WRONG — error path returns nil, handler clears the dropdown
func (v *View) fetchNetworks() tea.Cmd {
    return func() tea.Msg {
        networks, err := v.client.ListNetworks(ctx, v.projectID)
        if err != nil {
            return networksLoadedMsg{networks: nil}  // nil = error
        }
        return networksLoadedMsg{networks: networks}
    }
}

// Handler unconditionally updates — wipes "default" on error
case networksLoadedMsg:
    field.SetOptionsFromStrings(networkDropdownOptions(msg.networks))  // empty list!
```

### ✅ Correct — Only Update Dropdown on Successful Fetch

```go
case networksLoadedMsg:
    if msg.networks != nil {  // only update on success
        field.SetOptionsFromStrings(networkDropdownOptions(msg.networks))
    }
```

**Rule**: When an async fetch populates dropdown options, only overwrite on success. On error, preserve existing options so the form remains submittable with defaults.

## Generation Tokens for Racing Async Fetches

When a view re-fetches in response to user input (time-range change,
filter toggle, manual refresh, auto-refresh tick), an old in-flight
request can land *after* a newer one and silently overwrite the fresher
state. The user sees mismatched UI — chart values that don't match the
range bar, log lines that don't match the toolbar toggles, etc.

The standard fix is a **generation counter** stored on the view. Every
re-fetch bumps it; each cmd captures the value at scheduling time; the
response carries the captured gen back; the handler drops responses
whose gen doesn't match the current value.

### Where this applies

Any view that satisfies all three:
1. User input triggers a new fetch (range change, filter, refresh).
2. The new fetch starts before the previous one finishes.
3. Both fetches return into the same response handler.

GKE Phase 2a hit this on Observability metrics, Nodes fan-out, Logs
first-page + LoadMore, and lazy compute-client construction — four
separate places in one PR.

### Pattern

```go
type subView struct {
    // generation increments on every Refresh / Init. Each fetch captures
    // the current value and tags its response message with it; the
    // Update handler drops responses whose tag is stale.
    generation int
    // ... other state
}

// Add gen to every response message type.
type myFetchLoadedMsg struct {
    gen  int
    data Foo
}
type myFetchErrorMsg struct {
    gen int
    err error
}

func (s *subView) Refresh() tea.Cmd {
    s.loading = true
    s.generation++       // bump BEFORE scheduling the new fetch
    return s.fetch()
}

func (s *subView) fetch() tea.Cmd {
    gen := s.generation  // capture in closure at schedule time
    return func() tea.Msg {
        data, err := api.Get(ctx)
        if err != nil {
            return myFetchErrorMsg{gen: gen, err: err}
        }
        return myFetchLoadedMsg{gen: gen, data: data}
    }
}

func (s *subView) Update(msg tea.Msg) tea.Cmd {
    switch m := msg.(type) {
    case myFetchLoadedMsg:
        if m.gen != s.generation {
            return nil   // stale — drop silently
        }
        s.loading = false
        s.data = m.data
        return nil
    case myFetchErrorMsg:
        if m.gen != s.generation {
            return nil
        }
        s.loading = false
        s.err = m.err
        return nil
    }
}
```

### Critical: every response carries gen

Forget to tag ONE response type and the bug returns for that path. GKE
caught this on `initComputeClient` — the error path returned a message
without gen, so init failures landed with gen=0 while Init had already
bumped to >=1; the stale-gen guard then dropped the error and the
loading state never cleared.

### Critical: bump BEFORE scheduling

`Refresh()` must increment `s.generation` before constructing the new
cmd, so the cmd captures the new value. If you bump after `tea.Batch`
the captured gen is the old one and your own fresh response gets
dropped.

### LoadMore / pagination wrinkle

If the view has both first-page and follow-up-page messages, both must
carry the same gen captured at the same level. Don't bump generation
inside `LoadMore()` — pagination follow-ups belong to the same logical
fetch chain as the first page. Refresh is what creates a new chain.

### What NOT to use this for

- Self-rescheduling tick loops (spinner.TickMsg, auto-refresh ticks)
  — those have their own gating mechanism (`!loading` for spinner,
  `(autoRefresh && tabActive)` for auto-refresh). Don't double-guard.
- Strictly serial fetches where there's no possible concurrent issue
  (e.g. a one-shot Init that can't be re-triggered). Generation tokens
  are dead weight when the race can't happen.

### Testing

A stale-gen drop test is cheap and high-signal:

```go
func TestStaleResponseDropped(t *testing.T) {
    s := newSubView()
    s.generation = 2
    s.loading = true
    s.data = nil

    // A response from the previous (gen=1) fetch arrives late.
    s.Update(myFetchLoadedMsg{gen: 1, data: Foo{Value: 999}})
    assert.Nil(t, s.data, "stale-gen response must not populate data")
    assert.True(t, s.loading, "stale-gen response must not flip loading=false")
}
```

See: `gke_observability.go`, `gke_nodes.go`, `gke_logs.go` for the
full pattern across multiple message types.
