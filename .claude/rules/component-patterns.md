---
description: UI component best practices and patterns
globs:
  - "internal/ui/**/*.go"
---

# Component Patterns

## Shared Helpers and Base Types

Use the shared helpers in `internal/ui/views/helpers.go` and `internal/ui/components/` to avoid boilerplate:

| Helper | Location | Use |
|--------|----------|-----|
| `components.NewGCPSpinner()` | `components/spinner.go` | Always use instead of inline `spinner.New()` |
| `renderLoading(spinner, msg)` | `views/helpers.go` | Standard loading state with spinner |
| `renderSaving(spinner, msg)` | `views/helpers.go` | Saving state with GCP-blue styled message |
| `components.RenderInlineError(err)` | `components/error_display.go` | Form-context errors (no retry hint) |
| `components.RenderError(err)` | `components/error_display.go` | Full error display with retry hint |
| `formWidthPadding` / `formHeightPadding` | `views/helpers.go` | Standard form sizing padding (both = 4) |
| `TableClickDelegate` | `views/helpers.go` | Embed in list views for mouse click delegation |
| `CreateViewBase` | `views/create_view_base.go` | Embed in creation views for full lifecycle |
| `metricchart.New(height)` | `components/metricchart/` | Braille time series charts for metrics |

## Metric Charts (metricchart package)

Use `metricchart.Chart` for rendering time series metrics in observability tabs. Charts are **stateful renderers**, not Bubble Tea models — no `tea.Msg` routing needed.

```go
// Single-series chart (CPU, memory, request count)
chart := metricchart.New(metricchart.HeightStandard)
chart.SetYRange(0, 100).
    SetStatsFormatter(metricchart.FormatPercentageStats).
    SetYLabelFormatter(metricchart.PercentYLabel)
chart.SetData(dataPoints)         // []gcp.DataPoint
output := chart.View()            // renders chart + stats line

// Multi-series overlay (latency p50/p95/p99, error rates 4xx/5xx)
chart := metricchart.New(metricchart.HeightStandard)
chart.SetStatsFormatter(nil).     // no per-series stats for multi-dataset
    SetYLabelFormatter(metricchart.LatencyYLabel)
chart.SetDataSets([]metricchart.DataSet{
    {Name: "p50", Data: p50Data, Color: "#34A853"},
    {Name: "p95", Data: p95Data, Color: "#FBBC04"},
    {Name: "p99", Data: p99Data, Color: "#EA4335"},
})
output := chart.View()            // renders chart + color-coded legend
```

Key points:
- Call `chart.Resize(width)` when parent width changes (subtract padding for indent)
- Heights: `HeightStandard = 8` for primary metrics, `HeightCompact = 5` for secondary
- Y-axis formatters: `humanYLabel` (default, SI suffixes), `PercentYLabel`, `LatencyYLabel`, `VCPUYLabel`
- Use `SetYRange(0, 100)` for percentage metrics to prevent misleading auto-scaling
- GCP color palette for multi-series: green=#34A853, yellow=#FBBC04, red=#EA4335
- **Data gaps are handled automatically**: `fillGaps()` inserts zero-value boundary points when consecutive data points are >3x the minimum interval apart. This prevents ntcharts from drawing misleading lines across periods with no data (e.g., Cloud Run scaled to zero). No action needed by callers — it runs inside `renderChart()`.

## Always Create Fresh Modal Instances

When opening modals/overlays, always create a new instance with current state rather than reusing stale instances:

```go
// Wrong - reuses stale instance if selectedProject is nil
if a.selectedProject != nil {
    a.projectSelector = projectselector.New(a.gcpClient, a.selectedProject.ID)
}
a.showProjectSelector = true
return a, a.projectSelector.Init()

// Correct - always creates fresh instance
currentProjectID := ""
if a.selectedProject != nil {
    currentProjectID = a.selectedProject.ID
}
a.projectSelector = projectselector.New(a.gcpClient, currentProjectID)
a.showProjectSelector = true
return a, a.projectSelector.Init()
```

## Defensive Bounds Checking for List Components

Always validate list bounds before accessing elements, especially in user input handlers:

```go
// Handling Enter key on a filtered list
case key.Matches(msg, m.keys.Select):
    // Check for empty list first
    if len(m.filteredProjects) == 0 {
        return nil
    }
    // Check cursor bounds
    if m.cursor < 0 || m.cursor >= len(m.filteredProjects) {
        m.cursor = 0
        return nil
    }
    // Safe to access now
    selectedItem := m.filteredProjects[m.cursor]
```

## Cursor Position After Filtering

