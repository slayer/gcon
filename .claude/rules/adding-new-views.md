# Adding New Views Checklist

When adding a new view type, multiple files need to be updated. Missing any of these causes subtle bugs (e.g., "View not implemented" errors).

## Required Changes

### 1. app.go - View Type and Field

```go
// Add constant to ViewType enum
const (
    // ...existing views...
    ViewXxx  // Your new view
)

// Add field to App struct
type App struct {
    // ...existing fields...
    xxxView *views.XxxView
}
```

### 2. app.go - getCurrentViewModel()

Add case to return the view model:

```go
func (a *App) getCurrentViewModel() ViewModel {
    switch a.currentView {
    // ...existing cases...
    case ViewXxx:
        return a.xxxView
    }
}
```

### 3. app_render.go - renderCurrentView()

**Often forgotten!** This is a separate switch that also needs the new view:

```go
func (a *App) renderCurrentView() string {
    switch a.currentView {
    // ...existing cases...
    case ViewXxx:
        if a.xxxView != nil {
            return a.xxxView.View()
        }
    }
}
```

### 4. app.go - Update() Message Handlers

Add cases for the view's messages:

```go
case views.XxxRequestMsg:
    return a, a.handleXxxRequest(msg)
case views.XxxCompletedMsg:
    return a, a.handleXxxCompleted(msg)
case views.XxxCanceledMsg:
    a.handleXxxCanceled()
    return a, nil
```

### 5. app_navigation.go - Navigation Handlers

Implement the handler functions:

```go
func (a *App) handleXxxRequest(msg views.XxxRequestMsg) tea.Cmd {
    a.viewStack = append(a.viewStack, a.currentView)
    a.currentView = ViewXxx
    a.xxxView = views.NewXxxView(/* params */)
    a.updateViewSizes()
    return a.xxxView.Init()
}
```

### 6. app_navigation.go - clearAllViews()

Add cleanup for project switching:

```go
func (a *App) clearAllViews() {
    // ...existing nils...
    a.xxxView = nil
}
```

### 7. app.go - updateViewSizes()

If view needs context:

```go
func (a *App) updateViewSizes() {
    // ...existing views...
    if a.xxxView != nil {
        a.xxxView.SetContext(a.ctx)
    }
}
```

### 8. app_navigation.go - updateSidebarActiveView()

If view should highlight a sidebar item:

```go
func (a *App) updateSidebarActiveView() {
    switch a.currentView {
    // ...existing cases...
    case ViewXxx:
        a.sidebar.SetActiveView(sidebar.ViewXxx)
    }
}
```

### 9. View with Text Input - HasTextInputFocused()

**If view has ANY text input (forms, dialogs, search fields)**, implement `TextInputFocusable`:

```go
// Required to prevent 'q' key from quitting when typing
func (v *XxxView) HasTextInputFocused() bool {
    // Return true when text input is active
    if v.showInputDialog && v.inputDialog != nil {
        return true
    }
    if v.form != nil {
        return v.form.HasTextInputFocused()
    }
    return false
}
```

**Note:** If your view embeds `CreateViewBase`, `HasTextInputFocused()` is already provided.

**Test**: Type 'q' in any text field - it should NOT quit the app.

## Shared Base Types

### List Views — Use `TableClickDelegate`

List views with a `table.Model` should embed `TableClickDelegate` to get mouse click handling for free:

```go
type XxxView struct {
    TableClickDelegate  // Provides UpdateRegions, GetRegions, HandleRegionClick
    table  table.Model
    // ...
}

func NewXxxView() *XxxView {
    t := table.New(columns, title)
    v := &XxxView{table: t}
    v.Table = &v.table  // Wire up the delegate
    return v
}
```

See: `projects.go`, `instances.go`, `disks.go`, `snapshots.go`, `images.go`, `buckets.go`, `objects.go`

### Creation Views — Use `CreateViewBase`

Views that create GCP resources via a form should embed `CreateViewBase` to get the full lifecycle for free:

```go
type XxxCreateView struct {
    CreateViewBase  // Provides Init, View, SetError, SetContext, HasTextInputFocused, etc.
    computeClient *gcp.ComputeClient
    // ... view-specific fields
}

func NewXxxCreateView(...) *XxxCreateView {
    v := &XxxCreateView{
        CreateViewBase: NewCreateViewBase("Creating xxx..."),
        // ... view-specific init
    }
    v.buildForm()
    return v
}
```

What `CreateViewBase` provides: `Init()`, `View()`, `SetError()`, `SetContext()`, `HasTextInputFocused()`, `BeginSaving()`, `HandleBaseUpdate()`, `UpdateForm()`.

What you implement: `buildForm()`, `Update()` (handle view-specific messages, delegate to base), `handleSubmit()`.

See: `snapshot_create.go`, `disk_create.go`, `image_create.go`

**Note:** `bucket_create.go` has a more complex lifecycle (diff viewer, dedicated error state) and does NOT use `CreateViewBase`.

### Shared Helpers

- **Spinners**: Always use `components.NewGCPSpinner()` — never create `spinner.New()` inline
- **Loading state**: Use `renderLoading(v.spinner, "Loading xxx...")` from `helpers.go`
- **Saving state**: Use `renderSaving(v.spinner, "Creating xxx...")` from `helpers.go`
- **Inline errors**: Use `components.RenderInlineError(v.err)` for form views (no retry hint)
- **Full errors**: Use `components.RenderError(v.err)` for non-form views (includes retry hint)
- **Form sizing**: Use `formWidthPadding` and `formHeightPadding` constants from `helpers.go`

