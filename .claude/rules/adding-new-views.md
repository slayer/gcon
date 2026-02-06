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

## Common Symptoms

- **"View not implemented"** when navigating → forgot `renderCurrentView()` in `app_render.go`
- **App quits when typing 'q'** in text field → forgot `HasTextInputFocused()` implementation