Cap cursor to the last valid position instead of always resetting to 0 when filtering reduces list size:

```go
// Reset cursor if out of bounds after filtering
if m.cursor >= len(m.filteredProjects) {
    if len(m.filteredProjects) > 0 {
        m.cursor = len(m.filteredProjects) - 1  // Preserve position
    } else {
        m.cursor = 0
    }
}
```

## Mouse Click Region Calculations

For click regions, use simple geometry based on known dimensions rather than re-rendering components:

```go
// Wrong - calls render again, potentially inconsistent
right3X := f.right3StartPos + (f.terminalWidth(f.renderRightGroup()) - f.right3Width)

// Correct - use simple geometry
right3X := f.width - f.right3Width  // Right-aligned section
```

## Complete State Cleanup on Context Switch

When switching contexts (e.g., projects), ensure ALL view instances are cleared to prevent stale state:

```go
func (a *App) clearAllViews() {
    a.projectView = nil           // Don't forget any views!
    a.instancesView = nil
    a.instanceDetailsView = nil
    // ... clear ALL view instances

    // Clear navigation state
    a.viewStack = nil

    // Clear selected resources
    a.selectedInstance = nil
    // ... clear ALL selected resources
}
```

## Deterministic Map Iteration for UI

Go maps iterate in random order. When rendering UI elements from maps (labels, tags, etc.), sort keys first for consistent output and testable behavior:

```go
// Wrong - non-deterministic order, flaky tests
for key, val := range labels {
    fields = append(fields, Field{Label: key, Value: val})
}

// Correct - deterministic alphabetical order
keys := make([]string, 0, len(labels))
for k := range labels {
    keys = append(keys, k)
}
sort.Strings(keys)
for _, key := range keys {
    fields = append(fields, Field{Label: key, Value: labels[key]})
}
```

## Allow Cancel During Async Operations

Users should be able to cancel/escape during loading or saving states. Check for cancel key before returning nil in async state handlers:

```go
func (v *View) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
    // Handle loading/saving states - allow cancel
    if v.state == stateLoading || v.state == stateSaving {
        if key.Matches(msg, v.keys.Cancel) {
            return func() tea.Msg { return CancelledMsg{} }
        }
        return nil  // Ignore other keys during async ops
    }
    // ... handle other states
}
```

## Nil Client Checks in Async Commands

GCP client may be nil in tests or error states. Add defensive checks at the start of async commands:

```go
func (v *View) loadData() tea.Cmd {
    return func() tea.Msg {
        if v.client == nil {
            return errorMsg{err: fmt.Errorf("client not initialized")}
        }
        // Safe to use client now
        data, err := v.client.GetData(ctx, v.resourceID)
        if err != nil {
            return errorMsg{err: err}
        }
        return dataLoadedMsg{data: data}
    }
}
```

## Ticker Goroutines Need a `done` Channel to Avoid Leaks

**Critical**: `time.Ticker.Stop()` stops sending ticks but does **not** close `ticker.C`. A goroutine blocking on `<-ticker.C` will hang forever after `Stop()` — this is a goroutine leak.

Always pair a ticker with a `done chan struct{}`. Use `select` on both, and `close(done)` when stopping.

```go
type View struct {
    autoRefreshTicker *time.Ticker
    autoRefreshDone   chan struct{}
}

func (v *View) tickAutoRefresh() tea.Cmd {
    ticker := v.autoRefreshTicker
    done := v.autoRefreshDone
    if ticker == nil || done == nil {
        return nil
    }
    return func() tea.Msg {
        select {
        case <-ticker.C:
            return refreshTickMsg{}
        case <-done:
            return nil
        }
    }
}

func (v *View) startAutoRefresh() {
    v.stopAutoRefresh() // clean up previous
    v.autoRefreshTicker = time.NewTicker(30 * time.Second)
    v.autoRefreshDone = make(chan struct{})
}

func (v *View) stopAutoRefresh() {
    if v.autoRefreshTicker != nil {
        v.autoRefreshTicker.Stop()
        v.autoRefreshTicker = nil
    }
    if v.autoRefreshDone != nil {
        close(v.autoRefreshDone)
        v.autoRefreshDone = nil
    }
}
```

**Checklist** when using tickers in `tea.Cmd`:
1. Add a `done chan struct{}` field alongside the ticker
2. Create both together (`NewTicker` + `make(chan struct{})`)
3. `select` on both `ticker.C` and `done` in the blocking goroutine
4. Close `done` in every code path that stops the ticker (toggle off, tab switch, `Close()`)
5. Set both to nil after cleanup to prevent double-close panics