### 10. Command Palette - Navigation Commands

**Often forgotten!** Add the new view to the command palette so users can navigate via `:` or `Ctrl+K`:

In `internal/ui/components/commandpalette/commands.go`:
1. Add `ViewType` constant to the iota (must match sidebar ordering)
2. Add icon constant
3. Add navigation command entry in `NavigationCommands()`

```go
// ViewType constant
const (
    // ...existing views...
    ViewXxx
)

// Icon
const (
    // ...existing icons...
    IconXxx = "■"
)

// Navigation command in NavigationCommands()
{
    ID:       "nav:xxx",
    Label:    "Category: Xxx",
    Icon:     IconXxx,
    Type:     CommandTypeNavigation,
    ViewType: ViewXxx,
    Enabled:  true,
},
```

### 11. Init() Must Be Idempotent

Views may have `Init()` called multiple times (e.g., after refresh, returning from a child view). Always reset loading/error state:

```go
func (v *XxxView) Init() tea.Cmd {
    // Reset state — don't assume Init() is only called once
    v.loading = true
    v.err = nil
    return tea.Batch(v.spinner.Tick, v.loadData())
}
```

**Symptom:** After deleting a resource and returning to the list, the view shows stale data without a loading spinner because `Init()` didn't reset `loading = true`.

### 12. Verify Message Producers and Consumers

Every message type must have both a **producer** (view that emits it) and a **consumer** (app handler that processes it). Dead messages cause silent failures:

- After defining a message type in `iam_messages.go` (or similar), verify there's a view that actually emits it
- After adding an app handler `case views.XxxMsg:`, verify a view calls `func() tea.Msg { return views.XxxMsg{} }`
- Search for the message type name — it should appear in at least 2 places (definition + usage)

**Symptom:** A "delete key" handler exists in `app_navigation.go` but no view ever emits `DeleteServiceAccountKeyMsg` — the handler is dead code.

### 13. Sidebar Guards Must Include All Views in Feature Hierarchy

Sidebar navigation guards (in `app_navigation.go`) prevent redundant navigation when already on a feature's views. These guards must include **all** views in the hierarchy, including deeply nested ones like edit/create views.

```go
// Wrong — misses ViewCloudRunServiceEdit, allowing sidebar nav that
// clears parent views while edit view remains in viewStack
case sidebar.ViewCloudRunServices:
    if a.currentView != ViewCloudRunServices && a.currentView != ViewCloudRunServiceDetails {

// Correct — includes all views in the feature hierarchy
case sidebar.ViewCloudRunServices:
    if a.currentView != ViewCloudRunServices && a.currentView != ViewCloudRunServiceDetails && a.currentView != ViewCloudRunServiceEdit {
```

**Symptom**: Navigating via sidebar while on a nested view, then pressing Esc pops back to a nil parent view → "View not implemented" fallback.

**Rule**: When adding a new child view (edit, create, details), always check all sidebar guard conditions for the parent feature and add the new view type.

### 14. Lazy-Loaded Sub-Views Must Be Created Before `updateViewportContent()`

When a tab lazily creates a sub-view (e.g., observability tab creates its view on first visit), the sub-view must exist **before** calling `updateViewportContent()`. Otherwise the viewport renders empty for one frame.

```go
// Wrong — updateViewportContent runs before observability exists, empty frame
case tabs.TabChangedMsg:
    v.updateViewportContent()  // observability is nil → renders empty
    if v.tabs.ActiveTab().ID == "observability" {
        if v.observability == nil {
            v.observability = newObservability()
        }
    }

// Correct — create first, then update viewport
case tabs.TabChangedMsg:
    if v.tabs.ActiveTab().ID == "observability" {
        if v.observability == nil {
            v.observability = newObservability()
        }
        v.updateViewportContent()  // observability exists → renders loading state
        return v.observability.Init()
    }
    v.updateViewportContent()
```

**Symptom**: Tab shows blank content for a single frame before loading spinner appears.

### 15. Views with Action Menus — Implement MenuOpener Interface

**If a view has an action menu (`.` key)**, it must implement the `MenuOpener` interface so the app routes `Esc` to close the menu instead of navigating back:

```go
// Wrong — Esc quits the app or navigates back while menu is open
type LogsView struct {
    menuOpen bool
    // ... other fields
}

// Correct — implement MenuOpener
func (v *LogsView) IsMenuOpen() bool {
    return v.menuOpen
}
```

The app checks `MenuOpener` before routing `Esc` to navigation:

```go
// app.go Update() — Esc handling
if opener, ok := a.getCurrentViewModel().(MenuOpener); ok && opener.IsMenuOpen() {
    // Route to view to close menu
    return a, a.getCurrentViewModel().Update(msg)
}
// Otherwise: navigate back
```

**Symptom**: Pressing `Esc` while the action menu is open quits the app or navigates back to the previous view.

## Common Symptoms

- **"View not implemented"** when navigating → forgot `renderCurrentView()` in `app_render.go`
- **"View not implemented"** after sidebar nav + Esc → sidebar guard missing nested view (step 13)
- **App quits when typing 'q'** in text field → forgot `HasTextInputFocused()` implementation
- **Esc quits app with action menu open** → forgot `IsMenuOpen()` / `MenuOpener` interface (step 15)
- **View not in command palette** → forgot step 10 (command palette navigation commands)
- **Stale data after refresh** → `Init()` not idempotent (step 11)
- **Dead handler code** → message has no producer (step 12)
- **Empty frame on first tab visit** → lazy sub-view created after `updateViewportContent()` (step 14)
