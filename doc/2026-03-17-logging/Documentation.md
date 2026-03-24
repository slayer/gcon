# Logging Explorer — Design Document

## Overview

A terminal-based GCP Logs Explorer, accessible as a top-level "Logging" sidebar item. Provides LQL query input, quick filters, a sparkline histogram, and an expandable log entry list with infinite scroll and tail mode.

## Layout

```
┌─────────────────────────────────────────────────────┐
│ Filter Bar                                          │
│ [All resources ▾]  [All log names ▾]  [All sev ▾]  │
├─────────────────────────────────────────────────────┤
│ LQL Query Input (single-line, auto-resize)          │
│ > severity>=WARNING AND resource.type="gce_instance"│
├─────────────────────────────────────────────────────┤
│ Sparkline  ▁▂▃▅▇▇▅▃▂▁▁▂▄▇█▇▅▃▂  59,888 results   │
├─────────────────────────────────────────────────────┤
│ ▸ W 2026-03-16 13:04:01  sidekiq   Connection t... │
│ ▾ E 2026-03-16 13:04:00  nginx     502 Bad Gate... │
│     resource.type: gce_instance                     │
│     resource.labels.zone: us-central1-a             │
│     jsonPayload.status: 502                    [+f] │
│ ▸ I 2026-03-16 13:03:59  cloud-run GET 200 12ms    │
│ ▸ I 2026-03-16 13:03:58  sidekiq   Job started     │
│ ...                                                 │
│                    (infinite scroll)                 │
├─────────────────────────────────────────────────────┤
│ Status: 59,888 results | Time: Last 1 hour | TAIL  │
└─────────────────────────────────────────────────────┘
```

- **Filter bar**: ~1 line. Three dropdown-style buttons (Resources, Log Names, Severities).
- **Query input**: 1 line default, auto-grows with `Shift+Enter` newlines.
- **Sparkline**: 1 line. Block characters (▁▂▃▄▅▆▇█) showing log density + result count.
- **Log entries**: Remaining vertical space. The main content area with infinite scroll.
- **Footer**: Handled by the existing status bar.

## Navigation

- Top-level "Logging" section in the sidebar.
- Command palette entry: "Logging: Logs Explorer".
- `ViewLogs` enum (already declared as placeholder in `app.go`).

## GCP API Integration

### New/Extended Methods in `internal/gcp/logging.go`

```go
// ListLogEntries executes an LQL query and returns paginated results.
ListLogEntries(ctx, projectID, query, timeRange, pageSize, pageToken) ([]LogEntry, nextPageToken, error)

// ListResourceTypes returns monitored resource types with log entries in the project.
ListResourceTypes(ctx, projectID) ([]string, error)

// ListLogNames returns log names present in the project.
ListLogNames(ctx, projectID) ([]string, error)

// GetLogEntryCount fetches the global logging.googleapis.com/log_entry_count metric
// for sparkline histogram display. Uses Cloud Monitoring API (not LQL-filtered).
MonitoringClient.GetLogEntryCount(ctx, timeRange) ([]DataPoint, error)
```

### LogEntry Model (Expanded)

```go
type LogEntry struct {
    Timestamp      time.Time
    Severity       string              // INFO, WARNING, ERROR, etc.
    Message        string              // textPayload or jsonPayload summary
    LogName        string              // e.g., "cloudaudit.googleapis.com/activity"
    ResourceType   string              // e.g., "gce_instance"
    ResourceLabels map[string]string   // instance_id, zone, etc.
    Labels         map[string]string   // user + system labels
    JSONPayload    map[string]any      // full structured payload
    TextPayload    string              // raw text if not JSON
    InsertID       string              // unique entry ID
    TraceID        string              // for future trace correlation
    SpanID         string
    SourceLocation *SourceLocation     // file, line, function
}
```

### Flattened Field Display

When expanded, all non-empty fields rendered as dot-notated key-value pairs, sorted alphabetically:

```
jsonPayload.httpRequest.method: GET
jsonPayload.httpRequest.status: 200
labels.compute.googleapis.com/resource_name: my-vm
resource.labels.instance_id: 1234567890
resource.labels.zone: us-central1-a
resource.type: gce_instance
```

### Severity Color Coding

| Severity | Color |
|----------|-------|
| DEBUG/DEFAULT | Gray `#9AA0A6` |
| INFO | Blue `#4285F4` |
| WARNING | Yellow `#FBBC04` |
| ERROR/CRITICAL/ALERT/EMERGENCY | Red `#EA4335` |

## Quick Filters

Three filter dropdowns in the filter bar:

| Filter | Options Source | Load Strategy |
|--------|---------------|---------------|
| Resources | `ListResourceTypes()` API | Lazy-load on first dropdown open |
| Log Names | `ListLogNames()` API | Lazy-load on first dropdown open |
| Severities | Static enum (DEFAULT through EMERGENCY) | Immediate |

Each opens a multi-select overlay when activated. Selected values get translated to LQL and combined (AND) with the user's manual query.

### Query Building

Quick filters translate to LQL clauses:

```
// User typed:          httpRequest.status>=500
// Resources selected:  gce_instance, cloud_run_revision
// Severities selected: WARNING, ERROR
// Effective query:
resource.type=("gce_instance" OR "cloud_run_revision")
AND severity>=(WARNING OR ERROR)
AND httpRequest.status>=500
```

The effective query is rebuilt whenever filters change or the user submits the query input.

## Key Bindings

### Main View