See: `cloudrun_observability.go`, `instance_details.go`

## `tea.Tick()` Messages Survive Context Switches

**Critical**: Unlike `time.Ticker` goroutines, `tea.Tick()` schedules a message on Bubble Tea's internal queue. There is no way to cancel a pending `tea.Tick()` message — it will always be delivered.

When using `tea.Tick()` for periodic refresh (e.g., auto-refresh on an observability tab), the tick handler must guard against stale delivery after the user has navigated away:

```go
type View struct {
    autoRefresh bool  // user preference
    tabActive   bool  // whether this tab is currently visible
}

func (v *View) tickAutoRefresh() tea.Cmd {
    // Don't schedule if either condition is false
    if !v.autoRefresh || !v.tabActive {
        return nil
    }
    return tea.Tick(30*time.Second, func(_ time.Time) tea.Msg {
        return refreshTickMsg{}
    })
}

// In Update():
case refreshTickMsg:
    // Drop stale ticks — user may have left the tab while tick was pending
    if !v.autoRefresh || !v.tabActive {
        return nil, true
    }
    // Safe to refresh
    return v.loadMetrics(), true
```

**Key difference from `time.Ticker`**: `time.Ticker` leaks goroutines if not properly stopped. `tea.Tick()` doesn't leak goroutines but delivers stale messages. Both need guards, but the mechanism differs.

See: `cloudrun_observability.go`

## Dialog Update() Must Accept tea.Msg (Not Just tea.KeyMsg)

Dialogs with text inputs (e.g., traffic split editor, inline editors) must accept `tea.Msg` in their `Update()` method, not just `tea.KeyMsg`. The `textinput.Model` component emits blink commands (`textinput.Blink`) that must be processed for the cursor to blink.

```go
// WRONG - cursor never blinks, textinput commands lost
func (d *MyDialog) Update(msg tea.KeyMsg) (tea.Cmd, bool) {
    // Only handles key events
    d.input, _ = d.input.Update(msg)
    return nil, false
}

// CORRECT - handles all message types including cursor blink
func (d *MyDialog) Update(msg tea.Msg) (tea.Cmd, bool) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle navigation, submit, cancel...
    }
    // Pass all messages to focused input for blink/cursor updates
    var cmd tea.Cmd
    d.input, cmd = d.input.Update(msg)
    return cmd, false
}
```

**Symptom**: Text input works (typing, cursor movement) but cursor doesn't blink. Functional but UX-inconsistent.

## Text Input Focus Handling (TextInputFocusable)

**Critical**: Views that show dialogs or modals with text input MUST implement `HasTextInputFocused()`.

The app skips global character keys (like 'q' for quit) when a view reports text input is focused. Without this, typing 'q' in a text field will quit the app.

```go
// WRONG - global 'q' key will quit app when input dialog is open
type MyView struct {
    inputDialog     *inputdialog.InputDialog
    showInputDialog bool
}

// CORRECT - implement TextInputFocusable interface
func (v *MyView) HasTextInputFocused() bool {
    // Return true when ANY text input component is active
    return v.showInputDialog && v.inputDialog != nil
}

// For views with forms:
func (v *MyView) HasTextInputFocused() bool {
    if v.form != nil {
        return v.form.HasTextInputFocused()
    }
    return false
}

// For views with multiple possible text inputs:
func (v *MyView) HasTextInputFocused() bool {
    if v.showInputDialog && v.inputDialog != nil {
        return true
    }
    if v.form != nil && v.form.HasTextInputFocused() {
        return true
    }
    return false
}
```

**Checklist when adding text input to a view:**
1. Does the view implement `HasTextInputFocused() bool`?
2. Does it return `true` when the text input is active?
3. Test by typing 'q' in the input field - it should NOT quit the app

## Links Component: RenderRow() Does Not Append Newline

`links.RenderRow()` returns styled text **without** a trailing `\n`, unlike `renderRow()` from `helpers.go` which always appends one. Additionally, `RenderRow()` prepends a 2-space cursor prefix — so labels should be rendered **outside** the call, not inside it.

```go
// Wrong — label gets cursor prefix ("  Network: main"), no newline
b.WriteString(v.networkLink.RenderRow(0, labelStyle.Render("Network:")+" "+d.Network))

// Correct — label separate, only value inside RenderRow, explicit newline
label := labelStyle.Render("Network:")
linkRendered := v.networkLink.RenderRow(0, d.Network)
b.WriteString(label + " " + linkRendered + "\n")
```

See: `snapshot_details.go` (correct pattern), `firewall_details.go` (has the prefix issue)
