# 2026-03-17: Logging Explorer

## Task Description

Implement a GCP Logs Explorer view in the TUI, similar to the GCP web UI Logs Explorer. Provides LQL query input, quick filters (Resources, Log Names, Severities), a sparkline histogram, and an expandable log entry list with infinite scroll and tail mode.

## Implementation Plan

### Phase 1: GCP API Layer
- [ ] Extend `LogEntry` struct with full fields (labels, resource, JSON payload, insert ID, trace)
- [ ] Implement `ListLogEntries()` with LQL query, pagination, time range
- [ ] Implement `ListResourceTypes()` for resource filter dropdown
- [ ] Implement `ListLogNames()` for log name filter dropdown
- [ ] Implement `GetLogHistogram()` using Cloud Monitoring API for sparkline data
- [ ] Add tests for new API methods

### Phase 2: LogViewer Component
- [ ] Create `internal/ui/components/logviewer/` package
- [ ] Implement `entry.go` — compact and expanded entry rendering with severity colors
- [ ] Implement field flattening (dot-notated key-value pairs from nested maps)
- [ ] Implement `logviewer.go` — entry list with expand/collapse, field cursor, infinite scroll
- [ ] Implement `sparkline.go` — block character sparkline renderer
- [ ] Add tests for rendering and expand/collapse logic

### Phase 3: LogsView
- [ ] Create `internal/ui/views/logs.go` — main view
- [ ] Create `internal/ui/views/logs_messages.go` — message types
- [ ] Implement filter bar (Resources, Log Names, Severities dropdowns)
- [ ] Implement single-line auto-resizing LQL query input
- [ ] Implement query building (combine filters + manual LQL)
- [ ] Implement time range selection (1-5 hotkeys)
- [ ] Implement tail mode with ticker + done channel
- [ ] Implement filter-by-field (Enter on expanded field line → append to query)
- [ ] Add `HasTextInputFocused()` for query input and filter dropdowns
- [ ] Add tests for query building, state management, key handling

### Phase 4: App Integration
- [ ] Add `logsView` field to App struct
- [ ] Add `getCurrentViewModel()` case for ViewLogs
- [ ] Add `renderCurrentView()` case for ViewLogs
- [ ] Add Update() message handlers for logs messages
- [ ] Add `handleLogsRequest()` in app_navigation.go
- [ ] Add ViewLogs to `clearAllViews()`
- [ ] Add ViewLogs to `updateViewSizes()`
- [ ] Add ViewLogs to `updateSidebarActiveView()`
- [ ] Add "Logging" section to sidebar menu
- [ ] Add "Logging: Logs Explorer" to command palette
- [ ] Add sidebar navigation guard for ViewLogs

### Phase 5: Polish
- [ ] Run full test suite
- [ ] Run linter
- [ ] Update key bindings documentation
- [ ] Update CLAUDE.md implemented features list
- [ ] Update README.md if needed

## Future TODOs
- [ ] Custom time range picker (hotkey `T`)
- [ ] Copy to clipboard (`y`)
- [ ] Show similar entries
- [ ] Pin/bookmark entries
- [ ] Saved queries
- [ ] Export to file (JSON/CSV)
- [ ] Trace correlation
- [ ] Log-based metrics
- [ ] Reuse logviewer in observability tabs
