# 2026-03-17: Logging Explorer

## Task Description

Implement a GCP Logs Explorer view in the TUI, similar to the GCP web UI Logs Explorer. Provides LQL query input, quick filters (Resources, Log Names, Severities), a sparkline histogram, and an expandable log entry list with infinite scroll and tail mode.

## Implementation Plan

### Phase 1: GCP API Layer
- [x] Extend `LogEntry` struct with full fields (labels, resource, JSON payload, insert ID, trace)
- [x] Implement `ListLogEntries()` with LQL query, pagination, time range
- [x] Implement `ListResourceTypes()` for resource filter dropdown
- [x] Implement `ListLogNames()` for log name filter dropdown
- [x] Implement `GetLogHistogram()` using Cloud Monitoring API for sparkline data
- [x] Add tests for new API methods

### Phase 2: LogViewer Component
- [x] Create `internal/ui/components/logviewer/` package
- [x] Implement `entry.go` — compact and expanded entry rendering with severity colors
- [x] Implement field flattening (dot-notated key-value pairs from nested maps)
- [x] Implement `logviewer.go` — entry list with expand/collapse, field cursor, infinite scroll
- [x] Implement `sparkline.go` — block character sparkline renderer
- [x] Add tests for rendering and expand/collapse logic

### Phase 3: LogsView
- [x] Create `internal/ui/views/logs.go` — main view
- [x] Create `internal/ui/views/logs_messages.go` — message types
- [x] Implement filter bar (Resources, Log Names, Severities dropdowns)
- [x] Implement single-line auto-resizing LQL query input
- [x] Implement query building (combine filters + manual LQL)
- [x] Implement time range selection (1-5 hotkeys)
- [x] Implement tail mode with ticker + done channel
- [x] Implement filter-by-field (Enter on expanded field line → append to query)
- [x] Add `HasTextInputFocused()` for query input and filter dropdowns
- [x] Add tests for query building, state management, key handling
- [x] Implement logfmt/protobuf key:value colorization (toggle with `c`)
- [x] Implement line wrapping toggle (`w` key)
- [x] Implement open in $PAGER (`p` key, respects color toggle)
- [x] Implement action menu with export (TXT/CSV/JSONL)
- [x] Implement ANSI-aware truncation and wrapping
- [x] Implement Tab cycling between entries, filters, query, time range
- [x] Implement search within filter dropdowns (`/` key)
- [x] Implement PgUp/PgDn for entries and filter dropdowns

### Phase 4: App Integration
- [x] Add `logsView` field to App struct
- [x] Add `getCurrentViewModel()` case for ViewLogs
- [x] Add `renderCurrentView()` case for ViewLogs
- [x] Add Update() message handlers for logs messages
- [x] Add `handleLogsRequest()` in app_navigation.go
- [x] Add ViewLogs to `clearAllViews()`
- [x] Add ViewLogs to `updateViewSizes()`
- [x] Add ViewLogs to `updateSidebarActiveView()`
- [x] Add "Logging" section to sidebar menu
- [x] Add "Logging: Logs Explorer" to command palette
- [x] Add sidebar navigation guard for ViewLogs

### Phase 5: Polish
- [x] Run full test suite
- [x] Run linter
- [x] Update key bindings documentation
- [x] Update CLAUDE.md implemented features list
- [x] Update README.md

## Future TODOs
- [ ] Custom time range picker (hotkey `T`)
- [ ] Copy to clipboard (`y`)
- [ ] Show similar entries
- [ ] Pin/bookmark entries
- [ ] Saved queries
- [ ] Trace correlation
- [ ] Log-based metrics
- [ ] Reuse logviewer in observability tabs
