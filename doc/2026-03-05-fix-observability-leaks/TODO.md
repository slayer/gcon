# Fix Observability Auto-Refresh Ticker Leaks and Data Ordering

## Tasks (original)

- [x] Fix ticker goroutine leak in `cloudrun_observability.go` — add `autoRefreshDone` channel
- [x] Fix ticker goroutine leak in `instance_details.go` — same done channel pattern
- [x] Sort data points by timestamp in `monitoring_cloudrun.go` and `monitoring.go`
- [x] Surface per-metric errors as warnings in Cloud Run observability
- [x] Tests pass, lint clean

## PR Review Fixes

- [x] Replace ticker with `tea.Tick` for auto-refresh (eliminates goroutine leak entirely)
- [x] Rename `Memory` → `BillableInstanceTime` (field, function, docs)
- [x] Guard `strings.Repeat` against negative width with `max(0, ...)`
- [x] Fix numeric key conflict: route observability 1-5 keys only when viewport is focused
- [x] Add time range filter to `GetCloudRunLogs` (was ignoring selected time range)
- [x] Add 30s context timeout to `loadMetrics()` and `loadLogs()`
- [x] Fix auto-refresh test to use `tea.KeyMsg` through `Update()`
- [x] Update key-bindings.md documentation
- [x] Update CLAUDE.md feature checklist
- [x] All tests pass, lint clean

## Summary

### tea.Tick replaces time.Ticker
Replaced long-lived `time.Ticker` with single-shot `tea.Tick` rescheduling. Each tick
schedules the next one via Bubble Tea's timing utilities. Stale ticks from before
auto-refresh was disabled are dropped in the `crRefreshTickMsg` handler. No goroutine
leaks possible.

### BillableInstanceTime naming
The metric queries `container/billable_instance_time` but was named "Memory" in code.
Renamed to `BillableInstanceTime` throughout: struct field, API function, UI label, docs.

### Key conflict resolution
Observability's 1-5 time range keys were handled before focus-based tab switching,
blocking `1/2/3/4` for tab navigation. Moved observability key routing inside the
`RegionViewport` case so it only fires when content area is focused.

### Time range filter for logs
`GetCloudRunLogs` now accepts a `duration` parameter and adds a `timestamp >= cutoff`
filter to the Cloud Logging query, matching the behavior of metric queries.
