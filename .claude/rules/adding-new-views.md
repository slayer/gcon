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

**Test**: Type 'q' in any text field - it should NOT quit the app.

## Common Symptoms

- **"View not implemented"** when navigating → forgot `renderCurrentView()` in `app_render.go`
- **App quits when typing 'q'** in text field → forgot `HasTextInputFocused()` implementation