| Key | Action |
|-----|--------|
| `/` | Focus query input |
| `Enter` | Run query (input focused) / Expand entry (list focused) |
| `Shift+Enter` | Newline in query input |
| `Esc` | Blur input / Collapse entry / Clear filter / Go back |
| `Tab` | Cycle focus (entries → filters → time range) |
| `Shift+Tab` | Cycle focus backwards |
| `j/k` or `↓/↑` | Navigate log entries |
| `→` or `Enter` | Expand selected entry / Enter field navigation |
| `←` | Collapse expanded entry / Exit field navigation |
| `PgUp/PgDn` | Page up/down through entries |
| `E` | Expand all visible entries |
| `C` | Collapse all visible entries |
| `w` | Toggle line wrapping |
| `c` | Toggle logfmt colorization |
| `1-5` | Time range (1h/6h/24h/7d/30d) |
| `f` | Toggle tail mode |
| `r` | Manual refresh (re-run query) |
| `R` | Open resource filter dropdown |
| `L` | Open log name filter dropdown |
| `V` | Open severity filter dropdown |
| `.` | Action menu |

### Expanded Entry (cursor on field line)

| Key | Action |
|-----|--------|
| `Enter` | Add field as LQL filter (append `field="value"` to query) |

### Filter Dropdowns (overlay multi-select)

| Key | Action |
|-----|--------|
| `j/k` or `↓/↑` | Navigate options |
| `PgUp/PgDn` | Page up/down through options |
| `Space`/`Tab`/`Enter` | Toggle selection |
| `/` | Search within filter options |
| `Esc` | Apply selection and close |

## Component Architecture

### New Files

```
internal/gcp/logging.go              — Extend existing (new methods)
internal/ui/views/logs.go            — Main LogsView
internal/ui/views/logs_messages.go   — Message types
internal/ui/components/logviewer/    — Reusable log entry list component
  ├── logviewer.go                   — Model (entry list, expand/collapse, infinite scroll)
  ├── entry.go                       — Single entry render (compact + expanded)
  └── sparkline.go                   — Sparkline renderer
```

### LogsView Responsibilities

- Filter bar rendering and dropdown management
- Query input management (auto-resize text input)
- Orchestrating API calls (query execution, filter option loading, histogram)
- Tail mode timer management (ticker + done channel pattern)
- Delegates log entry rendering to `logviewer` component

### LogViewer Component Responsibilities

- Log entry list with virtual scrolling
- Expand/collapse individual entries
- Field-level cursor within expanded entries
- Sparkline rendering
- Infinite scroll (emits "need more" message when near bottom)

### Message Flow

```
LogsView → RunQueryMsg → App handler → API call → LogsLoadedMsg → LogsView
LogViewer → NeedMoreLogsMsg → LogsView → next page API call → AppendLogsMsg
FilterDropdown → FilterChangedMsg → LogsView (rebuilds query, re-runs)
TailTicker → tailTickMsg → LogsView → API call (timestamp > last) → TailLogsMsg
```

## State Management

```go
type LogsView struct {
    // State
    state           logsState  // idle | loading | error | tailing
    query           string     // user's raw LQL input
    effectiveQuery  string     // combined: filters + user query

    // Data
    entries         []gcp.LogEntry
    nextPageToken   string
    totalCount      int64
    histogramData   []gcp.DataPoint

    // Filters
    selectedResources  []string
    selectedLogNames   []string
    selectedSeverities []string
    availableResources []string  // lazy-loaded
    availableLogNames  []string  // lazy-loaded

    // UI state
    focusArea       focusArea  // filterBar | queryInput | logList
    expandedEntries map[int]bool
    fieldCursor     int
    tailMode        bool
    tailTicker      *time.Ticker
    tailDone        chan struct{}
    timeRange       time.Duration

    // Components
    logViewer       *logviewer.Model
    queryInput      textinput.Model
    spinner         spinner.Model
}
```

## Tail Mode

- Toggle with `f` hotkey.
- Polls `ListLogEntries` every 15 seconds with `timestamp > lastEntryTimestamp`.
- New entries prepended at the top, cursor position preserved.
- Status bar shows `TAIL` badge when active.
- Auto-disables when user scrolls up or changes query.
- Uses `time.Ticker` + `done` channel pattern (consistent with existing observability tabs).

## Loading Strategy

- **Initial load**: First page (100 entries) + histogram data fetched in parallel.
- **Infinite scroll**: Next page fetched when user scrolls within 10 entries of bottom.
- **Tail mode**: Incremental fetch every 15s, prepend new entries at top.
- **Filter/query change**: Clear all entries, reset page token, fetch fresh.

## App Integration Checklist

Per `adding-new-views.md`:

1. `app.go` — `ViewLogs` already in enum. Add `logsView *views.LogsView` field.
2. `app.go` — `getCurrentViewModel()` case for `ViewLogs`.
3. `app_render.go` — `renderCurrentView()` case for `ViewLogs`.
4. `app.go` — `Update()` message handlers for logs messages.
5. `app_navigation.go` — `handleLogsRequest()` navigation handler.
6. `app_navigation.go` — `clearAllViews()` add `a.logsView = nil`.
7. `app.go` — `updateViewSizes()` add context for logsView.
8. `app_navigation.go` — `updateSidebarActiveView()` case for `ViewLogs`.
9. `HasTextInputFocused()` — Must return true when query input or filter dropdown is active.
10. Command palette — Add "Logging: Logs Explorer" navigation command.
11. Sidebar — Add "Logging" section with "Logs Explorer" item.

## Future TODOs

1. Custom time range picker (hotkey `T`, start/end datetime input dialog)
2. Copy to clipboard (`y` to copy selected entry or field value)
3. Show similar entries (filter to matching resource type + log name)
4. Pin/bookmark entries for reference while scrolling
5. Saved queries (store frequently used LQL queries)
6. Export visible log entries to file (JSON/CSV)
7. Trace correlation (click trace ID to view related spans)
8. Log-based metrics (view/create from queries)
9. Reuse logviewer component in Cloud Run and Compute observability tabs
