# Architecture Best Practices (gcon-adapted)

Best practices for Bubble Tea development in gcon. Generic tips are preserved where they complement gcon-specific guidance.

> **Authoritative source**: `.claude/rules/` contains the canonical project rules. This document supplements with architectural context.

## gcon-Specific Helpers

### Always Use These

| Helper | Usage |
|--------|-------|
| `components.NewGCPSpinner()` | Never create `spinner.New()` inline — use the GCP-blue styled spinner |
| `renderLoading(spinner, msg)` | Standard loading state: `"\n  {spinner} Loading instances...\n"` |
| `renderSaving(spinner, msg)` | Saving state with GCP-blue styling |
| `components.RenderError(err)` | Full error with retry hint (for list/detail views) |
| `components.RenderInlineError(err)` | Simple error without retry hint (for form views) |
| `formWidthPadding` / `formHeightPadding` | Standard form sizing (both = 4) |

### Base Types to Embed

| Base Type | When | What It Provides |
|-----------|------|------------------|
| `CreateViewBase` | Resource creation forms | State machine, spinner, cancel-during-saving, error display, form sizing, `HasTextInputFocused()` |
| `TableClickDelegate` | List views with table | Mouse click handling via `UpdateRegions`, `GetRegions`, `HandleRegionClick` |

## Model Design

### Keep State Flat

```go
// gcon views keep state flat with clear fields
type InstancesView struct {
    table     table.Model
    spinner   spinner.Model
    loading   bool
    err       error
    instances []gcp.Instance
}
```

### Separate Concerns
- **UI state** in view struct fields
- **GCP API calls** in `tea.Cmd` functions (async)
- **Cross-view communication** via messages handled in App.Update()

### Component Ownership
Each view owns its components. The App struct owns view instances but doesn't reach into their internals (except `SetError()` and `SetContext()`).

## Update Function

### Message Routing in Views

Views handle their own messages and emit cross-view messages for the App:

```go
func (v *InstancesView) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return v.handleKeyMsg(msg)
    case instancesLoadedMsg:
        v.loading = false
        v.instances = msg.instances
        v.updateTable()
        return nil
    case instancesErrorMsg:
        v.loading = false
        v.err = msg.err
        return nil
    }
    // Delegate to table for scroll/selection
    return v.table.Update(msg)
}
```

### Command Batching

```go
var cmds []tea.Cmd
cmds = append(cmds, v.spinner.Tick, v.loadInstances())
return tea.Batch(cmds...)
```

## View Function

### Responsive Layouts via SetContext

gcon views receive dimensions through `SetContext()`, not `WindowSizeMsg`:

```go
func (v *InstancesView) SetContext(ctx *context.ProgramContext) {
    v.ctx = ctx
    v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight)
}
```

### Height Matching for Sidebar Layout

When sidebar is visible, content and sidebar must output the same newline count, or the layout breaks visually. Use lipgloss containers with matching heights.

## Performance

### Minimize Allocations
Reuse slices and strings — avoid allocating in `View()`.

### Cache Column Widths

```go
// Only recalculate table columns when width changes
func (m *Model) SetSize(width, height int) {
    if width != m.lastWidth || !m.columnsComputed {
        m.adjustColumnWidths(width)
        m.lastWidth = width
        m.columnsComputed = true
    }
}
```

### Defer Heavy Operations to Commands
All GCP API calls happen in `tea.Cmd` — never block `Update()`.

### Debounce Search Input
Don't re-filter on every keystroke for large lists.

## Error Handling

### User-Friendly Errors
- Show actionable messages (not raw API errors)
- Use Google-red `#EA4335` for error text
- Forms: `RenderInlineError()` (no retry)
- Lists: `RenderError()` (with retry hint)

### Error Recovery
- Allow retry via `r` key
- Allow cancel/escape during saving state
- Propagate errors back to views via `SetError()`

### Nil Client Checks

```go
func (v *View) loadData() tea.Cmd {
    return func() tea.Msg {
        if v.client == nil {
            return errorMsg{err: fmt.Errorf("client not initialized")}
        }
        data, err := v.client.GetData(ctx, v.resourceID)
        // ...
    }
}
```

## Testing

### Test Pure Functions
Extract business logic for easy testing.

### Table-Driven Tests with testify

```go
func TestEffectiveState(t *testing.T) {
    tests := []struct {
        name     string
        state    string
        policy   string
        expected string
    }{
        {"running", "RUNNABLE", "ALWAYS", "RUNNABLE"},
        {"stopped", "RUNNABLE", "NEVER", "STOPPED"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.expected, effectiveState(tt.state, tt.policy))
        })
    }
}
```

### Deterministic Map Iteration
Sort keys before rendering UI from maps — prevents flaky tests.

## Accessibility

### Keyboard-First
All features accessible via keyboard. Mouse is supplementary.

### Clear Indicators
- Status: `●` green (running), `●` red (stopped), `○` yellow (transitioning)
- Use lipgloss colors, not emoji (avoids width miscounting)
- Focus indicators on active component

### Help Text
`?` key toggles help overlay. Each view defines its own `KeyMap`.

## Code Organization

### gcon File Structure

```
internal/ui/
├── app.go              # App struct, Update(), view type enum
├── app_render.go       # renderCurrentView() switch
├── app_navigation.go   # Navigation handlers, clearAllViews()
├── keys.go             # Global key bindings
├── styles.go           # Lip Gloss styles
├── messages.go         # Shared message types
├── views/
│   ├── view.go         # View interface definition
│   ├── helpers.go      # Shared helpers, TableClickDelegate
│   ├── create_view_base.go  # CreateViewBase
│   ├── instances.go    # Compute instances list
│   ├── instance_details.go
│   ├── snapshot_create.go   # Uses CreateViewBase
│   └── ...
└── components/
    ├── table/          # Custom table with sort/filter
    ├── forms/          # Form framework
    ├── sidebar/        # Navigation sidebar
    ├── actionmenu/     # Context menu
    ├── commandpalette/ # Command palette
    ├── confirm/        # Type-to-confirm dialogs
    └── ...
```

### One View Per File
Each view gets its own file. Related messages in `xxx_messages.go`.

## Debugging

### Debug Logging
gcon uses `internal/debug/` package for structured debug logging.

### Debug Mode
gcon does not provide an in-app debug/logs view. Use `internal/debug` logging and terminal output to inspect application state during development.

## Common Pitfalls

1. **Forgetting `renderCurrentView()`** in `app_render.go` — new view shows "View not implemented"
2. **Not implementing `HasTextInputFocused()`** — typing 'q' in a text field quits the app
3. **Not calling `SetError()` in app handler** — view stays stuck in saving state with spinner forever
4. **Using `tea.ClearScreen`** — clears entire terminal including app chrome
5. **Incorrect or missing `SetContext()` sizing logic** — views and child components don't resize with the window
6. **Blocking in `Update()`** — freezes UI; always use `tea.Cmd` for async
7. **Non-idempotent `Init()`** — stale data after refresh; always reset `loading = true` and `err = nil`
8. **Unicode width miscounting** — `lipgloss.Width()` miscounts some symbols; use `SafeWidth()` helper
9. **Forgetting to add view to command palette** — view not discoverable via `:` or `Ctrl+K`
