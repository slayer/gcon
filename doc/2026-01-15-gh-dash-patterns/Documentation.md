# gh-dash Patterns Documentation

## Summary

This PR introduces foundational patterns from the gh-dash TUI into gcon, providing building blocks for improved code organization, UX, and performance. All views now use a centralized `ProgramContext` for dimension and style management through a new `View` interface with `SetContext` method.

## Changes Made

### 1. Centralized Context Package (`internal/ui/context/`)

**New Files:**
- `context.go` - ProgramContext struct and Styles
- `messages.go` - Task lifecycle messages
- `context_test.go` - Unit tests

**Features:**
- `ProgramContext` holds shared state (dimensions, styles, tasks)
- `Task` struct tracks async operations with state machine (Running → Finished/Error)
- `Styles` organizes all UI styles by component (Common, Table, Sidebar, Footer, Help)
- GCP-inspired color palette with semantic colors

**Usage:**
```go
ctx := context.New()
ctx.SetDimensions(screenW, screenH, contentW, contentH)

// Track async tasks
ctx.Tasks["load"] = context.Task{
    ID:          "load",
    Description: "Loading instances...",
    State:       context.TaskRunning,
    StartTime:   time.Now(),
}

// Check for active tasks
if ctx.HasActiveTask() {
    desc := ctx.ActiveTaskDescription()
}
```

### 1a. View Interface (`internal/ui/views/`)

**New File:**
- `view.go` - View interface definition

**Features:**
- `View` interface defines contract for all views
- `SetContext(ctx *ProgramContext)` method for context propagation
- Views read dimensions from context instead of separate parameters

**Usage:**
```go
// All views implement the View interface
type View interface {
    Init() tea.Cmd
    Update(msg tea.Msg) tea.Cmd
    View() string
    SetContext(ctx *context.ProgramContext)
}

// App propagates context to views
func (a *App) updateViewSizes() {
    a.syncContext()
    a.projectView.SetContext(a.ctx)
    if a.instancesView != nil {
        a.instancesView.SetContext(a.ctx)
    }
    // ... other views
}
```

### 2. Enhanced Table Component (`internal/ui/components/table/`)

**New Features:**
- `Column` struct with Title, Width, Hidden, Grow, ComputedWidth fields
- `NewWithColumns()` constructor for enhanced column definitions
- Width caching to prevent recalculation on every render
- `Grow` flag for flexible columns that expand to fill space
- `SetColumnHidden()` for runtime visibility control
- `SetLoading()` and `SetEmptyText()` for state handling

**Usage:**
```go
// Enhanced columns with grow flag
cols := []table.Column{
    {Title: "Status", Width: 3, Hidden: false, Grow: false},
    {Title: "Name", Width: 20, Hidden: false, Grow: true},  // Expands
    {Title: "Zone", Width: 15, Hidden: false, Grow: false},
}
t := table.NewWithColumns(cols, "Instances")

// Loading state
t.SetLoading(true, "Loading instances...")

// Empty state
t.SetEmptyText("No instances in this project")

// Hide/show columns at runtime
t.SetColumnHidden("Zone", true)
```

**Backward Compatibility:**
```go
// Old API still works unchanged
cols := []table.Column{{Title: "Name", Width: 20}}
t := table.New(cols, "Title")
```

### 3. Enhanced Footer Component (`internal/ui/components/footer/`)

**New Files:**
- `footer.go` - Footer model with dynamic sections
- `footer_test.go` - Unit tests

**Features:**
- Left/right sections with dynamic content
- Active task indicator with elapsed time
- Dynamic spacing to fill width
- Confirm quit mode
- Helper functions for formatting

**Usage:**
```go
ctx := context.New()
f := footer.New(ctx)
f.SetWidth(terminalWidth)
f.SetLeftSection("5 instances")
f.SetHelpText("? help • q quit")

// Format helpers
footer.FormatResourceCount("instances", 5)  // "5 instances"
footer.FormatResourceCount("instances", 1)  // "1 instance"
footer.FormatLastRefresh(lastRefreshTime)   // "5 mins ago"
```

## Architecture Decisions

### Why Foundation Components First

The implementation creates foundation components without modifying existing views because:

1. **Low Risk** - No changes to working code means no regressions
2. **Incremental Adoption** - Views can adopt patterns one at a time
3. **Testable** - New components are fully tested in isolation
4. **Backward Compatible** - Existing `table.New()` works unchanged

### Pattern Choices from gh-dash

| Pattern | Adopted | Reason |
|---------|---------|--------|
| ProgramContext | Yes | Clean state sharing across components |
| Task tracking | Yes | Better UX for async operations |
| Column caching | Yes | Performance optimization |
| Grow columns | Yes | Flexible layouts |
| Footer sections | Yes | Status display |
| Carousel tabs | No | gcon uses sidebar navigation |
| Config keybindings | No | Lower priority |

## Testing

All new code includes comprehensive unit tests:

```bash
# Run all tests
make test

# Run specific package tests
go test ./internal/ui/context/... -v
go test ./internal/ui/components/table/... -v
go test ./internal/ui/components/footer/... -v
```

## Future Work

Items deferred to future PRs:

1. **Task System Integration** - Connect async operations to task tracking
2. **Autocomplete** - Fuzzy filtering for resource lists
3. **Caching** - API response caching with otter library
4. **Auto-refresh** - Configurable polling for mutable views

## File Changes

```
internal/ui/context/
├── context.go      (new) - ProgramContext, Task, Styles
├── messages.go     (new) - Task lifecycle messages
└── context_test.go (new) - Unit tests

internal/ui/views/
├── view.go           (new) - View interface definition
├── projects.go       (modified) - Added SetContext, ctx field
├── instances.go      (modified) - Added SetContext, ctx field
├── instance_details.go (modified) - Added SetContext, ctx field
├── disks.go          (modified) - Added SetContext, ctx field
├── disk_details.go   (modified) - Added SetContext, ctx field
├── buckets.go        (modified) - Added SetContext, ctx field
├── objects.go        (modified) - Added SetContext, ctx field
└── buckets_test.go   (modified) - Updated to use SetContext

internal/ui/
└── app.go            (modified) - Updated updateViewSizes to use SetContext

internal/ui/components/table/
├── table.go        (modified) - Added Column struct, caching, states
└── table_test.go   (new) - Unit tests for new features

internal/ui/components/footer/
├── footer.go       (new) - Footer component
└── footer_test.go  (new) - Unit tests
```
